// Package indexer provides filtering and processing of AT Protocol records
// for the Subcults Jetstream indexer.
package indexer

import (
	"github.com/prometheus/client_golang/prometheus"
)

// Metrics names as constants for consistency.
const (
	MetricMessagesProcessed = "indexer_messages_processed_total"
	MetricMessagesError     = "indexer_messages_error_total"
	MetricUpserts           = "indexer_upserts_total"
	MetricTrustRecompute    = "indexer_trust_recompute_total"
	MetricIngestLatency     = "indexer_ingest_latency_seconds"
)

// Metrics contains Prometheus metrics for the indexer.
// All operations are thread-safe.
type Metrics struct {
	messagesProcessed prometheus.Counter
	messagesError     prometheus.Counter
	upserts           prometheus.Counter
	trustRecompute    prometheus.Counter
	ingestLatency     prometheus.Histogram
}

// NewMetrics creates and returns a new Metrics instance with all collectors initialized.
// The metrics are not registered; call Register to register them with a registry.
func NewMetrics() *Metrics {
	return &Metrics{
		messagesProcessed: prometheus.NewCounter(prometheus.CounterOpts{
			Name: MetricMessagesProcessed,
			Help: "Total number of messages processed by the indexer",
		}),
		messagesError: prometheus.NewCounter(prometheus.CounterOpts{
			Name: MetricMessagesError,
			Help: "Total number of messages that resulted in processing errors",
		}),
		upserts: prometheus.NewCounter(prometheus.CounterOpts{
			Name: MetricUpserts,
			Help: "Total number of database upsert operations",
		}),
		trustRecompute: prometheus.NewCounter(prometheus.CounterOpts{
			Name: MetricTrustRecompute,
			Help: "Total number of trust score recomputation operations",
		}),
		ingestLatency: prometheus.NewHistogram(prometheus.HistogramOpts{
			Name:    MetricIngestLatency,
			Help:    "Histogram of message ingestion latency in seconds",
			Buckets: prometheus.DefBuckets,
		}),
	}
}

// Register registers all metrics with the given registry.
// Returns an error if registration fails.
func (m *Metrics) Register(reg prometheus.Registerer) error {
	collectors := []prometheus.Collector{
		m.messagesProcessed,
		m.messagesError,
		m.upserts,
		m.trustRecompute,
		m.ingestLatency,
	}

	for _, c := range collectors {
		if err := reg.Register(c); err != nil {
			return err
		}
	}
	return nil
}

// IncMessagesProcessed increments the messages processed counter.
func (m *Metrics) IncMessagesProcessed() {
	m.messagesProcessed.Inc()
}

// IncMessagesError increments the messages error counter.
func (m *Metrics) IncMessagesError() {
	m.messagesError.Inc()
}

// IncUpserts increments the upserts counter.
func (m *Metrics) IncUpserts() {
	m.upserts.Inc()
}

// IncTrustRecompute increments the trust recompute counter.
func (m *Metrics) IncTrustRecompute() {
	m.trustRecompute.Inc()
}

// ObserveIngestLatency records an ingestion latency sample.
func (m *Metrics) ObserveIngestLatency(seconds float64) {
	m.ingestLatency.Observe(seconds)
}

// Collectors returns all Prometheus collectors for testing.
func (m *Metrics) Collectors() []prometheus.Collector {
	return []prometheus.Collector{
		m.messagesProcessed,
		m.messagesError,
		m.upserts,
		m.trustRecompute,
		m.ingestLatency,
	}
}
