// Package api provides HTTP handlers for the Subcults API.
package api

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/onnwee/subcults/internal/alliance"
	"github.com/onnwee/subcults/internal/middleware"
	"github.com/onnwee/subcults/internal/scene"
	"github.com/onnwee/subcults/internal/trust"
)

// CreateAllianceRequest represents the request body for creating an alliance.
type CreateAllianceRequest struct {
	FromSceneID string  `json:"from_scene_id"`
	ToSceneID   string  `json:"to_scene_id"`
	Weight      float64 `json:"weight"`
	Reason      *string `json:"reason,omitempty"`
}

// UpdateAllianceRequest represents the request body for updating an alliance.
type UpdateAllianceRequest struct {
	Weight *float64 `json:"weight,omitempty"`
	Reason *string  `json:"reason,omitempty"`
}

// AllianceHandlers holds dependencies for alliance HTTP handlers.
type AllianceHandlers struct {
	allianceService   *alliance.Service
	sceneRepo         scene.SceneRepository
	trustDataSource   trust.DataSource
	trustDirtyTracker *trust.DirtyTracker
}

// NewAllianceHandlers creates a new AllianceHandlers instance.
func NewAllianceHandlers(
	allianceService *alliance.Service,
	sceneRepo scene.SceneRepository,
	trustDataSource trust.DataSource,
	trustDirtyTracker *trust.DirtyTracker,
) *AllianceHandlers {
	return &AllianceHandlers{
		allianceService:   allianceService,
		sceneRepo:         sceneRepo,
		trustDataSource:   trustDataSource,
		trustDirtyTracker: trustDirtyTracker,
	}
}

// CreateAlliance handles POST /alliances - creates a new alliance.
func (h *AllianceHandlers) CreateAlliance(w http.ResponseWriter, r *http.Request) {
	// Check authentication
	userDID := middleware.GetUserDID(r.Context())
	if userDID == "" {
		ctx := middleware.SetErrorCode(r.Context(), ErrCodeAuthFailed)
		WriteError(w, ctx, http.StatusUnauthorized, ErrCodeAuthFailed, "Authentication required")
		return
	}

	// Parse request body
	var req CreateAllianceRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		ctx := middleware.SetErrorCode(r.Context(), ErrCodeBadRequest)
		WriteError(w, ctx, http.StatusBadRequest, ErrCodeBadRequest, "Invalid JSON in request body")
		return
	}

	// Verify from_scene exists and user owns it
	fromScene, err := h.sceneRepo.GetByID(req.FromSceneID)
	if err != nil {
		if err == scene.ErrSceneNotFound || err == scene.ErrSceneDeleted {
			ctx := middleware.SetErrorCode(r.Context(), ErrCodeNotFound)
			WriteError(w, ctx, http.StatusNotFound, ErrCodeNotFound, "From scene not found")
			return
		}
		slog.ErrorContext(r.Context(), "failed to retrieve from scene", "error", err, "scene_id", req.FromSceneID)
		ctx := middleware.SetErrorCode(r.Context(), ErrCodeInternal)
		WriteError(w, ctx, http.StatusInternalServerError, ErrCodeInternal, "Failed to retrieve from scene")
		return
	}

	// Check ownership
	if !fromScene.IsOwner(userDID) {
		ctx := middleware.SetErrorCode(r.Context(), ErrCodeForbidden)
		WriteError(w, ctx, http.StatusForbidden, ErrCodeForbidden, "Only scene owner can create alliances")
		return
	}

	// Verify to_scene exists
	_, err = h.sceneRepo.GetByID(req.ToSceneID)
	if err != nil {
		if err == scene.ErrSceneNotFound || err == scene.ErrSceneDeleted {
			ctx := middleware.SetErrorCode(r.Context(), ErrCodeNotFound)
			WriteError(w, ctx, http.StatusNotFound, ErrCodeNotFound, "To scene not found")
			return
		}
		slog.ErrorContext(r.Context(), "failed to retrieve to scene", "error", err, "scene_id", req.ToSceneID)
		ctx := middleware.SetErrorCode(r.Context(), ErrCodeInternal)
		WriteError(w, ctx, http.StatusInternalServerError, ErrCodeInternal, "Failed to retrieve to scene")
		return
	}

	// Delegate to service
	created, err := h.allianceService.CreateAlliance(req.FromSceneID, req.ToSceneID, req.Weight, req.Reason)
	if err != nil {
		slog.ErrorContext(r.Context(), "failed to create alliance", "error", err)
		ctx := middleware.SetErrorCode(r.Context(), ErrCodeInternal)
		WriteAPIError(w, ctx, err)
		return
	}

	// Sync alliance to trust data source for trust score computation
	h.trustDataSource.AddAlliance(trust.Alliance{
		FromSceneID: created.FromSceneID,
		ToSceneID:   created.ToSceneID,
		Weight:      created.Weight,
	})

	// Mark scene as dirty for trust recomputation
	h.trustDirtyTracker.MarkDirty(created.FromSceneID)

	// Return created alliance
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	if err := json.NewEncoder(w).Encode(created); err != nil {
		slog.ErrorContext(r.Context(), "failed to encode response", "error", err)
	}
}

