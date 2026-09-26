package handlers

import (
	"archive/zip"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"path"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/KennethhPoenadi/PixelCloud/api/internal/apierr"
	"github.com/KennethhPoenadi/PixelCloud/api/internal/db"
	"github.com/KennethhPoenadi/PixelCloud/api/internal/httpx"
)

type createBatchRequest struct {
	ImageIDs []uuid.UUID     `json:"image_ids"`
	Pipeline json.RawMessage `json:"pipeline"`
	outputOptions
}

type batchJobResponse struct {
	ID          uuid.UUID `json:"id"`
	ImageID     uuid.UUID `json:"image_id"`
	Filename    string    `json:"filename"`
	Status      string    `json:"status"`
	Error       *string   `json:"error"`
	WorkerID    *string   `json:"worker_id"`
	ResultURL   *string   `json:"result_url"`
	DownloadURL *string   `json:"download_url"`
}

type batchResponse struct {
	ID         uuid.UUID          `json:"id"`
	Pipeline   json.RawMessage    `json:"pipeline"`
	TotalJobs  int32              `json:"total_jobs"`
	Queued     int                `json:"queued"`
	Processing int                `json:"processing"`
	Done       int                `json:"done"`
	Failed     int                `json:"failed"`
	Finished   bool               `json:"finished"`
	CreatedAt  time.Time          `json:"created_at"`
	Jobs       []batchJobResponse `json:"jobs"`
}

func (s *Server) toBatchResponse(ctx context.Context, b db.Batch, jobs []db.Job) (batchResponse, error) {
	res := batchResponse{ID: b.ID, Pipeline: b.Pipeline, TotalJobs: b.TotalJobs, CreatedAt: b.CreatedAt,
		Jobs: make([]batchJobResponse, 0, len(jobs))}
	for _, j := range jobs {
		switch j.Status {
		case "queued":
			res.Queued++
		case "processing":
			res.Processing++
		case "done":
			res.Done++
		case "failed":
			res.Failed++
		}
		jr, err := s.toJobResponse(ctx, j)
		if err != nil {
			return res, err
		}
		res.Jobs = append(res.Jobs, batchJobResponse{
			ID: j.ID, ImageID: j.ImageID, Filename: j.ImageFilename, Status: j.Status, Error: j.Error,
			WorkerID: j.WorkerID, ResultURL: jr.ResultURL, DownloadURL: jr.DownloadURL,
		})
	}
	res.Finished = res.Done+res.Failed == len(jobs)
	return res, nil
}

func (s *Server) createBatch(w http.ResponseWriter, r *http.Request) {
	p := mustPrincipal(r)
	var req createBatchRequest
	if err := httpx.DecodeJSON(r, &req); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	// De-duplicate while keeping the caller's order.
	seen := map[uuid.UUID]bool{}
	ids := make([]uuid.UUID, 0, len(req.ImageIDs))
	for _, id := range req.ImageIDs {
		if !seen[id] && id != uuid.Nil {
			seen[id] = true
			ids = append(ids, id)
		}
	}
	if len(ids) == 0 {
		httpx.WriteError(w, r, apierr.New(apierr.Validation, "image_ids must contain at least one image"))
		return
	}
	format, quality, err := req.normalize()
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	pl, err := validatePipeline(req.Pipeline)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), db.QueryTimeout)
	defer cancel()
	plan, err := s.store.PlanForUser(ctx, p.UserID)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	if len(ids) > int(plan.MaxBatchSize) {
		httpx.WriteError(w, r, apierr.New(apierr.Forbidden,
			"the %s plan allows at most %d images per batch", plan.Name, plan.MaxBatchSize))
		return
	}

	specs := make([]db.JobSpec, len(ids))
	for i, id := range ids {
		specs[i] = db.JobSpec{ImageID: id, Pipeline: pl, OutputFormat: format, OutputQuality: quality}
	}
	_, batchID, err := s.enqueueJobs(r, specs, pl)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	b, jobs, err := s.store.BatchForUser(ctx, p.UserID, *batchID)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	res, err := s.toBatchResponse(ctx, b, jobs)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	w.Header().Set("Location", "/api/v1/batches/"+batchID.String())
	httpx.WriteJSON(w, http.StatusAccepted, res)
}

