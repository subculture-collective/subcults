// Package api provides HTTP handlers for the Subcults API.
package api

import (
	"context"
	"encoding/json"
	"fmt"
	"html"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/onnwee/subcults/internal/audit"
	"github.com/onnwee/subcults/internal/middleware"
	"github.com/onnwee/subcults/internal/scene"
)

// Event title validation constraints
const (
	MinEventTitleLength = 3
	MaxEventTitleLength = 80
)

// CreateEventRequest represents the request body for creating an event.
type CreateEventRequest struct {
	SceneID       string         `json:"scene_id"`
	Title         string         `json:"title"`
	Description   string         `json:"description,omitempty"`
	AllowPrecise  bool           `json:"allow_precise"`
	PrecisePoint  *scene.Point   `json:"precise_point,omitempty"`
	CoarseGeohash string         `json:"coarse_geohash"`
	Tags          []string       `json:"tags,omitempty"`
	StartsAt      time.Time      `json:"starts_at"`
	EndsAt        *time.Time     `json:"ends_at,omitempty"`
}

// UpdateEventRequest represents the request body for updating an event.
// Only includes mutable fields (scene_id is immutable).
type UpdateEventRequest struct {
	Title         *string        `json:"title,omitempty"`
	Description   *string        `json:"description,omitempty"`
	Tags          []string       `json:"tags,omitempty"`
	AllowPrecise  *bool          `json:"allow_precise,omitempty"`
	PrecisePoint  *scene.Point   `json:"precise_point,omitempty"`
	CoarseGeohash *string        `json:"coarse_geohash,omitempty"`
	StartsAt      *time.Time     `json:"starts_at,omitempty"`
	EndsAt        *time.Time     `json:"ends_at,omitempty"`
}

// CancelEventRequest represents the request body for cancelling an event.
type CancelEventRequest struct {
	Reason *string `json:"reason,omitempty"`
}

// EventHandlers holds dependencies for event HTTP handlers.
type EventHandlers struct {
	eventRepo scene.EventRepository
	sceneRepo scene.SceneRepository
	auditRepo audit.Repository
}

// NewEventHandlers creates a new EventHandlers instance.
func NewEventHandlers(eventRepo scene.EventRepository, sceneRepo scene.SceneRepository, auditRepo audit.Repository) *EventHandlers {
	return &EventHandlers{
		eventRepo: eventRepo,
		sceneRepo: sceneRepo,
		auditRepo: auditRepo,
	}
}

// validateEventTitle validates event title according to requirements.
// Returns error message if validation fails, empty string if valid.
func validateEventTitle(title string) string {
	// Trim whitespace first
	trimmed := strings.TrimSpace(title)
	
	if len(trimmed) < MinEventTitleLength {
		return fmt.Sprintf("event title must be at least %d characters", MinEventTitleLength)
	}
	if len(trimmed) > MaxEventTitleLength {
		return fmt.Sprintf("event title must not exceed %d characters", MaxEventTitleLength)
	}
	return ""
}

// sanitizeEventTitle sanitizes event title to prevent HTML injection.
// Should be called after validation passes.
func sanitizeEventTitle(title string) string {
	return html.EscapeString(strings.TrimSpace(title))
}

// validateTimeWindow validates that start time is before end time.
// Returns error message if validation fails, empty string if valid.
func validateTimeWindow(startsAt time.Time, endsAt *time.Time) string {
	if endsAt != nil && !startsAt.Before(*endsAt) {
		return "start time must be before end time"
	}
	return ""
}

// isSceneOwner checks if the given userDID owns the scene.
func (h *EventHandlers) isSceneOwner(ctx context.Context, sceneID, userDID string) (bool, error) {
	foundScene, err := h.sceneRepo.GetByID(sceneID)
	if err != nil {
		return false, err
	}
	return foundScene.IsOwner(userDID), nil
}

