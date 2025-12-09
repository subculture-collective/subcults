// Package api provides HTTP handlers for the Subcults API.
package api

import (
	"encoding/json"
	"html"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/onnwee/subcults/internal/middleware"
	"github.com/onnwee/subcults/internal/scene"
)

// Scene name validation constraints
const (
	MinSceneNameLength = 3
	MaxSceneNameLength = 64
)

// sceneNamePattern allows letters, numbers, spaces, and limited punctuation
var sceneNamePattern = regexp.MustCompile(`^[a-zA-Z0-9\s\-_'.&]+$`)

// CreateSceneRequest represents the request body for creating a scene.
type CreateSceneRequest struct {
	Name          string           `json:"name"`
	Description   string           `json:"description,omitempty"`
	OwnerDID      string           `json:"owner_did"`
	AllowPrecise  bool             `json:"allow_precise"`
	PrecisePoint  *scene.Point     `json:"precise_point,omitempty"`
	CoarseGeohash string           `json:"coarse_geohash"`
	Tags          []string         `json:"tags,omitempty"`
	Visibility    string           `json:"visibility,omitempty"`
	Palette       *scene.Palette   `json:"palette,omitempty"`
}

// UpdateSceneRequest represents the request body for updating a scene.
// Only includes mutable fields (owner is immutable).
type UpdateSceneRequest struct {
	Name         *string          `json:"name,omitempty"`
	Description  *string          `json:"description,omitempty"`
	Tags         []string         `json:"tags,omitempty"`
	Visibility   *string          `json:"visibility,omitempty"`
	Palette      *scene.Palette   `json:"palette,omitempty"`
	AllowPrecise *bool            `json:"allow_precise,omitempty"`
	PrecisePoint *scene.Point     `json:"precise_point,omitempty"`
}

// SceneHandlers holds dependencies for scene HTTP handlers.
type SceneHandlers struct {
	repo scene.SceneRepository
}

// NewSceneHandlers creates a new SceneHandlers instance.
func NewSceneHandlers(repo scene.SceneRepository) *SceneHandlers {
	return &SceneHandlers{repo: repo}
}

// validateSceneName validates scene name according to requirements.
// Returns error message if validation fails, empty string if valid.
func validateSceneName(name string) string {
	// Trim whitespace first
	trimmed := strings.TrimSpace(name)
	
	if len(trimmed) < MinSceneNameLength {
		return "scene name must be at least 3 characters"
	}
	if len(trimmed) > MaxSceneNameLength {
		return "scene name must not exceed 64 characters"
	}
	if !sceneNamePattern.MatchString(trimmed) {
		return "scene name contains invalid characters (allowed: letters, numbers, spaces, -, _, ', ., &)"
	}
	return ""
}

// sanitizeSceneName sanitizes scene name to prevent HTML injection.
// Should be called after validation passes.
func sanitizeSceneName(name string) string {
	return html.EscapeString(strings.TrimSpace(name))
}

// validateVisibility validates the visibility mode.
func validateVisibility(visibility string) string {
	if visibility == "" {
		return "" // Empty is OK, will default to "public"
	}
	if visibility != "public" && visibility != "private" && visibility != "unlisted" {
		return "visibility must be 'public', 'private', or 'unlisted'"
	}
	return ""
}

