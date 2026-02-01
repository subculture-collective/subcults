package api

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/onnwee/subcults/internal/middleware"
)

// TelemetryHandlers provides endpoints for frontend performance telemetry.
type TelemetryHandlers struct {
	// Future: could add a telemetry store/aggregator here
}

// NewTelemetryHandlers creates a new telemetry handler.
func NewTelemetryHandlers() *TelemetryHandlers {
	return &TelemetryHandlers{}
}

// PerformanceMetric represents a single web vitals metric from the frontend.
type PerformanceMetric struct {
	Name            string  `json:"name"`
	Value           float64 `json:"value"`
	Rating          string  `json:"rating"`
	Delta           float64 `json:"delta"`
	ID              string  `json:"id"`
	NavigationType  string  `json:"navigationType"`
	Timestamp       int64   `json:"timestamp"`
}

// TelemetryMetricsRequest represents the request payload for POST /api/telemetry/metrics.
type TelemetryMetricsRequest struct {
	Metrics   []PerformanceMetric `json:"metrics"`
	UserAgent string              `json:"userAgent"`
	URL       string              `json:"url"`
}

// PostMetrics handles POST /api/telemetry/metrics.
// Accepts web vitals performance metrics from the frontend.
func (h *TelemetryHandlers) PostMetrics(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Only accept POST method
	if r.Method != http.MethodPost {
		ctx = middleware.SetErrorCode(ctx, ErrCodeBadRequest)
		WriteError(w, ctx, http.StatusMethodNotAllowed, ErrCodeBadRequest, "Method not allowed")
		return
	}

	// Parse request body
	var req TelemetryMetricsRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		ctx = middleware.SetErrorCode(ctx, ErrCodeBadRequest)
		WriteError(w, ctx, http.StatusBadRequest, ErrCodeBadRequest, "Invalid request body")
		return
	}

	// Validate request
	if len(req.Metrics) == 0 {
		ctx = middleware.SetErrorCode(ctx, ErrCodeBadRequest)
		WriteError(w, ctx, http.StatusBadRequest, ErrCodeBadRequest, "At least one metric required")
		return
	}

	// Validate and sanitize metrics
	validMetrics := 0
	for _, metric := range req.Metrics {
		// Validate metric values
		if !isValidMetric(metric) {
			continue // Skip invalid metrics
		}
		validMetrics++

		// Sanitize URL to remove PII (query params, hash)
		sanitizedURL := sanitizeURL(req.URL)
		
		// Parse User-Agent to extract only browser family (no versions)
		browserFamily := parseBrowserFamily(req.UserAgent)

		// Log metrics (structured logging for observability)
		// In production, these would be sent to a telemetry aggregator/time-series DB
		slog.InfoContext(ctx, "performance_metric",
			"metric_name", metric.Name,
			"value", metric.Value,
			"rating", metric.Rating,
			"delta", metric.Delta,
			"navigation_type", metric.NavigationType,
			"browser_family", browserFamily,
			"url_path", sanitizedURL,
			"timestamp", time.Unix(0, metric.Timestamp*int64(time.Millisecond)),
		)
	}

	// Return success response
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusAccepted)
	if err := json.NewEncoder(w).Encode(map[string]interface{}{
		"status":           "accepted",
		"metrics_received": validMetrics,
	}); err != nil {
		slog.ErrorContext(ctx, "failed to encode telemetry response", "error", err)
	}
}

// isValidMetric validates metric values to prevent invalid/malicious data
func isValidMetric(metric PerformanceMetric) bool {
	// Value must be non-negative and reasonable (< 60 seconds)
	if metric.Value < 0 || metric.Value > 60000 {
		return false
	}

	// Rating must be one of the valid values
	validRatings := map[string]bool{
		"good":              true,
		"needs-improvement": true,
		"poor":              true,
	}
	if !validRatings[metric.Rating] {
		return false
	}

	// Timestamp should be recent (within last hour) and not in future
	now := time.Now().UnixMilli()
	oneHourAgo := now - (60 * 60 * 1000)
	if metric.Timestamp < oneHourAgo || metric.Timestamp > now {
		return false
	}

	return true
}

// sanitizeURL removes query parameters and hash to prevent PII leakage
func sanitizeURL(rawURL string) string {
	if rawURL == "" {
		return ""
	}

	parsed, err := url.Parse(rawURL)
	if err != nil {
		return "" // Return empty on parse error
	}

	// Return only the path component (no query, no fragment, no sensitive info)
	return parsed.Path
}

// parseBrowserFamily extracts browser family from User-Agent without version details
func parseBrowserFamily(userAgent string) string {
	if userAgent == "" {
		return "unknown"
	}

	ua := strings.ToLower(userAgent)

	// Simple browser detection (family only, no versions)
	switch {
	case strings.Contains(ua, "edg"):
		return "edge"
	case strings.Contains(ua, "chrome"):
		return "chrome"
	case strings.Contains(ua, "safari"):
		return "safari"
	case strings.Contains(ua, "firefox"):
		return "firefox"
	case strings.Contains(ua, "opera") || strings.Contains(ua, "opr"):
		return "opera"
	default:
		return "other"
	}
}
