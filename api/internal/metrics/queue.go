package metrics

import (
	"context"
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

// QueueStatsFunc returns (entries in the stream, entries delivered but not acked).
type QueueStatsFunc func(ctx context.Context) (length, pending int64, err error)

// queueCollector reads the queue depth from Redis at scrape time.
type queueCollector struct {
	stats   QueueStatsFunc
	length  *prometheus.Desc
	pending *prometheus.Desc
	up      *prometheus.Desc
}

func NewQueueCollector(stats QueueStatsFunc) prometheus.Collector {
	return &queueCollector{
		stats:   stats,
		length:  prometheus.NewDesc("job_queue_length", "Jobs waiting or in flight in the Redis stream.", nil, nil),
		pending: prometheus.NewDesc("job_queue_pending", "Jobs delivered to a worker but not yet acknowledged.", nil, nil),
		up:      prometheus.NewDesc("job_queue_up", "1 if the queue could be read at scrape time.", nil, nil),
	}
}

func (c *queueCollector) Describe(ch chan<- *prometheus.Desc) {
	ch <- c.length
	ch <- c.pending
	ch <- c.up
}

func (c *queueCollector) Collect(ch chan<- prometheus.Metric) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	length, pending, err := c.stats(ctx)
	if err != nil {
		ch <- prometheus.MustNewConstMetric(c.up, prometheus.GaugeValue, 0)
		return
	}
	ch <- prometheus.MustNewConstMetric(c.up, prometheus.GaugeValue, 1)
	ch <- prometheus.MustNewConstMetric(c.length, prometheus.GaugeValue, float64(length))
	ch <- prometheus.MustNewConstMetric(c.pending, prometheus.GaugeValue, float64(pending))
}
