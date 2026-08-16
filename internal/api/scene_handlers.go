// Package api provides HTTP handlers for the Subcults API.
package api

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/onnwee/subcults/internal/membership"
	"github.com/onnwee/subcults/internal/middleware"
	"github.com/onnwee/subcults/internal/scene"
	"github.com/onnwee/subcults/internal/stream"
)

// CreateSceneRequest represents the request body for creating a scene.
type CreateSceneRequest struct {
	Name          string         `json:"name"`
	Description   string         `json:"description,omitempty"`
	OwnerDID      string         `json:"owner_did"`
	AllowPrecise  bool           `json:"allow_precise"`
	PrecisePoint  *scene.Point   `json:"precise_point,omitempty"`
	CoarseGeohash string         `json:"coarse_geohash"`
	Tags          []string       `json:"tags,omitempty"`
	Visibility    string         `json:"visibility,omitempty"`
	Palette       *scene.Palette `json:"palette,omitempty"`
}

// UpdateSceneRequest represents the request body for updating a scene.
// Only includes mutable fields (owner is immutable).
type UpdateSceneRequest struct {
	Version      int64          `json:"version"`
	Name         *string        `json:"name,omitempty"`
	Description  *string        `json:"description,omitempty"`
	Tags         []string       `json:"tags,omitempty"`
	Visibility   *string        `json:"visibility,omitempty"`
	Palette      *scene.Palette `json:"palette,omitempty"`
	AllowPrecise *bool          `json:"allow_precise,omitempty"`
	PrecisePoint *scene.Point   `json:"precise_point,omitempty"`
}

// UpdateScenePaletteRequest represents the request body for updating scene palette.
type UpdateScenePaletteRequest struct {
	Palette scene.Palette `json:"palette"`
}

// SceneHandlers holds dependencies for scene HTTP handlers.
type SceneHandlers struct {
	sceneService   *scene.Service
	membershipRepo membership.MembershipRepository
	streamRepo     stream.SessionRepository
}

// NewSceneHandlers creates a new SceneHandlers instance.
func NewSceneHandlers(sceneService *scene.Service, membershipRepo membership.MembershipRepository, streamRepo stream.SessionRepository) *SceneHandlers {
	return &SceneHandlers{
		sceneService:   sceneService,
		membershipRepo: membershipRepo,
		streamRepo:     streamRepo,
	}
}

