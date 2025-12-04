// Package middleware provides HTTP middleware components for the API server.
package middleware

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"
)

// RateLimitConfig defines the rate limiting configuration.
// Valid values:
//   - RequestsPerWindow: must be > 0
//   - WindowDuration: must be > 0
type RateLimitConfig struct {
	// RequestsPerWindow is the maximum number of requests allowed per window.
	// Must be > 0.
	RequestsPerWindow int
	// WindowDuration is the time window for the rate limit.
	// Must be > 0.
	WindowDuration time.Duration
}

// Validate checks that the RateLimitConfig has valid values.
// Returns an error if RequestsPerWindow <= 0 or WindowDuration <= 0.
func (c RateLimitConfig) Validate() error {
	if c.RequestsPerWindow <= 0 {
		return fmt.Errorf("RequestsPerWindow must be > 0 (got %d)", c.RequestsPerWindow)
	}
	if c.WindowDuration <= 0 {
		return fmt.Errorf("WindowDuration must be > 0 (got %s)", c.WindowDuration)
	}
	return nil
}

// defaultGlobalLimit is the default global rate limit (100 requests per minute).
var defaultGlobalLimit = RateLimitConfig{
	RequestsPerWindow: 100,
	WindowDuration:    time.Minute,
}

// defaultAuthLimit is the default auth endpoint rate limit (10 requests per minute).
var defaultAuthLimit = RateLimitConfig{
	RequestsPerWindow: 10,
	WindowDuration:    time.Minute,
}

// defaultSearchLimit is the default search endpoint rate limit (30 requests per minute).
var defaultSearchLimit = RateLimitConfig{
	RequestsPerWindow: 30,
	WindowDuration:    time.Minute,
}

// DefaultGlobalLimit returns a copy of the default global rate limit config.
func DefaultGlobalLimit() RateLimitConfig {
	return defaultGlobalLimit
}

// DefaultAuthLimit returns a copy of the default auth endpoint rate limit config.
func DefaultAuthLimit() RateLimitConfig {
	return defaultAuthLimit
}

// DefaultSearchLimit returns a copy of the default search endpoint rate limit config.
func DefaultSearchLimit() RateLimitConfig {
	return defaultSearchLimit
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
// Recommended cleanup interval is 2-5x the longest configured WindowDuration
// to balance memory usage and overhead.
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
				// Set error code for logging middleware
				ctx := SetErrorCode(r.Context(), "rate_limit_exceeded")
				r = r.WithContext(ctx)

				w.Header().Set("Retry-After", strconv.Itoa(retryAfter))
				// X-RateLimit-Reset should be a Unix timestamp per API conventions
				resetTime := time.Now().Add(time.Duration(retryAfter) * time.Second).Unix()
				w.Header().Set("X-RateLimit-Reset", strconv.FormatInt(resetTime, 10))
				http.Error(w, "Too Many Requests", http.StatusTooManyRequests)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}
