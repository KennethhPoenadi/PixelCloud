package health

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestReadinessHysteresis(t *testing.T) {
	fail := false
	c := New("n1", map[string]CheckFunc{
		"dep": func(context.Context) error {
			if fail {
				return errors.New("down")
			}
			return nil
		},
	})
	ctx := context.Background()
	if c.Ready() {
		t.Fatal("must start not ready")
	}
	c.evaluate(ctx)
	if !c.Ready() {
		t.Fatal("ready after first success")
	}
	fail = true
	c.evaluate(ctx)
	c.evaluate(ctx)
	if !c.Ready() {
		t.Fatal("two failures must not flip readiness")
	}
	c.evaluate(ctx)
	if c.Ready() {
		t.Fatal("third consecutive failure must mark not ready")
	}
	fail = false
	c.evaluate(ctx)
	if !c.Ready() {
		t.Fatal("one success recovers")
	}

	c.Drain()
	if c.Ready() {
		t.Fatal("draining node is not ready")
	}
	rec := httptest.NewRecorder()
	c.Gate(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { t.Fatal("gate let request through") })).
		ServeHTTP(rec, httptest.NewRequest("GET", "/", nil))
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("gate status = %d", rec.Code)
	}
}
