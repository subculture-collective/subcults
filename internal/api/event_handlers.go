// Package api provides HTTP handlers for the Subcults API.
package api

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/onnwee/subcults/internal/audit"
	"github.com/onnwee/subcults/internal/middleware"
	"github.com/onnwee/subcults/internal/scene"
	"github.com/onnwee/subcults/internal/stream"
	"github.com/onnwee/subcults/internal/validate"
)

// CreateEventRequest represents the request body for creating an event.
type CreateEventRequest struct {
	SceneID        string       `json:"scene_id"`
	Title          string       `json:"title"`
	Description    string       `json:"description,omitempty"`
	AllowPrecise   bool         `json:"allow_precise"`
	PrecisePoint   *scene.Point `json:"precise_point,omitempty"`
	CoarseGeohash  string       `json:"coarse_geohash"`
	Tags           []string     `json:"tags,omitempty"`
	StartsAt       time.Time    `json:"starts_at"`
	EndsAt         *time.Time   `json:"ends_at,omitempty"`
	LocationAccess string       `json:"location_access,omitempty"`
	PlaceID        *string      `json:"place_id,omitempty"`
	VenueID        *string      `json:"venue_id,omitempty"`
	Kind           string       `json:"kind,omitempty"`
}

// UpdateEventRequest represents the request body for updating an event.
type UpdateEventRequest struct {
	Version        int64        `json:"version"`
	Title          *string      `json:"title,omitempty"`
	Description    *string      `json:"description,omitempty"`
	Tags           []string     `json:"tags,omitempty"`
	AllowPrecise   *bool        `json:"allow_precise,omitempty"`
	PrecisePoint   *scene.Point `json:"precise_point,omitempty"`
	CoarseGeohash  *string      `json:"coarse_geohash,omitempty"`
	StartsAt       *time.Time   `json:"starts_at,omitempty"`
	EndsAt         *time.Time   `json:"ends_at,omitempty"`
	LocationAccess *string      `json:"location_access,omitempty"`
}

// CancelEventRequest represents the request body for cancelling an event.
type CancelEventRequest struct {
	Reason *string `json:"reason,omitempty"`
}

// EventHandlers holds dependencies for event HTTP handlers.
type EventHandlers struct {
	sceneService   *scene.Service
	auditRepo      audit.Repository
	streamRepo     stream.SessionRepository
	trustScoreStore TrustScoreStore
}

// TrustScoreStore defines the interface for retrieving trust scores.
type TrustScoreStore interface {
	GetScore(sceneID string) (score *TrustScore, err error)
}

// TrustScore represents a trust score value.
type TrustScore struct {
	SceneID string
	Score   float64
}

// NewEventHandlers creates a new EventHandlers instance.
func NewEventHandlers(sceneService *scene.Service, auditRepo audit.Repository, streamRepo stream.SessionRepository, trustScoreStore TrustScoreStore) *EventHandlers {
	return &EventHandlers{
		sceneService:   sceneService,
		auditRepo:      auditRepo,
		streamRepo:     streamRepo,
		trustScoreStore: trustScoreStore,
	}
}

// EventWithRSVPCounts represents an event with aggregated RSVP counts and active stream info.
type EventWithRSVPCounts struct {
	*scene.Event
	RSVPCounts   *scene.RSVPCounts        `json:"rsvp_counts"`
	Scene        *SceneSearchResult       `json:"scene,omitempty"`
	ActiveStream *stream.ActiveStreamInfo `json:"active_stream,omitempty"`
	Occurrence   *PublicOccurrence        `json:"occurrence,omitempty"`
}

// PublicOccurrence is the only location projection map clients should use for
// Events.
type PublicOccurrence struct {
	CoarseGeohash string       `json:"coarse_geohash"`
	DisplayPoint  *scene.Point `json:"display_point,omitempty"`
	Precision     string       `json:"precision"`
}

