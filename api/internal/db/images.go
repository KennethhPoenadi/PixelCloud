package db

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
)

type Image struct {
	ID         uuid.UUID
	UserID     uuid.UUID
	StorageKey string
	Filename   string
	MimeType   string
	SizeBytes  int64
	Width      int32
	Height     int32
	CreatedAt  time.Time
	ExpiresAt  *time.Time

	// Latest job for this image (history badge), if any.
	LatestJob *JobSummary
}

type JobSummary struct {
	ID        uuid.UUID
	Status    string
	ResultKey *string
	Format    string
}

const imageColumns = `i.id, i.user_id, i.storage_key, i.filename, i.mime_type, i.size_bytes,
	i.width, i.height, i.created_at, i.expires_at`

func imageDest(im *Image) []any {
	return []any{&im.ID, &im.UserID, &im.StorageKey, &im.Filename, &im.MimeType, &im.SizeBytes,
		&im.Width, &im.Height, &im.CreatedAt, &im.ExpiresAt}
}

func (s *Store) CreateImage(ctx context.Context, im Image) (Image, error) {
	err := s.Pool.QueryRow(ctx, `
		INSERT INTO images (id, user_id, storage_key, filename, mime_type, size_bytes, width, height, expires_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		RETURNING created_at`,
		im.ID, im.UserID, im.StorageKey, im.Filename, im.MimeType, im.SizeBytes, im.Width, im.Height, im.ExpiresAt,
	).Scan(&im.CreatedAt)
	if err != nil {
		return Image{}, fmt.Errorf("insert image: %w", err)
	}
	return im, nil
}

// ImageForUser returns an image only if it belongs to userID.
func (s *Store) ImageForUser(ctx context.Context, userID, id uuid.UUID) (Image, error) {
	var im Image
	err := s.Pool.QueryRow(ctx, `SELECT `+imageColumns+` FROM images i WHERE i.id = $1 AND i.user_id = $2`,
		id, userID).Scan(imageDest(&im)...)
	if err != nil {
		return Image{}, notFound(err)
	}
	return im, nil
}

// Cursor is the keyset position (created_at, id) of the last returned row.
type Cursor struct {
	CreatedAt time.Time
	ID        uuid.UUID
}

// ListImages returns up to limit images newest-first, starting after cursor.
func (s *Store) ListImages(ctx context.Context, userID uuid.UUID, after *Cursor, limit int, search string) ([]Image, error) {
	args := []any{userID, limit, "%" + search + "%"}
	cond := ""
	if after != nil {
		cond = "AND (i.created_at, i.id) < ($4, $5)"
		args = append(args, after.CreatedAt, after.ID)
	}
	rows, err := s.Pool.Query(ctx, `
		SELECT `+imageColumns+`, j.id, j.status::text, j.result_key, j.output_format
		FROM images i
		LEFT JOIN LATERAL (
			SELECT id, status, result_key, output_format FROM jobs
			WHERE jobs.image_id = i.id ORDER BY created_at DESC LIMIT 1
		) j ON TRUE
		WHERE i.user_id = $1 AND ($3 = '%%' OR i.filename ILIKE $3) `+cond+`
		ORDER BY i.created_at DESC, i.id DESC
		LIMIT $2`, args...)
	if err != nil {
		return nil, fmt.Errorf("list images: %w", err)
	}
	defer rows.Close()

	var out []Image
	for rows.Next() {
		var im Image
		var jobID *uuid.UUID
		var status, format *string
		var resultKey *string
		dest := append(imageDest(&im), &jobID, &status, &resultKey, &format)
		if err := rows.Scan(dest...); err != nil {
			return nil, fmt.Errorf("scan image: %w", err)
		}
		if jobID != nil {
			im.LatestJob = &JobSummary{ID: *jobID, Status: *status, ResultKey: resultKey, Format: *format}
		}
		out = append(out, im)
	}
	return out, rows.Err()
}

// DeleteImage removes the image row (jobs cascade) and returns every storage
// key that belonged to it so the caller can delete the objects.
func (s *Store) DeleteImage(ctx context.Context, userID, id uuid.UUID) ([]string, error) {
	rows, err := s.Pool.Query(ctx, `
		WITH img AS (
			DELETE FROM images WHERE id = $1 AND user_id = $2 RETURNING id, storage_key
		)
		SELECT storage_key FROM img
		UNION ALL
		SELECT j.result_key FROM jobs j JOIN img ON j.image_id = img.id WHERE j.result_key IS NOT NULL`,
		id, userID)
	if err != nil {
		return nil, fmt.Errorf("delete image: %w", err)
	}
	defer rows.Close()
	var keys []string
	for rows.Next() {
		var k string
		if err := rows.Scan(&k); err != nil {
			return nil, fmt.Errorf("scan key: %w", err)
		}
		keys = append(keys, k)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("delete image: %w", err)
	}
	if len(keys) == 0 {
		return nil, ErrNotFound
	}
	return keys, nil
}
