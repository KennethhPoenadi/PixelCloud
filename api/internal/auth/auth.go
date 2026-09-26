// Package auth handles password hashing, JWT sessions and API keys.
package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
)

const bcryptCost = 12

// dummyHash is compared against when a login email does not exist so both
// paths take roughly the same time (no user enumeration via timing).
var dummyHash, _ = bcrypt.GenerateFromPassword([]byte("pixelcloud-dummy-password"), bcryptCost)

func HashPassword(password string) (string, error) {
	h, err := bcrypt.GenerateFromPassword([]byte(password), bcryptCost)
	if err != nil {
		return "", fmt.Errorf("hash password: %w", err)
	}
	return string(h), nil
}

// CheckPassword reports whether password matches hash. An empty hash burns
// the same CPU as a real comparison and returns false.
func CheckPassword(hash, password string) bool {
	if hash == "" {
		_ = bcrypt.CompareHashAndPassword(dummyHash, []byte(password))
		return false
	}
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)) == nil
}

var ErrInvalidToken = errors.New("invalid token")

type Tokens struct {
	secret []byte
	ttl    time.Duration
}

func NewTokens(secret []byte, ttl time.Duration) *Tokens {
	return &Tokens{secret: secret, ttl: ttl}
}

func (t *Tokens) Issue(userID uuid.UUID) (string, time.Time, error) {
	now := time.Now()
	exp := now.Add(t.ttl)
	claims := jwt.RegisteredClaims{
		Subject:   userID.String(),
		Issuer:    "pixelcloud",
		IssuedAt:  jwt.NewNumericDate(now),
		ExpiresAt: jwt.NewNumericDate(exp),
	}
	signed, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(t.secret)
	if err != nil {
		return "", time.Time{}, fmt.Errorf("sign token: %w", err)
	}
	return signed, exp, nil
}

func (t *Tokens) Parse(token string) (uuid.UUID, error) {
	var claims jwt.RegisteredClaims
	_, err := jwt.ParseWithClaims(token, &claims, func(*jwt.Token) (any, error) { return t.secret, nil },
		jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}),
		jwt.WithIssuer("pixelcloud"),
		jwt.WithExpirationRequired(),
	)
	if err != nil {
		return uuid.Nil, ErrInvalidToken
	}
	id, err := uuid.Parse(claims.Subject)
	if err != nil {
		return uuid.Nil, ErrInvalidToken
	}
	return id, nil
}

const apiKeyPrefix = "pc_"

// NewAPIKey returns a random key (shown to the user once) and its hash (stored).
func NewAPIKey() (plain, hash string, err error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", "", fmt.Errorf("generate api key: %w", err)
	}
	plain = apiKeyPrefix + hex.EncodeToString(b)
	return plain, HashAPIKey(plain), nil
}

// HashAPIKey uses SHA-256: keys are 256-bit random, so a slow KDF adds nothing.
func HashAPIKey(plain string) string {
	sum := sha256.Sum256([]byte(strings.TrimSpace(plain)))
	return hex.EncodeToString(sum[:])
}

type Principal struct {
	UserID    uuid.UUID
	ViaAPIKey bool
}

type ctxKey struct{}

func WithPrincipal(ctx context.Context, p Principal) context.Context {
	return context.WithValue(ctx, ctxKey{}, p)
}

func FromContext(ctx context.Context) (Principal, bool) {
	p, ok := ctx.Value(ctxKey{}).(Principal)
	return p, ok
}
