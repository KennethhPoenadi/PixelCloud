package auth

import (
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestPasswordRoundTrip(t *testing.T) {
	h, err := HashPassword("correct horse battery")
	if err != nil {
		t.Fatal(err)
	}
	if !CheckPassword(h, "correct horse battery") {
		t.Fatal("expected password to match")
	}
	if CheckPassword(h, "wrong") {
		t.Fatal("expected wrong password to fail")
	}
	if CheckPassword("", "anything") {
		t.Fatal("empty hash must never match")
	}
}

func TestTokens(t *testing.T) {
	secret := []byte(strings.Repeat("s", 32))
	tok := NewTokens(secret, time.Hour)
	id := uuid.New()

	signed, exp, err := tok.Issue(id)
	if err != nil {
		t.Fatal(err)
	}
	if time.Until(exp) < 59*time.Minute {
		t.Fatalf("unexpected expiry %v", exp)
	}
	got, err := tok.Parse(signed)
	if err != nil || got != id {
		t.Fatalf("parse = %v, %v; want %v", got, err, id)
	}

	other := NewTokens([]byte(strings.Repeat("x", 32)), time.Hour)
	if _, err := other.Parse(signed); err == nil {
		t.Fatal("token signed with another secret must be rejected")
	}

	expired := NewTokens(secret, -time.Minute)
	old, _, _ := expired.Issue(id)
	if _, err := tok.Parse(old); err == nil {
		t.Fatal("expired token must be rejected")
	}
}

func TestAPIKey(t *testing.T) {
	plain, hash, err := NewAPIKey()
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(plain, "pc_") || len(plain) != 3+64 {
		t.Fatalf("unexpected key format %q", plain)
	}
	if HashAPIKey(plain) != hash || HashAPIKey(" "+plain+"\n") != hash {
		t.Fatal("hash must be stable and ignore surrounding whitespace")
	}
	if strings.Contains(hash, plain[3:]) {
		t.Fatal("hash must not contain the key")
	}
}
