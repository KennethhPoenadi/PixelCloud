package handlers

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"path"
	"strings"
	"time"
	"unicode"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/KennethhPoenadi/PixelCloud/api/internal/apierr"
	"github.com/KennethhPoenadi/PixelCloud/api/internal/db"
	"github.com/KennethhPoenadi/PixelCloud/api/internal/httpx"
	"github.com/KennethhPoenadi/PixelCloud/api/internal/quota"
	"github.com/KennethhPoenadi/PixelCloud/api/internal/upload"
)

const storageTimeout = 30 * time.Second

type latestJobResponse struct {
	ID        uuid.UUID `json:"id"`
	Status    string    `json:"status"`
	ResultURL *string   `json:"result_url"`
}

type imageResponse struct {
	ID        uuid.UUID          `json:"id"`
	Filename  string             `json:"filename"`
	MimeType  string             `json:"mime_type"`
	SizeBytes int64              `json:"size_bytes"`
	Width     int32              `json:"width"`
	Height    int32              `json:"height"`
	CreatedAt time.Time          `json:"created_at"`
	ExpiresAt *time.Time         `json:"expires_at"`
	URL       string             `json:"url"`
	LatestJob *latestJobResponse `json:"latest_job"`
}

func (s *Server) toImageResponse(ctx context.Context, im db.Image) (imageResponse, error) {
	u, err := s.storage.PresignGet(ctx, im.StorageKey, s.cfg.PresignTTL, "")
	if err != nil {
		return imageResponse{}, err
	}
	res := imageResponse{
		ID: im.ID, Filename: im.Filename, MimeType: im.MimeType, SizeBytes: im.SizeBytes,
		Width: im.Width, Height: im.Height, CreatedAt: im.CreatedAt, ExpiresAt: im.ExpiresAt, URL: u,
	}
	if j := im.LatestJob; j != nil {
		lj := &latestJobResponse{ID: j.ID, Status: j.Status}
		if j.ResultKey != nil {
			ru, err := s.storage.PresignGet(ctx, *j.ResultKey, s.cfg.PresignTTL, "")
			if err != nil {
				return imageResponse{}, err
			}
			lj.ResultURL = &ru
		}
		res.LatestJob = lj
	}
	return res, nil
}

