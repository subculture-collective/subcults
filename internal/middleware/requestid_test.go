package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestRequestID_GeneratesNewID(t *testing.T) {
	handler := RequestID(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify request ID is in context
		requestID := GetRequestID(r.Context())
		if requestID == "" {
			t.Error("expected request ID in context, got empty string")
		}
	}))

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	// Verify X-Request-ID header is set in response
	responseID := rr.Header().Get(RequestIDHeader)
	if responseID == "" {
		t.Error("expected X-Request-ID header in response, got empty string")
	}
}

func TestRequestID_UsesExistingHeader(t *testing.T) {
	existingID := "existing-request-id-123"
	var capturedID string

	handler := RequestID(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedID = GetRequestID(r.Context())
	}))

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set(RequestIDHeader, existingID)
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	// Verify existing ID is preserved
	if capturedID != existingID {
		t.Errorf("expected request ID %q, got %q", existingID, capturedID)
	}

	// Verify response header has the same ID
	responseID := rr.Header().Get(RequestIDHeader)
	if responseID != existingID {
		t.Errorf("expected response header %q, got %q", existingID, responseID)
	}
}

func TestGetRequestID_EmptyContextReturnsEmptyString(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	requestID := GetRequestID(req.Context())
	if requestID != "" {
		t.Errorf("expected empty string, got %q", requestID)
	}
}
