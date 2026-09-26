package handlers

import (
	"fmt"
	"log/slog"
	"net/http"
	"regexp"
	"time"

	"github.com/go-chi/chi/v5/middleware"
	"github.com/google/uuid"

	"github.com/KennethhPoenadi/PixelCloud/api/internal/httpx"
)

var requestIDPattern = regexp.MustCompile(`^[A-Za-z0-9-]{8,64}$`)

// requestContext assigns a request ID (reusing Nginx's X-Request-ID when valid),
// attaches a scoped logger and writes one structured access log line per request.
func (s *Server) requestContext(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		rid := r.Header.Get("X-Request-ID")
		if !requestIDPattern.MatchString(rid) {
			rid = uuid.NewString()
		}
		logger := s.log.With("request_id", rid)
		ctx := httpx.WithLogger(httpx.WithRequestID(r.Context(), rid), logger)

		w.Header().Set("X-Request-ID", rid)
		w.Header().Set("X-Node-ID", s.cfg.NodeID)
		ww := middleware.NewWrapResponseWriter(w, r.ProtoMajor)
		next.ServeHTTP(ww, r.WithContext(ctx))

		if r.URL.Path == "/metrics" || r.URL.Path == "/healthz" {
			return
		}
		logger.LogAttrs(ctx, slog.LevelInfo, "http request",
			slog.String("method", r.Method),
			slog.String("path", r.URL.Path),
			slog.Int("status", ww.Status()),
			slog.Int("bytes", ww.BytesWritten()),
			slog.Duration("duration", time.Since(start)),
		)
	})
}

// recoverer turns a panic into a logged INTERNAL error instead of a dropped connection.
func recoverer(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if rec := recover(); rec != nil {
				if rec == http.ErrAbortHandler {
					panic(rec)
				}
				httpx.WriteError(w, r, fmt.Errorf("panic: %v", rec))
			}
		}()
		next.ServeHTTP(w, r)
	})
}
