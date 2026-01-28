package middleware

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strconv"
	"sync"
	"testing"
	"time"
)

func TestInMemoryRateLimitStore_Allow(t *testing.T) {
	tests := []struct {
		name           string
		requestCount   int
		limit          int
		windowDuration time.Duration
		wantAllowed    []bool
	}{
		{
			name:           "allows requests under limit",
			requestCount:   3,
			limit:          5,
			windowDuration: time.Minute,
			wantAllowed:    []bool{true, true, true},
		},
		{
			name:           "blocks requests at limit",
			requestCount:   6,
			limit:          5,
			windowDuration: time.Minute,
			wantAllowed:    []bool{true, true, true, true, true, false},
		},
		{
			name:           "single request limit",
			requestCount:   3,
			limit:          1,
			windowDuration: time.Minute,
			wantAllowed:    []bool{true, false, false},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store := NewInMemoryRateLimitStore()
			config := RateLimitConfig{
				RequestsPerWindow: tt.limit,
				WindowDuration:    tt.windowDuration,
			}
			ctx := context.Background()

			for i := 0; i < tt.requestCount; i++ {
				allowed, _, _ := store.Allow(ctx, "test-key", config)
				if allowed != tt.wantAllowed[i] {
					t.Errorf("request %d: got allowed=%v, want %v", i+1, allowed, tt.wantAllowed[i])
				}
			}
		})
	}
}

func TestInMemoryRateLimitStore_RetryAfter(t *testing.T) {
	store := NewInMemoryRateLimitStore()
	config := RateLimitConfig{
		RequestsPerWindow: 1,
		WindowDuration:    10 * time.Second,
	}
	ctx := context.Background()

	// First request should be allowed
	allowed, remaining, retryAfter := store.Allow(ctx, "test-key", config)
	if !allowed {
		t.Error("first request should be allowed")
	}
	if remaining != 0 {
		t.Errorf("first request remaining should be 0 (limit=1), got %d", remaining)
	}
	if retryAfter != 0 {
		t.Errorf("first request retryAfter should be 0, got %d", retryAfter)
	}

	// Second request should be blocked with retryAfter
	allowed, remaining, retryAfter = store.Allow(ctx, "test-key", config)
	if allowed {
		t.Error("second request should be blocked")
	}
	if remaining != 0 {
		t.Errorf("second request remaining should be 0 when blocked, got %d", remaining)
	}
	if retryAfter <= 0 || retryAfter > 10 {
		t.Errorf("retryAfter should be between 1 and 10, got %d", retryAfter)
	}
}

func TestInMemoryRateLimitStore_DifferentKeys(t *testing.T) {
	store := NewInMemoryRateLimitStore()
	config := RateLimitConfig{
		RequestsPerWindow: 1,
		WindowDuration:    time.Minute,
	}
	ctx := context.Background()

	// Each key gets its own bucket
	allowed1, _, _ := store.Allow(ctx, "key1", config)
	allowed2, _, _ := store.Allow(ctx, "key2", config)

	if !allowed1 || !allowed2 {
		t.Error("different keys should each be allowed their own requests")
	}

	// Now both should be blocked
	blocked1, _, _ := store.Allow(ctx, "key1", config)
	blocked2, _, _ := store.Allow(ctx, "key2", config)

	if blocked1 || blocked2 {
		t.Error("both keys should now be blocked")
	}
}

func TestInMemoryRateLimitStore_WindowExpiry(t *testing.T) {
	store := NewInMemoryRateLimitStore()
	config := RateLimitConfig{
		RequestsPerWindow: 1,
		WindowDuration:    50 * time.Millisecond,
	}
	ctx := context.Background()

	// Use up the limit
	allowed, _, _ := store.Allow(ctx, "test-key", config)
	if !allowed {
		t.Error("first request should be allowed")
	}

	// Should be blocked
	allowed, _, _ = store.Allow(ctx, "test-key", config)
	if allowed {
		t.Error("second request should be blocked")
	}

	// Wait for window to expire
	time.Sleep(60 * time.Millisecond)

	// Should be allowed again
	allowed, _, _ = store.Allow(ctx, "test-key", config)
	if !allowed {
		t.Error("request after window expiry should be allowed")
	}
}

