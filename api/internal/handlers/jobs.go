package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"path"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/KennethhPoenadi/PixelCloud/api/internal/apierr"
	"github.com/KennethhPoenadi/PixelCloud/api/internal/db"
	"github.com/KennethhPoenadi/PixelCloud/api/internal/httpx"
	"github.com/KennethhPoenadi/PixelCloud/api/internal/pipeline"
	"github.com/KennethhPoenadi/PixelCloud/api/internal/queue"
	"github.com/KennethhPoenadi/PixelCloud/api/internal/quota"
)

var outputExt = map[string]string{"jpeg": "jpg", "png": "png", "webp": "webp"}

type outputOptions struct {
	OutputFormat  string `json:"output_format"`
	OutputQuality *int   `json:"output_quality"`
}

// normalize applies defaults (jpeg, quality 90) and validates the values.
func (o outputOptions) normalize() (string, int16, error) {
	format := strings.ToLower(o.OutputFormat)
	if format == "" {
		format = "jpeg"
	}
	if _, ok := outputExt[format]; !ok {
		return "", 0, apierr.New(apierr.Validation, "output_format must be jpeg, png or webp")
	}
	quality := 90
	if o.OutputQuality != nil {
		quality = *o.OutputQuality
	}
	if quality < 1 || quality > 100 {
		return "", 0, apierr.New(apierr.Validation, "output_quality must be between 1 and 100")
	}
	return format, int16(quality), nil
}

func validatePipeline(raw json.RawMessage) (json.RawMessage, error) {
	p, err := pipeline.Validate(raw)
	if err != nil {
		var pe *pipeline.Error
		if errors.As(err, &pe) {
			return nil, apierr.New(apierr.Validation, "pipeline: %s", pe.Error())
		}
		return nil, err
	}
	return p, nil
}

type createJobRequest struct {
	ImageID  uuid.UUID       `json:"image_id"`
	Pipeline json.RawMessage `json:"pipeline"`
	outputOptions
}

type jobResponse struct {
	ID            uuid.UUID       `json:"id"`
	ImageID       uuid.UUID       `json:"image_id"`
	BatchID       *uuid.UUID      `json:"batch_id"`
	Status        string          `json:"status"`
	Pipeline      json.RawMessage `json:"pipeline"`
	OutputFormat  string          `json:"output_format"`
	OutputQuality int16           `json:"output_quality"`
	Attempts      int16           `json:"attempts"`
	WorkerID      *string         `json:"worker_id"`
	Error         *string         `json:"error"`
	CreatedAt     time.Time       `json:"created_at"`
	StartedAt     *time.Time      `json:"started_at"`
	FinishedAt    *time.Time      `json:"finished_at"`
	ResultURL     *string         `json:"result_url"`
	DownloadURL   *string         `json:"download_url"`
}

// resultFilename is the name offered on download: "<original>-pixelcloud.<ext>".
func resultFilename(original, format string) string {
	base := strings.TrimSuffix(original, path.Ext(original))
	if base == "" {
		base = "image"
	}
	return base + "-pixelcloud." + outputExt[format]
}

func (s *Server) toJobResponse(ctx context.Context, j db.Job) (jobResponse, error) {
	res := jobResponse{
		ID: j.ID, ImageID: j.ImageID, BatchID: j.BatchID, Status: j.Status, Pipeline: j.Pipeline,
		OutputFormat: j.OutputFormat, OutputQuality: j.OutputQuality, Attempts: j.Attempts,
		WorkerID: j.WorkerID, Error: j.Error, CreatedAt: j.CreatedAt, StartedAt: j.StartedAt, FinishedAt: j.FinishedAt,
	}
	if j.Status == "done" && j.ResultKey != nil {
		view, err := s.storage.PresignGet(ctx, *j.ResultKey, s.cfg.PresignTTL, "")
		if err != nil {
			return res, err
		}
		dl, err := s.storage.PresignGet(ctx, *j.ResultKey, s.cfg.PresignTTL, resultFilename(j.ImageFilename, j.OutputFormat))
		if err != nil {
			return res, err
		}
		res.ResultURL, res.DownloadURL = &view, &dl
	}
	return res, nil
}

// enqueueJobs creates the jobs (quota checked in the same transaction) and
// publishes them. If publishing fails the jobs are aborted and quota refunded.
func (s *Server) enqueueJobs(r *http.Request, specs []db.JobSpec, batchPipeline []byte) ([]db.Job, *uuid.UUID, error) {
	p := mustPrincipal(r)
	ctx, cancel := context.WithTimeout(r.Context(), db.QueryTimeout)
	defer cancel()

	jobs, batchID, err := s.store.CreateJobs(ctx, p.UserID, specs, batchPipeline)
	switch {
	case errors.Is(err, db.ErrImageNotFound):
		return nil, nil, apierr.New(apierr.NotFound, "image not found")
	case errors.Is(err, quota.ErrExceeded):
		s.metrics.QuotaRejections.Inc()
		return nil, nil, apierr.New(apierr.QuotaExceeded, "monthly edit quota exhausted; upgrade your plan to keep editing")
	case err != nil:
		return nil, nil, err
	}

	rid := httpx.RequestID(r.Context())
	msgs := make([]queue.Message, len(jobs))
	ids := make([]uuid.UUID, len(jobs))
	for i, j := range jobs {
		msgs[i] = queue.Message{JobID: j.ID.String(), RequestID: rid}
		ids[i] = j.ID
	}
	if err := s.queue.Enqueue(ctx, msgs...); err != nil {
		if abortErr := s.store.AbortJobs(context.WithoutCancel(ctx), p.UserID, ids, "could not be queued"); abortErr != nil {
			httpx.Logger(ctx).Error("abort jobs after enqueue failure", "err", abortErr)
		}
		return nil, nil, err
	}
	s.metrics.JobsEnqueued.Add(float64(len(jobs)))
	httpx.Logger(ctx).Info("jobs enqueued", "count", len(jobs), "first_job_id", ids[0].String())
	return jobs, batchID, nil
}

func (s *Server) createJob(w http.ResponseWriter, r *http.Request) {
	var req createJobRequest
	if err := httpx.DecodeJSON(r, &req); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	if req.ImageID == uuid.Nil {
		httpx.WriteError(w, r, apierr.New(apierr.Validation, "image_id is required"))
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
	jobs, _, err := s.enqueueJobs(r, []db.JobSpec{{
		ImageID: req.ImageID, Pipeline: pl, OutputFormat: format, OutputQuality: quality,
	}}, nil)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	res, err := s.toJobResponse(r.Context(), jobs[0])
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	w.Header().Set("Location", "/api/v1/jobs/"+jobs[0].ID.String())
	httpx.WriteJSON(w, http.StatusAccepted, res)
}

func (s *Server) getJob(w http.ResponseWriter, r *http.Request) {
	p := mustPrincipal(r)
	id, err := parseID(r, "jobID")
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), db.QueryTimeout)
	defer cancel()
	j, err := s.store.JobForUser(ctx, p.UserID, id)
	if errors.Is(err, db.ErrNotFound) {
		httpx.WriteError(w, r, apierr.New(apierr.NotFound, "job not found"))
		return
	}
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	res, err := s.toJobResponse(ctx, j)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, res)
}
