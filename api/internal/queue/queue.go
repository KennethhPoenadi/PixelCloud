// Package queue publishes jobs to a Redis Stream consumed by the workers
// through a consumer group (reliable delivery + reclaim on worker death).
package queue

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
)

const (
	Stream = "pixelcloud:jobs"
	Group  = "workers"
)

type Message struct {
	JobID     string
	RequestID string
}

type Queue struct {
	rdb *redis.Client
}

func New(rdb *redis.Client) *Queue { return &Queue{rdb: rdb} }

// EnsureGroup creates the stream and consumer group if they do not exist.
func (q *Queue) EnsureGroup(ctx context.Context) error {
	err := q.rdb.XGroupCreateMkStream(ctx, Stream, Group, "0").Err()
	if err != nil && !strings.Contains(err.Error(), "BUSYGROUP") {
		return fmt.Errorf("create consumer group: %w", err)
	}
	return nil
}

// Enqueue publishes all messages in a single round trip.
func (q *Queue) Enqueue(ctx context.Context, msgs ...Message) error {
	if len(msgs) == 0 {
		return nil
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	_, err := q.rdb.Pipelined(ctx, func(p redis.Pipeliner) error {
		for _, m := range msgs {
			p.XAdd(ctx, &redis.XAddArgs{
				Stream: Stream,
				Values: map[string]any{"job_id": m.JobID, "request_id": m.RequestID, "enqueued_at": now},
			})
		}
		return nil
	})
	if err != nil {
		return fmt.Errorf("enqueue %d jobs: %w", len(msgs), err)
	}
	return nil
}

type Stats struct {
	// Length counts entries still in the stream: waiting + in-flight
	// (workers delete entries once a job is finished).
	Length int64
	// Pending counts entries delivered to a worker but not yet acknowledged.
	Pending int64
}

func (q *Queue) Stats(ctx context.Context) (Stats, error) {
	var st Stats
	n, err := q.rdb.XLen(ctx, Stream).Result()
	if err != nil {
		return st, fmt.Errorf("xlen: %w", err)
	}
	st.Length = n
	p, err := q.rdb.XPending(ctx, Stream, Group).Result()
	if err != nil && err != redis.Nil {
		return st, fmt.Errorf("xpending: %w", err)
	}
	if p != nil {
		st.Pending = p.Count
	}
	return st, nil
}