func TestInMemoryRateLimitStore_Concurrency(t *testing.T) {
	store := NewInMemoryRateLimitStore()
	config := RateLimitConfig{
		RequestsPerWindow: 100,
		WindowDuration:    time.Minute,
	}
	ctx := context.Background()

	var wg sync.WaitGroup
	var allowedCount int
	var mu sync.Mutex

	// Simulate 200 concurrent requests
	for i := 0; i < 200; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			allowed, _, _ := store.Allow(ctx, "concurrent-key", config)
			if allowed {
				mu.Lock()
				allowedCount++
				mu.Unlock()
			}
		}()
	}

	wg.Wait()

	// Exactly 100 should be allowed
	if allowedCount != 100 {
		t.Errorf("expected 100 allowed requests, got %d", allowedCount)
	}
}

func TestInMemoryRateLimitStore_Cleanup(t *testing.T) {
	store := NewInMemoryRateLimitStore()
	config := RateLimitConfig{
		RequestsPerWindow: 1,
		WindowDuration:    50 * time.Millisecond,
	}
	ctx := context.Background()

	// Create some buckets and use up their limits
	_, _, _ = store.Allow(ctx, "key1", config)
	_, _, _ = store.Allow(ctx, "key2", config)

	// Verify they are blocked (limits used up)
	allowed1, _, _ := store.Allow(ctx, "key1", config)
	allowed2, _, _ := store.Allow(ctx, "key2", config)
	if allowed1 || allowed2 {
		t.Error("requests should be blocked before cleanup")
	}

	// Wait for windows to expire
	time.Sleep(60 * time.Millisecond)

	// Cleanup should remove expired buckets
	store.Cleanup()

	// After cleanup, new requests should be allowed (buckets were removed)
	allowed1, _, _ = store.Allow(ctx, "key1", config)
	allowed2, _, _ = store.Allow(ctx, "key2", config)
	if !allowed1 || !allowed2 {
		t.Errorf("expected new requests to be allowed after cleanup, got allowed1=%v allowed2=%v", allowed1, allowed2)
	}
}

func TestIPKeyFunc(t *testing.T) {
	keyFunc := IPKeyFunc()

	tests := []struct {
		name          string
		remoteAddr    string
		xForwardedFor string
		xRealIP       string
		wantKey       string
	}{
		{
			name:       "uses RemoteAddr",
			remoteAddr: "192.168.1.1:12345",
			wantKey:    "192.168.1.1",
		},
		{
			name:       "uses RemoteAddr without port",
			remoteAddr: "192.168.1.1",
			wantKey:    "192.168.1.1",
		},
		{
			name:          "prefers X-Forwarded-For",
			remoteAddr:    "10.0.0.1:12345",
			xForwardedFor: "203.0.113.50",
			wantKey:       "203.0.113.50",
		},
		{
			name:          "uses first IP from X-Forwarded-For chain",
			remoteAddr:    "10.0.0.1:12345",
			xForwardedFor: "203.0.113.50, 198.51.100.1, 10.0.0.1",
			wantKey:       "203.0.113.50",
		},
		{
			name:       "prefers X-Real-IP over RemoteAddr",
			remoteAddr: "10.0.0.1:12345",
			xRealIP:    "203.0.113.50",
			wantKey:    "203.0.113.50",
		},
		{
			name:          "prefers X-Forwarded-For over X-Real-IP",
			remoteAddr:    "10.0.0.1:12345",
			xForwardedFor: "203.0.113.50",
			xRealIP:       "198.51.100.1",
			wantKey:       "203.0.113.50",
		},
		{
			name:       "handles IPv6 RemoteAddr",
			remoteAddr: "[::1]:12345",
			wantKey:    "::1",
		},
		{
			name:          "trims whitespace in X-Forwarded-For chain",
			remoteAddr:    "10.0.0.1:12345",
			xForwardedFor: "  203.0.113.50  ,  198.51.100.1  ",
			wantKey:       "203.0.113.50",
		},
		{
			name:          "trims whitespace in single X-Forwarded-For",
			remoteAddr:    "10.0.0.1:12345",
			xForwardedFor: "  203.0.113.50  ",
			wantKey:       "203.0.113.50",
		},
		{
			name:       "trims whitespace in X-Real-IP",
			remoteAddr: "10.0.0.1:12345",
			xRealIP:    "  203.0.113.50  ",
			wantKey:    "203.0.113.50",
		},
		{
			name:       "handles IPv6 RemoteAddr full",
			remoteAddr: "[2001:db8::1]:8080",
			wantKey:    "2001:db8::1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/test", nil)
			req.RemoteAddr = tt.remoteAddr
			if tt.xForwardedFor != "" {
				req.Header.Set("X-Forwarded-For", tt.xForwardedFor)
			}
			if tt.xRealIP != "" {
				req.Header.Set("X-Real-IP", tt.xRealIP)
			}

			got := keyFunc(req)
			if got != tt.wantKey {
				t.Errorf("IPKeyFunc() = %q, want %q", got, tt.wantKey)
			}
		})
	}
}

