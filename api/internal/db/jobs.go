package db

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/KennethhPoenadi/PixelCloud/api/internal/quota"
)

type Job struct {
	ID            uuid.UUID
	UserID        uuid.UUID
	ImageID       uuid.UUID
	BatchID       *uuid.UUID
	Pipeline      []byte
	OutputFormat  string
	OutputQuality int16
	Status        string
	ResultKey     *string
	Error         *string
	Attempts      int16
	WorkerID      *string
	CreatedAt     time.Time
	StartedAt     *time.Time
	FinishedAt    *time.Time

	ImageFilename string // joined from images, used for download names
}

// JobSpec is what the caller wants processed.
type JobSpec struct {
	ImageID       uuid.UUID
	Pipeline      []byte
	OutputFormat  string
	OutputQuality int16
}

const jobColumns = `j.id, j.user_id, j.image_id, j.batch_id, j.pipeline, j.output_format, j.output_quality,
	j.status::text, j.result_key, j.error, j.attempts, j.worker_id, j.created_at, j.started_at, j.finished_at,
	i.filename`

func jobDest(j *Job) []any {
	return []any{&j.ID, &j.UserID, &j.ImageID, &j.BatchID, &j.Pipeline, &j.OutputFormat, &j.OutputQuality,
		&j.Status, &j.ResultKey, &j.Error, &j.Attempts, &j.WorkerID, &j.CreatedAt, &j.StartedAt, &j.FinishedAt,
		&j.ImageFilename}
}

var ErrImageNotFound = errors.New("image not found")

// CreateJobs reserves quota and inserts one queued job per spec in a single
// transaction. When batchPipeline is non-nil a batch row groups the jobs.
func (s *Store) CreateJobs(ctx context.Context, userID uuid.UUID, specs []JobSpec, batchPipeline []byte) ([]Job, *uuid.UUID, error) {
	var jobs []Job
	var batchID *uuid.UUID
	now := time.Now()
	err := s.InTx(ctx, func(tx pgx.Tx) error {
		ids := make([]uuid.UUID, len(specs))
		for i, sp := range specs {
			ids[i] = sp.ImageID
		}
		var owned int
		if err := tx.QueryRow(ctx, `SELECT count(DISTINCT id) FROM images WHERE user_id = $1 AND id = ANY($2)`,
			userID, ids).Scan(&owned); err != nil {
			return fmt.Errorf("check image ownership: %w", err)
		}
		if owned != len(uniqueIDs(ids)) {
			return ErrImageNotFound
		}

		var limit *int32
		if err := tx.QueryRow(ctx, `SELECT p.monthly_quota FROM users u JOIN plans p ON p.id = u.plan_id WHERE u.id = $1`,
			userID).Scan(&limit); err != nil {
			return fmt.Errorf("load plan quota: %w", err)
		}
		if _, err := quota.Reserve(ctx, tx, userID, limit, len(specs), now); err != nil {
			return err
		}

		if batchPipeline != nil {
			var id uuid.UUID
			if err := tx.QueryRow(ctx, `INSERT INTO batches (user_id, pipeline, total_jobs) VALUES ($1, $2, $3) RETURNING id`,
				userID, string(batchPipeline), len(specs)).Scan(&id); err != nil {
				return fmt.Errorf("insert batch: %w", err)
			}
			batchID = &id
		}

		jobs = make([]Job, 0, len(specs))
		for _, sp := range specs {
			var j Job
			err := tx.QueryRow(ctx, `
				WITH j AS (
					INSERT INTO jobs (user_id, image_id, batch_id, pipeline, output_format, output_quality)
					VALUES ($1, $2, $3, $4, $5, $6)
					RETURNING *
				)
				SELECT `+jobColumns+` FROM j JOIN images i ON i.id = j.image_id`,
				userID, sp.ImageID, batchID, string(sp.Pipeline), sp.OutputFormat, sp.OutputQuality,
			).Scan(jobDest(&j)...)
			if err != nil {
				return fmt.Errorf("insert job: %w", err)
			}
			jobs = append(jobs, j)
		}
		return nil
	})
	if err != nil {
		return nil, nil, err
	}
	return jobs, batchID, nil
}

// AbortJobs marks jobs failed and refunds their quota; used when enqueueing fails.
func (s *Store) AbortJobs(ctx context.Context, userID uuid.UUID, ids []uuid.UUID, reason string) error {
	return s.InTx(ctx, func(tx pgx.Tx) error {
		tag, err := tx.Exec(ctx, `
			UPDATE jobs SET status = 'failed', error = $3, finished_at = now()
			WHERE user_id = $1 AND id = ANY($2) AND status = 'queued'`, userID, ids, reason)
		if err != nil {
			return fmt.Errorf("abort jobs: %w", err)
		}
		return quota.Release(ctx, tx, userID, int(tag.RowsAffected()), time.Now())
	})
}

func (s *Store) JobForUser(ctx context.Context, userID, id uuid.UUID) (Job, error) {
	var j Job
	err := s.Pool.QueryRow(ctx, `SELECT `+jobColumns+` FROM jobs j JOIN images i ON i.id = j.image_id
		WHERE j.id = $1 AND j.user_id = $2`, id, userID).Scan(jobDest(&j)...)
	if err != nil {
		return Job{}, notFound(err)
	}
	return j, nil
}

func uniqueIDs(ids []uuid.UUID) map[uuid.UUID]struct{} {
	m := make(map[uuid.UUID]struct{}, len(ids))
	for _, id := range ids {
		m[id] = struct{}{}
	}
	return m
}
