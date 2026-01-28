// Package middleware provides metrics for HTTP middleware components.
package middleware

import (
	"github.com/prometheus/client_golang/prometheus"
)

// Metrics names as constants for consistency.
const (
	MetricRateLimitRequests = "rate_limit_requests_total"
	MetricRateLimitBlocked  = "rate_limit_blocked_total"
)

// Metrics contains Prometheus metrics for middleware operations.
// All operations are thread-safe.
type Metrics struct {
	rateLimitRequests *prometheus.CounterVec
	rateLimitBlocked  *prometheus.CounterVec
}

// NewMetrics creates and returns a new Metrics instance with all collectors initialized.
// The metrics are not registered; call Register to register them with a registry.
func NewMetrics() *Metrics {
	return &Metrics{
		rateLimitRequests: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: MetricRateLimitRequests,
				Help: "Total number of rate limit checks by endpoint",
			},
			[]string{"endpoint", "key_type"},
		),
		rateLimitBlocked: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: MetricRateLimitBlocked,
				Help: "Total number of rate limit violations (blocked requests) by endpoint",
			},
			[]string{"endpoint", "key_type"},
		),
	}
}

// Register registers all metrics with the given registry.
// Returns an error if registration fails.
func (m *Metrics) Register(reg prometheus.Registerer) error {
	collectors := []prometheus.Collector{
		m.rateLimitRequests,
		m.rateLimitBlocked,
	}

	for _, c := range collectors {
		if err := reg.Register(c); err != nil {
			return err
		}
	}
	return nil
}

// IncRateLimitRequests increments the rate limit requests counter.
// endpoint: The API endpoint being accessed (e.g., "/search/events")
// keyType: The type of rate limit key (e.g., "user", "ip")
func (m *Metrics) IncRateLimitRequests(endpoint, keyType string) {
	m.rateLimitRequests.WithLabelValues(endpoint, keyType).Inc()
}

// IncRateLimitBlocked increments the rate limit blocked counter.
// endpoint: The API endpoint being accessed (e.g., "/search/events")
// keyType: The type of rate limit key (e.g., "user", "ip")
func (m *Metrics) IncRateLimitBlocked(endpoint, keyType string) {
	m.rateLimitBlocked.WithLabelValues(endpoint, keyType).Inc()
}

// Collectors returns all Prometheus collectors for testing.
func (m *Metrics) Collectors() []prometheus.Collector {
	return []prometheus.Collector{
		m.rateLimitRequests,
		m.rateLimitBlocked,
	}
}
