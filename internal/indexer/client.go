// Package indexer provides filtering and processing of AT Protocol records
// for the Subcults Jetstream indexer.
package indexer

import (
	"context"
	"log/slog"
	"math/rand"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gorilla/websocket"
)

// MessageHandler is a callback function for processing incoming messages.
// The handler receives the message type and payload.
// Return an error to signal the client should disconnect.
type MessageHandler func(messageType int, payload []byte) error

// Client is a resilient WebSocket client for connecting to Jetstream.
// It automatically reconnects with exponential backoff and jitter.
type Client struct {
	config  Config
	handler MessageHandler
	logger  *slog.Logger

	mu          sync.Mutex
	rng         *rand.Rand // protected by mu
	conn        *websocket.Conn
	isConnected bool

	// reconnectCount tracks consecutive reconnection attempts (atomic)
	reconnectCount int64
}

// NewClient creates a new Jetstream WebSocket client with the given configuration.
// The handler function will be called for each incoming message.
func NewClient(config Config, handler MessageHandler, logger *slog.Logger) (*Client, error) {
	if err := config.Validate(); err != nil {
		return nil, err
	}
	if logger == nil {
		logger = slog.Default()
	}
	return &Client{
		config:  config,
		handler: handler,
		logger:  logger,
		rng:     rand.New(rand.NewSource(time.Now().UnixNano())),
	}, nil
}

// Run starts the WebSocket client and blocks until the context is cancelled.
// It will automatically reconnect with exponential backoff on connection failures.
func (c *Client) Run(ctx context.Context) error {
	for {
		select {
		case <-ctx.Done():
			c.logger.Info("jetstream client stopping due to context cancellation")
			c.close()
			return ctx.Err()
		default:
		}

		// Attempt to connect
		if err := c.connect(ctx); err != nil {
			attempt := atomic.LoadInt64(&c.reconnectCount) + 1
			c.logger.Warn("jetstream connection failed",
				slog.String("error", err.Error()),
				slog.Int64("attempt", attempt))

			// Schedule reconnect with backoff
			delay := c.computeBackoff()
			atomic.AddInt64(&c.reconnectCount, 1)

			c.logger.Info("scheduling reconnect",
				slog.Duration("delay", delay),
				slog.Int64("attempt", atomic.LoadInt64(&c.reconnectCount)))

			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(delay):
				continue
			}
		}

		// Reset reconnect count on successful connection
		atomic.StoreInt64(&c.reconnectCount, 0)

		// Read messages until connection closes
		c.readLoop(ctx)
	}
}

// connect establishes a WebSocket connection to the Jetstream endpoint.
func (c *Client) connect(ctx context.Context) error {
	c.logger.Info("connecting to jetstream", slog.String("url", c.config.URL))

	dialer := websocket.Dialer{
		HandshakeTimeout: 10 * time.Second,
	}

	conn, _, err := dialer.DialContext(ctx, c.config.URL, nil)
	if err != nil {
		return err
	}

	c.mu.Lock()
	c.conn = conn
	c.isConnected = true
	c.mu.Unlock()

	c.logger.Info("connected to jetstream")
	return nil
}

// readLoop reads messages from the WebSocket connection until it closes.
func (c *Client) readLoop(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		// Get connection under lock to prevent race with close()
		c.mu.Lock()
		conn := c.conn
		c.mu.Unlock()

		if conn == nil {
			// Connection was closed, exit loop
			return
		}

		messageType, payload, err := conn.ReadMessage()
		if err != nil {
			c.logger.Warn("jetstream connection closed",
				slog.String("error", err.Error()))
			c.close()
			return
		}

		// Process message through handler (without logging payload content)
		if c.handler != nil {
			if err := c.handler(messageType, payload); err != nil {
				c.logger.Error("message handler error",
					slog.String("error", err.Error()))
				c.close()
				return
			}
		}
	}
}

// close cleanly closes the WebSocket connection.
func (c *Client) close() {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.conn != nil {
		_ = c.conn.Close()
		c.conn = nil
	}
	c.isConnected = false
}

// computeBackoff calculates the next reconnection delay with exponential backoff and jitter.
func (c *Client) computeBackoff() time.Duration {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Exponential backoff: baseDelay * 2^attempts using bit shifting
	// Cap the shift at 30 to prevent overflow (2^30 = ~1 billion)
	reconnectCount := atomic.LoadInt64(&c.reconnectCount)
	shift := uint(reconnectCount)
	if shift > 30 {
		shift = 30
	}
	backoff := float64(c.config.BaseDelay) * float64(uint64(1)<<shift)

	// Cap at max delay
	if backoff > float64(c.config.MaxDelay) {
		backoff = float64(c.config.MaxDelay)
	}

	// Apply jitter: delay * (1 - jitter/2 + rand*jitter)
	// This creates a range of [delay*(1-jitter/2), delay*(1+jitter/2)]
	if c.config.JitterFactor > 0 {
		jitter := (c.rng.Float64() - 0.5) * c.config.JitterFactor
		backoff = backoff * (1 + jitter)
	}

	return time.Duration(backoff)
}

// IsConnected returns whether the client is currently connected.
func (c *Client) IsConnected() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.isConnected
}
