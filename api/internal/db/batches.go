package db

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
)

type Batch struct {
	ID        uuid.UUID
	UserID    uuid.UUID
	Pipeline  []byte
	TotalJobs int32
	CreatedAt time.Time
}

// BatchForUser returns the batch and its jobs (oldest first) if owned by userID.
func (s *Store) BatchForUser(ctx context.Context, userID, id uuid.UUID) (Batch, []Job, error) {
	var b Batch
	err := s.Pool.QueryRow(ctx, `
		SELECT id, user_id, pipeline, total_jobs, created_at FROM batches WHERE id = $1 AND user_id = $2`,
		id, userID).Scan(&b.ID, &b.UserID, &b.Pipeline, &b.TotalJobs, &b.CreatedAt)
	if err != nil {
		return Batch{}, nil, notFound(err)
	}
	rows, err := s.Pool.Query(ctx, `SELECT `+jobColumns+` FROM jobs j JOIN images i ON i.id = j.image_id
		WHERE j.batch_id = $1 ORDER BY j.created_at, j.id`, id)
	if err != nil {
		return Batch{}, nil, fmt.Errorf("list batch jobs: %w", err)
	}
	defer rows.Close()
	var jobs []Job
	for rows.Next() {
		var j Job
		if err := rows.Scan(jobDest(&j)...); err != nil {
			return Batch{}, nil, fmt.Errorf("scan job: %w", err)
		}
		jobs = append(jobs, j)
	}
	return b, jobs, rows.Err()
}