// CreateEvent handles POST /events - creates a new event.
func (h *EventHandlers) CreateEvent(w http.ResponseWriter, r *http.Request) {
	var req CreateEventRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		ctx := middleware.SetErrorCode(r.Context(), ErrCodeBadRequest)
		WriteError(w, ctx, http.StatusBadRequest, ErrCodeBadRequest, "Invalid JSON in request body")
		return
	}

	// Validate title
	if errMsg := validateEventTitle(req.Title); errMsg != "" {
		ctx := middleware.SetErrorCode(r.Context(), ErrCodeValidation)
		WriteError(w, ctx, http.StatusBadRequest, ErrCodeValidation, errMsg)
		return
	}

	// Sanitize title after validation
	req.Title = sanitizeEventTitle(req.Title)

	// Validate scene_id
	if strings.TrimSpace(req.SceneID) == "" {
		ctx := middleware.SetErrorCode(r.Context(), ErrCodeValidation)
		WriteError(w, ctx, http.StatusBadRequest, ErrCodeValidation, "scene_id is required")
		return
	}

	// Validate coarse_geohash
	if strings.TrimSpace(req.CoarseGeohash) == "" {
		ctx := middleware.SetErrorCode(r.Context(), ErrCodeValidation)
		WriteError(w, ctx, http.StatusBadRequest, ErrCodeValidation, "coarse_geohash is required")
		return
	}

	// Validate time window
	if errMsg := validateTimeWindow(req.StartsAt, req.EndsAt); errMsg != "" {
		ctx := middleware.SetErrorCode(r.Context(), ErrCodeInvalidTimeRange)
		WriteError(w, ctx, http.StatusBadRequest, ErrCodeInvalidTimeRange, errMsg)
		return
	}

	// Get user DID from context (set by auth middleware)
	userDID := middleware.GetUserDID(r.Context())
	if userDID == "" {
		ctx := middleware.SetErrorCode(r.Context(), ErrCodeAuthFailed)
		WriteError(w, ctx, http.StatusUnauthorized, ErrCodeAuthFailed, "Authentication required")
		return
	}

	// Check if user is scene owner (authorization)
	isOwner, err := h.isSceneOwner(r.Context(), req.SceneID, userDID)
	if err != nil {
		if err == scene.ErrSceneNotFound || err == scene.ErrSceneDeleted {
			ctx := middleware.SetErrorCode(r.Context(), ErrCodeNotFound)
			WriteError(w, ctx, http.StatusNotFound, ErrCodeNotFound, "Scene not found")
			return
		}
		slog.ErrorContext(r.Context(), "failed to check scene ownership", "error", err, "scene_id", req.SceneID)
		ctx := middleware.SetErrorCode(r.Context(), ErrCodeInternal)
		WriteError(w, ctx, http.StatusInternalServerError, ErrCodeInternal, "Failed to verify scene ownership")
		return
	}
	if !isOwner {
		ctx := middleware.SetErrorCode(r.Context(), ErrCodeForbidden)
		WriteError(w, ctx, http.StatusForbidden, ErrCodeForbidden, "You do not have permission to create events for this scene")
		return
	}

	// Sanitize description to prevent HTML injection
	req.Description = html.EscapeString(req.Description)

	// Sanitize tags to prevent HTML injection
	sanitizedTags := make([]string, len(req.Tags))
	for i, tag := range req.Tags {
		sanitizedTags[i] = html.EscapeString(tag)
	}

	// Create event
	now := time.Now()
	newEvent := &scene.Event{
		ID:            uuid.New().String(),
		SceneID:       req.SceneID,
		Title:         req.Title,
		Description:   req.Description,
		AllowPrecise:  req.AllowPrecise,
		PrecisePoint:  req.PrecisePoint,
		CoarseGeohash: req.CoarseGeohash,
		Tags:          sanitizedTags,
		Status:        "scheduled", // Default status
		StartsAt:      req.StartsAt,
		EndsAt:        req.EndsAt,
		CreatedAt:     &now,
		UpdatedAt:     &now,
	}

	// Insert into repository (will automatically enforce location consent).
	// If AllowPrecise is false, PrecisePoint will be cleared before storage.
	if err := h.eventRepo.Insert(newEvent); err != nil {
		slog.ErrorContext(r.Context(), "failed to insert event", "error", err, "event_id", newEvent.ID)
		ctx := middleware.SetErrorCode(r.Context(), ErrCodeInternal)
		WriteError(w, ctx, http.StatusInternalServerError, ErrCodeInternal, "Failed to create event")
		return
	}

	// Retrieve the stored event to get privacy-enforced version
	stored, err := h.eventRepo.GetByID(newEvent.ID)
	if err != nil {
		slog.ErrorContext(r.Context(), "failed to retrieve created event", "error", err, "event_id", newEvent.ID)
		ctx := middleware.SetErrorCode(r.Context(), ErrCodeInternal)
		WriteError(w, ctx, http.StatusInternalServerError, ErrCodeInternal, "Failed to retrieve created event")
		return
	}

	// Return created event
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	if err := json.NewEncoder(w).Encode(stored); err != nil {
		// Log error but response already started
		slog.ErrorContext(r.Context(), "failed to encode event response", "error", err)
	}
}