// GetAlliance handles GET /alliances/{id} - retrieves an alliance by ID.
func (h *AllianceHandlers) GetAlliance(w http.ResponseWriter, r *http.Request) {
	// Extract alliance ID from URL path
	pathParts := strings.Split(strings.TrimPrefix(r.URL.Path, "/alliances/"), "/")
	if len(pathParts) == 0 || pathParts[0] == "" {
		ctx := middleware.SetErrorCode(r.Context(), ErrCodeBadRequest)
		WriteError(w, ctx, http.StatusBadRequest, ErrCodeBadRequest, "Alliance ID is required")
		return
	}
	allianceID := pathParts[0]

	// Retrieve alliance
	foundAlliance, err := h.allianceService.GetAlliance(allianceID)
	if err != nil {
		if err == alliance.ErrAllianceNotFound {
			ctx := middleware.SetErrorCode(r.Context(), ErrCodeNotFound)
			WriteError(w, ctx, http.StatusNotFound, ErrCodeNotFound, "Alliance not found")
			return
		}
		if err == alliance.ErrAllianceDeleted {
			ctx := middleware.SetErrorCode(r.Context(), ErrCodeAllianceDeleted)
			WriteError(w, ctx, http.StatusNotFound, ErrCodeAllianceDeleted, "Alliance not found")
			return
		}
		slog.ErrorContext(r.Context(), "failed to retrieve alliance", "error", err, "alliance_id", allianceID)
		ctx := middleware.SetErrorCode(r.Context(), ErrCodeInternal)
		WriteError(w, ctx, http.StatusInternalServerError, ErrCodeInternal, "Failed to retrieve alliance")
		return
	}

	// Return alliance
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(foundAlliance); err != nil {
		slog.ErrorContext(r.Context(), "failed to encode response", "error", err)
	}
}

