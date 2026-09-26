// Package quota enforces the monthly job quota of a plan. The check and the
// increment happen in one UPDATE so concurrent requests on api-1 and api-2
// cannot both slip under the limit.
package quota

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

var ErrExceeded = errors.New("monthly quota exceeded")

// PeriodStart returns the first day of the month (UTC) that t falls in.
func PeriodStart(t time.Time) time.Time {
	t = t.UTC()
	return time.Date(t.Year(), t.Month(), 1, 0, 0, 0, 0, time.UTC)
}

// Reserve adds n jobs to the user's usage for the current period, or returns
// ErrExceeded if that would go over limit (nil limit = unlimited). Must run
// inside the transaction that inserts the jobs.
func Reserve(ctx context.Context, tx pgx.Tx, userID uuid.UUID, limit *int32, n int, now time.Time) (int32, error) {
	period := PeriodStart(now)
	if _, err := tx.Exec(ctx, `
		INSERT INTO usage_monthly (user_id, period) VALUES ($1, $2)
		ON CONFLICT (user_id, period) DO NOTHING`, userID, period); err != nil {
		return 0, fmt.Errorf("init usage row: %w", err)
	}
	var used int32
	err := tx.QueryRow(ctx, `
		UPDATE usage_monthly SET jobs_count = jobs_count + $3
		WHERE user_id = $1 AND period = $2 AND ($4::int IS NULL OR jobs_count + $3 <= $4)
		RETURNING jobs_count`, userID, period, n, limit).Scan(&used)
	if errors.Is(err, pgx.ErrNoRows) {
		return 0, ErrExceeded
	}
	if err != nil {
		return 0, fmt.Errorf("reserve quota: %w", err)
	}
	return used, nil
}

// Release gives back n reserved jobs (e.g. when enqueueing failed).
func Release(ctx context.Context, tx pgx.Tx, userID uuid.UUID, n int, now time.Time) error {
	_, err := tx.Exec(ctx, `
		UPDATE usage_monthly SET jobs_count = GREATEST(jobs_count - $3, 0)
		WHERE user_id = $1 AND period = $2`, userID, PeriodStart(now), n)
	if err != nil {
		return fmt.Errorf("release quota: %w", err)
	}
	return nil
}
