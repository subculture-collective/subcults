// Package api provides HTTP handlers for the Subcults API.
package api

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/onnwee/subcults/internal/audit"
	"github.com/onnwee/subcults/internal/middleware"
	"github.com/onnwee/subcults/internal/scene"
	"github.com/onnwee/subcults/internal/stream"
)

// CreateStreamRequest represents the request body for creating a stream session.
type CreateStreamRequest struct {
	SceneID *string `json:"scene_id,omitempty"`
	EventID *string `json:"event_id,omitempty"`
}

// StreamSessionResponse represents the response for stream session operations.
type StreamSessionResponse struct {
	ID       string  `json:"id"`
	RoomName string  `json:"room_name"`
	SceneID  *string `json:"scene_id,omitempty"`
	EventID  *string `json:"event_id,omitempty"`
	Status   string  `json:"status"` // "active" or "ended"
}

// StreamHandlers holds dependencies for stream session HTTP handlers.
type StreamHandlers struct {
	streamRepo    stream.SessionRepository
	sceneRepo     scene.SceneRepository
	eventRepo     scene.EventRepository
	auditRepo     audit.Repository
	streamMetrics *stream.Metrics
}

// NewStreamHandlers creates a new StreamHandlers instance.
func NewStreamHandlers(
	streamRepo stream.SessionRepository,
	sceneRepo scene.SceneRepository,
	eventRepo scene.EventRepository,
	auditRepo audit.Repository,
	streamMetrics *stream.Metrics,
) *StreamHandlers {
	return &StreamHandlers{
		streamRepo:    streamRepo,
		sceneRepo:     sceneRepo,
		eventRepo:     eventRepo,
		auditRepo:     auditRepo,
		streamMetrics: streamMetrics,
	}
}