func toPublicOccurrence(event *scene.Event) *PublicOccurrence {
	if event == nil {
		return nil
	}

	occurrence := &PublicOccurrence{
		CoarseGeohash: event.CoarseGeohash,
		Precision:     "coarse",
	}
	if event.AllowPrecise && event.LocationAccess != "protected" && event.PrecisePoint != nil {
		pointCopy := *event.PrecisePoint
		occurrence.DisplayPoint = &pointCopy
		occurrence.Precision = "precise"
		return occurrence
	}

	point, err := scene.DecodeCoarseGeohash(event.CoarseGeohash)
	if err != nil {
		return occurrence
	}
	occurrence.DisplayPoint = applyJitter(point)
	return occurrence
}

func publicEventCopy(event *scene.Event) *scene.Event {
	if event == nil {
		return nil
	}
	copy := *event
	if copy.LocationAccess == "protected" {
		copy.PrecisePoint = nil
	}
	return &copy
}

// sceneBatchFetcher is an optional repository capability for batch scene lookups.
type sceneBatchFetcher interface {
	GetByIDs(ids []string) ([]*scene.Scene, error)
}

// toSceneSearchResult converts an internal scene model to a public search-safe payload.
func toSceneSearchResult(parentScene *scene.Scene) *SceneSearchResult {
	if parentScene == nil {
		return nil
	}
	result := &SceneSearchResult{
		ID:            parentScene.ID,
		Name:          parentScene.Name,
		Description:   parentScene.Description,
		CoarseGeohash: parentScene.CoarseGeohash,
		Tags:          parentScene.Tags,
		Visibility:    parentScene.Visibility,
	}
	if parentScene.PrecisePoint != nil {
		result.JitteredPoint = applyJitter(parentScene.PrecisePoint)
	}
	return result
}

