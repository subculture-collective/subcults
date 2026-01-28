// Package api provides HTTP handlers for the Subcults API.
package api

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"regexp"
	"strings"

	"github.com/onnwee/subcults/internal/audit"
	"github.com/onnwee/subcults/internal/livekit"
	"github.com/onnwee/subcults/internal/middleware"
)

// LiveKitTokenRequest represents the request body for generating a LiveKit token.
type LiveKitTokenRequest struct {
	RoomID  string  `json:"room_id"`            // Required: Room identifier
	SceneID *string `json:"scene_id,omitempty"` // Optional: Associated scene ID
	EventID *string `json:"event_id,omitempty"` // Optional: Associated event ID
}

// LiveKitTokenResponse represents the response for a LiveKit token request.
type LiveKitTokenResponse struct {
	Token     string `json:"token"`      // The JWT access token
	ExpiresAt string `json:"expires_at"` // Token expiration time in RFC3339 format
}

// LiveKitHandlers holds dependencies for LiveKit HTTP handlers.
type LiveKitHandlers struct {
	tokenService *livekit.TokenService
	auditRepo    audit.Repository
}

// NewLiveKitHandlers creates a new LiveKitHandlers instance.
func NewLiveKitHandlers(tokenService *livekit.TokenService, auditRepo audit.Repository) *LiveKitHandlers {
	return &LiveKitHandlers{
		tokenService: tokenService,
		auditRepo:    auditRepo,
	}
}

// Room ID validation: alphanumeric, hyphens, underscores, colons (max 128 chars)
// This prevents injection attacks and restricts to safe characters.
var roomIDPattern = regexp.MustCompile(`^[a-zA-Z0-9_:-]{1,128}$`)

// validateRoomID checks if the room ID matches the allowed pattern.
func validateRoomID(roomID string) bool {
	return roomIDPattern.MatchString(roomID)
}

// IssueToken handles POST /livekit/token requests.
// Generates a short-lived LiveKit access token for authenticated users.
// Requires valid JWT authentication and associates the user's DID with the LiveKit session.
func (h *LiveKitHandlers) IssueToken(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Extract user DID from context (set by auth middleware)
	userDID := middleware.GetUserDID(ctx)
	if userDID == "" {
		ctx = middleware.SetErrorCode(ctx, ErrCodeAuthFailed)
		WriteError(w, ctx, http.StatusUnauthorized, ErrCodeAuthFailed, "Authentication required")
		return
	}

	// Parse request body
	var req LiveKitTokenRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		ctx = middleware.SetErrorCode(ctx, ErrCodeBadRequest)
		WriteError(w, ctx, http.StatusBadRequest, ErrCodeBadRequest, "Invalid request body")
		return
	}

	// Validate room ID
	req.RoomID = strings.TrimSpace(req.RoomID)
	if req.RoomID == "" {
		ctx = middleware.SetErrorCode(ctx, ErrCodeValidation)
		WriteError(w, ctx, http.StatusBadRequest, ErrCodeValidation, "room_id is required")
		return
	}

	if !validateRoomID(req.RoomID) {
		ctx = middleware.SetErrorCode(ctx, ErrCodeValidation)
		WriteError(w, ctx, http.StatusBadRequest, ErrCodeValidation, "room_id contains invalid characters or exceeds maximum length")
		return
	}

	// TODO: Future enhancement - verify membership if room is restricted
	// For now, any authenticated user can join any room

	// Generate participant identity: user-{uuid}
	// Extract UUID from DID if possible, otherwise use a new UUID
	participantID := generateParticipantID(userDID)

	// Prepare metadata
	metadata := make(map[string]interface{})
	metadata["did"] = userDID
	if req.SceneID != nil && *req.SceneID != "" {
		metadata["sceneId"] = *req.SceneID
	}
	if req.EventID != nil && *req.EventID != "" {
		metadata["eventId"] = *req.EventID
	}

	// Generate LiveKit token
	tokenReq := &livekit.TokenRequest{
		RoomName: req.RoomID,
		Identity: participantID,
		Expiry:   0, // Use default expiry (5 minutes)
		Metadata: metadata,
	}

	tokenResp, err := h.tokenService.GenerateToken(tokenReq)
	if err != nil {
		slog.ErrorContext(ctx, "failed to generate LiveKit token",
			"error", err,
			"room_id", req.RoomID,
			"user_did", userDID,
		)
		ctx = middleware.SetErrorCode(ctx, ErrCodeInternal)
		WriteError(w, ctx, http.StatusInternalServerError, ErrCodeInternal, "Failed to generate token")
		return
	}

	// Log token issuance for audit (never log the token itself)
	// Use "livekit_room" as entity type and room_id as entity ID
	auditEntry := audit.LogEntry{
		UserDID:    userDID,
		EntityType: "livekit_room",
		EntityID:   req.RoomID,
		Action:     "token_issued",
		RequestID:  middleware.GetRequestID(ctx),
	}

	if _, err := h.auditRepo.LogAccess(auditEntry); err != nil {
		// Log error but don't fail the request
		slog.ErrorContext(ctx, "failed to log token issuance audit entry",
			"error", err,
			"room_id", req.RoomID,
			"user_did", userDID,
		)
	}

	// Return token response
	response := LiveKitTokenResponse{
		Token:     tokenResp.Token,
		ExpiresAt: tokenResp.ExpiresAt.Format("2006-01-02T15:04:05Z07:00"), // RFC3339
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(response); err != nil {
		slog.ErrorContext(ctx, "failed to encode token response", "error", err)
	}
}

// generateParticipantID creates a deterministic participant identity from a user DID.
// Format: user-{stable-identifier}
//
// Note: This generates a stable identity based on the user's DID, ensuring that
// the same user always gets the same participant ID. This is important for LiveKit's
// room management, where participants with the same identity are treated as the same user,
// enabling features like:
// - Reconnection to the same session after temporary disconnection
// - Consistent participant tracking across multiple join attempts
// - Proper cleanup of previous sessions when rejoining
//
// The identifier is extracted from the DID and truncated if needed to maintain reasonable length.
func generateParticipantID(did string) string {
	// DIDs have format: did:method:identifier (e.g., did:plc:abc123...)
	// We'll use the identifier part to create a stable ID

	// Split on colons and take the last part (the identifier)
	parts := strings.Split(did, ":")
	var identifier string

	if len(parts) >= 3 {
		// Use the identifier portion (last part)
		identifier = parts[len(parts)-1]
	} else {
		// Fallback: if DID format is unexpected, use the whole DID
		identifier = did
	}

	// Ensure identifier is safe for LiveKit (alphanumeric, hyphens, underscores)
	// and truncate to reasonable length (max 48 chars to keep total under 64)
	maxLen := 48
	if len(identifier) > maxLen {
		identifier = identifier[:maxLen]
	}

	return fmt.Sprintf("user-%s", identifier)
}