func TestUserKeyFunc(t *testing.T) {
	keyFunc := UserKeyFunc()

	tests := []struct {
		name       string
		remoteAddr string
		userDID    string
		wantKey    string
	}{
		{
			name:       "uses IP when no user",
			remoteAddr: "192.168.1.1:12345",
			wantKey:    "ip:192.168.1.1",
		},
		{
			name:       "uses user DID when authenticated",
			remoteAddr: "192.168.1.1:12345",
			userDID:    "did:web:example.com:user123",
			wantKey:    "user:did:web:example.com:user123",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/test", nil)
			req.RemoteAddr = tt.remoteAddr

			if tt.userDID != "" {
				ctx := SetUserDID(req.Context(), tt.userDID)
				req = req.WithContext(ctx)
			}

			got := keyFunc(req)
			if got != tt.wantKey {
				t.Errorf("UserKeyFunc() = %q, want %q", got, tt.wantKey)
			}
		})
	}
}

func TestRateLimiter_AllowsNormalTraffic(t *testing.T) {
	store := NewInMemoryRateLimitStore()
	config := RateLimitConfig{
		RequestsPerWindow: 100,
		WindowDuration:    time.Minute,
	}

	handler := RateLimiter(store, config, IPKeyFunc(), nil)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("OK"))
	}))

	// Send 50 requests (50% of limit) - all should succeed
	for i := 0; i < 50; i++ {
		req := httptest.NewRequest(http.MethodGet, "/api/test", nil)
		req.RemoteAddr = "192.168.1.1:12345"
		rr := httptest.NewRecorder()

		handler.ServeHTTP(rr, req)

		if rr.Code != http.StatusOK {
			t.Errorf("request %d: got status %d, want %d", i+1, rr.Code, http.StatusOK)
		}
	}
}

func TestRateLimiter_BlocksExcessiveTraffic(t *testing.T) {
	store := NewInMemoryRateLimitStore()
	config := RateLimitConfig{
		RequestsPerWindow: 10,
		WindowDuration:    time.Minute,
	}

	handler := RateLimiter(store, config, IPKeyFunc(), nil)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	// Send 15 requests - first 10 should succeed, next 5 should be blocked
	for i := 0; i < 15; i++ {
		req := httptest.NewRequest(http.MethodGet, "/api/test", nil)
		req.RemoteAddr = "192.168.1.1:12345"
		rr := httptest.NewRecorder()

		handler.ServeHTTP(rr, req)

		if i < 10 {
			if rr.Code != http.StatusOK {
				t.Errorf("request %d: got status %d, want %d (should be allowed)", i+1, rr.Code, http.StatusOK)
			}
		} else {
			if rr.Code != http.StatusTooManyRequests {
				t.Errorf("request %d: got status %d, want %d (should be blocked)", i+1, rr.Code, http.StatusTooManyRequests)
			}
		}
	}
}