// isSceneOwner checks if the given userDID owns the scene.
func (h *EventHandlers) isSceneOwner(sceneID, userDID string) (bool, error) {
	foundScene, err := h.sceneService.SceneRepo().GetByID(sceneID)
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

	// Get user DID from context
	userDID := middleware.GetUserDID(r.Context())
	if userDID == "" {
		ctx := middleware.SetErrorCode(r.Context(), ErrCodeAuthFailed)
		WriteError(w, ctx, http.StatusUnauthorized, ErrCodeAuthFailed, "Authentication required")
		return
	}

	// Check if user is scene owner
	isOwner, err := h.isSceneOwner(req.SceneID, userDID)
	if err != nil {
		if errors.Is(err, scene.ErrSceneNotFound) || errors.Is(err, scene.ErrSceneDeleted) {
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

	publicationStatus := "published"
	if strings.Contains(r.URL.Path, "/studio/") {
		publicationStatus = "draft"
	}

	event, err := h.sceneService.CreateEvent(
		r.Context(),
		req.SceneID,
		req.Title,
		req.Description,
		req.CoarseGeohash,
		req.AllowPrecise,
		req.PrecisePoint,
		req.Tags,
		req.StartsAt,
		req.EndsAt,
		req.LocationAccess,
		req.PlaceID,
		req.VenueID,
		req.Kind,
		publicationStatus,
	)
	if err != nil {
		WriteAPIError(w, r.Context(), err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	if err := json.NewEncoder(w).Encode(publicEventCopy(event)); err != nil {
		slog.ErrorContext(r.Context(), "failed to encode event response", "error", err)
	}
}

// UpdateEvent handles PATCH /events/{id} - updates an existing event.
func (h *EventHandlers) UpdateEvent(w http.ResponseWriter, r *http.Request) {
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
	existingEvent, err := h.sceneService.EventRepo().GetByID(eventID)
	if err != nil {
		WriteAPIError(w, r.Context(), err)
		return
	}

	// Get user DID from context
	userDID := middleware.GetUserDID(r.Context())
	if userDID == "" {
		ctx := middleware.SetErrorCode(r.Context(), ErrCodeAuthFailed)
		WriteError(w, ctx, http.StatusUnauthorized, ErrCodeAuthFailed, "Authentication required")
		return
	}

	// Check if user is scene owner
	isOwner, err := h.isSceneOwner(existingEvent.SceneID, userDID)
	if err != nil {
		if errors.Is(err, scene.ErrSceneNotFound) || errors.Is(err, scene.ErrSceneDeleted) {
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

	// Validate past-event time update restriction
	if req.StartsAt != nil && existingEvent.StartsAt.Before(time.Now()) {
		ctx := middleware.SetErrorCode(r.Context(), ErrCodeValidation)
		WriteError(w, ctx, http.StatusBadRequest, ErrCodeValidation, "Cannot update start time for past events")
		return
	}

	updated, err := h.sceneService.UpdateEvent(r.Context(), eventID, scene.UpdateEventParams{
		Version:        req.Version,
		Title:          req.Title,
		Description:    req.Description,
		Tags:           req.Tags,
		AllowPrecise:   req.AllowPrecise,
		PrecisePoint:   req.PrecisePoint,
		CoarseGeohash:  req.CoarseGeohash,
		StartsAt:       req.StartsAt,
		EndsAt:         req.EndsAt,
		LocationAccess: req.LocationAccess,
	})
	if err != nil {
		WriteAPIError(w, r.Context(), err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(publicEventCopy(updated)); err != nil {
		slog.ErrorContext(r.Context(), "failed to encode event response", "error", err)
	}
}

// GetEvent handles GET /events/{id} - retrieves an event.
func (h *EventHandlers) GetEvent(w http.ResponseWriter, r *http.Request) {
	pathParts := strings.Split(strings.TrimPrefix(r.URL.Path, "/events/"), "/")
	if len(pathParts) == 0 || pathParts[0] == "" {
		ctx := middleware.SetErrorCode(r.Context(), ErrCodeBadRequest)
		WriteError(w, ctx, http.StatusBadRequest, ErrCodeBadRequest, "Event ID is required")
		return
	}
	eventID := pathParts[0]

	// Get the event (delegates to service for GetEvent, then adds RSVP counts and active stream)
	foundEvent, err := h.sceneService.GetEvent(r.Context(), eventID)
	if err != nil {
		WriteAPIError(w, r.Context(), err)
		return
	}

	// Get RSVP counts for the event
	rsvpCounts, err := h.sceneService.RSVPRepo().GetCountsByEvent(eventID)
	if err != nil {
		slog.ErrorContext(r.Context(), "failed to get RSVP counts", "error", err, "event_id", eventID)
		ctx := middleware.SetErrorCode(r.Context(), ErrCodeInternal)
		WriteError(w, ctx, http.StatusInternalServerError, ErrCodeInternal, "Failed to retrieve RSVP counts")
		return
	}

	// Get active stream for the event
	activeStream, err := h.streamRepo.GetActiveStreamForEvent(eventID)
	if err != nil {
		slog.ErrorContext(r.Context(), "failed to get active stream", "error", err, "event_id", eventID)
		ctx := middleware.SetErrorCode(r.Context(), ErrCodeInternal)
		WriteError(w, ctx, http.StatusInternalServerError, ErrCodeInternal, "Failed to retrieve active stream")
		return
	}

	response := EventWithRSVPCounts{
		Event:        publicEventCopy(foundEvent),
		RSVPCounts:   rsvpCounts,
		ActiveStream: activeStream,
		Occurrence:   toPublicOccurrence(foundEvent),
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(response); err != nil {
		slog.ErrorContext(r.Context(), "failed to encode event response", "error", err)
	}
}

// CancelEvent handles POST /events/{id}/cancel - cancels an event.
func (h *EventHandlers) CancelEvent(w http.ResponseWriter, r *http.Request) {
	pathParts := strings.Split(strings.TrimPrefix(r.URL.Path, "/events/"), "/")
	if len(pathParts) == 0 || pathParts[0] == "" {
		ctx := middleware.SetErrorCode(r.Context(), ErrCodeBadRequest)
		WriteError(w, ctx, http.StatusBadRequest, ErrCodeBadRequest, "Event ID is required")
		return
	}
	eventID := pathParts[0]

	// Parse request body
	var req CancelEventRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil && err != io.EOF {
		ctx := middleware.SetErrorCode(r.Context(), ErrCodeBadRequest)
		WriteError(w, ctx, http.StatusBadRequest, ErrCodeBadRequest, "Invalid JSON in request body")
		return
	}

	// Sanitize reason
	if req.Reason != nil {
		sanitized := validate.SanitizeHTML(*req.Reason)
		req.Reason = &sanitized
	}

	// Get existing event
	existingEvent, err := h.sceneService.EventRepo().GetByID(eventID)
	if err != nil {
		WriteAPIError(w, r.Context(), err)
		return
	}

	// Get user DID from context
	userDID := middleware.GetUserDID(r.Context())
	if userDID == "" {
		ctx := middleware.SetErrorCode(r.Context(), ErrCodeAuthFailed)
		WriteError(w, ctx, http.StatusUnauthorized, ErrCodeAuthFailed, "Authentication required")
		return
	}

	// Check if user is scene owner
	isOwner, err := h.isSceneOwner(existingEvent.SceneID, userDID)
	if err != nil {
		if errors.Is(err, scene.ErrSceneNotFound) || errors.Is(err, scene.ErrSceneDeleted) {
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

	alreadyCancelled := existingEvent.Status == "cancelled" && existingEvent.CancelledAt != nil

	if err := h.sceneService.CancelEvent(r.Context(), eventID, req.Reason); err != nil {
		WriteAPIError(w, r.Context(), err)
		return
	}

	// Emit audit log only if this was the first cancellation
	if !alreadyCancelled {
		if err := audit.LogAccessFromRequest(r, h.auditRepo, "event", eventID, "event_cancel", audit.OutcomeSuccess); err != nil {
			slog.ErrorContext(r.Context(), "failed to log event cancellation", "error", err, "event_id", eventID)
		}
	}

	cancelledEvent, err := h.sceneService.EventRepo().GetByID(eventID)
	if err != nil {
		WriteAPIError(w, r.Context(), err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(publicEventCopy(cancelledEvent)); err != nil {
		slog.ErrorContext(r.Context(), "failed to encode event response", "error", err)
	}
}

// SearchEventsResponse represents the response for event search with active stream info.
type SearchEventsResponse struct {
	Events     []*EventWithRSVPCounts `json:"events"`
	NextCursor string                 `json:"next_cursor,omitempty"`
}

// SearchEvents handles GET /search/events - searches events by bbox and time range.
func (h *EventHandlers) SearchEvents(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query()
	bboxStr := query.Get("bbox")
	if bboxStr == "" {
		ctx := middleware.SetErrorCode(r.Context(), ErrCodeValidation)
		WriteError(w, ctx, http.StatusBadRequest, ErrCodeValidation, "bbox parameter is required")
		return
	}

	bboxParts := strings.Split(bboxStr, ",")
	if len(bboxParts) != 4 {
		ctx := middleware.SetErrorCode(r.Context(), ErrCodeValidation)
		WriteError(w, ctx, http.StatusBadRequest, ErrCodeValidation, "bbox must be in format: minLng,minLat,maxLng,maxLat")
		return
	}

	// Parse and validate bbox coordinates
	var minLng, minLat, maxLng, maxLat float64
	var err error

	if minLng, err = parseFloat(bboxParts[0], "minLng"); err != nil {
		ctx := middleware.SetErrorCode(r.Context(), ErrCodeValidation)
		WriteError(w, ctx, http.StatusBadRequest, ErrCodeValidation, err.Error())
		return
	}
	if minLat, err = parseFloat(bboxParts[1], "minLat"); err != nil {
		ctx := middleware.SetErrorCode(r.Context(), ErrCodeValidation)
		WriteError(w, ctx, http.StatusBadRequest, ErrCodeValidation, err.Error())
		return
	}
	if maxLng, err = parseFloat(bboxParts[2], "maxLng"); err != nil {
		ctx := middleware.SetErrorCode(r.Context(), ErrCodeValidation)
		WriteError(w, ctx, http.StatusBadRequest, ErrCodeValidation, err.Error())
		return
	}
	if maxLat, err = parseFloat(bboxParts[3], "maxLat"); err != nil {
		ctx := middleware.SetErrorCode(r.Context(), ErrCodeValidation)
		WriteError(w, ctx, http.StatusBadRequest, ErrCodeValidation, err.Error())
		return
	}

	// Validate bbox ranges
	if minLng < -180 || minLng > 180 || maxLng < -180 || maxLng > 180 {
		ctx := middleware.SetErrorCode(r.Context(), ErrCodeValidation)
		WriteError(w, ctx, http.StatusBadRequest, ErrCodeValidation, "longitude must be between -180 and 180")
		return
	}
	if minLat < -90 || minLat > 90 || maxLat < -90 || maxLat > 90 {
		ctx := middleware.SetErrorCode(r.Context(), ErrCodeValidation)
		WriteError(w, ctx, http.StatusBadRequest, ErrCodeValidation, "latitude must be between -90 and 90")
		return
	}
	if minLng >= maxLng {
		ctx := middleware.SetErrorCode(r.Context(), ErrCodeValidation)
		WriteError(w, ctx, http.StatusBadRequest, ErrCodeValidation, "minLng must be less than maxLng")
		return
	}
	if minLat >= maxLat {
		ctx := middleware.SetErrorCode(r.Context(), ErrCodeValidation)
		WriteError(w, ctx, http.StatusBadRequest, ErrCodeValidation, "minLat must be less than maxLat")
		return
	}

	// Parse time range
	fromStr := strings.TrimSpace(query.Get("from"))
	if fromStr == "" {
		fromStr = strings.TrimSpace(query.Get("start_date"))
	}
	toStr := strings.TrimSpace(query.Get("to"))
	if toStr == "" {
		toStr = strings.TrimSpace(query.Get("end_date"))
	}

	if fromStr == "" || toStr == "" {
		ctx := middleware.SetErrorCode(r.Context(), ErrCodeValidation)
		WriteError(w, ctx, http.StatusBadRequest, ErrCodeValidation, "both 'from' and 'to' parameters are required")
		return
	}

	from, err := time.Parse(time.RFC3339, fromStr)
	if err != nil {
		ctx := middleware.SetErrorCode(r.Context(), ErrCodeValidation)
		WriteError(w, ctx, http.StatusBadRequest, ErrCodeValidation, "invalid 'from' timestamp, must be RFC3339 format")
		return
	}

	to, err := time.Parse(time.RFC3339, toStr)
	if err != nil {
		ctx := middleware.SetErrorCode(r.Context(), ErrCodeValidation)
		WriteError(w, ctx, http.StatusBadRequest, ErrCodeValidation, "invalid 'to' timestamp, must be RFC3339 format")
		return
	}

	if !from.Before(to) {
		ctx := middleware.SetErrorCode(r.Context(), ErrCodeInvalidTimeRange)
		WriteError(w, ctx, http.StatusBadRequest, ErrCodeInvalidTimeRange, "'from' must be before 'to'")
		return
	}

	maxWindow := 30 * 24 * time.Hour
	if to.Sub(from) > maxWindow {
		ctx := middleware.SetErrorCode(r.Context(), ErrCodeValidation)
		WriteError(w, ctx, http.StatusBadRequest, ErrCodeValidation, "time window cannot exceed 30 days")
		return
	}

	searchQuery := query.Get("q")
	statusFilter := strings.ToLower(strings.TrimSpace(query.Get("status")))
	if statusFilter != "" && statusFilter != "upcoming" && statusFilter != "live" && statusFilter != "past" && statusFilter != "cancelled" {
		ctx := middleware.SetErrorCode(r.Context(), ErrCodeValidation)
		WriteError(w, ctx, http.StatusBadRequest, ErrCodeValidation, "status must be one of: upcoming, live, past, cancelled")
		return
	}
	sceneIDFilter := strings.TrimSpace(query.Get("scene_id"))
	organizerFilter := strings.TrimSpace(query.Get("organizer"))
	organizerSceneIDs := make([]string, 0)
	if organizerFilter != "" {
		organizerScenes, err := h.sceneService.SceneRepo().ListByOwner(organizerFilter)
		if err != nil {
			slog.ErrorContext(r.Context(), "failed to list organizer scenes", "error", err, "organizer", organizerFilter)
			ctx := middleware.SetErrorCode(r.Context(), ErrCodeInternal)
			WriteError(w, ctx, http.StatusInternalServerError, ErrCodeInternal, "Failed to search events")
			return
		}
		for _, organizerScene := range organizerScenes {
			organizerSceneIDs = append(organizerSceneIDs, organizerScene.ID)
		}
		if len(organizerSceneIDs) == 0 {
			response := SearchEventsResponse{
				Events: make([]*EventWithRSVPCounts, 0),
			}
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			if err := json.NewEncoder(w).Encode(response); err != nil {
				slog.ErrorContext(r.Context(), "failed to encode search response", "error", err)
			}
			return
		}

		if sceneIDFilter != "" {
			ownsScene := false
			for _, organizerSceneID := range organizerSceneIDs {
				if organizerSceneID == sceneIDFilter {
					ownsScene = true
					break
				}
			}
			if !ownsScene {
				response := SearchEventsResponse{
					Events: make([]*EventWithRSVPCounts, 0),
				}
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusOK)
				if err := json.NewEncoder(w).Encode(response); err != nil {
					slog.ErrorContext(r.Context(), "failed to encode search response", "error", err)
				}
				return
			}
			organizerSceneIDs = []string{sceneIDFilter}
		}
	}

	limitStr := query.Get("limit")
	limit := 50
	if limitStr != "" {
		parsedLimit, err := parseIntInRange(limitStr, "limit", 1, 100)
		if err != nil {
			ctx := middleware.SetErrorCode(r.Context(), ErrCodeValidation)
			WriteError(w, ctx, http.StatusBadRequest, ErrCodeValidation, err.Error())
			return
		}
		limit = parsedLimit
	}

	cursor := query.Get("cursor")

	// Build trust scores map
	var trustScores map[string]float64
	if h.trustScoreStore != nil {
		trustScores = make(map[string]float64)
	}

	eventRepo := h.sceneService.EventRepo()
	sceneRepo := h.sceneService.SceneRepo()
	rsvpRepo := h.sceneService.RSVPRepo()

	// Search events
	events, nextCursor, err := eventRepo.SearchEvents(scene.EventSearchOptions{
		MinLng:      minLng,
		MinLat:      minLat,
		MaxLng:      maxLng,
		MaxLat:      maxLat,
		From:        from,
		To:          to,
		Query:       searchQuery,
		Status:      statusFilter,
		SceneID:     sceneIDFilter,
		SceneIDs:    organizerSceneIDs,
		Limit:       limit,
		Cursor:      cursor,
		TrustScores: trustScores,
	})
	if err != nil {
		slog.ErrorContext(r.Context(), "failed to search events", "error", err)
		ctx := middleware.SetErrorCode(r.Context(), ErrCodeInternal)
		WriteError(w, ctx, http.StatusInternalServerError, ErrCodeInternal, "Failed to search events")
		return
	}

	// Fetch trust scores on-demand if store is available
	if h.trustScoreStore != nil && len(events) > 0 {
		sceneIDs := make(map[string]bool)
		for _, event := range events {
			sceneIDs[event.SceneID] = true
		}

		for sceneID := range sceneIDs {
			score, err := h.trustScoreStore.GetScore(sceneID)
			if err != nil {
				slog.WarnContext(r.Context(), "failed to get trust score", "scene_id", sceneID, "error", err)
				continue
			}
			if score != nil {
				trustScores[sceneID] = score.Score
			}
		}

		if len(trustScores) > 0 {
			events, nextCursor, err = eventRepo.SearchEvents(scene.EventSearchOptions{
				MinLng:      minLng,
				MinLat:      minLat,
				MaxLng:      maxLng,
				MaxLat:      maxLat,
				From:        from,
				To:          to,
				Query:       searchQuery,
				Status:      statusFilter,
				SceneID:     sceneIDFilter,
				SceneIDs:    organizerSceneIDs,
				Limit:       limit,
				Cursor:      cursor,
				TrustScores: trustScores,
			})
			if err != nil {
				slog.ErrorContext(r.Context(), "failed to search events with trust scores", "error", err)
				ctx := middleware.SetErrorCode(r.Context(), ErrCodeInternal)
				WriteError(w, ctx, http.StatusInternalServerError, ErrCodeInternal, "Failed to search events")
				return
			}
		}
	}

	// Batch fetch active streams
	eventIDs := make([]string, len(events))
	for i, event := range events {
		eventIDs[i] = event.ID
	}

	activeStreamsMap, err := h.streamRepo.GetActiveStreamsForEvents(eventIDs)
	if err != nil {
		slog.ErrorContext(r.Context(), "failed to get active streams", "error", err)
		ctx := middleware.SetErrorCode(r.Context(), ErrCodeInternal)
		WriteError(w, ctx, http.StatusInternalServerError, ErrCodeInternal, "Failed to retrieve active streams")
		return
	}

	// Batch fetch RSVP counts
	rsvpCountsMap, err := rsvpRepo.GetCountsForEvents(eventIDs)
	if err != nil {
		slog.ErrorContext(r.Context(), "failed to get RSVP counts", "error", err)
		ctx := middleware.SetErrorCode(r.Context(), ErrCodeInternal)
		WriteError(w, ctx, http.StatusInternalServerError, ErrCodeInternal, "Failed to retrieve RSVP counts")
		return
	}

	sceneMap := make(map[string]*SceneSearchResult)
	if len(events) > 0 {
		sceneIDs := make(map[string]struct{}, len(events))
		orderedSceneIDs := make([]string, 0)
		for _, event := range events {
			if _, seen := sceneIDs[event.SceneID]; seen {
				continue
			}
			sceneIDs[event.SceneID] = struct{}{}
			orderedSceneIDs = append(orderedSceneIDs, event.SceneID)
		}

		if batchRepo, ok := sceneRepo.(sceneBatchFetcher); ok {
			parentScenes, err := batchRepo.GetByIDs(orderedSceneIDs)
			if err != nil {
				slog.WarnContext(r.Context(), "failed to batch fetch scenes for event search response; falling back to individual fetches", "error", err)
			} else {
				for _, parentScene := range parentScenes {
					sceneMap[parentScene.ID] = toSceneSearchResult(parentScene)
				}
			}
		}

		for _, sceneID := range orderedSceneIDs {
			if _, ok := sceneMap[sceneID]; ok {
				continue
			}
			parentScene, err := sceneRepo.GetByID(sceneID)
			if err != nil {
				slog.WarnContext(r.Context(), "failed to fetch scene for event search response", "scene_id", sceneID, "error", err)
				continue
			}
			sceneMap[sceneID] = toSceneSearchResult(parentScene)
		}
	}

	eventsWithData := make([]*EventWithRSVPCounts, len(events))
	for i, event := range events {
		eventsWithData[i] = &EventWithRSVPCounts{
			Event:        publicEventCopy(event),
			RSVPCounts:   rsvpCountsMap[event.ID],
			Scene:        sceneMap[event.SceneID],
			ActiveStream: activeStreamsMap[event.ID],
			Occurrence:   toPublicOccurrence(event),
		}
	}

	response := SearchEventsResponse{
		Events:     eventsWithData,
		NextCursor: nextCursor,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(response); err != nil {
		slog.ErrorContext(r.Context(), "failed to encode search response", "error", err)
	}
}

func parseFloat(s, fieldName string) (float64, error) {
	s = strings.TrimSpace(s)
	val, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return 0, fmt.Errorf("%s must be a valid number", fieldName)
	}
	return val, nil
}

func parseIntInRange(s, fieldName string, min, max int) (int, error) {
	s = strings.TrimSpace(s)
	val, err := strconv.Atoi(s)
	if err != nil {
		return 0, fmt.Errorf("%s must be a valid integer", fieldName)
	}
	if val < min || val > max {
		return 0, fmt.Errorf("%s must be between %d and %d", fieldName, min, max)
	}
	return val, nil
}

// RegisterEventRoutes registers all event-related routes on the given mux.
func RegisterEventRoutes(mux *http.ServeMux, deps *RouteDeps, h *EventHandlers, rsvpH *RSVPHandlers, postH *PostHandlers, protectedLocationH *ProtectedLocationHandlers) {
	// Event creation (with rate limiting: 5 req/hour per user)
	eventCreationLimit := middleware.RateLimitConfig{
		RequestsPerWindow: 5,
		WindowDuration:    time.Hour,
	}
	eventCreationHandler := deps.RateLimit(h.CreateEvent, eventCreationLimit, middleware.UserKeyFunc())

	mux.HandleFunc("/events", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodPost:
			eventCreationHandler.ServeHTTP(w, r)
		default:
			ctx := middleware.SetErrorCode(r.Context(), ErrCodeBadRequest)
			WriteError(w, ctx, http.StatusMethodNotAllowed, ErrCodeBadRequest, "Method not allowed")
		}
	})

	mux.HandleFunc("/events/", func(w http.ResponseWriter, r *http.Request) {
		// Parse path to check for special endpoints
		// Expected patterns: /events/{id}, /events/{id}/cancel, /events/{id}/rsvp, /events/{id}/feed
		pathParts := strings.Split(strings.TrimPrefix(r.URL.Path, "/events/"), "/")

		// Check if this is a feed request: /events/{id}/feed
		if len(pathParts) == 2 && pathParts[0] != "" && pathParts[1] == "feed" && r.Method == http.MethodGet {
			postH.GetEventFeed(w, r)
			return
		}

		// Check if this is a cancel request: /events/{id}/cancel
		if len(pathParts) == 2 && pathParts[0] != "" && pathParts[1] == "cancel" && r.Method == http.MethodPost {
			h.CancelEvent(w, r)
			return
		}

		// Check if this is an RSVP request: /events/{id}/rsvp
		if len(pathParts) == 2 && pathParts[0] != "" && pathParts[1] == "rsvp" {
			switch r.Method {
			case http.MethodPost:
				rsvpH.CreateOrUpdateRSVP(w, r)
			case http.MethodDelete:
				rsvpH.DeleteRSVP(w, r)
			default:
				ctx := middleware.SetErrorCode(r.Context(), ErrCodeBadRequest)
				WriteError(w, ctx, http.StatusMethodNotAllowed, ErrCodeBadRequest, "Method not allowed")
			}
			return
		}

		switch r.Method {
		case http.MethodGet:
			h.GetEvent(w, r)
		case http.MethodPatch:
			h.UpdateEvent(w, r)
		default:
			ctx := middleware.SetErrorCode(r.Context(), ErrCodeBadRequest)
			WriteError(w, ctx, http.StatusMethodNotAllowed, ErrCodeBadRequest, "Method not allowed")
		}
	})

	// /api/v1/events/ sub-routes (protected location and RSVP)
	mux.Handle("/api/v1/events/", v1EventSubrouter(protectedLocationH, rsvpH.CreateOrUpdateRSVP))
}