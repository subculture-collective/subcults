// Package api provides HTTP handlers for stream participant WebSocket subscriptions.
package api

import (
	"log/slog"
	"net/http"
	"strings"

	"github.com/gorilla/websocket"
	"github.com/onnwee/subcults/internal/middleware"
	"github.com/onnwee/subcults/internal/stream"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		// TODO: Implement proper CORS checking based on configuration
		// For now, allow all origins (should be restricted in production)
		return true
	},
}

// ParticipantWebSocketHandlers holds dependencies for WebSocket handlers.
type ParticipantWebSocketHandlers struct {
	streamRepo       stream.SessionRepository
	eventBroadcaster *stream.EventBroadcaster
}

// NewParticipantWebSocketHandlers creates a new ParticipantWebSocketHandlers instance.
func NewParticipantWebSocketHandlers(
	streamRepo stream.SessionRepository,
	eventBroadcaster *stream.EventBroadcaster,
) *ParticipantWebSocketHandlers {
	return &ParticipantWebSocketHandlers{
		streamRepo:       streamRepo,
		eventBroadcaster: eventBroadcaster,
	}
}

// SubscribeToParticipantEvents handles WebSocket connections for real-time participant updates.
// GET /streams/{id}/participants/ws
func (h *ParticipantWebSocketHandlers) SubscribeToParticipantEvents(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Extract stream ID from URL path
	// Expected: /streams/{id}/participants/ws
	pathParts := strings.Split(strings.TrimPrefix(r.URL.Path, "/streams/"), "/")
	if len(pathParts) != 3 || pathParts[0] == "" || pathParts[1] != "participants" || pathParts[2] != "ws" {
		WriteError(w, ctx, http.StatusBadRequest, ErrCodeBadRequest, "Invalid URL path")
		return
	}
	streamID := pathParts[0]

	// Verify stream exists
	_, err := h.streamRepo.GetByID(streamID)
	if err != nil {
		if err == stream.ErrStreamNotFound {
			WriteError(w, ctx, http.StatusNotFound, ErrCodeNotFound, "Stream session not found")
		} else {
			slog.ErrorContext(ctx, "failed to get stream session", "error", err)
			WriteError(w, ctx, http.StatusInternalServerError, ErrCodeInternal, "Internal server error")
		}
		return
	}

	// Upgrade HTTP connection to WebSocket
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		slog.ErrorContext(ctx, "failed to upgrade websocket connection",
			"error", err,
			"stream_id", streamID,
		)
		return
	}

	// Subscribe to events
	h.eventBroadcaster.Subscribe(streamID, conn)

	// Log subscription
	requestID := middleware.GetRequestID(ctx)
	slog.InfoContext(ctx, "websocket client subscribed to participant events",
		"stream_id", streamID,
		"request_id", requestID,
	)

	// Handle connection lifecycle
	defer func() {
		h.eventBroadcaster.Unsubscribe(conn)
		conn.Close()
		slog.InfoContext(ctx, "websocket client unsubscribed",
			"stream_id", streamID,
			"request_id", requestID,
		)
	}()

	// Keep connection alive - read messages to detect disconnection
	// We don't expect clients to send messages, but we need to read to detect when they disconnect
	for {
		_, _, err := conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				slog.WarnContext(ctx, "websocket connection closed unexpectedly",
					"error", err,
					"stream_id", streamID,
				)
			}
			break
		}
	}
}