// UpdateEvent handles PATCH /events/{id} - updates an existing event.
func (h *EventHandlers) UpdateEvent(w http.ResponseWriter, r *http.Request) {
	// Extract event ID from URL path
	pathParts := strings.Split(strings.TrimPrefix(r.URL.Path, "/events/"), "/")
	if len(pathParts) == 0 || pathParts[0] == "" {
		ctx := middleware.SetErrorCode(r.Context(), ErrCodeBadRequest)
		WriteError(w, ctx, http.StatusBadRequest, ErrCodeBadRequest, "Event ID is required")
		return
	}
	eventID := pathParts[0]

	var req UpdateEventRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		ctx := middleware.SetErrorCode(r.Context(), ErrCodeBadRequest)
		WriteError(w, ctx, http.StatusBadRequest, ErrCodeBadRequest, "Invalid JSON in request body")
		return
	}

	// Get existing event
	existingEvent, err := h.eventRepo.GetByID(eventID)
	if err != nil {
		if err == scene.ErrEventNotFound {
			ctx := middleware.SetErrorCode(r.Context(), ErrCodeNotFound)
			WriteError(w, ctx, http.StatusNotFound, ErrCodeNotFound, "Event not found")
			return
		}
		slog.ErrorContext(r.Context(), "failed to get event", "error", err, "event_id", eventID)
		ctx := middleware.SetErrorCode(r.Context(), ErrCodeInternal)
		WriteError(w, ctx, http.StatusInternalServerError, ErrCodeInternal, "Failed to retrieve event")
		return
	}

	// Get user DID from context (set by auth middleware)
	userDID := middleware.GetUserDID(r.Context())
	if userDID == "" {
		ctx := middleware.SetErrorCode(r.Context(), ErrCodeAuthFailed)
		WriteError(w, ctx, http.StatusUnauthorized, ErrCodeAuthFailed, "Authentication required")
		return
	}

	// Check if user is scene owner (authorization)
	isOwner, err := h.isSceneOwner(r.Context(), existingEvent.SceneID, userDID)
	if err != nil {
		if err == scene.ErrSceneNotFound || err == scene.ErrSceneDeleted {
			ctx := middleware.SetErrorCode(r.Context(), ErrCodeNotFound)
			WriteError(w, ctx, http.StatusNotFound, ErrCodeNotFound, "Scene not found")
			return
		}
		slog.ErrorContext(r.Context(), "failed to check scene ownership", "error", err, "scene_id", existingEvent.SceneID)
		ctx := middleware.SetErrorCode(r.Context(), ErrCodeInternal)
		WriteError(w, ctx, http.StatusInternalServerError, ErrCodeInternal, "Failed to verify scene ownership")
		return
	}
	if !isOwner {
		ctx := middleware.SetErrorCode(r.Context(), ErrCodeForbidden)
		WriteError(w, ctx, http.StatusForbidden, ErrCodeForbidden, "You do not have permission to update this event")
		return
	}

	// Apply updates to existing event
	updatedEvent := *existingEvent

	if req.Title != nil {
		// Validate title
		if errMsg := validateEventTitle(*req.Title); errMsg != "" {
			ctx := middleware.SetErrorCode(r.Context(), ErrCodeValidation)
			WriteError(w, ctx, http.StatusBadRequest, ErrCodeValidation, errMsg)
			return
		}
		updatedEvent.Title = sanitizeEventTitle(*req.Title)
	}

	if req.Description != nil {
		updatedEvent.Description = html.EscapeString(*req.Description)
	}

	if req.Tags != nil {
		sanitizedTags := make([]string, len(req.Tags))
		for i, tag := range req.Tags {
			sanitizedTags[i] = html.EscapeString(tag)
		}
		updatedEvent.Tags = sanitizedTags
	}

	if req.AllowPrecise != nil {
		updatedEvent.AllowPrecise = *req.AllowPrecise
	}

	if req.PrecisePoint != nil {
		updatedEvent.PrecisePoint = req.PrecisePoint
	}

	if req.CoarseGeohash != nil {
		if strings.TrimSpace(*req.CoarseGeohash) == "" {
			ctx := middleware.SetErrorCode(r.Context(), ErrCodeValidation)
			WriteError(w, ctx, http.StatusBadRequest, ErrCodeValidation, "coarse_geohash cannot be empty")
			return
		}
		updatedEvent.CoarseGeohash = *req.CoarseGeohash
	}

	// Handle time updates with validation
	startsAt := updatedEvent.StartsAt
	endsAt := updatedEvent.EndsAt

	if req.StartsAt != nil {
		// Only allow updates if event is still in the future
		if existingEvent.StartsAt.Before(time.Now()) {
			ctx := middleware.SetErrorCode(r.Context(), ErrCodeValidation)
			WriteError(w, ctx, http.StatusBadRequest, ErrCodeValidation, "Cannot update start time for past events")
			return
		}
		startsAt = *req.StartsAt
	}

	if req.EndsAt != nil {
		endsAt = req.EndsAt
	}

	// Validate time window after applying updates
	if errMsg := validateTimeWindow(startsAt, endsAt); errMsg != "" {
		ctx := middleware.SetErrorCode(r.Context(), ErrCodeInvalidTimeRange)
		WriteError(w, ctx, http.StatusBadRequest, ErrCodeInvalidTimeRange, errMsg)
		return
	}

	updatedEvent.StartsAt = startsAt
	updatedEvent.EndsAt = endsAt

	// Update timestamp
	now := time.Now()
	updatedEvent.UpdatedAt = &now

	// Update in repository (will automatically enforce location consent)
	if err := h.eventRepo.Update(&updatedEvent); err != nil {
		slog.ErrorContext(r.Context(), "failed to update event", "error", err, "event_id", eventID)
		ctx := middleware.SetErrorCode(r.Context(), ErrCodeInternal)
		WriteError(w, ctx, http.StatusInternalServerError, ErrCodeInternal, "Failed to update event")
		return
	}

	// Retrieve the stored event to get privacy-enforced version
	stored, err := h.eventRepo.GetByID(eventID)
	if err != nil {
		slog.ErrorContext(r.Context(), "failed to retrieve updated event", "error", err, "event_id", eventID)
		ctx := middleware.SetErrorCode(r.Context(), ErrCodeInternal)
		WriteError(w, ctx, http.StatusInternalServerError, ErrCodeInternal, "Failed to retrieve updated event")
		return
	}

	// Return updated event
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(stored); err != nil {
		// Log error but response already started
		slog.ErrorContext(r.Context(), "failed to encode event response", "error", err)
	}
}

