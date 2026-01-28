// Package middleware provides HTTP middleware components for the API server.
package middleware

import (
	"bytes"
	"context"
	"io"
	"log/slog"
	"net/http"
	
	"github.com/onnwee/subcults/internal/idempotency"
)

// IdempotencyKeyHeader is the HTTP header name for idempotency keys.
const IdempotencyKeyHeader = "Idempotency-Key"

// idempotencyKeyContextKey is the context key for storing the idempotency key.
type idempotencyKeyContextKey struct{}

// idempotencyResponseWriter is a custom response writer that captures the response.
type idempotencyResponseWriter struct {
	http.ResponseWriter
	statusCode int
	body       *bytes.Buffer
	written    bool
}

// newIdempotencyResponseWriter creates a new idempotency response writer.
func newIdempotencyResponseWriter(w http.ResponseWriter) *idempotencyResponseWriter {
	return &idempotencyResponseWriter{
		ResponseWriter: w,
		statusCode:     http.StatusOK,
		body:           &bytes.Buffer{},
		written:        false,
	}
}

// WriteHeader captures the status code.
func (w *idempotencyResponseWriter) WriteHeader(statusCode int) {
	if !w.written {
		w.statusCode = statusCode
		w.written = true
	}
	w.ResponseWriter.WriteHeader(statusCode)
}

// Write captures the response body.
func (w *idempotencyResponseWriter) Write(b []byte) (int, error) {
	// Always write to the actual response
	n, err := w.ResponseWriter.Write(b)
	// Also capture for idempotency storage
	w.body.Write(b)
	return n, err
}

// SetIdempotencyKey stores the idempotency key in the context.
func SetIdempotencyKey(ctx context.Context, key string) context.Context {
	return context.WithValue(ctx, idempotencyKeyContextKey{}, key)
}

// GetIdempotencyKey retrieves the idempotency key from context. Returns empty string if not present.
func GetIdempotencyKey(ctx context.Context) string {
	if key, ok := ctx.Value(idempotencyKeyContextKey{}).(string); ok {
		return key
	}
	return ""
}

// IdempotencyMiddleware returns a middleware that enforces idempotency for requests.
// It requires an Idempotency-Key header for POST requests to specified routes.
// If a duplicate key is detected, the cached response is returned.
// Otherwise, the response is cached for future duplicate requests.
func IdempotencyMiddleware(repo idempotency.Repository, routes map[string]bool) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Only apply to configured routes
			if !routes[r.URL.Path] {
				next.ServeHTTP(w, r)
				return
			}
			
			// Only apply to POST requests
			if r.Method != http.MethodPost {
				next.ServeHTTP(w, r)
				return
			}
			
			// Get idempotency key from header
			key := r.Header.Get(IdempotencyKeyHeader)
			
			// Validate key presence
			if key == "" {
				ctx := SetErrorCode(r.Context(), "missing_idempotency_key")
				WriteErrorFunc := func(w http.ResponseWriter, ctx context.Context, status int, code, message string) {
					// Update context in response writer if it supports it
					UpdateResponseContext(w, ctx)
					
					// Write error response
					w.Header().Set("Content-Type", "application/json")
					w.WriteHeader(status)
					io.WriteString(w, `{"error":"`+code+`","message":"`+message+`"}`)
				}
				WriteErrorFunc(w, ctx, http.StatusBadRequest, "missing_idempotency_key", "Idempotency-Key header is required for this request")
				return
			}
			
			// Validate key format
			if err := idempotency.ValidateKey(key); err != nil {
				var code, message string
				status := http.StatusBadRequest
				
				switch err {
				case idempotency.ErrKeyTooLong:
					code = "idempotency_key_too_long"
					message = "Idempotency-Key exceeds maximum length of 64 characters"
				default:
					code = "invalid_idempotency_key"
					message = "Invalid Idempotency-Key format"
				}
				
				ctx := SetErrorCode(r.Context(), code)
				WriteErrorFunc := func(w http.ResponseWriter, ctx context.Context, status int, code, message string) {
					UpdateResponseContext(w, ctx)
					w.Header().Set("Content-Type", "application/json")
					w.WriteHeader(status)
					io.WriteString(w, `{"error":"`+code+`","message":"`+message+`"}`)
				}
				WriteErrorFunc(w, ctx, status, code, message)
				return
			}
			
			// Store key in context for potential use by handlers
			ctx := SetIdempotencyKey(r.Context(), key)
			r = r.WithContext(ctx)
			
			// Check if key already exists
			existing, err := repo.Get(key)
			if err == nil {
				// Key exists - return cached response
				slog.InfoContext(ctx, "idempotency key found, returning cached response",
					"key", key,
					"status", existing.ResponseStatusCode,
				)
				
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(existing.ResponseStatusCode)
				io.WriteString(w, existing.ResponseBody)
				return
			}
			
			// Key not found - proceed with handler and cache response
			if err != idempotency.ErrKeyNotFound {
				// Unexpected error - log and continue without idempotency
				slog.ErrorContext(ctx, "failed to check idempotency key", "key", key, "error", err)
				next.ServeHTTP(w, r)
				return
			}
			
			// Wrap response writer to capture response
			captureWriter := newIdempotencyResponseWriter(w)
			
			// Call next handler
			next.ServeHTTP(captureWriter, r)
			
			// Only cache successful responses (2xx status codes)
			if captureWriter.statusCode >= 200 && captureWriter.statusCode < 300 {
				responseBody := captureWriter.body.String()
				responseHash := idempotency.ComputeResponseHash(responseBody)
				
				// Store the idempotency key with cached response
				record := &idempotency.IdempotencyKey{
					Key:                key,
					Method:             r.Method,
					Route:              r.URL.Path,
					ResponseHash:       responseHash,
					Status:             idempotency.StatusCompleted,
					ResponseBody:       responseBody,
					ResponseStatusCode: captureWriter.statusCode,
				}
				
				if err := repo.Store(record); err != nil {
					// Log error but don't fail the request - response already sent
					slog.ErrorContext(ctx, "failed to store idempotency key", "key", key, "error", err)
				} else {
					slog.InfoContext(ctx, "stored idempotency key", "key", key, "status", captureWriter.statusCode)
				}
			}
		})
	}
}
