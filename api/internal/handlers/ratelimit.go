package handlers

import (
	"math"
	"net/http"
	"strconv"

	"github.com/KennethhPoenadi/PixelCloud/api/internal/apierr"
	"github.com/KennethhPoenadi/PixelCloud/api/internal/httpx"
)

// rateLimitUser enforces the per-user request budget. If Redis is unavailable
// the request is allowed (fail open) — availability beats strict limiting here.
func (s *Server) rateLimitUser(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if s.limiter == nil {
			next.ServeHTTP(w, r)
			return
		}
		res, err := s.limiter.Allow(r.Context(), "user:"+mustPrincipal(r).UserID.String())
		if err != nil {
			httpx.Logger(r.Context()).Warn("rate limiter unavailable, allowing request", "err", err)
			next.ServeHTTP(w, r)
			return
		}
		if !res.Allowed {
			w.Header().Set("Retry-After", strconv.Itoa(int(math.Ceil(res.RetryAfter.Seconds()))))
			httpx.WriteError(w, r, apierr.New(apierr.RateLimited, "too many requests, slow down"))
			return
		}
		w.Header().Set("X-RateLimit-Remaining", strconv.FormatInt(res.Remaining, 10))
		next.ServeHTTP(w, r)
	})
}
