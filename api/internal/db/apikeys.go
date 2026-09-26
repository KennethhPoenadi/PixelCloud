package db

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
)

type APIKey struct {
	ID         uuid.UUID  `json:"id"`
	Label      *string    `json:"label"`
	LastUsedAt *time.Time `json:"last_used_at"`
	RevokedAt  *time.Time `json:"revoked_at"`
	CreatedAt  time.Time  `json:"created_at"`
}

// MaxAPIKeys caps active keys per user.
const MaxAPIKeys = 20

func (s *Store) ListAPIKeys(ctx context.Context, userID uuid.UUID) ([]APIKey, error) {
	rows, err := s.Pool.Query(ctx, `
		SELECT id, label, last_used_at, revoked_at, created_at FROM api_keys
		WHERE user_id = $1 ORDER BY created_at DESC`, userID)
	if err != nil {
		return nil, fmt.Errorf("list api keys: %w", err)
	}
	defer rows.Close()
	out := []APIKey{}
	for rows.Next() {
		var k APIKey
		if err := rows.Scan(&k.ID, &k.Label, &k.LastUsedAt, &k.RevokedAt, &k.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan api key: %w", err)
		}
		out = append(out, k)
	}
	return out, rows.Err()
}

func (s *Store) CreateAPIKey(ctx context.Context, userID uuid.UUID, label *string, hash string) (APIKey, error) {
	k := APIKey{Label: label}
	err := s.Pool.QueryRow(ctx, `
		INSERT INTO api_keys (user_id, key_hash, label)
		SELECT $1, $2, $3
		WHERE (SELECT count(*) FROM api_keys WHERE user_id = $1 AND revoked_at IS NULL) < $4
		RETURNING id, created_at`, userID, hash, label, MaxAPIKeys).Scan(&k.ID, &k.CreatedAt)
	if err != nil {
		return APIKey{}, notFound(err)
	}
	return k, nil
}

func (s *Store) RevokeAPIKey(ctx context.Context, userID, id uuid.UUID) error {
	tag, err := s.Pool.Exec(ctx, `
		UPDATE api_keys SET revoked_at = now() WHERE id = $1 AND user_id = $2 AND revoked_at IS NULL`, id, userID)
	if err != nil {
		return fmt.Errorf("revoke api key: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// UserByAPIKey resolves an active key to its owner and whether their plan
// allows API access. last_used_at is refreshed at most once a minute.
func (s *Store) UserByAPIKey(ctx context.Context, hash string) (uuid.UUID, bool, error) {
	var userID uuid.UUID
	var allowed bool
	err := s.Pool.QueryRow(ctx, `
		WITH k AS (
			UPDATE api_keys SET last_used_at = now()
			WHERE key_hash = $1 AND revoked_at IS NULL
			  AND (last_used_at IS NULL OR last_used_at < now() - interval '1 minute')
			RETURNING user_id
		)
		SELECT a.user_id, p.api_access
		FROM api_keys a JOIN users u ON u.id = a.user_id JOIN plans p ON p.id = u.plan_id
		WHERE a.key_hash = $1 AND a.revoked_at IS NULL`, hash).Scan(&userID, &allowed)
	if err != nil {
		return uuid.Nil, false, notFound(err)
	}
	return userID, allowed, nil
}
