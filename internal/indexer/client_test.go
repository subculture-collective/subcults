package indexer

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

// newTestLogger creates a logger that discards all output to reduce test noise
func newTestLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

func TestClient_NewClient_ValidConfig(t *testing.T) {
	config := DefaultConfig("wss://jetstream.example.com")
	client, err := NewClient(config, nil, nil)
	if err != nil {
		t.Fatalf("NewClient() unexpected error = %v", err)
	}
	if client == nil {
		t.Fatal("NewClient() returned nil client")
	}
}

func TestClient_NewClient_InvalidConfig(t *testing.T) {
	tests := []struct {
		name    string
		config  Config
		wantErr error
	}{
		{
			name:    "empty URL",
			config:  Config{URL: "", BaseDelay: 100, MaxDelay: 200, JitterFactor: 0.5},
			wantErr: ErrEmptyURL,
		},
		{
			name:    "invalid base delay",
			config:  Config{URL: "wss://test.com", BaseDelay: 0, MaxDelay: 200, JitterFactor: 0.5},
			wantErr: ErrInvalidDelay,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := NewClient(tt.config, nil, nil)
			if err != tt.wantErr {
				t.Errorf("NewClient() error = %v, want %v", err, tt.wantErr)
			}
		})
	}
}

// mockServer creates a test WebSocket server that can be controlled for testing.
type mockServer struct {
	server       *httptest.Server
	upgrader     websocket.Upgrader
	mu           sync.Mutex
	connections  []*websocket.Conn
	messagesSent int32
	closeAfterN  int32 // Close connection after N messages sent
}

func newMockServer(closeAfterN int) *mockServer {
	ms := &mockServer{
		upgrader: websocket.Upgrader{
			CheckOrigin: func(r *http.Request) bool { return true },
		},
		closeAfterN: int32(closeAfterN),
	}

	ms.server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := ms.upgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}

		ms.mu.Lock()
		ms.connections = append(ms.connections, conn)
		ms.mu.Unlock()

		// Send messages until closeAfterN is reached
		for {
			// First send the message
			err := conn.WriteMessage(websocket.TextMessage, []byte(`{"test":"message"}`))
			if err != nil {
				return
			}

			// Then increment counter and check if we should close
			count := atomic.AddInt32(&ms.messagesSent, 1)
			if ms.closeAfterN > 0 && count >= ms.closeAfterN {
				conn.Close()
				return
			}
			time.Sleep(10 * time.Millisecond)
		}
	}))

	return ms
}

func (ms *mockServer) URL() string {
	return "ws" + strings.TrimPrefix(ms.server.URL, "http")
}

func (ms *mockServer) Close() {
	ms.mu.Lock()
	for _, conn := range ms.connections {
		conn.Close()
	}
	ms.mu.Unlock()
	ms.server.Close()
}

func (ms *mockServer) ConnectionCount() int {
	ms.mu.Lock()
	defer ms.mu.Unlock()
	return len(ms.connections)
}

func (ms *mockServer) MessagesSent() int32 {
	return atomic.LoadInt32(&ms.messagesSent)
}

func TestClient_Connect_Success(t *testing.T) {
	ms := newMockServer(0) // Don't close automatically
	defer ms.Close()

	config := Config{
		URL:          ms.URL(),
		BaseDelay:    10 * time.Millisecond,
		MaxDelay:     100 * time.Millisecond,
		JitterFactor: 0,
	}

	var receivedMessages int32
	handler := func(msgType int, payload []byte) error {
		atomic.AddInt32(&receivedMessages, 1)
		return nil
	}

	client, err := NewClient(config, handler, newTestLogger())
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	// Run client in background
	go func() {
		_ = client.Run(ctx)
	}()

	// Wait a bit for messages to be received
	time.Sleep(50 * time.Millisecond)

	if !client.IsConnected() {
		t.Error("expected client to be connected")
	}

	if atomic.LoadInt32(&receivedMessages) == 0 {
		t.Error("expected to receive at least one message")
	}
}

func TestClient_Reconnect_AfterForcedClose(t *testing.T) {
	// Server will close after 2 messages to make reconnection faster
	ms := newMockServer(2)
	defer ms.Close()

	config := Config{
		URL:          ms.URL(),
		BaseDelay:    5 * time.Millisecond, // Very short backoff for testing
		MaxDelay:     10 * time.Millisecond,
		JitterFactor: 0,
	}

	client, err := NewClient(config, nil, newTestLogger())
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}

	// Run for enough time to reconnect at least once (longer timeout for reliability)
	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()

	_ = client.Run(ctx)

	// Check server-side connection count - should have seen multiple connections
	connCount := ms.ConnectionCount()
	if connCount < 2 {
		t.Errorf("expected at least 2 connections due to reconnect, got %d", connCount)
	}
}

func TestClient_BackoffDelayWithinMaxWindow(t *testing.T) {
	// Server that always closes immediately after first message
	ms := newMockServer(1)
	defer ms.Close()

	config := Config{
		URL:          ms.URL(),
		BaseDelay:    10 * time.Millisecond,
		MaxDelay:     50 * time.Millisecond,
		JitterFactor: 0, // No jitter for predictable timing
	}

	client, err := NewClient(config, nil, newTestLogger())
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}

	// Run for enough time to have several reconnects
	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Millisecond)
	defer cancel()

	start := time.Now()
	_ = client.Run(ctx)
	elapsed := time.Since(start)

	// With max backoff of 50ms and context timeout of 300ms,
	// we should have had multiple reconnection attempts
	// The total time should be close to 300ms (context timeout)
	if elapsed < 250*time.Millisecond {
		t.Errorf("expected close to 300ms elapsed, got %v", elapsed)
	}
}

