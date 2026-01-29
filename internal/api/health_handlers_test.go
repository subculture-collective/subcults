package api

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// mockHealthChecker is a mock implementation of HealthChecker for testing.
type mockHealthChecker struct {
	shouldFail bool
	err        error
}

func (m *mockHealthChecker) HealthCheck(ctx context.Context) error {
	if m.shouldFail {
		if m.err != nil {
			return m.err
		}
		return errors.New("health check failed")
	}
	return nil
}

// TestHealth_Success tests the basic health check endpoint.
func TestHealth_Success(t *testing.T) {
	handlers := NewHealthHandlers(HealthHandlersConfig{
		MetricsEnabled: true,
	})

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	w := httptest.NewRecorder()

	handlers.Health(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	var response HealthResponse
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if response.Status != "healthy" {
		t.Errorf("expected status 'healthy', got %s", response.Status)
	}

	if response.Checks["runtime"] != "ok" {
		t.Errorf("expected runtime check to be 'ok', got %s", response.Checks["runtime"])
	}

	if response.Timestamp == "" {
		t.Error("expected timestamp to be set")
	}

	// Verify timestamp is valid RFC3339
	if _, err := time.Parse(time.RFC3339, response.Timestamp); err != nil {
		t.Errorf("timestamp is not valid RFC3339: %v", err)
	}
}

// TestHealth_MethodNotAllowed tests that non-GET requests are rejected.
func TestHealth_MethodNotAllowed(t *testing.T) {
	handlers := NewHealthHandlers(HealthHandlersConfig{})

	req := httptest.NewRequest(http.MethodPost, "/health", nil)
	w := httptest.NewRecorder()

	handlers.Health(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("expected status 405, got %d", w.Code)
	}
}

// TestReady_AllHealthy tests readiness when all services are healthy.
func TestReady_AllHealthy(t *testing.T) {
	dbChecker := &mockHealthChecker{shouldFail: false}
	livekitChecker := &mockHealthChecker{shouldFail: false}
	stripeChecker := &mockHealthChecker{shouldFail: false}

	handlers := NewHealthHandlers(HealthHandlersConfig{
		DBChecker:      dbChecker,
		LiveKitChecker: livekitChecker,
		StripeChecker:  stripeChecker,
		MetricsEnabled: true,
	})

	req := httptest.NewRequest(http.MethodGet, "/ready", nil)
	w := httptest.NewRecorder()

	handlers.Ready(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	var response HealthResponse
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if response.Status != "healthy" {
		t.Errorf("expected status 'healthy', got %s", response.Status)
	}

	expectedChecks := map[string]string{
		"database": "ok",
		"livekit":  "ok",
		"stripe":   "ok",
		"metrics":  "ok",
	}

	for check, expectedStatus := range expectedChecks {
		if response.Checks[check] != expectedStatus {
			t.Errorf("expected %s check to be %s, got %s", check, expectedStatus, response.Checks[check])
		}
	}
}

// TestReady_DatabaseUnhealthy tests readiness when database is unhealthy.
func TestReady_DatabaseUnhealthy(t *testing.T) {
	dbChecker := &mockHealthChecker{shouldFail: true}
	livekitChecker := &mockHealthChecker{shouldFail: false}
	stripeChecker := &mockHealthChecker{shouldFail: false}

	handlers := NewHealthHandlers(HealthHandlersConfig{
		DBChecker:      dbChecker,
		LiveKitChecker: livekitChecker,
		StripeChecker:  stripeChecker,
		MetricsEnabled: true,
	})

	req := httptest.NewRequest(http.MethodGet, "/ready", nil)
	w := httptest.NewRecorder()

	handlers.Ready(w, req)

	if w.Code != http.StatusServiceUnavailable {
		t.Errorf("expected status 503, got %d", w.Code)
	}

	var response HealthResponse
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if response.Status != "unhealthy" {
		t.Errorf("expected status 'unhealthy', got %s", response.Status)
	}

	if response.Checks["database"] != "error" {
		t.Errorf("expected database check to be 'error', got %s", response.Checks["database"])
	}

	// Other checks should still be ok
	if response.Checks["livekit"] != "ok" {
		t.Errorf("expected livekit check to be 'ok', got %s", response.Checks["livekit"])
	}
	if response.Checks["stripe"] != "ok" {
		t.Errorf("expected stripe check to be 'ok', got %s", response.Checks["stripe"])
	}
}

// TestReady_LiveKitUnhealthy tests readiness when LiveKit is unhealthy.
func TestReady_LiveKitUnhealthy(t *testing.T) {
	dbChecker := &mockHealthChecker{shouldFail: false}
	livekitChecker := &mockHealthChecker{shouldFail: true}
	stripeChecker := &mockHealthChecker{shouldFail: false}

	handlers := NewHealthHandlers(HealthHandlersConfig{
		DBChecker:      dbChecker,
		LiveKitChecker: livekitChecker,
		StripeChecker:  stripeChecker,
		MetricsEnabled: true,
	})

	req := httptest.NewRequest(http.MethodGet, "/ready", nil)
	w := httptest.NewRecorder()

	handlers.Ready(w, req)

	if w.Code != http.StatusServiceUnavailable {
		t.Errorf("expected status 503, got %d", w.Code)
	}

	var response HealthResponse
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if response.Status != "unhealthy" {
		t.Errorf("expected status 'unhealthy', got %s", response.Status)
	}

	if response.Checks["livekit"] != "error" {
		t.Errorf("expected livekit check to be 'error', got %s", response.Checks["livekit"])
	}
}

// TestReady_StripeUnhealthy tests readiness when Stripe is unhealthy.
func TestReady_StripeUnhealthy(t *testing.T) {
	dbChecker := &mockHealthChecker{shouldFail: false}
	livekitChecker := &mockHealthChecker{shouldFail: false}
	stripeChecker := &mockHealthChecker{shouldFail: true}

	handlers := NewHealthHandlers(HealthHandlersConfig{
		DBChecker:      dbChecker,
		LiveKitChecker: livekitChecker,
		StripeChecker:  stripeChecker,
		MetricsEnabled: true,
	})

	req := httptest.NewRequest(http.MethodGet, "/ready", nil)
	w := httptest.NewRecorder()

	handlers.Ready(w, req)

	if w.Code != http.StatusServiceUnavailable {
		t.Errorf("expected status 503, got %d", w.Code)
	}

	var response HealthResponse
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if response.Status != "unhealthy" {
		t.Errorf("expected status 'unhealthy', got %s", response.Status)
	}

	if response.Checks["stripe"] != "error" {
		t.Errorf("expected stripe check to be 'error', got %s", response.Checks["stripe"])
	}
}

// TestReady_MultipleUnhealthy tests readiness when multiple services are unhealthy.
func TestReady_MultipleUnhealthy(t *testing.T) {
	dbChecker := &mockHealthChecker{shouldFail: true}
	livekitChecker := &mockHealthChecker{shouldFail: true}
	stripeChecker := &mockHealthChecker{shouldFail: false}

	handlers := NewHealthHandlers(HealthHandlersConfig{
		DBChecker:      dbChecker,
		LiveKitChecker: livekitChecker,
		StripeChecker:  stripeChecker,
		MetricsEnabled: true,
	})

	req := httptest.NewRequest(http.MethodGet, "/ready", nil)
	w := httptest.NewRecorder()

	handlers.Ready(w, req)

	if w.Code != http.StatusServiceUnavailable {
		t.Errorf("expected status 503, got %d", w.Code)
	}

	var response HealthResponse
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if response.Status != "unhealthy" {
		t.Errorf("expected status 'unhealthy', got %s", response.Status)
	}

	if response.Checks["database"] != "error" {
		t.Errorf("expected database check to be 'error', got %s", response.Checks["database"])
	}
	if response.Checks["livekit"] != "error" {
		t.Errorf("expected livekit check to be 'error', got %s", response.Checks["livekit"])
	}
	if response.Checks["stripe"] != "ok" {
		t.Errorf("expected stripe check to be 'ok', got %s", response.Checks["stripe"])
	}
}

// TestReady_NoCheckers tests readiness when no external checkers are configured.
func TestReady_NoCheckers(t *testing.T) {
	handlers := NewHealthHandlers(HealthHandlersConfig{
		MetricsEnabled: true,
	})

	req := httptest.NewRequest(http.MethodGet, "/ready", nil)
	w := httptest.NewRecorder()

	handlers.Ready(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	var response HealthResponse
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if response.Status != "healthy" {
		t.Errorf("expected status 'healthy', got %s", response.Status)
	}

	// All checks should be ok when not configured
	if response.Checks["database"] != "ok" {
		t.Errorf("expected database check to be 'ok', got %s", response.Checks["database"])
	}
	if response.Checks["livekit"] != "ok" {
		t.Errorf("expected livekit check to be 'ok', got %s", response.Checks["livekit"])
	}
	if response.Checks["stripe"] != "ok" {
		t.Errorf("expected stripe check to be 'ok', got %s", response.Checks["stripe"])
	}
	if response.Checks["metrics"] != "ok" {
		t.Errorf("expected metrics check to be 'ok', got %s", response.Checks["metrics"])
	}
}

// TestReady_MethodNotAllowed tests that non-GET requests are rejected.
func TestReady_MethodNotAllowed(t *testing.T) {
	handlers := NewHealthHandlers(HealthHandlersConfig{})

	req := httptest.NewRequest(http.MethodPost, "/ready", nil)
	w := httptest.NewRecorder()

	handlers.Ready(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("expected status 405, got %d", w.Code)
	}
}

// TestReady_ContentType tests that the response has correct Content-Type.
func TestReady_ContentType(t *testing.T) {
	handlers := NewHealthHandlers(HealthHandlersConfig{})

	req := httptest.NewRequest(http.MethodGet, "/ready", nil)
	w := httptest.NewRecorder()

	handlers.Ready(w, req)

	contentType := w.Header().Get("Content-Type")
	if contentType != "application/json" {
		t.Errorf("expected Content-Type 'application/json', got %s", contentType)
	}
}

// TestHealth_ContentType tests that the health response has correct Content-Type.
func TestHealth_ContentType(t *testing.T) {
	handlers := NewHealthHandlers(HealthHandlersConfig{})

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	w := httptest.NewRecorder()

	handlers.Health(w, req)

	contentType := w.Header().Get("Content-Type")
	if contentType != "application/json" {
		t.Errorf("expected Content-Type 'application/json', got %s", contentType)
	}
}
