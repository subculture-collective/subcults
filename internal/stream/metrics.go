// Package stream provides metrics for streaming session analytics.
package stream

import (
	"github.com/prometheus/client_golang/prometheus"
)

// Metrics names as constants for consistency.
const (
	MetricStreamJoins       = "stream_joins_total"
	MetricStreamLeaves      = "stream_leaves_total"
	MetricStreamJoinLatency = "stream_join_latency_seconds"
)

// Metrics contains Prometheus metrics for streaming sessions.
// All operations are thread-safe.
type Metrics struct {
	streamJoins       prometheus.Counter
	streamLeaves      prometheus.Counter
	streamJoinLatency prometheus.Histogram
}

// NewMetrics creates and returns a new Metrics instance with all collectors initialized.
// The metrics are not registered; call Register to register them with a registry.
func NewMetrics() *Metrics {
	return &Metrics{
		streamJoins: prometheus.NewCounter(prometheus.CounterOpts{
			Name: MetricStreamJoins,
			Help: "Total number of stream join events",
		}),
		streamLeaves: prometheus.NewCounter(prometheus.CounterOpts{
			Name: MetricStreamLeaves,
			Help: "Total number of stream leave events",
		}),
		streamJoinLatency: prometheus.NewHistogram(prometheus.HistogramOpts{
			Name:    MetricStreamJoinLatency,
			Help:    "Histogram of stream join completion latency in seconds (from token issuance to first audio track subscription)",
			Buckets: []float64{0.1, 0.25, 0.5, 1.0, 2.0, 5.0, 10.0, 30.0, 60.0},
		}),
	}
}

// Register registers all metrics with the given registry.
// Returns an error if registration fails.
func (m *Metrics) Register(reg prometheus.Registerer) error {
	collectors := []prometheus.Collector{
		m.streamJoins,
		m.streamLeaves,
		m.streamJoinLatency,
	}

	for _, c := range collectors {
		if err := reg.Register(c); err != nil {
			return err
		}
	}
	return nil
}

// IncStreamJoins increments the stream joins counter.
func (m *Metrics) IncStreamJoins() {
	m.streamJoins.Inc()
}

// IncStreamLeaves increments the stream leaves counter.
func (m *Metrics) IncStreamLeaves() {
	m.streamLeaves.Inc()
}

// ObserveStreamJoinLatency records a stream join latency sample.
func (m *Metrics) ObserveStreamJoinLatency(seconds float64) {
	m.streamJoinLatency.Observe(seconds)
}

// Collectors returns all Prometheus collectors for testing.
func (m *Metrics) Collectors() []prometheus.Collector {
	return []prometheus.Collector{
		m.streamJoins,
		m.streamLeaves,
		m.streamJoinLatency,
	}
}