// GetEvent handles GET /events/{id} - retrieves an event.
func (h *EventHandlers) GetEvent(w http.ResponseWriter, r *http.Request) {
	// Extract event ID from URL path
	pathParts := strings.Split(strings.TrimPrefix(r.URL.Path, "/events/"), "/")
	if len(pathParts) == 0 || pathParts[0] == "" {
		ctx := middleware.SetErrorCode(r.Context(), ErrCodeBadRequest)
		WriteError(w, ctx, http.StatusBadRequest, ErrCodeBadRequest, "Event ID is required")
		return
	}
	eventID := pathParts[0]

	// Get the event
	foundEvent, err := h.eventRepo.GetByID(eventID)
	if err != nil {
		if err == scene.ErrEventNotFound {
			ctx := middleware.SetErrorCode(r.Context(), ErrCodeNotFound)
			WriteError(w, ctx, http.StatusNotFound, ErrCodeNotFound, "Event not found")
			return
		}
		slog.ErrorContext(r.Context(), "failed to get event", "error", err, "event_id", eventID)
		ctx := middleware.SetErrorCode(r.Context(), ErrCodeInternal)
		WriteError(w, ctx, http.StatusInternalServerError, ErrCodeInternal, "Failed to retrieve event")
		return
	}

	// Privacy enforcement is handled by the repository
	// The repository automatically enforces location consent via EnforceLocationConsent()

	// Return event
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(foundEvent); err != nil {
		// Log error but response already started
		slog.ErrorContext(r.Context(), "failed to encode event response", "error", err)
	}
}

