// Package config loads API settings from environment variables (12-factor).
package config

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"time"
)

type Config struct {
	NodeID   string
	HTTPAddr string

	DatabaseURL string
	RedisURL    string

	MinioEndpoint       string
	MinioAccessKey      string
	MinioSecretKey      string
	MinioBucket         string
	MinioUseSSL         bool
	MinioPublicEndpoint string // host the browser uses; presigned URLs are signed for it
	MinioPublicUseSSL   bool
	MinioRegion         string

	JWTSecret  []byte
	JWTTTL     time.Duration
	PresignTTL time.Duration

	RetentionDays       int
	UserRateLimitPerMin int
	// Hard cap on request body size regardless of plan, protects memory on upload.
	MaxUploadMB int

	ShutdownTimeout time.Duration
}

func Load() (Config, error) {
	var errs []error
	get := func(key, def string) string {
		if v, ok := os.LookupEnv(key); ok && v != "" {
			return v
		}
		return def
	}
	must := func(key string) string {
		v := os.Getenv(key)
		if v == "" {
			errs = append(errs, fmt.Errorf("%s is required", key))
		}
		return v
	}
	dur := func(key, def string) time.Duration {
		d, err := time.ParseDuration(get(key, def))
		if err != nil {
			errs = append(errs, fmt.Errorf("%s: %w", key, err))
		}
		return d
	}
	num := func(key string, def int) int {
		n, err := strconv.Atoi(get(key, strconv.Itoa(def)))
		if err != nil {
			errs = append(errs, fmt.Errorf("%s: %w", key, err))
		}
		return n
	}
	boolean := func(key string, def bool) bool {
		b, err := strconv.ParseBool(get(key, strconv.FormatBool(def)))
		if err != nil {
			errs = append(errs, fmt.Errorf("%s: %w", key, err))
		}
		return b
	}

	c := Config{
		NodeID:              get("NODE_ID", hostname()),
		HTTPAddr:            get("HTTP_ADDR", ":8080"),
		DatabaseURL:         must("DATABASE_URL"),
		RedisURL:            must("REDIS_URL"),
		MinioEndpoint:       must("MINIO_ENDPOINT"),
		MinioAccessKey:      must("MINIO_ACCESS_KEY"),
		MinioSecretKey:      must("MINIO_SECRET_KEY"),
		MinioBucket:         get("MINIO_BUCKET", "pixelcloud"),
		MinioUseSSL:         boolean("MINIO_USE_SSL", false),
		MinioPublicEndpoint: get("MINIO_PUBLIC_ENDPOINT", "localhost:8080"),
		MinioPublicUseSSL:   boolean("MINIO_PUBLIC_USE_SSL", false),
		MinioRegion:         get("MINIO_REGION", "us-east-1"),
		JWTSecret:           []byte(must("JWT_SECRET")),
		JWTTTL:              dur("JWT_TTL", "1h"),
		PresignTTL:          dur("PRESIGN_TTL", "10m"),
		RetentionDays:       num("RETENTION_DAYS", 30),
		UserRateLimitPerMin: num("USER_RATE_LIMIT_PER_MIN", 300),
		MaxUploadMB:         num("MAX_UPLOAD_MB", 60),
		ShutdownTimeout:     dur("SHUTDOWN_TIMEOUT", "20s"),
	}
	if len(c.JWTSecret) > 0 && len(c.JWTSecret) < 32 {
		errs = append(errs, errors.New("JWT_SECRET must be at least 32 bytes"))
	}
	return c, errors.Join(errs...)
}

func hostname() string {
	h, err := os.Hostname()
	if err != nil {
		return "api"
	}
	return h
}
