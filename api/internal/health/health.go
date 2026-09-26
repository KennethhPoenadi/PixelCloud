// Package health implements liveness/readiness. Readiness is computed in the
// background so probes are cheap, and a middleware sheds traffic (503) while the
// node is not ready so Nginx fails over to the other node.
package health

import (
	"context"
	"net/http"
	"sync"
	"sync/atomic"
	"time"

	"github.com/KennethhPoenadi/PixelCloud/api/internal/apierr"
	"github.com/KennethhPoenadi/PixelCloud/api/internal/httpx"
)

type CheckFunc func(ctx context.Context) error

type Checker struct {
	nodeID   string
	checks   map[string]CheckFunc
	interval time.Duration
	timeout  time.Duration

	ready    atomic.Bool
	draining atomic.Bool
	mu       sync.RWMutex
	status   map[string]string
}

func New(nodeID string, checks map[string]CheckFunc) *Checker {
	return &Checker{
		nodeID:   nodeID,
		checks:   checks,
		interval: 2 * time.Second,
		timeout:  time.Second,
		status:   map[string]string{},
	}
}

// Run evaluates all checks until ctx is cancelled.
func (c *Checker) Run(ctx context.Context) {
	c.evaluate(ctx)
	t := time.NewTicker(c.interval)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			c.evaluate(ctx)
		}
	}
}

func (c *Checker) evaluate(ctx context.Context) {
	status := make(map[string]string, len(c.checks))
	ok := true
	var mu sync.Mutex
	var wg sync.WaitGroup
	for name, check := range c.checks {
		wg.Go(func() {
			cctx, cancel := context.WithTimeout(ctx, c.timeout)
			defer cancel()
			res := "ok"
			if err := check(cctx); err != nil {
				res = err.Error()
			}
			mu.Lock()
			defer mu.Unlock()
			status[name] = res
			if res != "ok" {
				ok = false
			}
		})
	}
	wg.Wait()
	c.mu.Lock()
	c.status = status
	c.mu.Unlock()
	c.ready.Store(ok)
}

// Drain marks the node as not ready ahead of shutdown.
func (c *Checker) Drain() { c.draining.Store(true) }

func (c *Checker) Ready() bool { return c.ready.Load() && !c.draining.Load() }

func (c *Checker) Liveness(w http.ResponseWriter, _ *http.Request) {
	httpx.WriteJSON(w, http.StatusOK, map[string]string{"status": "ok", "node": c.nodeID})
}

func (c *Checker) Readiness(w http.ResponseWriter, _ *http.Request) {
	c.mu.RLock()
	checks := make(map[string]string, len(c.status))
	for k, v := range c.status {
		checks[k] = v
	}
	c.mu.RUnlock()
	status, code := "ready", http.StatusOK
	if !c.Ready() {
		status, code = "not_ready", http.StatusServiceUnavailable
	}
	httpx.WriteJSON(w, code, map[string]any{"status": status, "node": c.nodeID, "checks": checks})
}

// Gate rejects requests with 503 while the node is not ready.
func (c *Checker) Gate(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !c.Ready() {
			w.Header().Set("Retry-After", "1")
			httpx.WriteError(w, r, apierr.New(apierr.Unavailable, "node %s is not ready", c.nodeID))
			return
		}
		next.ServeHTTP(w, r)
	})
}
