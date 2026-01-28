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
	"github.com/onnwee/subcults/internal/livekit"
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
	streamRepo       stream.SessionRepository
	participantRepo  stream.ParticipantRepository
	analyticsRepo    stream.AnalyticsRepository
	sceneRepo        scene.SceneRepository
	eventRepo        scene.EventRepository
	auditRepo        audit.Repository
	streamMetrics    *stream.Metrics
	eventBroadcaster *stream.EventBroadcaster
	roomService      *livekit.RoomService
}

// NewStreamHandlers creates a new StreamHandlers instance.
func NewStreamHandlers(
	streamRepo stream.SessionRepository,
	participantRepo stream.ParticipantRepository,
	analyticsRepo stream.AnalyticsRepository,
	sceneRepo scene.SceneRepository,
	eventRepo scene.EventRepository,
	auditRepo audit.Repository,
	streamMetrics *stream.Metrics,
	eventBroadcaster *stream.EventBroadcaster,
	roomService *livekit.RoomService,
) *StreamHandlers {
	return &StreamHandlers{
		streamRepo:       streamRepo,
		participantRepo:  participantRepo,
		analyticsRepo:    analyticsRepo,
		sceneRepo:        sceneRepo,
		eventRepo:        eventRepo,
		auditRepo:        auditRepo,
		streamMetrics:    streamMetrics,
		eventBroadcaster: eventBroadcaster,
		roomService:      roomService,
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

	// Compute analytics for the ended stream
	if h.analyticsRepo != nil {
		_, err = h.analyticsRepo.ComputeAnalytics(streamID)
		if err != nil {
			// Log error but don't fail the request
			slog.ErrorContext(ctx, "failed to compute stream analytics",
				"error", err,
				"stream_id", streamID,
				"user_did", userDID,
			)
		} else {
			slog.InfoContext(ctx, "computed stream analytics",
				"stream_id", streamID,
				"user_did", userDID,
			)
		}
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
	TokenIssuedAt string  `json:"token_issued_at"`          // RFC3339 timestamp from token issuance
	GeohashPrefix *string `json:"geohash_prefix,omitempty"` // Optional 4-char geohash for geographic tracking
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

	// Generate participant ID from user DID
	participantID := stream.GenerateParticipantID(userDID)

	// Record participant join in participant repository
	var isReconnection bool
	if h.participantRepo != nil {
		participant, reconnection, err := h.participantRepo.RecordJoin(streamID, participantID, userDID)
		if err != nil {
			if errors.Is(err, stream.ErrParticipantAlreadyActive) {
				// Participant is already active, this is a duplicate join request
				slog.WarnContext(ctx, "participant already active in stream",
					"stream_id", streamID,
					"participant_id", participantID,
					"user_did", userDID,
				)
				// Continue with join count increment and return success
			} else {
				slog.ErrorContext(ctx, "failed to record participant join",
					"error", err,
					"stream_id", streamID,
					"user_did", userDID,
				)
				ctx = middleware.SetErrorCode(ctx, ErrCodeInternal)
				WriteError(w, ctx, http.StatusInternalServerError, ErrCodeInternal, "Failed to record participant join")
				return
			}
		} else {
			isReconnection = reconnection

			// Broadcast participant joined event via WebSocket
			if h.eventBroadcaster != nil {
				activeCount, _ := h.participantRepo.GetActiveCount(streamID)
				event := &stream.ParticipantStateEvent{
					Type:            "participant_joined",
					StreamSessionID: streamID,
					ParticipantID:   participant.ParticipantID,
					UserDID:         participant.UserDID,
					Timestamp:       participant.JoinedAt,
					IsReconnection:  isReconnection,
					ActiveCount:     activeCount,
				}
				h.eventBroadcaster.Broadcast(streamID, event)
			}
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

	// Record participant event for analytics
	if h.analyticsRepo != nil {
		// Validate and sanitize geohash prefix if provided
		var geohashPrefix *string
		if req.GeohashPrefix != nil && len(strings.TrimSpace(*req.GeohashPrefix)) >= 4 {
			// Take only first 4 characters for privacy
			prefix := strings.TrimSpace(*req.GeohashPrefix)[:4]
			geohashPrefix = &prefix
		}

		if err := h.analyticsRepo.RecordParticipantEvent(streamID, userDID, "join", geohashPrefix); err != nil {
			// Log error but don't fail the request
			slog.ErrorContext(ctx, "failed to record participant join event",
				"error", err,
				"stream_id", streamID,
				"user_did", userDID,
			)
		}
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

	// Generate participant ID from user DID
	participantID := stream.GenerateParticipantID(userDID)

	// Record participant leave in participant repository
	if h.participantRepo != nil {
		if err := h.participantRepo.RecordLeave(streamID, participantID); err != nil {
			if errors.Is(err, stream.ErrParticipantNotFound) {
				// Participant was not active, log warning but continue
				slog.WarnContext(ctx, "participant not found or already left",
					"stream_id", streamID,
					"participant_id", participantID,
					"user_did", userDID,
				)
			} else {
				slog.ErrorContext(ctx, "failed to record participant leave",
					"error", err,
					"stream_id", streamID,
					"user_did", userDID,
				)
				ctx = middleware.SetErrorCode(ctx, ErrCodeInternal)
				WriteError(w, ctx, http.StatusInternalServerError, ErrCodeInternal, "Failed to record participant leave")
				return
			}
		} else {
			// Broadcast participant left event via WebSocket
			if h.eventBroadcaster != nil {
				activeCount, _ := h.participantRepo.GetActiveCount(streamID)
				event := &stream.ParticipantStateEvent{
					Type:            "participant_left",
					StreamSessionID: streamID,
					ParticipantID:   participantID,
					UserDID:         userDID,
					Timestamp:       time.Now(),
					IsReconnection:  false,
					ActiveCount:     activeCount,
				}
				h.eventBroadcaster.Broadcast(streamID, event)
			}
		}
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

	// Record participant event for analytics
	if h.analyticsRepo != nil {
		if err := h.analyticsRepo.RecordParticipantEvent(streamID, userDID, "leave", nil); err != nil {
			// Log error but don't fail the request
			slog.ErrorContext(ctx, "failed to record participant leave event",
				"error", err,
				"stream_id", streamID,
				"user_did", userDID,
			)
		}
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

// GetStreamAnalytics handles GET /streams/{id}/analytics - retrieves analytics for a stream session.
// Only accessible by the stream host (scene/event owner).
func (h *StreamHandlers) GetStreamAnalytics(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Extract user DID from context (set by auth middleware)
	userDID := middleware.GetUserDID(ctx)
	if userDID == "" {
		ctx = middleware.SetErrorCode(ctx, ErrCodeAuthFailed)
		WriteError(w, ctx, http.StatusUnauthorized, ErrCodeAuthFailed, "Authentication required")
		return
	}

	// Extract stream ID from URL path
	// Expected: /streams/{id}/analytics
	pathParts := strings.Split(strings.TrimPrefix(r.URL.Path, "/streams/"), "/")
	if len(pathParts) != 2 || pathParts[0] == "" || pathParts[1] != "analytics" {
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
		WriteError(w, ctx, http.StatusForbidden, ErrCodeForbidden, "You must be the stream host to view analytics")
		return
	}

	// Check if analytics repository is available
	if h.analyticsRepo == nil {
		ctx = middleware.SetErrorCode(ctx, ErrCodeInternal)
		WriteError(w, ctx, http.StatusInternalServerError, ErrCodeInternal, "Analytics not available")
		return
	}

	// Get analytics
	analytics, err := h.analyticsRepo.GetAnalytics(streamID)
	if err != nil {
		if errors.Is(err, stream.ErrAnalyticsNotFound) {
			// Analytics not computed yet - check if stream has ended
			if session.EndedAt == nil {
				ctx = middleware.SetErrorCode(ctx, ErrCodeValidation)
				WriteError(w, ctx, http.StatusBadRequest, ErrCodeValidation, "Analytics not available until stream ends")
			} else {
				ctx = middleware.SetErrorCode(ctx, ErrCodeNotFound)
				WriteError(w, ctx, http.StatusNotFound, ErrCodeNotFound, "Analytics not yet computed for this stream")
			}
		} else {
			slog.ErrorContext(ctx, "failed to get stream analytics", "error", err)
			ctx = middleware.SetErrorCode(ctx, ErrCodeInternal)
			WriteError(w, ctx, http.StatusInternalServerError, ErrCodeInternal, "Internal server error")
		}
		return
	}

	// Log analytics access for audit
	auditEntry := audit.LogEntry{
		UserDID:    userDID,
		EntityType: "stream_analytics",
		EntityID:   streamID,
		Action:     "viewed",
		RequestID:  middleware.GetRequestID(ctx),
	}

	if _, err := h.auditRepo.LogAccess(auditEntry); err != nil {
		// Log error but don't fail the request
		slog.ErrorContext(ctx, "failed to log analytics access",
			"error", err,
			"stream_id", streamID,
			"user_did", userDID,
		)
	}

	// Return analytics (no PII exposed - only aggregate data)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(analytics); err != nil {
		slog.ErrorContext(ctx, "failed to encode analytics response", "error", err)
	}
}

// GetActiveParticipants handles GET /streams/{id}/participants - retrieves active participants.
// Returns minimal participant info (no PII) for UI display.
func (h *StreamHandlers) GetActiveParticipants(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Extract stream ID from URL path
	// Expected: /streams/{id}/participants
	pathParts := strings.Split(strings.TrimPrefix(r.URL.Path, "/streams/"), "/")
	if len(pathParts) != 2 || pathParts[0] == "" || pathParts[1] != "participants" {
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

	// Get active count (efficient, uses denormalized field)
	activeCount := session.ActiveParticipantCount

	// Return participant count only (no PII)
	// Individual participant identities are not exposed to preserve privacy
	response := map[string]interface{}{
		"stream_id":    streamID,
		"active_count": activeCount,
		"room_name":    session.RoomName,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(response); err != nil {
		slog.ErrorContext(ctx, "failed to encode participants response", "error", err)
	}
}

// MuteParticipantRequest represents the request body for muting a participant.
type MuteParticipantRequest struct {
Muted bool `json:"muted"` // True to mute, false to unmute
}

// MuteParticipant handles POST /streams/{stream_id}/participants/{participant_id}/mute
// Mutes or unmutes a participant's audio in the stream.
// Only the stream host (organizer) can perform this action.
func (h *StreamHandlers) MuteParticipant(w http.ResponseWriter, r *http.Request) {
ctx := r.Context()

// Extract user DID from context (set by auth middleware)
userDID := middleware.GetUserDID(ctx)
if userDID == "" {
ctx = middleware.SetErrorCode(ctx, ErrCodeAuthFailed)
WriteError(w, ctx, http.StatusUnauthorized, ErrCodeAuthFailed, "Authentication required")
return
}

// Extract stream ID and participant ID from URL path
// Expected: /streams/{stream_id}/participants/{participant_id}/mute
pathParts := strings.Split(strings.TrimPrefix(r.URL.Path, "/streams/"), "/")
if len(pathParts) != 4 || pathParts[0] == "" || pathParts[1] != "participants" || pathParts[2] == "" || pathParts[3] != "mute" {
ctx = middleware.SetErrorCode(ctx, ErrCodeBadRequest)
WriteError(w, ctx, http.StatusBadRequest, ErrCodeBadRequest, "Invalid URL path")
return
}
streamID := pathParts[0]
participantID := pathParts[2]

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

// Verify that the user is the stream host (organizer)
if session.HostDID != userDID {
ctx = middleware.SetErrorCode(ctx, ErrCodeForbidden)
WriteError(w, ctx, http.StatusForbidden, ErrCodeForbidden, "Only the stream host can mute participants")
return
}

// Parse request body
var req MuteParticipantRequest
if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
ctx = middleware.SetErrorCode(ctx, ErrCodeBadRequest)
WriteError(w, ctx, http.StatusBadRequest, ErrCodeBadRequest, "Invalid JSON in request body")
return
}

// Check if room service is available
if h.roomService == nil {
ctx = middleware.SetErrorCode(ctx, ErrCodeInternal)
WriteError(w, ctx, http.StatusServiceUnavailable, ErrCodeInternal, "Stream control operations are not available")
return
}

// Get participant info from LiveKit to find audio tracks
participant, err := h.roomService.GetParticipant(ctx, session.RoomName, participantID)
if err != nil {
slog.ErrorContext(ctx, "failed to get participant from LiveKit",
"error", err,
"stream_id", streamID,
"participant_id", participantID,
)
ctx = middleware.SetErrorCode(ctx, ErrCodeNotFound)
WriteError(w, ctx, http.StatusNotFound, ErrCodeNotFound, "Participant not found in stream")
return
}

// Mute all audio tracks for the participant
var mutedTracks []string
for _, track := range participant.Tracks {
if track.Type.String() == "AUDIO" {
err = h.roomService.MuteParticipantTrack(ctx, session.RoomName, participantID, track.Sid, req.Muted)
if err != nil {
slog.ErrorContext(ctx, "failed to mute track",
"error", err,
"stream_id", streamID,
"participant_id", participantID,
"track_sid", track.Sid,
)
// Continue trying other tracks
} else {
mutedTracks = append(mutedTracks, track.Sid)
}
}
}

if len(mutedTracks) == 0 {
slog.WarnContext(ctx, "no audio tracks found or muted",
"stream_id", streamID,
"participant_id", participantID,
)
}

// Log action for audit
action := "muted"
if !req.Muted {
action = "unmuted"
}
auditEntry := audit.LogEntry{
UserDID:    userDID,
EntityType: "stream_participant",
EntityID:   streamID + ":" + participantID,
Action:     action,
RequestID:  middleware.GetRequestID(ctx),
}

if _, err := h.auditRepo.LogAccess(auditEntry); err != nil {
// Log error but don't fail the request
slog.ErrorContext(ctx, "failed to log mute audit entry",
"error", err,
"stream_id", streamID,
"participant_id", participantID,
)
}

// Return success response
response := map[string]interface{}{
"stream_id":      streamID,
"participant_id": participantID,
"muted":          req.Muted,
"tracks_muted":   len(mutedTracks),
}

w.Header().Set("Content-Type", "application/json")
w.WriteHeader(http.StatusOK)
if err := json.NewEncoder(w).Encode(response); err != nil {
slog.ErrorContext(ctx, "failed to encode mute response", "error", err)
}
}

// KickParticipant handles POST /streams/{stream_id}/participants/{participant_id}/kick
// Removes a participant from the stream.
// Only the stream host (organizer) can perform this action.
func (h *StreamHandlers) KickParticipant(w http.ResponseWriter, r *http.Request) {
ctx := r.Context()

// Extract user DID from context (set by auth middleware)
userDID := middleware.GetUserDID(ctx)
if userDID == "" {
ctx = middleware.SetErrorCode(ctx, ErrCodeAuthFailed)
WriteError(w, ctx, http.StatusUnauthorized, ErrCodeAuthFailed, "Authentication required")
return
}

// Extract stream ID and participant ID from URL path
// Expected: /streams/{stream_id}/participants/{participant_id}/kick
pathParts := strings.Split(strings.TrimPrefix(r.URL.Path, "/streams/"), "/")
if len(pathParts) != 4 || pathParts[0] == "" || pathParts[1] != "participants" || pathParts[2] == "" || pathParts[3] != "kick" {
ctx = middleware.SetErrorCode(ctx, ErrCodeBadRequest)
WriteError(w, ctx, http.StatusBadRequest, ErrCodeBadRequest, "Invalid URL path")
return
}
streamID := pathParts[0]
participantID := pathParts[2]

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

// Verify that the user is the stream host (organizer)
if session.HostDID != userDID {
ctx = middleware.SetErrorCode(ctx, ErrCodeForbidden)
WriteError(w, ctx, http.StatusForbidden, ErrCodeForbidden, "Only the stream host can kick participants")
return
}

// Check if room service is available
if h.roomService == nil {
ctx = middleware.SetErrorCode(ctx, ErrCodeInternal)
WriteError(w, ctx, http.StatusServiceUnavailable, ErrCodeInternal, "Stream control operations are not available")
return
}

// Remove participant from LiveKit room
err = h.roomService.RemoveParticipant(ctx, session.RoomName, participantID)
if err != nil {
slog.ErrorContext(ctx, "failed to remove participant from LiveKit",
"error", err,
"stream_id", streamID,
"participant_id", participantID,
)
ctx = middleware.SetErrorCode(ctx, ErrCodeInternal)
WriteError(w, ctx, http.StatusInternalServerError, ErrCodeInternal, "Failed to remove participant from stream")
return
}

// Log action for audit
auditEntry := audit.LogEntry{
UserDID:    userDID,
EntityType: "stream_participant",
EntityID:   streamID + ":" + participantID,
Action:     "kicked",
RequestID:  middleware.GetRequestID(ctx),
}

if _, err := h.auditRepo.LogAccess(auditEntry); err != nil {
// Log error but don't fail the request
slog.ErrorContext(ctx, "failed to log kick audit entry",
"error", err,
"stream_id", streamID,
"participant_id", participantID,
)
}

// Return success response
response := map[string]interface{}{
"stream_id":      streamID,
"participant_id": participantID,
"status":         "kicked",
}

w.Header().Set("Content-Type", "application/json")
w.WriteHeader(http.StatusOK)
if err := json.NewEncoder(w).Encode(response); err != nil {
slog.ErrorContext(ctx, "failed to encode kick response", "error", err)
}
}

// SetFeaturedParticipantRequest represents the request body for setting a featured participant.
type SetFeaturedParticipantRequest struct {
ParticipantID *string `json:"participant_id"` // Participant ID to feature, or null to clear
}

// SetFeaturedParticipant handles PATCH /streams/{stream_id}/featured_participant
// Sets or clears the featured (spotlighted) participant in the stream.
// Only the stream host (organizer) can perform this action.
func (h *StreamHandlers) SetFeaturedParticipant(w http.ResponseWriter, r *http.Request) {
ctx := r.Context()

// Extract user DID from context (set by auth middleware)
userDID := middleware.GetUserDID(ctx)
if userDID == "" {
ctx = middleware.SetErrorCode(ctx, ErrCodeAuthFailed)
WriteError(w, ctx, http.StatusUnauthorized, ErrCodeAuthFailed, "Authentication required")
return
}

// Extract stream ID from URL path
// Expected: /streams/{stream_id}/featured_participant
pathParts := strings.Split(strings.TrimPrefix(r.URL.Path, "/streams/"), "/")
if len(pathParts) != 2 || pathParts[0] == "" || pathParts[1] != "featured_participant" {
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

// Verify that the user is the stream host (organizer)
if session.HostDID != userDID {
ctx = middleware.SetErrorCode(ctx, ErrCodeForbidden)
WriteError(w, ctx, http.StatusForbidden, ErrCodeForbidden, "Only the stream host can set featured participant")
return
}

// Parse request body
var req SetFeaturedParticipantRequest
if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
ctx = middleware.SetErrorCode(ctx, ErrCodeBadRequest)
WriteError(w, ctx, http.StatusBadRequest, ErrCodeBadRequest, "Invalid JSON in request body")
return
}

// If participant is being set (not cleared), optionally validate they exist in the room
// This is a soft validation - we still update the database even if LiveKit validation fails
if req.ParticipantID != nil && *req.ParticipantID != "" && h.roomService != nil {
_, err := h.roomService.GetParticipant(ctx, session.RoomName, *req.ParticipantID)
if err != nil {
slog.WarnContext(ctx, "featured participant not found in LiveKit room",
"error", err,
"stream_id", streamID,
"participant_id", *req.ParticipantID,
)
// Continue anyway - participant might join later
}
}

// Update featured participant in database
err = h.streamRepo.SetFeaturedParticipant(streamID, req.ParticipantID)
if err != nil {
slog.ErrorContext(ctx, "failed to set featured participant",
"error", err,
"stream_id", streamID,
)
ctx = middleware.SetErrorCode(ctx, ErrCodeInternal)
WriteError(w, ctx, http.StatusInternalServerError, ErrCodeInternal, "Failed to set featured participant")
return
}

// Log action for audit
action := "featured_participant_set"
if req.ParticipantID == nil || *req.ParticipantID == "" {
action = "featured_participant_cleared"
}
auditEntry := audit.LogEntry{
UserDID:    userDID,
EntityType: "stream_session",
EntityID:   streamID,
Action:     action,
RequestID:  middleware.GetRequestID(ctx),
}

if _, err := h.auditRepo.LogAccess(auditEntry); err != nil {
// Log error but don't fail the request
slog.ErrorContext(ctx, "failed to log featured participant audit entry",
"error", err,
"stream_id", streamID,
)
}

// Return success response
response := map[string]interface{}{
"stream_id":            streamID,
"featured_participant": req.ParticipantID,
}

w.Header().Set("Content-Type", "application/json")
w.WriteHeader(http.StatusOK)
if err := json.NewEncoder(w).Encode(response); err != nil {
slog.ErrorContext(ctx, "failed to encode featured participant response", "error", err)
}
}

// LockStreamRequest represents the request body for locking/unlocking a stream.
type LockStreamRequest struct {
Locked bool `json:"locked"` // True to lock, false to unlock
}

// LockStream handles PATCH /streams/{stream_id}/lock
// Locks or unlocks the stream to prevent new participants from joining.
// Only the stream host (organizer) can perform this action.
func (h *StreamHandlers) LockStream(w http.ResponseWriter, r *http.Request) {
ctx := r.Context()

// Extract user DID from context (set by auth middleware)
userDID := middleware.GetUserDID(ctx)
if userDID == "" {
ctx = middleware.SetErrorCode(ctx, ErrCodeAuthFailed)
WriteError(w, ctx, http.StatusUnauthorized, ErrCodeAuthFailed, "Authentication required")
return
}

// Extract stream ID from URL path
// Expected: /streams/{stream_id}/lock
pathParts := strings.Split(strings.TrimPrefix(r.URL.Path, "/streams/"), "/")
if len(pathParts) != 2 || pathParts[0] == "" || pathParts[1] != "lock" {
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

// Verify that the user is the stream host (organizer)
if session.HostDID != userDID {
ctx = middleware.SetErrorCode(ctx, ErrCodeForbidden)
WriteError(w, ctx, http.StatusForbidden, ErrCodeForbidden, "Only the stream host can lock the stream")
return
}

// Parse request body
var req LockStreamRequest
if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
ctx = middleware.SetErrorCode(ctx, ErrCodeBadRequest)
WriteError(w, ctx, http.StatusBadRequest, ErrCodeBadRequest, "Invalid JSON in request body")
return
}

// Update lock status in database
err = h.streamRepo.SetLockStatus(streamID, req.Locked)
if err != nil {
slog.ErrorContext(ctx, "failed to set lock status",
"error", err,
"stream_id", streamID,
)
ctx = middleware.SetErrorCode(ctx, ErrCodeInternal)
WriteError(w, ctx, http.StatusInternalServerError, ErrCodeInternal, "Failed to update stream lock status")
return
}

// Log action for audit
action := "locked"
if !req.Locked {
action = "unlocked"
}
auditEntry := audit.LogEntry{
UserDID:    userDID,
EntityType: "stream_session",
EntityID:   streamID,
Action:     action,
RequestID:  middleware.GetRequestID(ctx),
}

if _, err := h.auditRepo.LogAccess(auditEntry); err != nil {
// Log error but don't fail the request
slog.ErrorContext(ctx, "failed to log lock audit entry",
"error", err,
"stream_id", streamID,
)
}

// Return success response
response := map[string]interface{}{
"stream_id": streamID,
"locked":    req.Locked,
}

w.Header().Set("Content-Type", "application/json")
w.WriteHeader(http.StatusOK)
if err := json.NewEncoder(w).Encode(response); err != nil {
slog.ErrorContext(ctx, "failed to encode lock response", "error", err)
}
}