// CreateScene handles POST /scenes - creates a new scene.
func (h *SceneHandlers) CreateScene(w http.ResponseWriter, r *http.Request) {
	var req CreateSceneRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		ctx := middleware.SetErrorCode(r.Context(), ErrCodeBadRequest)
		WriteError(w, ctx, http.StatusBadRequest, ErrCodeBadRequest, "Invalid JSON in request body")
		return
	}

	// Validate name
	if errMsg := validateSceneName(req.Name); errMsg != "" {
		ctx := middleware.SetErrorCode(r.Context(), ErrCodeValidation)
		WriteError(w, ctx, http.StatusBadRequest, ErrCodeValidation, errMsg)
		return
	}

	// Sanitize name after validation
	req.Name = sanitizeSceneName(req.Name)

	// Validate owner_did
	if strings.TrimSpace(req.OwnerDID) == "" {
		ctx := middleware.SetErrorCode(r.Context(), ErrCodeValidation)
		WriteError(w, ctx, http.StatusBadRequest, ErrCodeValidation, "owner_did is required")
		return
	}

	// Validate coarse_geohash
	if strings.TrimSpace(req.CoarseGeohash) == "" {
		ctx := middleware.SetErrorCode(r.Context(), ErrCodeValidation)
		WriteError(w, ctx, http.StatusBadRequest, ErrCodeValidation, "coarse_geohash is required")
		return
	}

	// Validate visibility
	if errMsg := validateVisibility(req.Visibility); errMsg != "" {
		ctx := middleware.SetErrorCode(r.Context(), ErrCodeValidation)
		WriteError(w, ctx, http.StatusBadRequest, ErrCodeValidation, errMsg)
		return
	}

	// Set default visibility
	if req.Visibility == "" {
		req.Visibility = "public"
	}

	// Check for duplicate name
	exists, err := h.repo.ExistsByOwnerAndName(req.OwnerDID, req.Name, "")
	if err != nil {
		ctx := middleware.SetErrorCode(r.Context(), ErrCodeInternal)
		WriteError(w, ctx, http.StatusInternalServerError, ErrCodeInternal, "Failed to check for duplicate scene name")
		return
	}
	if exists {
		ctx := middleware.SetErrorCode(r.Context(), ErrCodeConflict)
		WriteError(w, ctx, http.StatusConflict, ErrCodeConflict, "Scene with this name already exists for this owner")
		return
	}

	// Create scene
	now := time.Now()
	newScene := &scene.Scene{
		ID:            uuid.New().String(),
		Name:          req.Name,
		Description:   req.Description,
		OwnerDID:      req.OwnerDID,
		AllowPrecise:  req.AllowPrecise,
		PrecisePoint:  req.PrecisePoint,
		CoarseGeohash: req.CoarseGeohash,
		Tags:          req.Tags,
		Visibility:    req.Visibility,
		Palette:       req.Palette,
		CreatedAt:     &now,
		UpdatedAt:     &now,
	}

	// Insert into repository (will enforce location consent)
	if err := h.repo.Insert(newScene); err != nil {
		ctx := middleware.SetErrorCode(r.Context(), ErrCodeInternal)
		WriteError(w, ctx, http.StatusInternalServerError, ErrCodeInternal, "Failed to create scene")
		return
	}

	// Retrieve the stored scene to get privacy-enforced version
	stored, err := h.repo.GetByID(newScene.ID)
	if err != nil {
		ctx := middleware.SetErrorCode(r.Context(), ErrCodeInternal)
		WriteError(w, ctx, http.StatusInternalServerError, ErrCodeInternal, "Failed to retrieve created scene")
		return
	}

	// Return created scene
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	if err := json.NewEncoder(w).Encode(stored); err != nil {
		// Log error but response already started
		return
	}
}

