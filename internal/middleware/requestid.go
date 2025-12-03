// Package middleware provides HTTP middleware components for the API server.
package middleware

import (
	"context"
	"net/http"

	"github.com/google/uuid"
)

// requestIDKey is the context key for request ID.
type requestIDKey struct{}

// RequestIDHeader is the HTTP header name for request ID.
const RequestIDHeader = "X-Request-ID"

// RequestID is a middleware that injects a request ID into the context.
// If the request already has an X-Request-ID header, it uses that value.
// Otherwise, it generates a new UUID.
func RequestID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestID := r.Header.Get(RequestIDHeader)
		if requestID == "" {
			requestID = uuid.New().String()
		}

		// Set the header in the response
		w.Header().Set(RequestIDHeader, requestID)

		// Add request ID to context
		ctx := context.WithValue(r.Context(), requestIDKey{}, requestID)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// GetRequestID returns the request ID from context. Returns empty string if not present.
func GetRequestID(ctx context.Context) string {
	if id, ok := ctx.Value(requestIDKey{}).(string); ok {
		return id
	}
	return ""
}
