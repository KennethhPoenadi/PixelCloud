package handlers

import (
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/KennethhPoenadi/PixelCloud/api/internal/db"
)

func TestCursorRoundTrip(t *testing.T) {
	c := db.Cursor{CreatedAt: time.Date(2026, 9, 1, 10, 0, 0, 123456000, time.UTC), ID: uuid.New()}
	got, err := decodeCursor(encodeCursor(c))
	if err != nil {
		t.Fatal(err)
	}
	if !got.CreatedAt.Equal(c.CreatedAt) || got.ID != c.ID {
		t.Fatalf("got %+v, want %+v", got, c)
	}
	for _, bad := range []string{"!!", "bm90LWEtY3Vyc29y", encodeCursor(c)[:10]} {
		if _, err := decodeCursor(bad); err == nil {
			t.Errorf("decodeCursor(%q) should fail", bad)
		}
	}
	if c, err := decodeCursor(""); c != nil || err != nil {
		t.Fatalf("empty cursor = %v, %v", c, err)
	}
}

func TestPageParams(t *testing.T) {
	_, limit, err := pageParams(httptest.NewRequest("GET", "/images", nil))
	if err != nil || limit != defaultPageSize {
		t.Fatalf("default limit = %d, %v", limit, err)
	}
	for _, q := range []string{"0", "101", "abc"} {
		if _, _, err := pageParams(httptest.NewRequest("GET", "/images?limit="+q, nil)); err == nil {
			t.Errorf("limit=%s should be rejected", q)
		}
	}
}

func TestSanitizeFilename(t *testing.T) {
	cases := map[string]string{
		"photo.jpg":              "photo.jpg",
		"../../etc/passwd":       "passwd",
		`C:\Users\me\pic.png`:    "pic.png",
		"bad\x00name\n.jpg":      "badname.jpg",
		`quote"d.jpg`:            "quoted.jpg",
		"":                       "image.jpg",
		strings.Repeat("a", 300): strings.Repeat("a", 200),
	}
	for in, want := range cases {
		if got := sanitizeFilename(in, "jpg"); got != want {
			t.Errorf("sanitizeFilename(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestEscapeLike(t *testing.T) {
	if got := escapeLike(`50%_off\`); got != `50\%\_off\\` {
		t.Fatalf("escapeLike = %q", got)
	}
}