func TestRateLimiter_ReturnsRetryAfterHeader(t *testing.T) {
	store := NewInMemoryRateLimitStore()
	config := RateLimitConfig{
		RequestsPerWindow: 1,
		WindowDuration:    30 * time.Second,
	}

	handler := RateLimiter(store, config, IPKeyFunc(), nil)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	// First request allowed
	req1 := httptest.NewRequest(http.MethodGet, "/api/test", nil)
	req1.RemoteAddr = "192.168.1.1:12345"
	rr1 := httptest.NewRecorder()
	handler.ServeHTTP(rr1, req1)

	if rr1.Code != http.StatusOK {
		t.Errorf("first request: got status %d, want %d", rr1.Code, http.StatusOK)
	}

	// Second request blocked with Retry-After header
	req2 := httptest.NewRequest(http.MethodGet, "/api/test", nil)
	req2.RemoteAddr = "192.168.1.1:12345"
	rr2 := httptest.NewRecorder()
	handler.ServeHTTP(rr2, req2)

	if rr2.Code != http.StatusTooManyRequests {
		t.Errorf("second request: got status %d, want %d", rr2.Code, http.StatusTooManyRequests)
	}

	retryAfter := rr2.Header().Get("Retry-After")
	if retryAfter == "" {
		t.Error("expected Retry-After header to be set")
	}

	retryAfterInt, err := strconv.Atoi(retryAfter)
	if err != nil {
		t.Errorf("Retry-After header should be an integer: %v", err)
	}
	if retryAfterInt <= 0 || retryAfterInt > 30 {
		t.Errorf("Retry-After should be between 1 and 30, got %d", retryAfterInt)
	}

	// Check X-RateLimit-Reset header is a Unix timestamp
	resetHeader := rr2.Header().Get("X-RateLimit-Reset")
	if resetHeader == "" {
		t.Error("expected X-RateLimit-Reset header to be set")
	}

	// Verify it's a valid Unix timestamp in the future
	resetTime, err := strconv.ParseInt(resetHeader, 10, 64)
	if err != nil {
		t.Errorf("X-RateLimit-Reset should be a Unix timestamp: %v", err)
	}
	now := time.Now().Unix()
	if resetTime <= now || resetTime > now+30 {
		t.Errorf("X-RateLimit-Reset should be a future timestamp within 30 seconds, got %d (now: %d)", resetTime, now)
	}
}

func TestRateLimiter_DifferentClientsIndependent(t *testing.T) {
	store := NewInMemoryRateLimitStore()
	config := RateLimitConfig{
		RequestsPerWindow: 5,
		WindowDuration:    time.Minute,
	}

	handler := RateLimiter(store, config, IPKeyFunc(), nil)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	// Client 1 uses all their requests
	for i := 0; i < 5; i++ {
		req := httptest.NewRequest(http.MethodGet, "/api/test", nil)
		req.RemoteAddr = "192.168.1.1:12345"
		rr := httptest.NewRecorder()
		handler.ServeHTTP(rr, req)

		if rr.Code != http.StatusOK {
			t.Errorf("client1 request %d should be allowed", i+1)
		}
	}

	// Client 1 is now blocked
	req := httptest.NewRequest(http.MethodGet, "/api/test", nil)
	req.RemoteAddr = "192.168.1.1:12345"
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)
	if rr.Code != http.StatusTooManyRequests {
		t.Error("client1 should be blocked")
	}

	// Client 2 should still be able to make requests
	for i := 0; i < 5; i++ {
		req := httptest.NewRequest(http.MethodGet, "/api/test", nil)
		req.RemoteAddr = "192.168.1.2:12345"
		rr := httptest.NewRecorder()
		handler.ServeHTTP(rr, req)

		if rr.Code != http.StatusOK {
			t.Errorf("client2 request %d should be allowed", i+1)
		}
	}
}

func TestRateLimiter_BurstSimulation(t *testing.T) {
	store := NewInMemoryRateLimitStore()
	config := RateLimitConfig{
		RequestsPerWindow: 10,
		WindowDuration:    time.Minute,
	}

	handler := RateLimiter(store, config, IPKeyFunc(), nil)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	var allowedCount, blockedCount int

	// Simulate a burst of 20 requests
	for i := 0; i < 20; i++ {
		req := httptest.NewRequest(http.MethodGet, "/api/test", nil)
		req.RemoteAddr = "192.168.1.1:12345"
		rr := httptest.NewRecorder()

		handler.ServeHTTP(rr, req)

		if rr.Code == http.StatusOK {
			allowedCount++
		} else if rr.Code == http.StatusTooManyRequests {
			blockedCount++
		}
	}

	if allowedCount != 10 {
		t.Errorf("expected 10 allowed requests, got %d", allowedCount)
	}
	if blockedCount != 10 {
		t.Errorf("expected 10 blocked requests, got %d", blockedCount)
	}
}

