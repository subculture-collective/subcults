package api

import (
	"encoding/json"
	"log/slog"
	"net/http"
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

	// Log metrics (structured logging for observability)
	// In production, these would be sent to a telemetry aggregator/time-series DB
	for _, metric := range req.Metrics {
		slog.InfoContext(ctx, "performance_metric",
			"metric_name", metric.Name,
			"value", metric.Value,
			"rating", metric.Rating,
			"delta", metric.Delta,
			"navigation_type", metric.NavigationType,
			"user_agent", req.UserAgent,
			"url", req.URL,
			"timestamp", time.Unix(0, metric.Timestamp*int64(time.Millisecond)),
		)
	}

	// Return success response
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusAccepted)
	if err := json.NewEncoder(w).Encode(map[string]interface{}{
		"status":           "accepted",
		"metrics_received": len(req.Metrics),
	}); err != nil {
		slog.ErrorContext(ctx, "failed to encode telemetry response", "error", err)
	}
}
