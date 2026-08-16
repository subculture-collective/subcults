// Package api provides HTTP handlers for the Subcults API.
package api

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"net/url"
	"strings"

	"github.com/onnwee/subcults/internal/audit"
	"github.com/onnwee/subcults/internal/membership"
	"github.com/onnwee/subcults/internal/middleware"
	"github.com/onnwee/subcults/internal/scene"
)

// MembershipHandlers holds dependencies for membership HTTP handlers.
type MembershipHandlers struct {
	membershipService *membership.Service
	sceneRepo         scene.SceneRepository
	auditRepo         audit.Repository
}

// NewMembershipHandlers creates a new MembershipHandlers instance.
func NewMembershipHandlers(
	membershipService *membership.Service,
	sceneRepo scene.SceneRepository,
	auditRepo audit.Repository,
) *MembershipHandlers {
	return &MembershipHandlers{
		membershipService: membershipService,
		sceneRepo:         sceneRepo,
		auditRepo:         auditRepo,
	}
}

// RequestMembership handles POST /scenes/{id}/membership/request
// Creates a pending membership request for the authenticated user.
func (h *MembershipHandlers) RequestMembership(w http.ResponseWriter, r *http.Request) {
	// Extract scene ID from URL path
	pathParts := strings.Split(strings.TrimPrefix(r.URL.Path, "/scenes/"), "/")
	if len(pathParts) < 2 || pathParts[0] == "" {
		ctx := middleware.SetErrorCode(r.Context(), ErrCodeBadRequest)
		WriteError(w, ctx, http.StatusBadRequest, ErrCodeBadRequest, "Scene ID is required")
		return
	}
	sceneID := pathParts[0]

	// Get authenticated user DID from context
	userDID := middleware.GetUserDID(r.Context())
	if userDID == "" {
		ctx := middleware.SetErrorCode(r.Context(), ErrCodeAuthFailed)
		WriteError(w, ctx, http.StatusUnauthorized, ErrCodeAuthFailed, "Authentication required")
		return
	}

	// Verify scene exists and user is not the owner
	existingScene, err := h.sceneRepo.GetByID(sceneID)
	if err != nil {
		if err == scene.ErrSceneNotFound {
			ctx := middleware.SetErrorCode(r.Context(), ErrCodeNotFound)
			WriteError(w, ctx, http.StatusNotFound, ErrCodeNotFound, "Scene not found")
			return
		}
		slog.ErrorContext(r.Context(), "failed to retrieve scene", "error", err, "scene_id", sceneID)
		ctx := middleware.SetErrorCode(r.Context(), ErrCodeInternal)
		WriteError(w, ctx, http.StatusInternalServerError, ErrCodeInternal, "Failed to retrieve scene")
		return
	}

	if existingScene.OwnerDID == userDID {
		ctx := middleware.SetErrorCode(r.Context(), ErrCodeConflict)
		WriteError(w, ctx, http.StatusConflict, ErrCodeConflict, "Scene owner cannot request membership")
		return
	}

	// Delegate to service
	createdMembership, err := h.membershipService.CreateRequest(sceneID, userDID)
	if err != nil {
		slog.ErrorContext(r.Context(), "failed to create membership request", "error", err, "scene_id", sceneID, "user_did", userDID)
		ctx := middleware.SetErrorCode(r.Context(), ErrCodeInternal)
		WriteAPIError(w, ctx, err)
		return
	}

	// Audit log the membership request
	if h.auditRepo != nil {
		if auditErr := audit.LogAccessFromRequest(r, h.auditRepo, "membership", createdMembership.ID, "membership_request", audit.OutcomeSuccess); auditErr != nil {
			slog.WarnContext(r.Context(), "failed to log membership request audit", "error", auditErr, "membership_id", createdMembership.ID)
		}
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	if err := json.NewEncoder(w).Encode(createdMembership); err != nil {
		return
	}
}

// ApproveMembership handles POST /scenes/{id}/membership/{userId}/approve
// Approves a pending membership request (scene owner only).
func (h *MembershipHandlers) ApproveMembership(w http.ResponseWriter, r *http.Request) {
	sceneID, targetUserDID, ok := h.parseApproveRejectPath(w, r)
	if !ok {
		return
	}

	// Get authenticated user DID from context
	ownerDID := middleware.GetUserDID(r.Context())
	if ownerDID == "" {
		ctx := middleware.SetErrorCode(r.Context(), ErrCodeAuthFailed)
		WriteError(w, ctx, http.StatusUnauthorized, ErrCodeAuthFailed, "Authentication required")
		return
	}

	// Verify scene exists and user is owner
	existingScene, err := h.sceneRepo.GetByID(sceneID)
	if err != nil {
		if err == scene.ErrSceneNotFound {
			ctx := middleware.SetErrorCode(r.Context(), ErrCodeNotFound)
			WriteError(w, ctx, http.StatusNotFound, ErrCodeNotFound, "Scene not found")
			return
		}
		slog.ErrorContext(r.Context(), "failed to retrieve scene", "error", err, "scene_id", sceneID)
		ctx := middleware.SetErrorCode(r.Context(), ErrCodeInternal)
		WriteError(w, ctx, http.StatusInternalServerError, ErrCodeInternal, "Failed to retrieve scene")
		return
	}

	if existingScene.OwnerDID != ownerDID {
		ctx := middleware.SetErrorCode(r.Context(), ErrCodeForbidden)
		WriteError(w, ctx, http.StatusForbidden, ErrCodeForbidden, "Only scene owner can approve memberships")
		return
	}

	// Delegate to service
	updatedMembership, err := h.membershipService.UpdateRequestStatus(sceneID, targetUserDID, "active")
	if err != nil {
		slog.ErrorContext(r.Context(), "failed to approve membership", "error", err, "scene_id", sceneID, "user_did", targetUserDID)
		ctx := middleware.SetErrorCode(r.Context(), ErrCodeInternal)
		WriteAPIError(w, ctx, err)
		return
	}

	// Audit log the approval
	if h.auditRepo != nil {
		if auditErr := audit.LogAccessFromRequest(r, h.auditRepo, "membership", updatedMembership.ID, "membership_approve", audit.OutcomeSuccess); auditErr != nil {
			slog.WarnContext(r.Context(), "failed to log membership approval audit", "error", auditErr, "membership_id", updatedMembership.ID)
		}
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(updatedMembership); err != nil {
		return
	}
}

// RejectMembership handles POST /scenes/{id}/membership/{userId}/reject
// Rejects a pending membership request (scene owner only).
func (h *MembershipHandlers) RejectMembership(w http.ResponseWriter, r *http.Request) {
	sceneID, targetUserDID, ok := h.parseApproveRejectPath(w, r)
	if !ok {
		return
	}

	// Get authenticated user DID from context
	ownerDID := middleware.GetUserDID(r.Context())
	if ownerDID == "" {
		ctx := middleware.SetErrorCode(r.Context(), ErrCodeAuthFailed)
		WriteError(w, ctx, http.StatusUnauthorized, ErrCodeAuthFailed, "Authentication required")
		return
	}

	// Verify scene exists and user is owner
	existingScene, err := h.sceneRepo.GetByID(sceneID)
	if err != nil {
		if err == scene.ErrSceneNotFound {
			ctx := middleware.SetErrorCode(r.Context(), ErrCodeNotFound)
			WriteError(w, ctx, http.StatusNotFound, ErrCodeNotFound, "Scene not found")
			return
		}
		slog.ErrorContext(r.Context(), "failed to retrieve scene", "error", err, "scene_id", sceneID)
		ctx := middleware.SetErrorCode(r.Context(), ErrCodeInternal)
		WriteError(w, ctx, http.StatusInternalServerError, ErrCodeInternal, "Failed to retrieve scene")
		return
	}

	if existingScene.OwnerDID != ownerDID {
		ctx := middleware.SetErrorCode(r.Context(), ErrCodeForbidden)
		WriteError(w, ctx, http.StatusForbidden, ErrCodeForbidden, "Only scene owner can reject memberships")
		return
	}

	// Delegate to service
	updatedMembership, err := h.membershipService.UpdateRequestStatus(sceneID, targetUserDID, "rejected")
	if err != nil {
		slog.ErrorContext(r.Context(), "failed to reject membership", "error", err, "scene_id", sceneID, "user_did", targetUserDID)
		ctx := middleware.SetErrorCode(r.Context(), ErrCodeInternal)
		WriteAPIError(w, ctx, err)
		return
	}

	// Audit log the rejection
	if h.auditRepo != nil {
		if auditErr := audit.LogAccessFromRequest(r, h.auditRepo, "membership", updatedMembership.ID, "membership_reject", audit.OutcomeSuccess); auditErr != nil {
			slog.WarnContext(r.Context(), "failed to log membership rejection audit", "error", auditErr, "membership_id", updatedMembership.ID)
		}
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(updatedMembership); err != nil {
		return
	}
}

// parseApproveRejectPath extracts scene ID and target user DID from the URL
// path for approve/reject endpoints. Returns false and writes error response
// if parsing fails.
func (h *MembershipHandlers) parseApproveRejectPath(w http.ResponseWriter, r *http.Request) (sceneID, targetUserDID string, ok bool) {
	pathParts := strings.Split(strings.TrimPrefix(r.URL.Path, "/scenes/"), "/")
	if len(pathParts) < 4 || pathParts[0] == "" || pathParts[2] == "" {
		ctx := middleware.SetErrorCode(r.Context(), ErrCodeBadRequest)
		WriteError(w, ctx, http.StatusBadRequest, ErrCodeBadRequest, "Scene ID and User DID are required")
		return "", "", false
	}
	sceneID = pathParts[0]

	var err error
	targetUserDID, err = url.PathUnescape(pathParts[2])
	if err != nil {
		ctx := middleware.SetErrorCode(r.Context(), ErrCodeBadRequest)
		WriteError(w, ctx, http.StatusBadRequest, ErrCodeBadRequest, "Invalid user DID in URL")
		return "", "", false
	}
	return sceneID, targetUserDID, true
}