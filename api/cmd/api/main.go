package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/redis/go-redis/v9"

	"github.com/KennethhPoenadi/PixelCloud/api/internal/config"
	"github.com/KennethhPoenadi/PixelCloud/api/internal/db"
	"github.com/KennethhPoenadi/PixelCloud/api/internal/handlers"
	"github.com/KennethhPoenadi/PixelCloud/api/internal/health"
	"github.com/KennethhPoenadi/PixelCloud/api/internal/metrics"
	"github.com/KennethhPoenadi/PixelCloud/api/internal/queue"
	"github.com/KennethhPoenadi/PixelCloud/api/internal/storage"
)

func main() {
	healthcheck := flag.Bool("healthcheck", false, "probe /healthz on the local server and exit (for Docker HEALTHCHECK)")
	flag.Parse()
	if *healthcheck {
		os.Exit(probe())
	}
	if err := run(); err != nil {
		slog.Error("api exited", "err", err)
		os.Exit(1)
	}
}

func run() error {
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil)).With("service", "api", "node_id", cfg.NodeID)
	slog.SetDefault(logger)

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	store, err := db.Open(ctx, cfg.DatabaseURL)
	if err != nil {
		return err
	}
	defer store.Close()

	redisOpts, err := redis.ParseURL(cfg.RedisURL)
	if err != nil {
		return fmt.Errorf("parse redis url: %w", err)
	}
	redisOpts.DialTimeout = 2 * time.Second
	redisOpts.ReadTimeout = 2 * time.Second
	redisOpts.WriteTimeout = 2 * time.Second
	rdb := redis.NewClient(redisOpts)
	defer func() { _ = rdb.Close() }()
	q := queue.New(rdb)

	st, err := storage.New(storage.Config{
		Endpoint:       cfg.MinioEndpoint,
		PublicEndpoint: cfg.MinioPublicEndpoint,
		AccessKey:      cfg.MinioAccessKey,
		SecretKey:      cfg.MinioSecretKey,
		Bucket:         cfg.MinioBucket,
		UseSSL:         cfg.MinioUseSSL,
		PublicUseSSL:   cfg.MinioPublicUseSSL,
		Region:         cfg.MinioRegion,
	})
	if err != nil {
		return err
	}

	// Bucket and consumer group are created lazily so the API can start before
	// its dependencies; readiness stays false until bootstrap succeeds.
	var bootstrapped atomic.Bool
	go bootstrap(ctx, logger, &bootstrapped, st, q)

	m := metrics.New(cfg.NodeID)
	m.Register(cfg.NodeID, metrics.NewQueueCollector(func(ctx context.Context) (int64, int64, error) {
		st, err := q.Stats(ctx)
		return st.Length, st.Pending, err
	}))
	checker := health.New(cfg.NodeID, map[string]health.CheckFunc{
		"postgres": store.Ping,
		"redis":    func(ctx context.Context) error { return rdb.Ping(ctx).Err() },
		"minio":    st.Ping,
		"bootstrap": func(context.Context) error {
			if !bootstrapped.Load() {
				return errors.New("bucket/queue not initialised yet")
			}
			return nil
		},
	})
	go checker.Run(ctx)

	srv := handlers.New(handlers.Deps{
		Config: cfg, Logger: logger, Store: store, Redis: rdb, Queue: q,
		Storage: st, Metrics: m, Health: checker,
	})
	httpServer := &http.Server{
		Addr:              cfg.HTTPAddr,
		Handler:           srv.Routes(),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       2 * time.Minute, // large uploads on slow links
		WriteTimeout:      5 * time.Minute, // batch ZIP downloads stream for a while
		IdleTimeout:       2 * time.Minute,
	}

	errCh := make(chan error, 1)
	go func() {
		logger.Info("api listening", "addr", cfg.HTTPAddr)
		if err := httpServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
		}
		close(errCh)
	}()

	select {
	case err := <-errCh:
		return fmt.Errorf("http server: %w", err)
	case <-ctx.Done():
	}

	// Graceful shutdown: report not-ready first so Nginx stops routing here,
	// then let in-flight requests finish.
	logger.Info("shutting down")
	checker.Drain()
	time.Sleep(time.Second)
	shutdownCtx, cancel := context.WithTimeout(context.Background(), cfg.ShutdownTimeout)
	defer cancel()
	if err := httpServer.Shutdown(shutdownCtx); err != nil {
		return fmt.Errorf("shutdown: %w", err)
	}
	logger.Info("shutdown complete")
	return nil
}

func bootstrap(ctx context.Context, logger *slog.Logger, done *atomic.Bool, st *storage.Storage, q *queue.Queue) {
	for {
		cctx, cancel := context.WithTimeout(ctx, 5*time.Second)
		err := st.EnsureBucket(cctx)
		if err == nil {
			err = q.EnsureGroup(cctx)
		}
		cancel()
		if err == nil {
			done.Store(true)
			logger.Info("bootstrap complete")
			return
		}
		logger.Warn("bootstrap failed, retrying", "err", err)
		select {
		case <-ctx.Done():
			return
		case <-time.After(2 * time.Second):
		}
	}
}

func probe() int {
	port := "8080"
	if addr := os.Getenv("HTTP_ADDR"); addr != "" {
		if _, p, err := net.SplitHostPort(addr); err == nil {
			port = p
		}
	}
	client := http.Client{Timeout: 2 * time.Second}
	resp, err := client.Get("http://127.0.0.1:" + port + "/healthz")
	if err != nil {
		return 1
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return 1
	}
	return 0
}