// UpdateAlliance handles PATCH /alliances/{id} - updates an existing alliance.
func (h *AllianceHandlers) UpdateAlliance(w http.ResponseWriter, r *http.Request) {
	// Check authentication
	userDID := middleware.GetUserDID(r.Context())
	if userDID == "" {
		ctx := middleware.SetErrorCode(r.Context(), ErrCodeAuthFailed)
		WriteError(w, ctx, http.StatusUnauthorized, ErrCodeAuthFailed, "Authentication required")
		return
	}

	// Extract alliance ID from URL path
	pathParts := strings.Split(strings.TrimPrefix(r.URL.Path, "/alliances/"), "/")
	if len(pathParts) == 0 || pathParts[0] == "" {
		ctx := middleware.SetErrorCode(r.Context(), ErrCodeBadRequest)
		WriteError(w, ctx, http.StatusBadRequest, ErrCodeBadRequest, "Alliance ID is required")
		return
	}
	allianceID := pathParts[0]

	// Parse request body
	var req UpdateAllianceRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		ctx := middleware.SetErrorCode(r.Context(), ErrCodeBadRequest)
		WriteError(w, ctx, http.StatusBadRequest, ErrCodeBadRequest, "Invalid JSON in request body")
		return
	}

	// Get existing alliance to verify ownership
	existingAlliance, err := h.allianceService.GetAlliance(allianceID)
	if err != nil {
		if err == alliance.ErrAllianceNotFound {
			ctx := middleware.SetErrorCode(r.Context(), ErrCodeNotFound)
			WriteError(w, ctx, http.StatusNotFound, ErrCodeNotFound, "Alliance not found")
			return
		}
		if err == alliance.ErrAllianceDeleted {
			ctx := middleware.SetErrorCode(r.Context(), ErrCodeAllianceDeleted)
			WriteError(w, ctx, http.StatusNotFound, ErrCodeAllianceDeleted, "Alliance not found")
			return
		}
		slog.ErrorContext(r.Context(), "failed to retrieve alliance", "error", err, "alliance_id", allianceID)
		ctx := middleware.SetErrorCode(r.Context(), ErrCodeInternal)
		WriteError(w, ctx, http.StatusInternalServerError, ErrCodeInternal, "Failed to retrieve alliance")
		return
	}

	// Verify from_scene ownership
	fromScene, err := h.sceneRepo.GetByID(existingAlliance.FromSceneID)
	if err != nil {
		if err == scene.ErrSceneNotFound || err == scene.ErrSceneDeleted {
			ctx := middleware.SetErrorCode(r.Context(), ErrCodeNotFound)
			WriteError(w, ctx, http.StatusNotFound, ErrCodeNotFound, "From scene not found")
			return
		}
		slog.ErrorContext(r.Context(), "failed to retrieve from scene", "error", err, "scene_id", existingAlliance.FromSceneID)
		ctx := middleware.SetErrorCode(r.Context(), ErrCodeInternal)
		WriteError(w, ctx, http.StatusInternalServerError, ErrCodeInternal, "Failed to retrieve from scene")
		return
	}

	// Check ownership
	if !fromScene.IsOwner(userDID) {
		ctx := middleware.SetErrorCode(r.Context(), ErrCodeForbidden)
		WriteError(w, ctx, http.StatusForbidden, ErrCodeForbidden, "Only scene owner can update alliances")
		return
	}

	// Delegate to service
	updated, err := h.allianceService.UpdateAlliance(allianceID, req.Weight, req.Reason)
	if err != nil {
		slog.ErrorContext(r.Context(), "failed to update alliance", "error", err, "alliance_id", allianceID)
		ctx := middleware.SetErrorCode(r.Context(), ErrCodeInternal)
		WriteAPIError(w, ctx, err)
		return
	}

	// Sync updated alliance to trust data source
	h.trustDataSource.ClearAlliances(updated.FromSceneID)
	h.trustDataSource.AddAlliance(trust.Alliance{
		FromSceneID: updated.FromSceneID,
		ToSceneID:   updated.ToSceneID,
		Weight:      updated.Weight,
	})

	// Mark scene as dirty for trust recomputation
	h.trustDirtyTracker.MarkDirty(updated.FromSceneID)

	// Return updated alliance
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(updated); err != nil {
		slog.ErrorContext(r.Context(), "failed to encode response", "error", err)
	}
}