// CreateScene handles POST /scenes - creates a new scene.
func (h *SceneHandlers) CreateScene(w http.ResponseWriter, r *http.Request) {
	var req CreateSceneRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		ctx := middleware.SetErrorCode(r.Context(), ErrCodeBadRequest)
		WriteError(w, ctx, http.StatusBadRequest, ErrCodeBadRequest, "Invalid JSON in request body")
		return
	}

	// Studio ownership comes from the verified bearer token, never request JSON.
	if strings.Contains(r.URL.Path, "/studio/") {
		req.OwnerDID = middleware.GetUserDID(r.Context())
	}

	// Determine publication status from URL path
	publicationStatus := "published"
	if strings.Contains(r.URL.Path, "/studio/") {
		publicationStatus = "draft"
	}

	scene, err := h.sceneService.CreateScene(
		r.Context(),
		req.Name,
		req.Description,
		req.OwnerDID,
		req.CoarseGeohash,
		req.Tags,
		req.Visibility,
		req.Palette,
		req.AllowPrecise,
		req.PrecisePoint,
		publicationStatus,
	)
	if err != nil {
		WriteAPIError(w, r.Context(), err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	if err := json.NewEncoder(w).Encode(scene); err != nil {
		return
	}
}

// GetScene handles GET /scenes/{id} - retrieves a scene with visibility enforcement.
func (h *SceneHandlers) GetScene(w http.ResponseWriter, r *http.Request) {
	// Extract scene ID from URL path
	pathParts := strings.Split(strings.TrimPrefix(r.URL.Path, "/scenes/"), "/")
	if len(pathParts) == 0 || pathParts[0] == "" {
		ctx := middleware.SetErrorCode(r.Context(), ErrCodeBadRequest)
		WriteError(w, ctx, http.StatusBadRequest, ErrCodeBadRequest, "Scene ID is required")
		return
	}
	sceneID := pathParts[0]

	// Get the scene
	foundScene, err := h.sceneService.GetScene(r.Context(), sceneID)
	if err != nil {
		// Handle deleted scenes with specific error code
		if err == scene.ErrSceneDeleted {
			slog.DebugContext(r.Context(), "scene deleted", "scene_id", sceneID)
			ctx := middleware.SetErrorCode(r.Context(), ErrCodeSceneDeleted)
			WriteError(w, ctx, http.StatusNotFound, ErrCodeSceneDeleted, "Scene not found")
			return
		}
		WriteAPIError(w, r.Context(), err)
		return
	}

	// Get requester DID (empty if not authenticated)
	requesterDID := middleware.GetUserDID(r.Context())

	// Check visibility permissions
	canAccess, err := h.canAccessScene(r.Context(), foundScene, requesterDID)
	if err != nil {
		slog.ErrorContext(r.Context(), "failed to check scene access", "error", err, "scene_id", sceneID)
		ctx := middleware.SetErrorCode(r.Context(), ErrCodeInternal)
		WriteError(w, ctx, http.StatusInternalServerError, ErrCodeInternal, "Failed to check access permissions")
		return
	}

	if !canAccess {
		slog.DebugContext(r.Context(), "scene access denied",
			"scene_id", sceneID,
			"visibility", foundScene.Visibility,
			"requester_did", requesterDID,
			"is_owner", foundScene.IsOwner(requesterDID))
		ctx := middleware.SetErrorCode(r.Context(), ErrCodeNotFound)
		WriteError(w, ctx, http.StatusNotFound, ErrCodeNotFound, "Scene not found")
		return
	}

	slog.DebugContext(r.Context(), "scene access granted",
		"scene_id", sceneID,
		"visibility", foundScene.Visibility,
		"requester_did", requesterDID)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(foundScene); err != nil {
		return
	}
}

// canAccessScene checks if a user can access a scene based on visibility rules.
func (h *SceneHandlers) canAccessScene(ctx context.Context, s *scene.Scene, requesterDID string) (bool, error) {
	// Owner always has access
	if s.IsOwner(requesterDID) {
		return true, nil
	}

	switch s.Visibility {
	case scene.VisibilityPublic:
		return true, nil

	case scene.VisibilityMembersOnly:
		if requesterDID == "" {
			return false, nil
		}
		m, err := h.membershipRepo.GetBySceneAndUser(s.ID, requesterDID)
		if err != nil {
			if err == membership.ErrMembershipNotFound {
				return false, nil
			}
			return false, err
		}
		return m.Status == "active", nil

	case scene.VisibilityHidden:
		return false, nil

	default:
		slog.WarnContext(ctx, "unknown visibility mode", "visibility", s.Visibility, "scene_id", s.ID)
		return false, nil
	}
}

// UpdateScene handles PATCH /scenes/{id} - updates an existing scene.
func (h *SceneHandlers) UpdateScene(w http.ResponseWriter, r *http.Request) {
	pathParts := strings.Split(strings.TrimPrefix(r.URL.Path, "/scenes/"), "/")
	if len(pathParts) == 0 || pathParts[0] == "" {
		ctx := middleware.SetErrorCode(r.Context(), ErrCodeBadRequest)
		WriteError(w, ctx, http.StatusBadRequest, ErrCodeBadRequest, "Scene ID is required")
		return
	}
	sceneID := pathParts[0]

	// Verify authenticated user
	userDID := middleware.GetUserDID(r.Context())
	if userDID == "" {
		ctx := middleware.SetErrorCode(r.Context(), ErrCodeAuthFailed)
		WriteError(w, ctx, http.StatusUnauthorized, ErrCodeAuthFailed, "Authentication required")
		return
	}

	// Parse request body
	var req UpdateSceneRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		ctx := middleware.SetErrorCode(r.Context(), ErrCodeBadRequest)
		WriteError(w, ctx, http.StatusBadRequest, ErrCodeBadRequest, "Invalid JSON in request body")
		return
	}

	// Get existing scene to verify ownership
	existingScene, err := h.sceneService.GetScene(r.Context(), sceneID)
	if err != nil {
		WriteAPIError(w, r.Context(), err)
		return
	}

	// Verify ownership
	if !existingScene.IsOwner(userDID) {
		ctx := middleware.SetErrorCode(r.Context(), ErrCodeForbidden)
		WriteError(w, ctx, http.StatusForbidden, ErrCodeForbidden, "You do not have permission to update this scene")
		return
	}

	updated, err := h.sceneService.UpdateScene(r.Context(), sceneID, scene.UpdateSceneParams{
		Version:     req.Version,
		Name:        req.Name,
		Description: req.Description,
		Tags:        req.Tags,
		Visibility:  req.Visibility,
		Palette:     req.Palette,
		AllowPrecise: req.AllowPrecise,
		PrecisePoint: req.PrecisePoint,
	})
	if err != nil {
		WriteAPIError(w, r.Context(), err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(updated); err != nil {
		return
	}
}

// DeleteScene handles DELETE /scenes/{id} - soft-deletes a scene.
func (h *SceneHandlers) DeleteScene(w http.ResponseWriter, r *http.Request) {
	pathParts := strings.Split(strings.TrimPrefix(r.URL.Path, "/scenes/"), "/")
	if len(pathParts) == 0 || pathParts[0] == "" {
		ctx := middleware.SetErrorCode(r.Context(), ErrCodeBadRequest)
		WriteError(w, ctx, http.StatusBadRequest, ErrCodeBadRequest, "Scene ID is required")
		return
	}
	sceneID := pathParts[0]

	// Verify authenticated user
	userDID := middleware.GetUserDID(r.Context())
	if userDID == "" {
		ctx := middleware.SetErrorCode(r.Context(), ErrCodeAuthFailed)
		WriteError(w, ctx, http.StatusUnauthorized, ErrCodeAuthFailed, "Authentication required")
		return
	}

	// Get existing scene to verify ownership
	existingScene, err := h.sceneService.GetScene(r.Context(), sceneID)
	if err != nil {
		WriteAPIError(w, r.Context(), err)
		return
	}

	// Verify ownership
	if !existingScene.IsOwner(userDID) {
		ctx := middleware.SetErrorCode(r.Context(), ErrCodeForbidden)
		WriteError(w, ctx, http.StatusForbidden, ErrCodeForbidden, "You do not have permission to delete this scene")
		return
	}

	// Soft delete the scene
	if err := h.sceneService.DeleteScene(r.Context(), sceneID); err != nil {
		WriteAPIError(w, r.Context(), err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// UpdateScenePalette handles PATCH /scenes/{id}/palette - updates scene color palette.
func (h *SceneHandlers) UpdateScenePalette(w http.ResponseWriter, r *http.Request) {
	pathParts := strings.Split(strings.TrimPrefix(r.URL.Path, "/scenes/"), "/")
	if len(pathParts) < 2 || pathParts[0] == "" {
		ctx := middleware.SetErrorCode(r.Context(), ErrCodeBadRequest)
		WriteError(w, ctx, http.StatusBadRequest, ErrCodeBadRequest, "Scene ID is required")
		return
	}
	sceneID := pathParts[0]

	// Parse request body
	var req UpdateScenePaletteRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		ctx := middleware.SetErrorCode(r.Context(), ErrCodeBadRequest)
		WriteError(w, ctx, http.StatusBadRequest, ErrCodeBadRequest, "Invalid JSON in request body")
		return
	}

	// Get existing scene first to check ownership
	existingScene, err := h.sceneService.GetScene(r.Context(), sceneID)
	if err != nil {
		WriteAPIError(w, r.Context(), err)
		return
	}

	// Authorization: Only the owner can update the palette
	userDID := middleware.GetUserDID(r.Context())
	if userDID == "" {
		ctx := middleware.SetErrorCode(r.Context(), ErrCodeAuthFailed)
		WriteError(w, ctx, http.StatusUnauthorized, ErrCodeAuthFailed, "Authentication required")
		return
	}
	if existingScene.OwnerDID != userDID {
		ctx := middleware.SetErrorCode(r.Context(), ErrCodeForbidden)
		WriteError(w, ctx, http.StatusForbidden, ErrCodeForbidden, "Forbidden: you do not own this scene")
		return
	}

	// Delegate palette validation and persistence to the service
	updated, err := h.sceneService.UpdateScenePalette(r.Context(), sceneID, &req.Palette)
	if err != nil {
		// If it's a color validation error, use the specific error code
		if _, ok := err.(interface{ InvalidPalette() bool }); ok {
			ctx := middleware.SetErrorCode(r.Context(), ErrCodeInvalidPalette)
			WriteError(w, ctx, http.StatusBadRequest, ErrCodeInvalidPalette, err.Error())
			return
		}
		// Check for common color validation patterns
		errMsg := err.Error()
		if strings.Contains(errMsg, "invalid hex color format") ||
			strings.Contains(errMsg, "color is required") ||
			strings.Contains(errMsg, "contrast") ||
			strings.Contains(errMsg, "Insufficient") {
			ctx := middleware.SetErrorCode(r.Context(), ErrCodeInvalidPalette)
			WriteError(w, ctx, http.StatusBadRequest, ErrCodeInvalidPalette, errMsg)
			return
		}
		WriteAPIError(w, r.Context(), err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(updated); err != nil {
		return
	}
}

// formatRatio formats a contrast ratio to 1 decimal place, removing trailing zeros.
func formatRatio(ratio float64) string {
	formatted := fmt.Sprintf("%.1f", ratio)
	formatted = strings.TrimRight(formatted, "0")
	formatted = strings.TrimRight(formatted, ".")
	return formatted
}

// OwnedSceneSummary represents a summary of a scene owned by the user.
type OwnedSceneSummary struct {
	ID              string     `json:"id"`
	Name            string     `json:"name"`
	Description     string     `json:"description,omitempty"`
	CoarseGeohash   string     `json:"coarse_geohash"`
	Tags            []string   `json:"tags,omitempty"`
	Visibility      string     `json:"visibility"`
	CreatedAt       *time.Time `json:"created_at,omitempty"`
	UpdatedAt       *time.Time `json:"updated_at,omitempty"`
	MembersCount    int        `json:"members_count"`
	HasActiveStream bool       `json:"has_active_stream"`
}

// ListOwnedScenes handles GET /scenes/owned - lists all scenes owned by the authenticated user.
func (h *SceneHandlers) ListOwnedScenes(w http.ResponseWriter, r *http.Request) {
	// Get authenticated user DID
	userDID := middleware.GetUserDID(r.Context())
	if userDID == "" {
		ctx := middleware.SetErrorCode(r.Context(), ErrCodeAuthFailed)
		WriteError(w, ctx, http.StatusUnauthorized, ErrCodeAuthFailed, "Authentication required")
		return
	}

	// Get all scenes owned by user
	scenes, err := h.sceneService.ListScenesByOwner(r.Context(), userDID)
	if err != nil {
		slog.ErrorContext(r.Context(), "failed to list owned scenes", "error", err, "user_did", userDID)
		ctx := middleware.SetErrorCode(r.Context(), ErrCodeInternal)
		WriteError(w, ctx, http.StatusInternalServerError, ErrCodeInternal, "Failed to retrieve scenes")
		return
	}

	// Early return if no scenes
	if len(scenes) == 0 {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		if err := json.NewEncoder(w).Encode([]OwnedSceneSummary{}); err != nil {
			slog.ErrorContext(r.Context(), "failed to encode response", "error", err)
		}
		return
	}

	// Collect all scene IDs for batch queries
	sceneIDs := make([]string, len(scenes))
	for i, sc := range scenes {
		sceneIDs[i] = sc.ID
	}

	// Batch query for membership counts (avoids N+1 query problem)
	membershipCounts, err := h.membershipRepo.CountByScenes(sceneIDs, "active")
	if err != nil {
		slog.ErrorContext(r.Context(), "failed to count memberships", "error", err, "user_did", userDID)
		ctx := middleware.SetErrorCode(r.Context(), ErrCodeInternal)
		WriteError(w, ctx, http.StatusInternalServerError, ErrCodeInternal, "Failed to retrieve membership counts")
		return
	}

	// Batch query for active streams (avoids N+1 query problem)
	activeStreams, err := h.streamRepo.HasActiveStreamsForScenes(sceneIDs)
	if err != nil {
		slog.ErrorContext(r.Context(), "failed to check active streams", "error", err, "user_did", userDID)
		ctx := middleware.SetErrorCode(r.Context(), ErrCodeInternal)
		WriteError(w, ctx, http.StatusInternalServerError, ErrCodeInternal, "Failed to check active streams")
		return
	}

	// Build summary for each scene
	summaries := make([]OwnedSceneSummary, 0, len(scenes))
	for _, sc := range scenes {
		summary := OwnedSceneSummary{
			ID:              sc.ID,
			Name:            sc.Name,
			Description:     sc.Description,
			CoarseGeohash:   sc.CoarseGeohash,
			Tags:            sc.Tags,
			Visibility:      sc.Visibility,
			CreatedAt:       sc.CreatedAt,
			UpdatedAt:       sc.UpdatedAt,
			MembersCount:    membershipCounts[sc.ID],
			HasActiveStream: activeStreams[sc.ID],
		}
		summaries = append(summaries, summary)
	}

	// Return summaries
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(summaries); err != nil {
		slog.ErrorContext(r.Context(), "failed to encode response", "error", err)
		return
	}
}

// RegisterSceneRoutes registers all scene-related routes on the given mux.
func RegisterSceneRoutes(mux *http.ServeMux, deps *RouteDeps, h *SceneHandlers, postH *PostHandlers, membershipH *MembershipHandlers) {
	// Scene creation (with rate limiting: 10 req/hour per user)
	sceneCreationLimit := middleware.RateLimitConfig{
		RequestsPerWindow: 10,
		WindowDuration:    time.Hour,
	}
	sceneCreationHandler := deps.RateLimit(h.CreateScene, sceneCreationLimit, middleware.UserKeyFunc())

	mux.HandleFunc("/scenes", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodPost:
			sceneCreationHandler.ServeHTTP(w, r)
		default:
			ctx := middleware.SetErrorCode(r.Context(), ErrCodeBadRequest)
			WriteError(w, ctx, http.StatusMethodNotAllowed, ErrCodeBadRequest, "Method not allowed")
		}
	})

	mux.HandleFunc("/scenes/owned", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			ctx := middleware.SetErrorCode(r.Context(), ErrCodeBadRequest)
			WriteError(w, ctx, http.StatusMethodNotAllowed, ErrCodeBadRequest, "Method not allowed")
			return
		}
		h.ListOwnedScenes(w, r)
	})

	// Ensure trailing-slash variant /scenes/owned/ does not fall through to the
	// /scenes/ catch-all, where "owned" would be treated as a scene ID.
	mux.HandleFunc("/scenes/owned/", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/scenes/owned", http.StatusMovedPermanently)
	})

	// Scene resource routes: /scenes/{id}, /scenes/{id}/feed, /scenes/{id}/palette, /scenes/{id}/membership/*
	mux.HandleFunc("/scenes/", func(w http.ResponseWriter, r *http.Request) {
		pathParts := strings.Split(strings.TrimPrefix(r.URL.Path, "/scenes/"), "/")

		if len(pathParts) == 0 || pathParts[0] == "" {
			ctx := middleware.SetErrorCode(r.Context(), ErrCodeBadRequest)
			WriteError(w, ctx, http.StatusBadRequest, ErrCodeBadRequest, "Scene ID is required")
			return
		}

		// Scene feed: /scenes/{id}/feed
		if len(pathParts) == 2 && pathParts[1] == "feed" && r.Method == http.MethodGet {
			postH.GetSceneFeed(w, r)
			return
		}

		// Scene palette: /scenes/{id}/palette
		if len(pathParts) == 2 && pathParts[1] == "palette" && r.Method == http.MethodPatch {
			h.UpdateScenePalette(w, r)
			return
		}

		// Membership request: /scenes/{id}/membership/request
		if len(pathParts) == 3 && pathParts[1] == "membership" && pathParts[2] == "request" && r.Method == http.MethodPost {
			membershipH.RequestMembership(w, r)
			return
		}

		// Membership approve/reject: /scenes/{id}/membership/{userDid}/approve|reject
		if len(pathParts) == 4 && pathParts[1] == "membership" && r.Method == http.MethodPost {
			if pathParts[3] == "approve" {
				membershipH.ApproveMembership(w, r)
				return
			} else if pathParts[3] == "reject" {
				membershipH.RejectMembership(w, r)
				return
			}
		}

		// Scene CRUD: /scenes/{id}
		if len(pathParts) == 1 {
			switch r.Method {
			case http.MethodGet:
				h.GetScene(w, r)
			case http.MethodPatch:
				h.UpdateScene(w, r)
			case http.MethodDelete:
				h.DeleteScene(w, r)
			default:
				ctx := middleware.SetErrorCode(r.Context(), ErrCodeBadRequest)
				WriteError(w, ctx, http.StatusMethodNotAllowed, ErrCodeBadRequest, "Method not allowed")
			}
			return
		}

		// No matching endpoint
		ctx := middleware.SetErrorCode(r.Context(), ErrCodeNotFound)
		WriteError(w, ctx, http.StatusNotFound, ErrCodeNotFound, "The requested resource was not found")
	})
}