// UpdateScene handles PATCH /scenes/{id} - updates an existing scene.
func (h *SceneHandlers) UpdateScene(w http.ResponseWriter, r *http.Request) {
	// Extract scene ID from URL path
	// For now, we'll use a simple path parsing; in production this would use chi or similar
	pathParts := strings.Split(strings.TrimPrefix(r.URL.Path, "/scenes/"), "/")
	if len(pathParts) == 0 || pathParts[0] == "" {
		ctx := middleware.SetErrorCode(r.Context(), ErrCodeBadRequest)
		WriteError(w, ctx, http.StatusBadRequest, ErrCodeBadRequest, "Scene ID is required")
		return
	}
	sceneID := pathParts[0]

	// Parse request body
	var req UpdateSceneRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		ctx := middleware.SetErrorCode(r.Context(), ErrCodeBadRequest)
		WriteError(w, ctx, http.StatusBadRequest, ErrCodeBadRequest, "Invalid JSON in request body")
		return
	}

	// Get existing scene
	existingScene, err := h.repo.GetByID(sceneID)
	if err != nil {
		if err == scene.ErrSceneNotFound {
			ctx := middleware.SetErrorCode(r.Context(), ErrCodeNotFound)
			WriteError(w, ctx, http.StatusNotFound, ErrCodeNotFound, "Scene not found")
			return
		}
		ctx := middleware.SetErrorCode(r.Context(), ErrCodeInternal)
		WriteError(w, ctx, http.StatusInternalServerError, ErrCodeInternal, "Failed to retrieve scene")
		return
	}

	// Validate and apply updates
	if req.Name != nil {
		newName := *req.Name
		if errMsg := validateSceneName(newName); errMsg != "" {
			ctx := middleware.SetErrorCode(r.Context(), ErrCodeValidation)
			WriteError(w, ctx, http.StatusBadRequest, ErrCodeValidation, errMsg)
			return
		}
		// Sanitize name after validation
		newName = sanitizeSceneName(newName)
		
		// Check for duplicate name (excluding current scene)
		exists, err := h.repo.ExistsByOwnerAndName(existingScene.OwnerDID, newName, sceneID)
		if err != nil {
			ctx := middleware.SetErrorCode(r.Context(), ErrCodeInternal)
			WriteError(w, ctx, http.StatusInternalServerError, ErrCodeInternal, "Failed to check for duplicate scene name")
			return
		}
		if exists {
			ctx := middleware.SetErrorCode(r.Context(), ErrCodeConflict)
			WriteError(w, ctx, http.StatusConflict, ErrCodeConflict, "Scene with this name already exists for this owner")
			return
		}
		existingScene.Name = newName
	}

	if req.Description != nil {
		existingScene.Description = *req.Description
	}

	if req.Tags != nil {
		existingScene.Tags = req.Tags
	}

	if req.Visibility != nil {
		if errMsg := validateVisibility(*req.Visibility); errMsg != "" {
			ctx := middleware.SetErrorCode(r.Context(), ErrCodeValidation)
			WriteError(w, ctx, http.StatusBadRequest, ErrCodeValidation, errMsg)
			return
		}
		existingScene.Visibility = *req.Visibility
	}

	if req.Palette != nil {
		existingScene.Palette = req.Palette
	}

	if req.AllowPrecise != nil {
		existingScene.AllowPrecise = *req.AllowPrecise
	}

	if req.PrecisePoint != nil {
		existingScene.PrecisePoint = req.PrecisePoint
	}

	// Update timestamp
	now := time.Now()
	existingScene.UpdatedAt = &now

	// Update in repository (will enforce location consent)
	if err := h.repo.Update(existingScene); err != nil {
		ctx := middleware.SetErrorCode(r.Context(), ErrCodeInternal)
		WriteError(w, ctx, http.StatusInternalServerError, ErrCodeInternal, "Failed to update scene")
		return
	}

	// Retrieve updated scene
	updated, err := h.repo.GetByID(sceneID)
	if err != nil {
		ctx := middleware.SetErrorCode(r.Context(), ErrCodeInternal)
		WriteError(w, ctx, http.StatusInternalServerError, ErrCodeInternal, "Failed to retrieve updated scene")
		return
	}

	// Return updated scene
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(updated); err != nil {
		return
	}
}

// DeleteScene handles DELETE /scenes/{id} - soft-deletes a scene.
func (h *SceneHandlers) DeleteScene(w http.ResponseWriter, r *http.Request) {
	// Extract scene ID from URL path
	pathParts := strings.Split(strings.TrimPrefix(r.URL.Path, "/scenes/"), "/")
	if len(pathParts) == 0 || pathParts[0] == "" {
		ctx := middleware.SetErrorCode(r.Context(), ErrCodeBadRequest)
		WriteError(w, ctx, http.StatusBadRequest, ErrCodeBadRequest, "Scene ID is required")
		return
	}
	sceneID := pathParts[0]

	// Soft delete the scene
	if err := h.repo.Delete(sceneID); err != nil {
		if err == scene.ErrSceneNotFound {
			ctx := middleware.SetErrorCode(r.Context(), ErrCodeNotFound)
			WriteError(w, ctx, http.StatusNotFound, ErrCodeNotFound, "Scene not found")
			return
		}
		ctx := middleware.SetErrorCode(r.Context(), ErrCodeInternal)
		WriteError(w, ctx, http.StatusInternalServerError, ErrCodeInternal, "Failed to delete scene")
		return
	}

	// Return success with no content
	w.WriteHeader(http.StatusNoContent)
}
