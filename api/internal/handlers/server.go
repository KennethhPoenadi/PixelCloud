// Package handlers wires HTTP routes to the database, queue and storage.
package handlers

import (
	"log/slog"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/redis/go-redis/v9"

	"github.com/KennethhPoenadi/PixelCloud/api/internal/apierr"
	"github.com/KennethhPoenadi/PixelCloud/api/internal/auth"
	"github.com/KennethhPoenadi/PixelCloud/api/internal/config"
	"github.com/KennethhPoenadi/PixelCloud/api/internal/db"
	"github.com/KennethhPoenadi/PixelCloud/api/internal/health"
	"github.com/KennethhPoenadi/PixelCloud/api/internal/httpx"
	"github.com/KennethhPoenadi/PixelCloud/api/internal/metrics"
	"github.com/KennethhPoenadi/PixelCloud/api/internal/queue"
	"github.com/KennethhPoenadi/PixelCloud/api/internal/ratelimit"
	"github.com/KennethhPoenadi/PixelCloud/api/internal/storage"
)

type Server struct {
	cfg     config.Config
	log     *slog.Logger
	store   *db.Store
	rdb     *redis.Client
	queue   *queue.Queue
	storage *storage.Storage
	metrics *metrics.Metrics
	health  *health.Checker
	tokens  *auth.Tokens
	limiter *ratelimit.Limiter
}

type Deps struct {
	Config  config.Config
	Logger  *slog.Logger
	Store   *db.Store
	Redis   *redis.Client
	Queue   *queue.Queue
	Storage *storage.Storage
	Metrics *metrics.Metrics
	Health  *health.Checker
}

func New(d Deps) *Server {
	s := &Server{
		cfg:     d.Config,
		log:     d.Logger,
		store:   d.Store,
		rdb:     d.Redis,
		queue:   d.Queue,
		storage: d.Storage,
		metrics: d.Metrics,
		health:  d.Health,
		tokens:  auth.NewTokens(d.Config.JWTSecret, d.Config.JWTTTL),
	}
	if d.Config.UserRateLimitPerMin > 0 && d.Redis != nil {
		s.limiter = ratelimit.New(d.Redis, d.Config.UserRateLimitPerMin, time.Minute)
	}
	return s
}

func (s *Server) Routes() http.Handler {
	r := chi.NewRouter()
	r.Use(s.requestContext, recoverer, s.metrics.Middleware)

	// Probes and metrics live at the root; Nginx only proxies /api/, so /metrics
	// is reachable from Prometheus inside the network but never publicly.
	r.Get("/healthz", s.health.Liveness)
	r.Get("/readyz", s.health.Readiness)
	r.Method(http.MethodGet, "/metrics", s.metrics.Handler())

	r.Route("/api/v1", func(r chi.Router) {
		r.Get("/healthz", s.health.Liveness)
		r.Get("/readyz", s.health.Readiness)

		r.Group(func(r chi.Router) {
			r.Use(s.health.Gate)

			r.Get("/plans", s.listPlans)
			r.Post("/auth/register", s.register)
			r.Post("/auth/login", s.login)

			r.Group(func(r chi.Router) {
				r.Use(s.authenticate, s.rateLimitUser)

				r.Get("/me", s.me)

				r.Post("/images", s.uploadImage)
				r.Get("/images", s.listImages)
				r.Get("/images/{imageID}", s.getImage)
				r.Delete("/images/{imageID}", s.deleteImage)

				r.Get("/presets", s.listPresets)
				r.Post("/presets", s.createPreset)
				r.Delete("/presets/{presetID}", s.deletePreset)

				r.Post("/jobs", s.createJob)
				r.Get("/jobs/{jobID}", s.getJob)

				r.Post("/batches", s.createBatch)
				r.Get("/batches/{batchID}", s.getBatch)
				r.Get("/batches/{batchID}/download", s.downloadBatch)

				r.Group(func(r chi.Router) {
					r.Use(s.requireAPIPlan)
					r.Get("/api-keys", s.listAPIKeys)
					r.Post("/api-keys", s.createAPIKey)
					r.Delete("/api-keys/{keyID}", s.revokeAPIKey)
				})
			})
		})
	})

	r.NotFound(func(w http.ResponseWriter, r *http.Request) {
		httpx.WriteError(w, r, apierr.New(apierr.NotFound, "route not found"))
	})
	r.MethodNotAllowed(func(w http.ResponseWriter, r *http.Request) {
		httpx.WriteError(w, r, apierr.New(apierr.NotFound, "method not allowed on this route"))
	})
	return r
}