func TestClient_ComputeBackoff(t *testing.T) {
	config := Config{
		URL:          "wss://test.example.com",
		BaseDelay:    100 * time.Millisecond,
		MaxDelay:     1 * time.Second,
		JitterFactor: 0, // No jitter for deterministic tests
	}

	client, _ := NewClient(config, nil, nil)

	tests := []struct {
		attempt  int64
		expected time.Duration
	}{
		{attempt: 0, expected: 100 * time.Millisecond}, // 100ms * 2^0
		{attempt: 1, expected: 200 * time.Millisecond}, // 100ms * 2^1
		{attempt: 2, expected: 400 * time.Millisecond}, // 100ms * 2^2
		{attempt: 3, expected: 800 * time.Millisecond}, // 100ms * 2^3
		{attempt: 4, expected: 1 * time.Second},        // Capped at max
		{attempt: 5, expected: 1 * time.Second},        // Still capped
		{attempt: 10, expected: 1 * time.Second},       // Still capped
	}

	for _, tt := range tests {
		atomic.StoreInt64(&client.reconnectCount, tt.attempt)
		got := client.computeBackoff()
		if got != tt.expected {
			t.Errorf("computeBackoff() with attempt=%d = %v, want %v", tt.attempt, got, tt.expected)
		}
	}
}

func TestClient_ComputeBackoff_WithJitter(t *testing.T) {
	config := Config{
		URL:          "wss://test.example.com",
		BaseDelay:    100 * time.Millisecond,
		MaxDelay:     1 * time.Second,
		JitterFactor: 0.5,
	}

	client, _ := NewClient(config, nil, nil)
	atomic.StoreInt64(&client.reconnectCount, 0)

	// With 50% jitter, delay should be in range [75ms, 125ms] for attempt 0
	minExpected := 75 * time.Millisecond
	maxExpected := 125 * time.Millisecond

	// Test multiple times to verify randomness
	for i := 0; i < 100; i++ {
		got := client.computeBackoff()
		if got < minExpected || got > maxExpected {
			t.Errorf("computeBackoff() with jitter = %v, want in range [%v, %v]", got, minExpected, maxExpected)
		}
	}
}

func TestClient_ContextCancellation(t *testing.T) {
	ms := newMockServer(0)
	defer ms.Close()

	config := Config{
		URL:          ms.URL(),
		BaseDelay:    10 * time.Millisecond,
		MaxDelay:     100 * time.Millisecond,
		JitterFactor: 0,
	}

	client, err := NewClient(config, nil, newTestLogger())
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())

	done := make(chan error, 1)
	go func() {
		done <- client.Run(ctx)
	}()

	// Wait for connection
	time.Sleep(50 * time.Millisecond)

	// Cancel context
	cancel()

	// Should exit promptly
	select {
	case err := <-done:
		if err != context.Canceled {
			t.Errorf("Run() error = %v, want context.Canceled", err)
		}
	case <-time.After(100 * time.Millisecond):
		t.Error("Run() did not exit after context cancellation")
	}
}

func TestClient_IsConnected(t *testing.T) {
	ms := newMockServer(0)
	defer ms.Close()

	config := Config{
		URL:          ms.URL(),
		BaseDelay:    10 * time.Millisecond,
		MaxDelay:     100 * time.Millisecond,
		JitterFactor: 0,
	}

	client, err := NewClient(config, nil, newTestLogger())
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}

	// Initially not connected
	if client.IsConnected() {
		t.Error("expected IsConnected() = false before Run()")
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		_ = client.Run(ctx)
	}()

	// Wait for connection
	time.Sleep(50 * time.Millisecond)

	// Should be connected now
	if !client.IsConnected() {
		t.Error("expected IsConnected() = true after connection")
	}

	// Cancel and wait
	cancel()
	time.Sleep(50 * time.Millisecond)

	// Should be disconnected
	if client.IsConnected() {
		t.Error("expected IsConnected() = false after context cancellation")
	}
}

func TestClient_ConnectionFailure_TriggersBackoff(t *testing.T) {
	// Use invalid URL to force connection failure
	config := Config{
		URL:          "ws://localhost:1", // Invalid port
		BaseDelay:    10 * time.Millisecond,
		MaxDelay:     50 * time.Millisecond,
		JitterFactor: 0,
	}

	client, err := NewClient(config, nil, newTestLogger())
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}

	start := time.Now()
	ctx, cancel := context.WithTimeout(context.Background(), 150*time.Millisecond)
	defer cancel()

	_ = client.Run(ctx)

	elapsed := time.Since(start)

	// Should have had multiple reconnection attempts with backoff
	// First attempt immediate, then 10ms, 20ms, 40ms backoff
	// Total should be at least 70ms of backoff time
	if elapsed < 50*time.Millisecond {
		t.Errorf("expected at least 50ms elapsed due to backoff, got %v", elapsed)
	}
}