// sanitizeFilename keeps only the base name, drops control characters and caps length.
func sanitizeFilename(name, ext string) string {
	name = path.Base(strings.ReplaceAll(name, `\`, "/"))
	name = strings.Map(func(r rune) rune {
		if unicode.IsControl(r) || r == '"' {
			return -1
		}
		return r
	}, name)
	name = strings.TrimSpace(name)
	if name == "" || name == "." || name == "/" {
		name = "image." + ext
	}
	if r := []rune(name); len(r) > 200 {
		name = string(r[:200])
	}
	return name
}

func (s *Server) uploadImage(w http.ResponseWriter, r *http.Request) {
	p := mustPrincipal(r)
	dbCtx, cancel := context.WithTimeout(r.Context(), db.QueryTimeout)
	plan, err := s.store.PlanForUser(dbCtx, p.UserID)
	cancel()
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}

	maxBytes := int64(plan.MaxFileMB) << 20
	tooLarge := apierr.New(apierr.FileTooLarge, "file is larger than the %d MB limit of the %s plan", plan.MaxFileMB, plan.Name)
	// Allow some slack for multipart headers; the file itself is checked exactly below.
	r.Body = http.MaxBytesReader(w, r.Body, maxBytes+64<<10)

	mr, err := r.MultipartReader()
	if err != nil {
		httpx.WriteError(w, r, apierr.New(apierr.Validation, "expected multipart/form-data with a \"file\" field"))
		return
	}
	var data []byte
	var filename string
	for {
		part, err := mr.NextPart()
		if errors.Is(err, io.EOF) {
			break
		}
		var mbe *http.MaxBytesError
		if errors.As(err, &mbe) {
			httpx.WriteError(w, r, tooLarge)
			return
		}
		if err != nil {
			httpx.WriteError(w, r, apierr.New(apierr.Validation, "malformed multipart body"))
			return
		}
		if part.FormName() != "file" {
			_ = part.Close()
			continue
		}
		filename = part.FileName()
		data, err = io.ReadAll(io.LimitReader(part, maxBytes+1))
		_ = part.Close()
		if errors.As(err, &mbe) {
			httpx.WriteError(w, r, tooLarge)
			return
		}
		if err != nil {
			httpx.WriteError(w, r, apierr.New(apierr.Validation, "could not read uploaded file"))
			return
		}
		break
	}
	if len(data) == 0 {
		httpx.WriteError(w, r, apierr.New(apierr.Validation, "a non-empty \"file\" field is required"))
		return
	}
	if int64(len(data)) > maxBytes {
		httpx.WriteError(w, r, tooLarge)
		return
	}

	info, err := upload.Inspect(data)
	if err != nil {
		httpx.WriteError(w, r, apierr.New(apierr.UnsupportedFormat, "only JPEG, PNG and WebP images are supported"))
		return
	}
	if longest := max(info.Width, info.Height); longest > int(plan.MaxResolution) {
		httpx.WriteError(w, r, apierr.New(apierr.FileTooLarge,
			"image is %dx%d px; the %s plan allows up to %d px on the longest side",
			info.Width, info.Height, plan.Name, plan.MaxResolution))
		return
	}
	clean, err := upload.StripMetadata(info.MIME, data)
	if err != nil {
		httpx.WriteError(w, r, apierr.New(apierr.UnsupportedFormat, "image file is corrupted"))
		return
	}

	id := uuid.New()
	key := fmt.Sprintf("uploads/%s/%s.%s", p.UserID, id, info.Ext)
	stCtx, stCancel := context.WithTimeout(r.Context(), storageTimeout)
	defer stCancel()
	if err := s.storage.Put(stCtx, key, bytes.NewReader(clean), int64(len(clean)), info.MIME); err != nil {
		httpx.WriteError(w, r, fmt.Errorf("upload image: %w", err))
		return
	}

	expires := time.Now().Add(time.Duration(s.cfg.RetentionDays) * 24 * time.Hour)
	dbCtx, cancel = context.WithTimeout(r.Context(), db.QueryTimeout)
	defer cancel()
	im, err := s.store.CreateImage(dbCtx, db.Image{
		ID: id, UserID: p.UserID, StorageKey: key, Filename: sanitizeFilename(filename, info.Ext),
		MimeType: info.MIME, SizeBytes: int64(len(clean)), Width: int32(info.Width), Height: int32(info.Height),
		ExpiresAt: &expires,
	})
	if err != nil {
		if rmErr := s.storage.Remove(stCtx, key); rmErr != nil {
			httpx.Logger(r.Context()).Warn("orphaned upload", "key", key, "err", rmErr)
		}
		httpx.WriteError(w, r, err)
		return
	}
	if err := s.store.AddBytesIn(dbCtx, p.UserID, quota.PeriodStart(time.Now()), int64(len(clean))); err != nil {
		httpx.Logger(r.Context()).Warn("usage bytes_in not recorded", "err", err)
	}

	res, err := s.toImageResponse(r.Context(), im)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusCreated, res)
}

func (s *Server) listImages(w http.ResponseWriter, r *http.Request) {
	p := mustPrincipal(r)
	cursor, limit, err := pageParams(r)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	search := strings.TrimSpace(r.URL.Query().Get("q"))
	if len(search) > 100 {
		search = search[:100]
	}
	ctx, cancel := context.WithTimeout(r.Context(), db.QueryTimeout)
	defer cancel()
	// Fetch one extra row to know whether another page exists.
	images, err := s.store.ListImages(ctx, p.UserID, cursor, limit+1, escapeLike(search))
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	var next *string
	if len(images) > limit {
		images = images[:limit]
		last := images[len(images)-1]
		c := encodeCursor(db.Cursor{CreatedAt: last.CreatedAt, ID: last.ID})
		next = &c
	}
	items := make([]imageResponse, 0, len(images))
	for _, im := range images {
		res, err := s.toImageResponse(ctx, im)
		if err != nil {
			httpx.WriteError(w, r, err)
			return
		}
		items = append(items, res)
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"items": items, "next_cursor": next})
}

func parseID(r *http.Request, name string) (uuid.UUID, error) {
	id, err := uuid.Parse(chi.URLParam(r, name))
	if err != nil {
		return uuid.Nil, apierr.New(apierr.NotFound, "%s not found", strings.TrimSuffix(name, "ID"))
	}
	return id, nil
}

func (s *Server) getImage(w http.ResponseWriter, r *http.Request) {
	p := mustPrincipal(r)
	id, err := parseID(r, "imageID")
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), db.QueryTimeout)
	defer cancel()
	im, err := s.store.ImageForUser(ctx, p.UserID, id)
	if errors.Is(err, db.ErrNotFound) {
		httpx.WriteError(w, r, apierr.New(apierr.NotFound, "image not found"))
		return
	}
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	res, err := s.toImageResponse(ctx, im)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, res)
}

func (s *Server) deleteImage(w http.ResponseWriter, r *http.Request) {
	p := mustPrincipal(r)
	id, err := parseID(r, "imageID")
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), db.QueryTimeout)
	defer cancel()
	keys, err := s.store.DeleteImage(ctx, p.UserID, id)
	if errors.Is(err, db.ErrNotFound) {
		httpx.WriteError(w, r, apierr.New(apierr.NotFound, "image not found"))
		return
	}
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	stCtx, stCancel := context.WithTimeout(r.Context(), storageTimeout)
	defer stCancel()
	if err := s.storage.Remove(stCtx, keys...); err != nil {
		// The rows are gone already; leftover objects are harmless and private.
		httpx.Logger(r.Context()).Warn("could not remove all objects", "image_id", id, "err", err)
	}
	w.WriteHeader(http.StatusNoContent)
}