func (s *Server) loadBatch(w http.ResponseWriter, r *http.Request) (db.Batch, []db.Job, bool) {
	p := mustPrincipal(r)
	id, err := parseID(r, "batchID")
	if err != nil {
		httpx.WriteError(w, r, err)
		return db.Batch{}, nil, false
	}
	ctx, cancel := context.WithTimeout(r.Context(), db.QueryTimeout)
	defer cancel()
	b, jobs, err := s.store.BatchForUser(ctx, p.UserID, id)
	if errors.Is(err, db.ErrNotFound) {
		httpx.WriteError(w, r, apierr.New(apierr.NotFound, "batch not found"))
		return db.Batch{}, nil, false
	}
	if err != nil {
		httpx.WriteError(w, r, err)
		return db.Batch{}, nil, false
	}
	return b, jobs, true
}

func (s *Server) getBatch(w http.ResponseWriter, r *http.Request) {
	b, jobs, ok := s.loadBatch(w, r)
	if !ok {
		return
	}
	res, err := s.toBatchResponse(r.Context(), b, jobs)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, res)
}

// uniqueName appends -2, -3, ... when several results would share a file name.
func uniqueName(name string, used map[string]int) string {
	used[name]++
	if n := used[name]; n > 1 {
		ext := path.Ext(name)
		return fmt.Sprintf("%s-%d%s", strings.TrimSuffix(name, ext), n, ext)
	}
	return name
}

// downloadBatch streams every finished result as a ZIP (stored, not deflated:
// the images are already compressed).
func (s *Server) downloadBatch(w http.ResponseWriter, r *http.Request) {
	b, jobs, ok := s.loadBatch(w, r)
	if !ok {
		return
	}
	var done []db.Job
	for _, j := range jobs {
		if j.Status == "done" && j.ResultKey != nil {
			done = append(done, j)
		}
	}
	if len(done) == 0 {
		httpx.WriteError(w, r, apierr.New(apierr.Conflict, "no finished results in this batch yet"))
		return
	}

	w.Header().Set("Content-Type", "application/zip")
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="pixelcloud-batch-%s.zip"`, b.ID.String()[:8]))
	zw := zip.NewWriter(w)
	used := map[string]int{}
	for _, j := range done {
		if err := s.addToZip(r.Context(), zw, j, uniqueName(resultFilename(j.ImageFilename, j.OutputFormat), used)); err != nil {
			// Headers are already sent; log and cut the archive short.
			httpx.Logger(r.Context()).Error("batch zip aborted", "batch_id", b.ID, "job_id", j.ID, "err", err)
			return
		}
	}
	if err := zw.Close(); err != nil {
		httpx.Logger(r.Context()).Error("finish zip", "batch_id", b.ID, "err", err)
	}
}

func (s *Server) addToZip(ctx context.Context, zw *zip.Writer, j db.Job, name string) error {
	ctx, cancel := context.WithTimeout(ctx, storageTimeout)
	defer cancel()
	obj, err := s.storage.Get(ctx, *j.ResultKey)
	if err != nil {
		return err
	}
	defer func() { _ = obj.Close() }()
	modified := time.Now()
	if j.FinishedAt != nil {
		modified = *j.FinishedAt
	}
	fw, err := zw.CreateHeader(&zip.FileHeader{Name: name, Method: zip.Store, Modified: modified})
	if err != nil {
		return fmt.Errorf("zip header: %w", err)
	}
	if _, err := io.Copy(fw, obj); err != nil {
		return fmt.Errorf("copy %s: %w", *j.ResultKey, err)
	}
	return nil
}