// CancelEvent handles POST /events/{id}/cancel - cancels an event.
func (h *EventHandlers) CancelEvent(w http.ResponseWriter, r *http.Request) {
	// Extract event ID from URL path
	// Note: The routing layer already validates this is a /events/{id}/cancel request
	pathParts := strings.Split(strings.TrimPrefix(r.URL.Path, "/events/"), "/")
	if len(pathParts) == 0 || pathParts[0] == "" {
		ctx := middleware.SetErrorCode(r.Context(), ErrCodeBadRequest)
		WriteError(w, ctx, http.StatusBadRequest, ErrCodeBadRequest, "Event ID is required")
		return
	}
	eventID := pathParts[0]

	// Parse request body (optional reason)
	var req CancelEventRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil && err != io.EOF {
		// Allow empty body (io.EOF) but reject malformed JSON
		ctx := middleware.SetErrorCode(r.Context(), ErrCodeBadRequest)
		WriteError(w, ctx, http.StatusBadRequest, ErrCodeBadRequest, "Invalid JSON in request body")
		return
	}

	// Sanitize reason if provided to prevent HTML injection
	if req.Reason != nil {
		sanitized := html.EscapeString(*req.Reason)
		req.Reason = &sanitized
	}

	// Get existing event
	existingEvent, err := h.eventRepo.GetByID(eventID)
	if err != nil {
		if err == scene.ErrEventNotFound {
			ctx := middleware.SetErrorCode(r.Context(), ErrCodeNotFound)
			WriteError(w, ctx, http.StatusNotFound, ErrCodeNotFound, "Event not found")
			return
		}
		slog.ErrorContext(r.Context(), "failed to get event", "error", err, "event_id", eventID)
		ctx := middleware.SetErrorCode(r.Context(), ErrCodeInternal)
		WriteError(w, ctx, http.StatusInternalServerError, ErrCodeInternal, "Failed to retrieve event")
		return
	}

	// Get user DID from context (set by auth middleware)
	userDID := middleware.GetUserDID(r.Context())
	if userDID == "" {
		ctx := middleware.SetErrorCode(r.Context(), ErrCodeAuthFailed)
		WriteError(w, ctx, http.StatusUnauthorized, ErrCodeAuthFailed, "Authentication required")
		return
	}

	// Check if user is scene owner (authorization)
	isOwner, err := h.isSceneOwner(r.Context(), existingEvent.SceneID, userDID)
	if err != nil {
		if err == scene.ErrSceneNotFound || err == scene.ErrSceneDeleted {
			ctx := middleware.SetErrorCode(r.Context(), ErrCodeNotFound)
			WriteError(w, ctx, http.StatusNotFound, ErrCodeNotFound, "Scene not found")
			return
		}
		slog.ErrorContext(r.Context(), "failed to check scene ownership", "error", err, "scene_id", existingEvent.SceneID)
		ctx := middleware.SetErrorCode(r.Context(), ErrCodeInternal)
		WriteError(w, ctx, http.StatusInternalServerError, ErrCodeInternal, "Failed to verify scene ownership")
		return
	}
	if !isOwner {
		ctx := middleware.SetErrorCode(r.Context(), ErrCodeForbidden)
		WriteError(w, ctx, http.StatusForbidden, ErrCodeForbidden, "You do not have permission to cancel this event")
		return
	}

	// Track whether event was already cancelled for audit log decision
	alreadyCancelled := existingEvent.Status == "cancelled" && existingEvent.CancelledAt != nil

	// Cancel the event (idempotent)
	if err := h.eventRepo.Cancel(eventID, req.Reason); err != nil {
		slog.ErrorContext(r.Context(), "failed to cancel event", "error", err, "event_id", eventID)
		ctx := middleware.SetErrorCode(r.Context(), ErrCodeInternal)
		WriteError(w, ctx, http.StatusInternalServerError, ErrCodeInternal, "Failed to cancel event")
		return
	}

	// Emit audit log only if this was the first cancellation (not idempotent case)
	if !alreadyCancelled {
		if err := audit.LogAccessFromRequest(r, h.auditRepo, "event", eventID, "event_cancel"); err != nil {
			slog.ErrorContext(r.Context(), "failed to log event cancellation", "error", err, "event_id", eventID)
			// Don't fail the request, but log the error
		}
	}

	// Retrieve the updated event
	cancelledEvent, err := h.eventRepo.GetByID(eventID)
	if err != nil {
		slog.ErrorContext(r.Context(), "failed to retrieve cancelled event", "error", err, "event_id", eventID)
		ctx := middleware.SetErrorCode(r.Context(), ErrCodeInternal)
		WriteError(w, ctx, http.StatusInternalServerError, ErrCodeInternal, "Failed to retrieve cancelled event")
		return
	}

	// Return cancelled event
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(cancelledEvent); err != nil {
		// Log error but response already started
		slog.ErrorContext(r.Context(), "failed to encode event response", "error", err)
	}
}
