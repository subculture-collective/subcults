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

	"github.com/redis/go-redis/v9"
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
	// Returns three values:
	// - allowed: true if request is allowed, false if rate limited
	// - remaining: number of requests remaining in current window
	// - retryAfter: number of seconds until the limit resets (relevant when allowed=false)
	Allow(ctx context.Context, key string, config RateLimitConfig) (allowed bool, remaining int, retryAfter int)
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
func (s *InMemoryRateLimitStore) Allow(ctx context.Context, key string, config RateLimitConfig) (bool, int, int) {
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
		remaining := config.RequestsPerWindow - 1
		return true, remaining, 0
	}

	// Check if we're within the limit
	if b.count < config.RequestsPerWindow {
		b.count++
		remaining := config.RequestsPerWindow - b.count
		return true, remaining, 0
	}

	// Rate limited
	retryAfter := int(b.windowEnd.Sub(now).Seconds())
	if retryAfter <= 0 {
		retryAfter = 1
	}
	return false, 0, retryAfter
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
// It also sets X-RateLimit-* headers to indicate quota status.
// Rate limit violations are logged via the logging middleware through error codes.
// If metrics is provided, rate limit events are tracked for observability.
func RateLimiter(store RateLimitStore, config RateLimitConfig, keyFunc KeyFunc, metrics *Metrics) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			key := keyFunc(r)
			allowed, remaining, retryAfter := store.Allow(r.Context(), key, config)

			// Determine key type for metrics
			keyType := "ip"
			if strings.HasPrefix(key, "user:") {
				keyType = "user"
			}

			// Track rate limit request in metrics
			if metrics != nil {
				metrics.IncRateLimitRequests(r.URL.Path, keyType)
			}

			// Set rate limit headers
			w.Header().Set("X-RateLimit-Limit", strconv.Itoa(config.RequestsPerWindow))
			w.Header().Set("X-RateLimit-Remaining", strconv.Itoa(remaining))

			if !allowed {
				// Track rate limit violation in metrics
				if metrics != nil {
					metrics.IncRateLimitBlocked(r.URL.Path, keyType)
				}

				// Set error code for logging middleware
				// The logging middleware will automatically log this with error_code="rate_limit_exceeded"
				ctx := SetErrorCode(r.Context(), "rate_limit_exceeded")
				
				// Store rate limit details in context for logging
				ctx = SetRateLimitKey(ctx, key)
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

// RedisRateLimitStore implements RateLimitStore using Redis with a token bucket algorithm.
// It uses a sliding window counter approach for accurate rate limiting.
// Thread-safe and suitable for distributed systems.
type RedisRateLimitStore struct {
	client  *redis.Client
	metrics *Metrics
}

// NewRedisRateLimitStore creates a new Redis-backed rate limit store.
func NewRedisRateLimitStore(client *redis.Client) *RedisRateLimitStore {
	return &RedisRateLimitStore{
		client:  client,
		metrics: nil,
	}
}

// NewRedisRateLimitStoreWithMetrics creates a new Redis-backed rate limit store with metrics.
func NewRedisRateLimitStoreWithMetrics(client *redis.Client, metrics *Metrics) *RedisRateLimitStore {
	return &RedisRateLimitStore{
		client:  client,
		metrics: metrics,
	}
}

// Allow checks if a request from the given key should be allowed using Redis.
// Implements the RateLimitStore interface with a sliding window algorithm.
func (s *RedisRateLimitStore) Allow(ctx context.Context, key string, config RateLimitConfig) (bool, int, int) {
	// Use a Lua script for atomic operations
	// This implements a sliding window counter algorithm
	luaScript := `
		local key = KEYS[1]
		local limit = tonumber(ARGV[1])
		local window = tonumber(ARGV[2])
		local now = tonumber(ARGV[3])
		
		-- Remove old entries outside the window
		redis.call('ZREMRANGEBYSCORE', key, '-inf', now - window)
		
		-- Count current requests in window
		local current = redis.call('ZCARD', key)
		
		if current < limit then
			-- Use a per-key sequence to ensure unique members for concurrent requests
			local seqKey = key .. ':seq'
			local seq = redis.call('INCR', seqKey)
			redis.call('EXPIRE', seqKey, window + 10)
			local member = tostring(now) .. '-' .. tostring(seq)
			-- Add current request with timestamp as score and unique member
			redis.call('ZADD', key, now, member)
			-- Set expiry on the key (window duration + buffer)
			redis.call('EXPIRE', key, window + 10)
			-- Return: allowed=1, remaining=limit-current-1
			return {1, limit - current - 1, 0}
		else
			-- Get the oldest request timestamp to calculate retry-after
			local oldest = redis.call('ZRANGE', key, 0, 0, 'WITHSCORES')
			local retryAfter = math.ceil((tonumber(oldest[2]) + window) - now)
			if retryAfter < 1 then
				retryAfter = 1
			end
			-- Return: allowed=0, remaining=0, retryAfter
			return {0, 0, retryAfter}
		end
	`

	now := time.Now().Unix()
	windowSeconds := int64(config.WindowDuration.Seconds())

	result, err := s.client.Eval(ctx, luaScript, []string{key}, config.RequestsPerWindow, windowSeconds, now).Result()
	if err != nil {
		// Track Redis error in metrics if available
		if s.metrics != nil {
			s.metrics.IncRateLimitRedisErrors()
		}
		// On Redis error, fail open (allow request)
		// This prevents Redis outages from taking down the entire API
		return true, config.RequestsPerWindow, 0
	}

	// Parse result from Lua script with safe type assertions
	resultSlice, ok := result.([]interface{})
	if !ok || len(resultSlice) != 3 {
		// Track parsing error in metrics if available
		if s.metrics != nil {
			s.metrics.IncRateLimitRedisErrors()
		}
		// Invalid result format, fail open
		return true, config.RequestsPerWindow, 0
	}

	allowedVal, ok := resultSlice[0].(int64)
	if !ok {
		// Track type assertion error in metrics if available
		if s.metrics != nil {
			s.metrics.IncRateLimitRedisErrors()
		}
		// Unexpected type for allowed flag, fail open
		return true, config.RequestsPerWindow, 0
	}
	remainingVal, ok := resultSlice[1].(int64)
	if !ok {
		// Track type assertion error in metrics if available
		if s.metrics != nil {
			s.metrics.IncRateLimitRedisErrors()
		}
		// Unexpected type for remaining count, fail open
		return true, config.RequestsPerWindow, 0
	}
	retryAfterVal, ok := resultSlice[2].(int64)
	if !ok {
		// Track type assertion error in metrics if available
		if s.metrics != nil {
			s.metrics.IncRateLimitRedisErrors()
		}
		// Unexpected type for retry-after, fail open
		return true, config.RequestsPerWindow, 0
	}

	allowed := allowedVal == 1
	remaining := int(remainingVal)
	retryAfter := int(retryAfterVal)

	return allowed, remaining, retryAfter
}
