package api

import (
	"net/http"

	"github.com/onnwee/subcults/internal/middleware"
	"github.com/onnwee/subcults/internal/notification"
)

type NotificationHandlers struct{ service *notification.Service }

func NewNotificationHandlers(service *notification.Service) *NotificationHandlers {
	return &NotificationHandlers{service: service}
}

func (h *NotificationHandlers) Subscribe(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())
	if userID == "" {
		WriteError(w, r.Context(), http.StatusUnauthorized, ErrCodeUnauthorized, "Authentication required")
		return
	}
	var body notification.BrowserSubscription
	if err := decodeLimitedJSON(w, r, &body); err != nil {
		WriteError(w, r.Context(), http.StatusBadRequest, ErrCodeBadRequest, "Invalid subscription")
		return
	}
	switch r.Method {
	case http.MethodPost:
		if err := h.service.Subscribe(r.Context(), userID, r.UserAgent(), body); err != nil {
			WriteError(w, r.Context(), http.StatusBadRequest, ErrCodeValidation, "Invalid subscription")
			return
		}
		w.WriteHeader(http.StatusNoContent)
	case http.MethodDelete:
		if err := h.service.Unsubscribe(r.Context(), userID, body.Endpoint); err != nil {
			WriteError(w, r.Context(), http.StatusInternalServerError, ErrCodeInternal, "Could not remove subscription")
			return
		}
		w.WriteHeader(http.StatusNoContent)
	default:
		w.Header().Set("Allow", "POST, DELETE")
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}
