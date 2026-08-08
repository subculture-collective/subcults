package api

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/onnwee/subcults/internal/locationaccess"
	"github.com/onnwee/subcults/internal/middleware"
	"github.com/onnwee/subcults/internal/scene"
)

type ProtectedLocationHandlers struct {
	repository locationaccess.Repository
	events     scene.EventRepository
	scenes     scene.SceneRepository
}

func NewProtectedLocationHandlers(repository locationaccess.Repository, events scene.EventRepository, scenes scene.SceneRepository) *ProtectedLocationHandlers {
	return &ProtectedLocationHandlers{repository: repository, events: events, scenes: scenes}
}

func (h *ProtectedLocationHandlers) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	parts := strings.Split(strings.Trim(strings.TrimPrefix(r.URL.Path, "/api/v1/events/"), "/"), "/")
	if len(parts) != 2 || parts[0] == "" {
		WriteError(w, r.Context(), http.StatusNotFound, ErrCodeNotFound, "Route not found")
		return
	}
	switch parts[1] {
	case "location":
		h.location(w, r, parts[0])
	case "location-grants":
		h.grants(w, r, parts[0])
	default:
		WriteError(w, r.Context(), http.StatusNotFound, ErrCodeNotFound, "Route not found")
	}
}

func (h *ProtectedLocationHandlers) location(w http.ResponseWriter, r *http.Request, eventID string) {
	if r.Method != http.MethodGet {
		methodNotAllowed(w, http.MethodGet)
		return
	}
	userID := middleware.GetUserID(r.Context())
	if userID == "" {
		WriteError(w, r.Context(), http.StatusUnauthorized, ErrCodeUnauthorized, "Authentication required")
		return
	}
	event, err := h.events.GetByID(eventID)
	if err != nil {
		WriteError(w, r.Context(), http.StatusNotFound, ErrCodeNotFound, "Event not found")
		return
	}
	if event.PrecisePoint == nil {
		WriteError(w, r.Context(), http.StatusNotFound, ErrCodeNotFound, "Precise event location is unavailable")
		return
	}
	if event.LocationAccess == "protected" {
		allowed, err := h.repository.CanView(r.Context(), eventID, userID, time.Now())
		if err != nil {
			WriteError(w, r.Context(), http.StatusInternalServerError, ErrCodeInternal, "Location authorization failed")
			return
		}
		if !allowed {
			WriteError(w, r.Context(), http.StatusForbidden, ErrCodeForbidden, "RSVP, ticket, membership, or organizer access is required")
			return
		}
	}
	writeData(w, http.StatusOK, map[string]any{"event_id": eventID, "point": event.PrecisePoint, "precision": "precise", "access": event.LocationAccess})
}

func (h *ProtectedLocationHandlers) grants(w http.ResponseWriter, r *http.Request, eventID string) {
	event, err := h.events.GetByID(eventID)
	if err != nil {
		WriteError(w, r.Context(), http.StatusNotFound, ErrCodeNotFound, "Event not found")
		return
	}
	userDID := middleware.GetUserDID(r.Context())
	userID := middleware.GetUserID(r.Context())
	parent, err := h.scenes.GetByID(event.SceneID)
	if err != nil || userDID == "" || userID == "" || !parent.IsOwner(userDID) {
		WriteError(w, r.Context(), http.StatusForbidden, ErrCodeForbidden, "Event organizer access required")
		return
	}
	var body struct {
		UserID    string     `json:"user_id"`
		Reason    string     `json:"reason"`
		ExpiresAt *time.Time `json:"expires_at,omitempty"`
	}
	if err := decodeLimitedJSON(w, r, &body); err != nil {
		WriteError(w, r.Context(), http.StatusBadRequest, ErrCodeBadRequest, "Invalid request body")
		return
	}
	if body.UserID == "" {
		WriteError(w, r.Context(), http.StatusBadRequest, ErrCodeValidation, "user_id is required")
		return
	}
	if r.Method == http.MethodDelete {
		if err := h.repository.Revoke(r.Context(), eventID, body.UserID, time.Now()); err != nil {
			WriteError(w, r.Context(), http.StatusInternalServerError, ErrCodeInternal, "Could not revoke location access")
			return
		}
		w.WriteHeader(http.StatusNoContent)
		return
	}
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", "POST, DELETE")
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	if body.Reason != "membership" && body.Reason != "rsvp" && body.Reason != "ticket" && body.Reason != "manual" {
		WriteError(w, r.Context(), http.StatusBadRequest, ErrCodeValidation, "Invalid grant reason")
		return
	}
	grant := locationaccess.Grant{EventID: eventID, UserID: body.UserID, Reason: body.Reason, GrantedByID: userID, GrantedAt: time.Now(), ExpiresAt: body.ExpiresAt}
	if err := h.repository.Grant(r.Context(), grant); err != nil {
		WriteError(w, r.Context(), http.StatusInternalServerError, ErrCodeInternal, "Could not grant location access")
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(map[string]any{"data": grant})
}