func TestRateLimiter_WindowResetsAllowsNewRequests(t *testing.T) {
	store := NewInMemoryRateLimitStore()
	config := RateLimitConfig{
		RequestsPerWindow: 2,
		WindowDuration:    50 * time.Millisecond,
	}

	handler := RateLimiter(store, config, IPKeyFunc(), nil)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	makeRequest := func() int {
		req := httptest.NewRequest(http.MethodGet, "/api/test", nil)
		req.RemoteAddr = "192.168.1.1:12345"
		rr := httptest.NewRecorder()
		handler.ServeHTTP(rr, req)
		return rr.Code
	}

	// Use up the limit
	if code := makeRequest(); code != http.StatusOK {
		t.Error("first request should be allowed")
	}
	if code := makeRequest(); code != http.StatusOK {
		t.Error("second request should be allowed")
	}
	if code := makeRequest(); code != http.StatusTooManyRequests {
		t.Error("third request should be blocked")
	}

	// Wait for window to reset
	time.Sleep(60 * time.Millisecond)

	// New requests should be allowed
	if code := makeRequest(); code != http.StatusOK {
		t.Error("request after window reset should be allowed")
	}
}

func TestDefaultLimits(t *testing.T) {
	// Verify default limits are set correctly
	globalLimit := DefaultGlobalLimit()
	if globalLimit.RequestsPerWindow != 100 {
		t.Errorf("DefaultGlobalLimit().RequestsPerWindow = %d, want 100", globalLimit.RequestsPerWindow)
	}
	if globalLimit.WindowDuration != time.Minute {
		t.Errorf("DefaultGlobalLimit().WindowDuration = %v, want %v", globalLimit.WindowDuration, time.Minute)
	}

	authLimit := DefaultAuthLimit()
	if authLimit.RequestsPerWindow != 10 {
		t.Errorf("DefaultAuthLimit().RequestsPerWindow = %d, want 10", authLimit.RequestsPerWindow)
	}
	if authLimit.WindowDuration != time.Minute {
		t.Errorf("DefaultAuthLimit().WindowDuration = %v, want %v", authLimit.WindowDuration, time.Minute)
	}

	searchLimit := DefaultSearchLimit()
	if searchLimit.RequestsPerWindow != 30 {
		t.Errorf("DefaultSearchLimit().RequestsPerWindow = %d, want 30", searchLimit.RequestsPerWindow)
	}
	if searchLimit.WindowDuration != time.Minute {
		t.Errorf("DefaultSearchLimit().WindowDuration = %v, want %v", searchLimit.WindowDuration, time.Minute)
	}
}

// TestRateLimitStore_Interface verifies that InMemoryRateLimitStore implements RateLimitStore.
func TestRateLimitStore_Interface(t *testing.T) {
	var _ RateLimitStore = (*InMemoryRateLimitStore)(nil)
}

func TestRateLimitConfig_Validate(t *testing.T) {
	tests := []struct {
		name      string
		config    RateLimitConfig
		wantError bool
	}{
		{
			name: "valid config",
			config: RateLimitConfig{
				RequestsPerWindow: 100,
				WindowDuration:    time.Minute,
			},
			wantError: false,
		},
		{
			name: "zero requests",
			config: RateLimitConfig{
				RequestsPerWindow: 0,
				WindowDuration:    time.Minute,
			},
			wantError: true,
		},
		{
			name: "negative requests",
			config: RateLimitConfig{
				RequestsPerWindow: -1,
				WindowDuration:    time.Minute,
			},
			wantError: true,
		},
		{
			name: "zero window duration",
			config: RateLimitConfig{
				RequestsPerWindow: 100,
				WindowDuration:    0,
			},
			wantError: true,
		},
		{
			name: "negative window duration",
			config: RateLimitConfig{
				RequestsPerWindow: 100,
				WindowDuration:    -time.Second,
			},
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()
			if tt.wantError && err == nil {
				t.Error("expected validation error, got nil")
			}
			if !tt.wantError && err != nil {
				t.Errorf("expected no validation error, got %v", err)
			}
		})
	}
}

func TestDefaultLimits_Immutability(t *testing.T) {
	// Get copies
	global1 := DefaultGlobalLimit()
	global2 := DefaultGlobalLimit()

	// Modify the first copy
	global1.RequestsPerWindow = 9999

	// Second copy should be unchanged
	if global2.RequestsPerWindow == 9999 {
		t.Error("modifying one copy should not affect other copies")
	}
	if global2.RequestsPerWindow != 100 {
		t.Errorf("DefaultGlobalLimit should return 100, got %d", global2.RequestsPerWindow)
	}
}