// CreateStream handles POST /streams - creates a new stream session.
func (h *StreamHandlers) CreateStream(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Extract user DID from context (set by auth middleware)
	userDID := middleware.GetUserDID(ctx)
	if userDID == "" {
		ctx = middleware.SetErrorCode(ctx, ErrCodeAuthFailed)
		WriteError(w, ctx, http.StatusUnauthorized, ErrCodeAuthFailed, "Authentication required")
		return
	}

	// Parse request body
	var req CreateStreamRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		ctx = middleware.SetErrorCode(ctx, ErrCodeBadRequest)
		WriteError(w, ctx, http.StatusBadRequest, ErrCodeBadRequest, "Invalid JSON in request body")
		return
	}

	// Validate that exactly one of scene_id or event_id is provided
	sceneIDProvided := req.SceneID != nil && strings.TrimSpace(*req.SceneID) != ""
	eventIDProvided := req.EventID != nil && strings.TrimSpace(*req.EventID) != ""

	if sceneIDProvided == eventIDProvided { // both true or both false
		ctx = middleware.SetErrorCode(ctx, ErrCodeValidation)
		WriteError(w, ctx, http.StatusBadRequest, ErrCodeValidation, "Exactly one of scene_id or event_id must be provided")
		return
	}

	// Trim whitespace from provided IDs
	if sceneIDProvided {
		trimmed := strings.TrimSpace(*req.SceneID)
		req.SceneID = &trimmed
	}
	if eventIDProvided {
		trimmed := strings.TrimSpace(*req.EventID)
		req.EventID = &trimmed
	}

	// Validate ownership
	if sceneIDProvided {
		// Check if user is the scene owner
		isOwner, err := h.isSceneOwner(ctx, *req.SceneID, userDID)
		if err != nil {
			if errors.Is(err, scene.ErrSceneNotFound) {
				ctx = middleware.SetErrorCode(ctx, ErrCodeNotFound)
				WriteError(w, ctx, http.StatusNotFound, ErrCodeNotFound, "Scene not found")
			} else {
				slog.ErrorContext(ctx, "failed to check scene ownership", "error", err)
				ctx = middleware.SetErrorCode(ctx, ErrCodeInternal)
				WriteError(w, ctx, http.StatusInternalServerError, ErrCodeInternal, "Internal server error")
			}
			return
		}
		if !isOwner {
			ctx = middleware.SetErrorCode(ctx, ErrCodeForbidden)
			WriteError(w, ctx, http.StatusForbidden, ErrCodeForbidden, "You must be the scene owner to create a stream")
			return
		}
	}

	if eventIDProvided {
		// Check if user is the event host (scene owner)
		event, err := h.eventRepo.GetByID(*req.EventID)
		if err != nil {
			if errors.Is(err, scene.ErrEventNotFound) {
				ctx = middleware.SetErrorCode(ctx, ErrCodeNotFound)
				WriteError(w, ctx, http.StatusNotFound, ErrCodeNotFound, "Event not found")
			} else {
				slog.ErrorContext(ctx, "failed to get event", "error", err)
				ctx = middleware.SetErrorCode(ctx, ErrCodeInternal)
				WriteError(w, ctx, http.StatusInternalServerError, ErrCodeInternal, "Internal server error")
			}
			return
		}

		// Check if user owns the scene that the event belongs to
		isOwner, err := h.isSceneOwner(ctx, event.SceneID, userDID)
		if err != nil {
			if errors.Is(err, scene.ErrSceneNotFound) {
				ctx = middleware.SetErrorCode(ctx, ErrCodeNotFound)
				WriteError(w, ctx, http.StatusNotFound, ErrCodeNotFound, "Scene not found")
			} else {
				slog.ErrorContext(ctx, "failed to check scene ownership", "error", err)
				ctx = middleware.SetErrorCode(ctx, ErrCodeInternal)
				WriteError(w, ctx, http.StatusInternalServerError, ErrCodeInternal, "Internal server error")
			}
			return
		}
		if !isOwner {
			ctx = middleware.SetErrorCode(ctx, ErrCodeForbidden)
			WriteError(w, ctx, http.StatusForbidden, ErrCodeForbidden, "You must be the event host to create a stream")
			return
		}
	}

	// Create stream session
	id, roomName, err := h.streamRepo.CreateStreamSession(req.SceneID, req.EventID, userDID)
	if err != nil {
		slog.ErrorContext(ctx, "failed to create stream session",
			"error", err,
			"user_did", userDID,
		)
		ctx = middleware.SetErrorCode(ctx, ErrCodeInternal)
		WriteError(w, ctx, http.StatusInternalServerError, ErrCodeInternal, "Failed to create stream session")
		return
	}

	// Log stream creation for audit
	auditEntry := audit.LogEntry{
		UserDID:    userDID,
		EntityType: "stream_session",
		EntityID:   id,
		Action:     "created",
		RequestID:  middleware.GetRequestID(ctx),
	}

	if _, err := h.auditRepo.LogAccess(auditEntry); err != nil {
		// Log error but don't fail the request
		slog.ErrorContext(ctx, "failed to log stream creation audit entry",
			"error", err,
			"stream_id", id,
			"user_did", userDID,
		)
	}

	// Return response
	response := StreamSessionResponse{
		ID:       id,
		RoomName: roomName,
		SceneID:  req.SceneID,
		EventID:  req.EventID,
		Status:   "active",
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	if err := json.NewEncoder(w).Encode(response); err != nil {
		slog.ErrorContext(ctx, "failed to encode stream response", "error", err)
	}
}

