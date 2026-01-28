// Package stream provides WebSocket event broadcasting for real-time participant updates.
package stream

import (
	"encoding/json"
	"log/slog"
	"sync"

	"github.com/gorilla/websocket"
)

// EventBroadcaster manages WebSocket connections and broadcasts participant events.
type EventBroadcaster struct {
	mu          sync.RWMutex
	connections map[string]map[*websocket.Conn]bool // streamSessionID -> connections
}

// NewEventBroadcaster creates a new event broadcaster.
func NewEventBroadcaster() *EventBroadcaster {
	return &EventBroadcaster{
		connections: make(map[string]map[*websocket.Conn]bool),
	}
}

// Subscribe registers a WebSocket connection for a stream session.
func (b *EventBroadcaster) Subscribe(streamSessionID string, conn *websocket.Conn) {
	b.mu.Lock()
	defer b.mu.Unlock()

	if b.connections[streamSessionID] == nil {
		b.connections[streamSessionID] = make(map[*websocket.Conn]bool)
	}
	b.connections[streamSessionID][conn] = true
}

// Unsubscribe removes a WebSocket connection from all stream sessions.
func (b *EventBroadcaster) Unsubscribe(conn *websocket.Conn) {
	b.mu.Lock()
	defer b.mu.Unlock()

	for streamID, conns := range b.connections {
		delete(conns, conn)
		if len(conns) == 0 {
			delete(b.connections, streamID)
		}
	}
}

// Broadcast sends a participant event to all subscribers of a stream.
func (b *EventBroadcaster) Broadcast(streamSessionID string, event *ParticipantStateEvent) {
	b.mu.RLock()
	defer b.mu.RUnlock()

	conns, exists := b.connections[streamSessionID]
	if !exists || len(conns) == 0 {
		return
	}

	// Serialize event once
	data, err := json.Marshal(event)
	if err != nil {
		slog.Error("failed to marshal participant event", "error", err)
		return
	}

	// Broadcast to all connections
	for conn := range conns {
		if err := conn.WriteMessage(websocket.TextMessage, data); err != nil {
			slog.Warn("failed to send message to websocket client",
				"error", err,
				"stream_session_id", streamSessionID,
			)
			// Connection will be cleaned up when client disconnects
		}
	}
}

// ConnectionCount returns the number of active WebSocket connections for a stream.
func (b *EventBroadcaster) ConnectionCount(streamSessionID string) int {
	b.mu.RLock()
	defer b.mu.RUnlock()

	if conns, exists := b.connections[streamSessionID]; exists {
		return len(conns)
	}
	return 0
}
