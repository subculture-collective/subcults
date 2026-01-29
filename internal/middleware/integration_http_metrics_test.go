package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/prometheus/client_golang/prometheus"
)

// TestHTTPMetrics_Integration verifies the full middleware chain works correctly
func TestHTTPMetrics_Integration(t *testing.T) {
	// Create metrics
	m := NewMetrics()
	reg := prometheus.NewRegistry()
	if err := m.Register(reg); err != nil {
		t.Fatalf("Register() failed: %v", err)
	}

	// Create a simple handler that returns 200
	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	})

	// Apply the middleware
	handler := HTTPMetrics(m)(testHandler)

	// Make a request
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	// Verify response
	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusOK)
	}

	// Gather metrics
	metrics, err := reg.Gather()
	if err != nil {
		t.Fatalf("Gather() failed: %v", err)
	}

	// Verify metrics were recorded
	foundMetrics := 0
	for _, mf := range metrics {
		switch mf.GetName() {
		case MetricHTTPRequestDuration,
			MetricHTTPRequestsTotal,
			MetricHTTPRequestSizeBytes,
			MetricHTTPResponseSizeBytes:
			foundMetrics++
		}
	}

	if foundMetrics != 4 {
		t.Errorf("expected 4 HTTP metrics, found %d", foundMetrics)
	}
}

// TestHTTPMetrics_MiddlewareOrdering verifies the middleware can be composed with other middleware
func TestHTTPMetrics_MiddlewareOrdering(t *testing.T) {
	// Create metrics
	m := NewMetrics()
	reg := prometheus.NewRegistry()
	if err := m.Register(reg); err != nil {
		t.Fatalf("Register() failed: %v", err)
	}

	// Create a test handler
	called := false
	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})

	// Create a dummy middleware that adds a header
	headerMiddleware := func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("X-Test", "value")
			next.ServeHTTP(w, r)
		})
	}

	// Compose middleware: headerMiddleware -> HTTPMetrics -> handler
	handler := headerMiddleware(HTTPMetrics(m)(testHandler))

	// Make a request
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	// Verify the handler was called
	if !called {
		t.Error("handler was not called")
	}

	// Verify the header was set
	if rec.Header().Get("X-Test") != "value" {
		t.Error("header middleware did not run")
	}

	// Verify metrics were recorded
	metrics, err := reg.Gather()
	if err != nil {
		t.Fatalf("Gather() failed: %v", err)
	}

	found := false
	for _, mf := range metrics {
		if mf.GetName() == MetricHTTPRequestsTotal {
			found = true
			break
		}
	}

	if !found {
		t.Error("HTTP metrics were not recorded")
	}
}
