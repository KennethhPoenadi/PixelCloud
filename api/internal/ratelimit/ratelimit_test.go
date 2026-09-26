package ratelimit

import (
	"context"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
)

func TestFixedWindow(t *testing.T) {
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	l := New(rdb, 3, time.Minute)
	now := time.Date(2026, 9, 26, 10, 0, 10, 0, time.UTC)
	l.now = func() time.Time { return now }
	ctx := context.Background()

	for i := range 3 {
		res, err := l.Allow(ctx, "user:a")
		if err != nil || !res.Allowed || res.Remaining != int64(2-i) {
			t.Fatalf("request %d: %+v, %v", i, res, err)
		}
	}
	res, _ := l.Allow(ctx, "user:a")
	if res.Allowed || res.RetryAfter != 50*time.Second {
		t.Fatalf("4th request should be limited with 50s retry, got %+v", res)
	}
	if res, _ := l.Allow(ctx, "user:b"); !res.Allowed {
		t.Fatal("other keys must have their own budget")
	}

	now = now.Add(time.Minute)
	if res, _ := l.Allow(ctx, "user:a"); !res.Allowed {
		t.Fatal("new window must reset the budget")
	}
}

func TestRedisDownFailsOpen(t *testing.T) {
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr(), MaxRetries: -1})
	mr.Close()
	res, err := New(rdb, 1, time.Minute).Allow(context.Background(), "user:a")
	if err == nil || !res.Allowed {
		t.Fatalf("expected error and allowed=true, got %+v, %v", res, err)
	}
}
