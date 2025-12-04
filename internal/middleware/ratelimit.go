// Package middleware provides HTTP middleware components for the API server.
package middleware

import (
	"context"
	"net"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"
)

// RateLimitConfig defines the rate limiting configuration.
type RateLimitConfig struct {
	// RequestsPerWindow is the maximum number of requests allowed per window.
	RequestsPerWindow int
	// WindowDuration is the time window for the rate limit.
	WindowDuration time.Duration
}

// DefaultGlobalLimit is the default global rate limit (100 requests per minute).
var DefaultGlobalLimit = RateLimitConfig{
	RequestsPerWindow: 100,
	WindowDuration:    time.Minute,
}

// DefaultAuthLimit is the default auth endpoint rate limit (10 requests per minute).
var DefaultAuthLimit = RateLimitConfig{
	RequestsPerWindow: 10,
	WindowDuration:    time.Minute,
}

// DefaultSearchLimit is the default search endpoint rate limit (30 requests per minute).
var DefaultSearchLimit = RateLimitConfig{
	RequestsPerWindow: 30,
	WindowDuration:    time.Minute,
}

// RateLimitStore defines the interface for rate limit state storage.
// This allows for different backends (in-memory, Redis, etc.).
type RateLimitStore interface {
	// Allow checks if a request from the given key should be allowed.
	// Returns true if allowed, false if rate limited.
	// The second return value is the number of seconds until the limit resets.
	Allow(ctx context.Context, key string, config RateLimitConfig) (allowed bool, retryAfter int)
}

// bucket represents a rate limit bucket for a single key.
type bucket struct {
	count     int
	windowEnd time.Time
}

// InMemoryRateLimitStore implements RateLimitStore using an in-memory map.
// It uses a simple fixed window counter algorithm.
// Thread-safe for concurrent access.
type InMemoryRateLimitStore struct {
	mu      sync.RWMutex
	buckets map[string]*bucket
}

// NewInMemoryRateLimitStore creates a new in-memory rate limit store.
func NewInMemoryRateLimitStore() *InMemoryRateLimitStore {
	return &InMemoryRateLimitStore{
		buckets: make(map[string]*bucket),
	}
}

// Allow checks if a request from the given key should be allowed.
// Implements the RateLimitStore interface.
func (s *InMemoryRateLimitStore) Allow(ctx context.Context, key string, config RateLimitConfig) (bool, int) {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now()

	b, exists := s.buckets[key]
	if !exists || now.After(b.windowEnd) {
		// New window or expired window
		s.buckets[key] = &bucket{
			count:     1,
			windowEnd: now.Add(config.WindowDuration),
		}
		return true, 0
	}

	// Check if we're within the limit
	if b.count < config.RequestsPerWindow {
		b.count++
		return true, 0
	}

	// Rate limited
	retryAfter := int(b.windowEnd.Sub(now).Seconds())
	if retryAfter <= 0 {
		retryAfter = 1
	}
	return false, retryAfter
}

// Cleanup removes expired buckets to prevent memory leaks.
// This should be called periodically in production.
func (s *InMemoryRateLimitStore) Cleanup() {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now()
	for key, b := range s.buckets {
		if now.After(b.windowEnd) {
			delete(s.buckets, key)
		}
	}
}

// KeyFunc extracts a rate limit key from an HTTP request.
type KeyFunc func(r *http.Request) string

// IPKeyFunc returns a KeyFunc that uses the client's IP address.
func IPKeyFunc() KeyFunc {
	return func(r *http.Request) string {
		// Check X-Forwarded-For header first (for proxied requests)
		if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
			// Use the first IP in the chain, trimming whitespace per RFC 7239
			if idx := strings.Index(xff, ","); idx != -1 {
				return strings.TrimSpace(xff[:idx])
			}
			return strings.TrimSpace(xff)
		}
		// Check X-Real-IP header
		if xri := r.Header.Get("X-Real-IP"); xri != "" {
			return strings.TrimSpace(xri)
		}
		// Fall back to RemoteAddr (strip port properly for both IPv4 and IPv6)
		host, _, err := net.SplitHostPort(r.RemoteAddr)
		if err != nil {
			// RemoteAddr might not have a port
			return r.RemoteAddr
		}
		return host
	}
}

// UserKeyFunc returns a KeyFunc that uses the authenticated user's DID if available,
// falling back to IP address.
func UserKeyFunc() KeyFunc {
	ipFunc := IPKeyFunc()
	return func(r *http.Request) string {
		if did := GetUserDID(r.Context()); did != "" {
			return "user:" + did
		}
		return "ip:" + ipFunc(r)
	}
}

// RateLimiter is a middleware that limits request rates.
// It returns HTTP 429 Too Many Requests when the limit is exceeded.
func RateLimiter(store RateLimitStore, config RateLimitConfig, keyFunc KeyFunc) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			key := keyFunc(r)
			allowed, retryAfter := store.Allow(r.Context(), key, config)

			if !allowed {
				w.Header().Set("Retry-After", strconv.Itoa(retryAfter))
				w.Header().Set("X-RateLimit-Reset", strconv.Itoa(retryAfter))
				http.Error(w, "Too Many Requests", http.StatusTooManyRequests)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}
