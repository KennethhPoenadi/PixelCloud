// Package metrics defines the API's Prometheus metrics. Every series carries a
// constant `node` label; labels never contain user or job IDs (cardinality).
package metrics

import (
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

type Metrics struct {
	registry *prometheus.Registry

	requests        *prometheus.CounterVec
	duration        *prometheus.HistogramVec
	JobsEnqueued    prometheus.Counter
	QuotaRejections prometheus.Counter
}

func New(nodeID string) *Metrics {
	reg := prometheus.NewRegistry()
	r := prometheus.WrapRegistererWith(prometheus.Labels{"node": nodeID}, reg)

	m := &Metrics{
		registry: reg,
		requests: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "http_requests_total",
			Help: "HTTP requests handled, by route template and status code.",
		}, []string{"method", "route", "status"}),
		duration: prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Name:    "http_request_duration_seconds",
			Help:    "HTTP request latency by route template.",
			Buckets: []float64{.005, .01, .025, .05, .1, .25, .5, 1, 2.5, 5, 10},
		}, []string{"method", "route"}),
		JobsEnqueued: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "jobs_enqueued_total",
			Help: "Jobs pushed to the processing queue.",
		}),
		QuotaRejections: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "quota_rejections_total",
			Help: "Job requests rejected because the monthly quota was exhausted.",
		}),
	}
	r.MustRegister(m.requests, m.duration, m.JobsEnqueued, m.QuotaRejections,
		collectors.NewGoCollector(), collectors.NewProcessCollector(collectors.ProcessCollectorOpts{}))
	return m
}

// Register adds extra collectors (e.g. queue gauges) under the node label.
func (m *Metrics) Register(nodeID string, cs ...prometheus.Collector) {
	prometheus.WrapRegistererWith(prometheus.Labels{"node": nodeID}, m.registry).MustRegister(cs...)
}

func (m *Metrics) Handler() http.Handler {
	return promhttp.HandlerFor(m.registry, promhttp.HandlerOpts{})
}

// Middleware records request count and latency using chi's route template
// (e.g. /api/v1/images/{id}) rather than the raw path.
func (m *Metrics) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		ww := middleware.NewWrapResponseWriter(w, r.ProtoMajor)
		next.ServeHTTP(ww, r)

		route := "unmatched"
		if rc := chi.RouteContext(r.Context()); rc != nil && rc.RoutePattern() != "" {
			route = rc.RoutePattern()
		}
		status := ww.Status()
		if status == 0 {
			status = http.StatusOK
		}
		m.requests.WithLabelValues(r.Method, route, strconv.Itoa(status)).Inc()
		m.duration.WithLabelValues(r.Method, route).Observe(time.Since(start).Seconds())
	})
}
