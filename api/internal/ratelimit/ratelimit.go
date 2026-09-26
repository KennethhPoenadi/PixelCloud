// Package ratelimit implements a per-key fixed-window limiter in Redis, so the
// limit is shared by every API node (nodes stay stateless).
package ratelimit

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

type Limiter struct {
	rdb    *redis.Client
	limit  int64
	window time.Duration
	now    func() time.Time
}

func New(rdb *redis.Client, limit int, window time.Duration) *Limiter {
	return &Limiter{rdb: rdb, limit: int64(limit), window: window, now: time.Now}
}

type Result struct {
	Allowed    bool
	Remaining  int64
	RetryAfter time.Duration
}

// Allow counts one request for key in the current window.
func (l *Limiter) Allow(ctx context.Context, key string) (Result, error) {
	now := l.now()
	windowStart := now.Truncate(l.window)
	redisKey := fmt.Sprintf("ratelimit:%s:%d", key, windowStart.Unix())

	pipe := l.rdb.TxPipeline()
	incr := pipe.Incr(ctx, redisKey)
	pipe.Expire(ctx, redisKey, l.window+time.Second)
	if _, err := pipe.Exec(ctx); err != nil {
		return Result{Allowed: true}, fmt.Errorf("rate limit: %w", err)
	}
	count := incr.Val()
	if count > l.limit {
		return Result{Allowed: false, RetryAfter: windowStart.Add(l.window).Sub(now)}, nil
	}
	return Result{Allowed: true, Remaining: l.limit - count}, nil
}
