// Package api provides HTTP API handlers for the Subcults API.
package api

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"time"

	"github.com/onnwee/subcults/internal/middleware"
)

// HealthChecker defines the interface for components that can be health checked.
type HealthChecker interface {
	HealthCheck(ctx context.Context) error
}

// HealthHandlers provides health and readiness check endpoints for Kubernetes probes.
type HealthHandlers struct {
	// External service checkers (optional)
	livekitChecker HealthChecker
	stripeChecker  HealthChecker

	// Database checker (optional, for when real DB is used)
	dbChecker HealthChecker

	// Metrics availability
	metricsEnabled bool
}

// HealthHandlersConfig configures the health check handlers.
type HealthHandlersConfig struct {
	LiveKitChecker HealthChecker
	StripeChecker  HealthChecker
	DBChecker      HealthChecker
	MetricsEnabled bool
}

// NewHealthHandlers creates a new health check handler.
func NewHealthHandlers(config HealthHandlersConfig) *HealthHandlers {
	return &HealthHandlers{
		livekitChecker: config.LiveKitChecker,
		stripeChecker:  config.StripeChecker,
		dbChecker:      config.DBChecker,
		metricsEnabled: config.MetricsEnabled,
	}
}

// HealthResponse represents the JSON response for health checks.
type HealthResponse struct {
	Status    string            `json:"status"`
	Checks    map[string]string `json:"checks"`
	Timestamp string            `json:"timestamp"`
}

// Health handles GET /health (liveness probe).
// Returns 200 if the application is running and can serve requests.
// This is a basic check that the process is alive.
func (h *HealthHandlers) Health(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		ctx := middleware.SetErrorCode(r.Context(), ErrCodeBadRequest)
		WriteError(w, ctx, http.StatusMethodNotAllowed, ErrCodeBadRequest, "Method not allowed")
		return
	}

	// Liveness check is simple - if we can respond, we're alive
	response := HealthResponse{
		Status:    "healthy",
		Checks:    make(map[string]string),
		Timestamp: time.Now().UTC().Format(time.RFC3339),
	}

	// Basic runtime check - we're alive if we can execute this
	response.Checks["runtime"] = "ok"

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(response); err != nil {
		slog.Error("failed to encode health response", "error", err)
	}
}

// Ready handles GET /ready (readiness probe).
// Returns 200 if the application is ready to serve traffic.
// Checks external dependencies and returns 503 if any critical service is unavailable.
func (h *HealthHandlers) Ready(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		ctx := middleware.SetErrorCode(r.Context(), ErrCodeBadRequest)
		WriteError(w, ctx, http.StatusMethodNotAllowed, ErrCodeBadRequest, "Method not allowed")
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	checks := make(map[string]string)
	healthy := true

	// Check database connectivity (if configured)
	if h.dbChecker != nil {
		if err := h.dbChecker.HealthCheck(ctx); err != nil {
			checks["database"] = "error"
			healthy = false
			slog.WarnContext(ctx, "database health check failed", "error", err)
		} else {
			checks["database"] = "ok"
		}
	} else {
		// Database not configured (using in-memory repos)
		checks["database"] = "ok"
	}

	// Check LiveKit availability (if configured)
	if h.livekitChecker != nil {
		if err := h.livekitChecker.HealthCheck(ctx); err != nil {
			checks["livekit"] = "error"
			healthy = false
			slog.WarnContext(ctx, "livekit health check failed", "error", err)
		} else {
			checks["livekit"] = "ok"
		}
	} else {
		// LiveKit not configured - this is OK, mark as not configured
		checks["livekit"] = "ok"
	}

	// Check Stripe availability (if configured)
	if h.stripeChecker != nil {
		if err := h.stripeChecker.HealthCheck(ctx); err != nil {
			checks["stripe"] = "error"
			healthy = false
			slog.WarnContext(ctx, "stripe health check failed", "error", err)
		} else {
			checks["stripe"] = "ok"
		}
	} else {
		// Stripe not configured - this is OK
		checks["stripe"] = "ok"
	}

	// Metrics are always available (Prometheus registry is always initialized)
	checks["metrics"] = "ok"

	status := "healthy"
	statusCode := http.StatusOK
	if !healthy {
		status = "unhealthy"
		statusCode = http.StatusServiceUnavailable
	}

	response := HealthResponse{
		Status:    status,
		Checks:    checks,
		Timestamp: time.Now().UTC().Format(time.RFC3339),
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	if err := json.NewEncoder(w).Encode(response); err != nil {
		slog.Error("failed to encode readiness response", "error", err)
	}
}
