package handlers

import (
	"encoding/base64"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/KennethhPoenadi/PixelCloud/api/internal/apierr"
	"github.com/KennethhPoenadi/PixelCloud/api/internal/db"
)

const (
	defaultPageSize = 24
	maxPageSize     = 100
)

func encodeCursor(c db.Cursor) string {
	raw := c.CreatedAt.UTC().Format(time.RFC3339Nano) + "|" + c.ID.String()
	return base64.RawURLEncoding.EncodeToString([]byte(raw))
}

func decodeCursor(s string) (*db.Cursor, error) {
	if s == "" {
		return nil, nil
	}
	invalid := apierr.New(apierr.Validation, "cursor is invalid")
	raw, err := base64.RawURLEncoding.DecodeString(s)
	if err != nil {
		return nil, invalid
	}
	ts, id, ok := strings.Cut(string(raw), "|")
	if !ok {
		return nil, invalid
	}
	t, err := time.Parse(time.RFC3339Nano, ts)
	if err != nil {
		return nil, invalid
	}
	uid, err := uuid.Parse(id)
	if err != nil {
		return nil, invalid
	}
	return &db.Cursor{CreatedAt: t, ID: uid}, nil
}

// pageParams reads ?cursor=&limit= with sane defaults and bounds.
func pageParams(r *http.Request) (*db.Cursor, int, error) {
	cursor, err := decodeCursor(r.URL.Query().Get("cursor"))
	if err != nil {
		return nil, 0, err
	}
	limit := defaultPageSize
	if v := r.URL.Query().Get("limit"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n < 1 || n > maxPageSize {
			return nil, 0, apierr.New(apierr.Validation, "limit must be between 1 and %d", maxPageSize)
		}
		limit = n
	}
	return cursor, limit, nil
}

// escapeLike escapes LIKE wildcards so user search text is matched literally.
func escapeLike(s string) string {
	return strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`).Replace(s)
}
