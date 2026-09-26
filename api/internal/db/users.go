package db

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

type Plan struct {
	ID            int16  `json:"-"`
	Code          string `json:"code"`
	Name          string `json:"name"`
	MonthlyQuota  *int32 `json:"monthly_quota"`
	MaxFileMB     int32  `json:"max_file_mb"`
	MaxResolution int32  `json:"max_resolution"`
	MaxBatchSize  int32  `json:"max_batch_size"`
	Watermark     bool   `json:"watermark"`
	APIAccess     bool   `json:"api_access"`
	PriceIDR      int32  `json:"price_idr"`
}

type User struct {
	ID           uuid.UUID `json:"id"`
	Email        string    `json:"email"`
	PasswordHash string    `json:"-"`
	DisplayName  *string   `json:"display_name"`
	CreatedAt    time.Time `json:"created_at"`
}

type Usage struct {
	Period    time.Time `json:"period"`
	JobsCount int32     `json:"jobs_count"`
	BytesIn   int64     `json:"bytes_in"`
}

var ErrEmailTaken = errors.New("email already registered")

const planColumns = `p.id, p.code, p.name, p.monthly_quota, p.max_file_mb, p.max_resolution,
	p.max_batch_size, p.watermark, p.api_access, p.price_idr`

func scanPlan(p *Plan) []any {
	return []any{&p.ID, &p.Code, &p.Name, &p.MonthlyQuota, &p.MaxFileMB, &p.MaxResolution,
		&p.MaxBatchSize, &p.Watermark, &p.APIAccess, &p.PriceIDR}
}

// CreateUser registers a user on the default "free" plan.
func (s *Store) CreateUser(ctx context.Context, email, passwordHash string, displayName *string) (User, error) {
	var u User
	err := s.Pool.QueryRow(ctx, `
		INSERT INTO users (email, password_hash, display_name, plan_id)
		VALUES ($1, $2, $3, (SELECT id FROM plans WHERE code = 'free'))
		RETURNING id, email, password_hash, display_name, created_at`,
		email, passwordHash, displayName,
	).Scan(&u.ID, &u.Email, &u.PasswordHash, &u.DisplayName, &u.CreatedAt)
	if isUniqueViolation(err) {
		return User{}, ErrEmailTaken
	}
	if err != nil {
		return User{}, fmt.Errorf("insert user: %w", err)
	}
	return u, nil
}

func (s *Store) UserByEmail(ctx context.Context, email string) (User, error) {
	var u User
	err := s.Pool.QueryRow(ctx, `
		SELECT id, email, password_hash, display_name, created_at FROM users WHERE email = $1`, email,
	).Scan(&u.ID, &u.Email, &u.PasswordHash, &u.DisplayName, &u.CreatedAt)
	if err != nil {
		return User{}, notFound(err)
	}
	return u, nil
}

// UserWithPlan loads a user and their current plan.
func (s *Store) UserWithPlan(ctx context.Context, id uuid.UUID) (User, Plan, error) {
	var u User
	var p Plan
	dest := append([]any{&u.ID, &u.Email, &u.PasswordHash, &u.DisplayName, &u.CreatedAt}, scanPlan(&p)...)
	err := s.Pool.QueryRow(ctx, `
		SELECT u.id, u.email, u.password_hash, u.display_name, u.created_at, `+planColumns+`
		FROM users u JOIN plans p ON p.id = u.plan_id
		WHERE u.id = $1`, id,
	).Scan(dest...)
	if err != nil {
		return User{}, Plan{}, notFound(err)
	}
	return u, p, nil
}

// PlanForUser loads only the plan of a user.
func (s *Store) PlanForUser(ctx context.Context, userID uuid.UUID) (Plan, error) {
	var p Plan
	err := s.Pool.QueryRow(ctx, `
		SELECT `+planColumns+` FROM users u JOIN plans p ON p.id = u.plan_id WHERE u.id = $1`, userID,
	).Scan(scanPlan(&p)...)
	if err != nil {
		return Plan{}, notFound(err)
	}
	return p, nil
}

// ListPlans returns all plans ordered by price (used by the pricing table).
func (s *Store) ListPlans(ctx context.Context) ([]Plan, error) {
	rows, err := s.Pool.Query(ctx, `SELECT `+planColumns+` FROM plans p ORDER BY p.price_idr, p.id`)
	if err != nil {
		return nil, fmt.Errorf("list plans: %w", err)
	}
	defer rows.Close()
	var out []Plan
	for rows.Next() {
		var p Plan
		if err := rows.Scan(scanPlan(&p)...); err != nil {
			return nil, fmt.Errorf("scan plan: %w", err)
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

// PeriodStart returns the first day of the month (UTC) that t falls in.
func PeriodStart(t time.Time) time.Time {
	t = t.UTC()
	return time.Date(t.Year(), t.Month(), 1, 0, 0, 0, 0, time.UTC)
}

func (s *Store) UsageForPeriod(ctx context.Context, userID uuid.UUID, period time.Time) (Usage, error) {
	u := Usage{Period: period}
	err := s.Pool.QueryRow(ctx, `
		SELECT jobs_count, bytes_in FROM usage_monthly WHERE user_id = $1 AND period = $2`, userID, period,
	).Scan(&u.JobsCount, &u.BytesIn)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return u, fmt.Errorf("load usage: %w", err)
	}
	return u, nil
}

func (s *Store) AddBytesIn(ctx context.Context, userID uuid.UUID, period time.Time, n int64) error {
	_, err := s.Pool.Exec(ctx, `
		INSERT INTO usage_monthly (user_id, period, bytes_in) VALUES ($1, $2, $3)
		ON CONFLICT (user_id, period) DO UPDATE SET bytes_in = usage_monthly.bytes_in + EXCLUDED.bytes_in`,
		userID, period, n)
	if err != nil {
		return fmt.Errorf("add bytes_in: %w", err)
	}
	return nil
}