// EndStream handles POST /streams/{id}/end - ends a stream session.
func (h *StreamHandlers) EndStream(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Extract user DID from context (set by auth middleware)
	userDID := middleware.GetUserDID(ctx)
	if userDID == "" {
		ctx = middleware.SetErrorCode(ctx, ErrCodeAuthFailed)
		WriteError(w, ctx, http.StatusUnauthorized, ErrCodeAuthFailed, "Authentication required")
		return
	}

	// Extract stream ID from URL path
	// Expected: /streams/{id}/end
	pathParts := strings.Split(strings.TrimPrefix(r.URL.Path, "/streams/"), "/")
	if len(pathParts) != 2 || pathParts[0] == "" || pathParts[1] != "end" {
		ctx = middleware.SetErrorCode(ctx, ErrCodeBadRequest)
		WriteError(w, ctx, http.StatusBadRequest, ErrCodeBadRequest, "Invalid URL path")
		return
	}
	streamID := pathParts[0]

	// Get the stream session to verify ownership
	session, err := h.streamRepo.GetByID(streamID)
	if err != nil {
		if errors.Is(err, stream.ErrStreamNotFound) {
			ctx = middleware.SetErrorCode(ctx, ErrCodeNotFound)
			WriteError(w, ctx, http.StatusNotFound, ErrCodeNotFound, "Stream session not found")
		} else {
			slog.ErrorContext(ctx, "failed to get stream session", "error", err)
			ctx = middleware.SetErrorCode(ctx, ErrCodeInternal)
			WriteError(w, ctx, http.StatusInternalServerError, ErrCodeInternal, "Internal server error")
		}
		return
	}

	// Verify that the user is the stream host
	if session.HostDID != userDID {
		ctx = middleware.SetErrorCode(ctx, ErrCodeForbidden)
		WriteError(w, ctx, http.StatusForbidden, ErrCodeForbidden, "You must be the stream host to end it")
		return
	}

	// End the stream session
	err = h.streamRepo.EndStreamSession(streamID)
	if err != nil {
		slog.ErrorContext(ctx, "failed to end stream session",
			"error", err,
			"stream_id", streamID,
			"user_did", userDID,
		)
		ctx = middleware.SetErrorCode(ctx, ErrCodeInternal)
		WriteError(w, ctx, http.StatusInternalServerError, ErrCodeInternal, "Failed to end stream session")
		return
	}

	// Log stream ending for audit
	auditEntry := audit.LogEntry{
		UserDID:    userDID,
		EntityType: "stream_session",
		EntityID:   streamID,
		Action:     "ended",
		RequestID:  middleware.GetRequestID(ctx),
	}

	if _, err := h.auditRepo.LogAccess(auditEntry); err != nil {
		// Log error but don't fail the request
		slog.ErrorContext(ctx, "failed to log stream ending audit entry",
			"error", err,
			"stream_id", streamID,
			"user_did", userDID,
		)
	}

	// Return response
	response := StreamSessionResponse{
		ID:       streamID,
		RoomName: session.RoomName,
		SceneID:  session.SceneID,
		EventID:  session.EventID,
		Status:   "ended",
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(response); err != nil {
		slog.ErrorContext(ctx, "failed to encode stream response", "error", err)
	}
}

// isSceneOwner checks if the given userDID owns the scene.
func (h *StreamHandlers) isSceneOwner(ctx context.Context, sceneID, userDID string) (bool, error) {
	foundScene, err := h.sceneRepo.GetByID(sceneID)
	if err != nil {
		return false, err
	}
	return foundScene.IsOwner(userDID), nil
}

// JoinStreamRequest represents the request body for recording a join event.
type JoinStreamRequest struct {
	TokenIssuedAt string `json:"token_issued_at"` // RFC3339 timestamp from token issuance
}

// JoinStream handles POST /streams/{id}/join - records a join event and metrics.
func (h *StreamHandlers) JoinStream(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Extract user DID from context (set by auth middleware)
	userDID := middleware.GetUserDID(ctx)
	if userDID == "" {
		ctx = middleware.SetErrorCode(ctx, ErrCodeAuthFailed)
		WriteError(w, ctx, http.StatusUnauthorized, ErrCodeAuthFailed, "Authentication required")
		return
	}

	// Extract stream ID from URL path
	// Expected: /streams/{id}/join
	pathParts := strings.Split(strings.TrimPrefix(r.URL.Path, "/streams/"), "/")
	if len(pathParts) != 2 || pathParts[0] == "" || pathParts[1] != "join" {
		ctx = middleware.SetErrorCode(ctx, ErrCodeBadRequest)
		WriteError(w, ctx, http.StatusBadRequest, ErrCodeBadRequest, "Invalid URL path")
		return
	}
	streamID := pathParts[0]

	// Verify stream exists
	session, err := h.streamRepo.GetByID(streamID)
	if err != nil {
		if errors.Is(err, stream.ErrStreamNotFound) {
			ctx = middleware.SetErrorCode(ctx, ErrCodeNotFound)
			WriteError(w, ctx, http.StatusNotFound, ErrCodeNotFound, "Stream session not found")
		} else {
			slog.ErrorContext(ctx, "failed to get stream session", "error", err)
			ctx = middleware.SetErrorCode(ctx, ErrCodeInternal)
			WriteError(w, ctx, http.StatusInternalServerError, ErrCodeInternal, "Internal server error")
		}
		return
	}

	// Parse optional request body for latency tracking
	var req JoinStreamRequest
	if r.Body != nil {
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			// Non-fatal: continue without latency tracking
			slog.WarnContext(ctx, "failed to decode join request body", "error", err)
		}
	}

	// Record join in repository
	if err := h.streamRepo.RecordJoin(streamID); err != nil {
		slog.ErrorContext(ctx, "failed to record join",
			"error", err,
			"stream_id", streamID,
			"user_did", userDID,
		)
		ctx = middleware.SetErrorCode(ctx, ErrCodeInternal)
		WriteError(w, ctx, http.StatusInternalServerError, ErrCodeInternal, "Failed to record join event")
		return
	}

	// Increment Prometheus counter
	if h.streamMetrics != nil {
		h.streamMetrics.IncStreamJoins()
	}

	// Calculate and record join latency if token_issued_at was provided
	if req.TokenIssuedAt != "" {
		tokenTime, err := time.Parse(time.RFC3339, req.TokenIssuedAt)
		if err == nil {
			now := time.Now()
			// Validate token time is not in the future (client clock skew)
			if tokenTime.After(now) {
				slog.WarnContext(ctx, "token_issued_at is in the future, skipping latency recording",
					"token_time", tokenTime,
					"current_time", now,
					"stream_id", streamID)
			} else {
				latency := now.Sub(tokenTime).Seconds()
				// Validate token is not too old (max 5 minutes to represent actual join time)
				const maxTokenAge = 5 * 60 // 5 minutes in seconds
				if latency > maxTokenAge {
					slog.WarnContext(ctx, "token_issued_at is too old, skipping latency recording",
						"token_age_seconds", latency,
						"max_age_seconds", maxTokenAge,
						"stream_id", streamID)
				} else if h.streamMetrics != nil {
					h.streamMetrics.ObserveStreamJoinLatency(latency)
				}
			}
		} else {
			slog.WarnContext(ctx, "invalid token_issued_at timestamp", "error", err, "value", req.TokenIssuedAt)
		}
	}

	// Log join event for audit
	auditEntry := audit.LogEntry{
		UserDID:    userDID,
		EntityType: "stream_session",
		EntityID:   streamID,
		Action:     "joined",
		RequestID:  middleware.GetRequestID(ctx),
	}

	if _, err := h.auditRepo.LogAccess(auditEntry); err != nil {
		// Log error but don't fail the request
		slog.ErrorContext(ctx, "failed to log join audit entry",
			"error", err,
			"stream_id", streamID,
			"user_did", userDID,
		)
	}

	// Re-fetch session to get the updated join count from storage
	updatedSession, err := h.streamRepo.GetByID(streamID)
	if err != nil {
		// Log error but continue using the previously loaded session
		slog.ErrorContext(ctx, "failed to refresh stream session after join",
			"error", err,
			"stream_id", streamID,
			"user_did", userDID,
		)
	} else {
		session = updatedSession
	}

	// Return success response with the persisted join count
	response := map[string]interface{}{
		"stream_id":  streamID,
		"room_name":  session.RoomName,
		"join_count": session.JoinCount,
		"status":     "joined",
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(response); err != nil {
		slog.ErrorContext(ctx, "failed to encode join response", "error", err)
	}
}

// LeaveStream handles POST /streams/{id}/leave - records a leave event and metrics.
func (h *StreamHandlers) LeaveStream(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Extract user DID from context (set by auth middleware)
	userDID := middleware.GetUserDID(ctx)
	if userDID == "" {
		ctx = middleware.SetErrorCode(ctx, ErrCodeAuthFailed)
		WriteError(w, ctx, http.StatusUnauthorized, ErrCodeAuthFailed, "Authentication required")
		return
	}

	// Extract stream ID from URL path
	// Expected: /streams/{id}/leave
	pathParts := strings.Split(strings.TrimPrefix(r.URL.Path, "/streams/"), "/")
	if len(pathParts) != 2 || pathParts[0] == "" || pathParts[1] != "leave" {
		ctx = middleware.SetErrorCode(ctx, ErrCodeBadRequest)
		WriteError(w, ctx, http.StatusBadRequest, ErrCodeBadRequest, "Invalid URL path")
		return
	}
	streamID := pathParts[0]

	// Verify stream exists
	session, err := h.streamRepo.GetByID(streamID)
	if err != nil {
		if errors.Is(err, stream.ErrStreamNotFound) {
			ctx = middleware.SetErrorCode(ctx, ErrCodeNotFound)
			WriteError(w, ctx, http.StatusNotFound, ErrCodeNotFound, "Stream session not found")
		} else {
			slog.ErrorContext(ctx, "failed to get stream session", "error", err)
			ctx = middleware.SetErrorCode(ctx, ErrCodeInternal)
			WriteError(w, ctx, http.StatusInternalServerError, ErrCodeInternal, "Internal server error")
		}
		return
	}

	// Record leave in repository
	if err := h.streamRepo.RecordLeave(streamID); err != nil {
		slog.ErrorContext(ctx, "failed to record leave",
			"error", err,
			"stream_id", streamID,
			"user_did", userDID,
		)
		ctx = middleware.SetErrorCode(ctx, ErrCodeInternal)
		WriteError(w, ctx, http.StatusInternalServerError, ErrCodeInternal, "Failed to record leave event")
		return
	}

	// Increment Prometheus counter
	if h.streamMetrics != nil {
		h.streamMetrics.IncStreamLeaves()
	}

	// Log leave event for audit
	auditEntry := audit.LogEntry{
		UserDID:    userDID,
		EntityType: "stream_session",
		EntityID:   streamID,
		Action:     "left",
		RequestID:  middleware.GetRequestID(ctx),
	}

	if _, err := h.auditRepo.LogAccess(auditEntry); err != nil {
		// Log error but don't fail the request
		slog.ErrorContext(ctx, "failed to log leave audit entry",
			"error", err,
			"stream_id", streamID,
			"user_did", userDID,
		)
	}

	// Re-fetch session to get the updated leave count from storage
	updatedSession, err := h.streamRepo.GetByID(streamID)
	if err != nil {
		// Log error but continue using the previously loaded session
		slog.ErrorContext(ctx, "failed to refresh stream session after leave",
			"error", err,
			"stream_id", streamID,
			"user_did", userDID,
		)
	} else {
		session = updatedSession
	}

	// Return success response with the persisted leave count
	response := map[string]interface{}{
		"stream_id":   streamID,
		"room_name":   session.RoomName,
		"leave_count": session.LeaveCount,
		"status":      "left",
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(response); err != nil {
		slog.ErrorContext(ctx, "failed to encode leave response", "error", err)
	}
}