// DeleteAlliance handles DELETE /alliances/{id} - soft-deletes an alliance.
func (h *AllianceHandlers) DeleteAlliance(w http.ResponseWriter, r *http.Request) {
	// Check authentication
	userDID := middleware.GetUserDID(r.Context())
	if userDID == "" {
		ctx := middleware.SetErrorCode(r.Context(), ErrCodeAuthFailed)
		WriteError(w, ctx, http.StatusUnauthorized, ErrCodeAuthFailed, "Authentication required")
		return
	}

	// Extract alliance ID from URL path
	pathParts := strings.Split(strings.TrimPrefix(r.URL.Path, "/alliances/"), "/")
	if len(pathParts) == 0 || pathParts[0] == "" {
		ctx := middleware.SetErrorCode(r.Context(), ErrCodeBadRequest)
		WriteError(w, ctx, http.StatusBadRequest, ErrCodeBadRequest, "Alliance ID is required")
		return
	}
	allianceID := pathParts[0]

	// Get existing alliance to check ownership
	existingAlliance, err := h.allianceService.GetAlliance(allianceID)
	if err != nil {
		if err == alliance.ErrAllianceNotFound {
			ctx := middleware.SetErrorCode(r.Context(), ErrCodeNotFound)
			WriteError(w, ctx, http.StatusNotFound, ErrCodeNotFound, "Alliance not found")
			return
		}
		if err == alliance.ErrAllianceDeleted {
			ctx := middleware.SetErrorCode(r.Context(), ErrCodeAllianceDeleted)
			WriteError(w, ctx, http.StatusNotFound, ErrCodeAllianceDeleted, "Alliance not found")
			return
		}
		slog.ErrorContext(r.Context(), "failed to retrieve alliance", "error", err, "alliance_id", allianceID)
		ctx := middleware.SetErrorCode(r.Context(), ErrCodeInternal)
		WriteError(w, ctx, http.StatusInternalServerError, ErrCodeInternal, "Failed to retrieve alliance")
		return
	}

	// Verify from_scene ownership
	fromScene, err := h.sceneRepo.GetByID(existingAlliance.FromSceneID)
	if err != nil {
		if err == scene.ErrSceneNotFound || err == scene.ErrSceneDeleted {
			ctx := middleware.SetErrorCode(r.Context(), ErrCodeNotFound)
			WriteError(w, ctx, http.StatusNotFound, ErrCodeNotFound, "From scene not found")
			return
		}
		slog.ErrorContext(r.Context(), "failed to retrieve from scene", "error", err, "scene_id", existingAlliance.FromSceneID)
		ctx := middleware.SetErrorCode(r.Context(), ErrCodeInternal)
		WriteError(w, ctx, http.StatusInternalServerError, ErrCodeInternal, "Failed to retrieve from scene")
		return
	}

	// Check ownership
	if !fromScene.IsOwner(userDID) {
		ctx := middleware.SetErrorCode(r.Context(), ErrCodeForbidden)
		WriteError(w, ctx, http.StatusForbidden, ErrCodeForbidden, "Only scene owner can delete alliances")
		return
	}

	// Delegate to service
	if err := h.allianceService.DeleteAlliance(allianceID); err != nil {
		if err == alliance.ErrAllianceNotFound {
			ctx := middleware.SetErrorCode(r.Context(), ErrCodeNotFound)
			WriteError(w, ctx, http.StatusNotFound, ErrCodeNotFound, "Alliance not found")
			return
		}
		if err == alliance.ErrAllianceDeleted {
			ctx := middleware.SetErrorCode(r.Context(), ErrCodeAllianceDeleted)
			WriteError(w, ctx, http.StatusNotFound, ErrCodeAllianceDeleted, "Alliance not found")
			return
		}
		slog.ErrorContext(r.Context(), "failed to delete alliance", "error", err, "alliance_id", allianceID)
		ctx := middleware.SetErrorCode(r.Context(), ErrCodeInternal)
		WriteError(w, ctx, http.StatusInternalServerError, ErrCodeInternal, "Failed to delete alliance")
		return
	}

	// Remove alliance from trust data source
	h.trustDataSource.ClearAlliances(existingAlliance.FromSceneID)

	// Mark scene as dirty for trust recomputation
	h.trustDirtyTracker.MarkDirty(existingAlliance.FromSceneID)

	// Return success with no content
	w.WriteHeader(http.StatusNoContent)
}

// RegisterAllianceRoutes registers all alliance-related routes on the given mux.
func RegisterAllianceRoutes(mux *http.ServeMux, deps *RouteDeps, h *AllianceHandlers) {
	// Alliance creation (with rate limiting: 10 req/hour per user)
	allianceCreationLimit := middleware.RateLimitConfig{
		RequestsPerWindow: 10,
		WindowDuration:    time.Hour,
	}
	allianceCreationHandler := deps.RateLimit(h.CreateAlliance, allianceCreationLimit, middleware.UserKeyFunc())

	mux.HandleFunc("/alliances", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			ctx := middleware.SetErrorCode(r.Context(), ErrCodeBadRequest)
			WriteError(w, ctx, http.StatusMethodNotAllowed, ErrCodeBadRequest, "Method not allowed")
			return
		}
		allianceCreationHandler.ServeHTTP(w, r)
	})

	mux.HandleFunc("/alliances/", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			h.GetAlliance(w, r)
		case http.MethodPatch:
			h.UpdateAlliance(w, r)
		case http.MethodDelete:
			h.DeleteAlliance(w, r)
		default:
			ctx := middleware.SetErrorCode(r.Context(), ErrCodeBadRequest)
			WriteError(w, ctx, http.StatusMethodNotAllowed, ErrCodeBadRequest, "Method not allowed")
		}
	})
}