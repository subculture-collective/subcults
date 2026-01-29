// Package middleware provides HTTP middleware components for the API server.
package middleware

import (
	"net/http"
	"strconv"
	"time"
)

// metricsResponseWriter wraps http.ResponseWriter to capture status code and response size.
type metricsResponseWriter struct {
	http.ResponseWriter
	statusCode  int
	size        int64
	wroteHeader bool
}

// WriteHeader captures the status code before writing it.
func (mrw *metricsResponseWriter) WriteHeader(code int) {
	if mrw.wroteHeader {
		return
	}
	mrw.statusCode = code
	mrw.wroteHeader = true
	mrw.ResponseWriter.WriteHeader(code)
}

// Write captures the response size and writes the data.
func (mrw *metricsResponseWriter) Write(b []byte) (int, error) {
	n, err := mrw.ResponseWriter.Write(b)
	mrw.size += int64(n)
	return n, err
}

// newMetricsResponseWriter creates a new metricsResponseWriter with default 200 status.
func newMetricsResponseWriter(w http.ResponseWriter) *metricsResponseWriter {
	return &metricsResponseWriter{
		ResponseWriter: w,
		statusCode:     http.StatusOK,
	}
}

// HTTPMetrics is a middleware that records HTTP request metrics.
// It captures duration, request/response sizes, and request counts.
// Health check endpoints (/health, /ready) are excluded from metrics to avoid cardinality issues.
func HTTPMetrics(metrics *Metrics) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Exclude health check endpoints from metrics
			if r.URL.Path == "/health" || r.URL.Path == "/ready" {
				next.ServeHTTP(w, r)
				return
			}

			start := time.Now()

			// Wrap response writer to capture status and size
			mrw := newMetricsResponseWriter(w)

			// Get request size from Content-Length header
			requestSize := int64(0)
			if contentLength := r.Header.Get("Content-Length"); contentLength != "" {
				if size, err := strconv.ParseInt(contentLength, 10, 64); err == nil {
					requestSize = size
				}
			}

			// Call the next handler
			next.ServeHTTP(mrw, r)

			// Calculate duration in seconds
			duration := time.Since(start).Seconds()

			// Record metrics
			metrics.ObserveHTTPRequest(
				r.Method,
				r.URL.Path,
				strconv.Itoa(mrw.statusCode),
				duration,
				requestSize,
				mrw.size,
			)
		})
	}
}
