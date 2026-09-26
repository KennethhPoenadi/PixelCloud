package db

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
)

type Preset struct {
	ID        uuid.UUID
	UserID    *uuid.UUID
	Name      string
	Pipeline  []byte
	IsSystem  bool
	CreatedAt time.Time
}

var (
	ErrPresetExists = errors.New("preset name already used")
	ErrPresetLimit  = errors.New("too many custom presets")
)

// MaxCustomPresets caps how many presets a single user can save.
const MaxCustomPresets = 50

// ListPresets returns system presets followed by the user's own, oldest first.
func (s *Store) ListPresets(ctx context.Context, userID uuid.UUID) ([]Preset, error) {
	rows, err := s.Pool.Query(ctx, `
		SELECT id, user_id, name, pipeline, is_system, created_at
		FROM filter_presets
		WHERE user_id IS NULL OR user_id = $1
		ORDER BY is_system DESC, created_at, name`, userID)
	if err != nil {
		return nil, fmt.Errorf("list presets: %w", err)
	}
	defer rows.Close()
	var out []Preset
	for rows.Next() {
		var p Preset
		if err := rows.Scan(&p.ID, &p.UserID, &p.Name, &p.Pipeline, &p.IsSystem, &p.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan preset: %w", err)
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

func (s *Store) CreatePreset(ctx context.Context, userID uuid.UUID, name string, pipeline []byte) (Preset, error) {
	p := Preset{UserID: &userID, Name: name, Pipeline: pipeline}
	// The count guard and insert are one statement so concurrent saves cannot exceed the cap.
	err := s.Pool.QueryRow(ctx, `
		INSERT INTO filter_presets (user_id, name, pipeline)
		SELECT $1, $2, $3
		WHERE (SELECT count(*) FROM filter_presets WHERE user_id = $1) < $4
		RETURNING id, created_at`, userID, name, string(pipeline), MaxCustomPresets,
	).Scan(&p.ID, &p.CreatedAt)
	if isUniqueViolation(err) {
		return Preset{}, ErrPresetExists
	}
	if err := notFound(err); errors.Is(err, ErrNotFound) {
		return Preset{}, ErrPresetLimit
	} else if err != nil {
		return Preset{}, fmt.Errorf("insert preset: %w", err)
	}
	return p, nil
}

// DeletePreset removes a custom preset owned by userID; system presets are immutable.
func (s *Store) DeletePreset(ctx context.Context, userID, id uuid.UUID) error {
	tag, err := s.Pool.Exec(ctx, `DELETE FROM filter_presets WHERE id = $1 AND user_id = $2 AND NOT is_system`, id, userID)
	if err != nil {
		return fmt.Errorf("delete preset: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}
