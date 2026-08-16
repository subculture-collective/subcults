package api

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/onnwee/subcults/internal/audience"
	"github.com/onnwee/subcults/internal/middleware"
	"github.com/onnwee/subcults/internal/signal"
)

// SignalHandlers exposes only draft creation, publication, and public revision metadata.
// Delivery execution is intentionally not reachable from this HTTP boundary.
type SignalHandlers struct {
	service  *signal.Service
	audience *audience.Service
}

func NewSignalHandlers(service *signal.Service, audienceServices ...*audience.Service) *SignalHandlers {
	handler := &SignalHandlers{service: service}
	if len(audienceServices) > 0 {
		handler.audience = audienceServices[0]
	}
	return handler
}

type CreateSignalRequest struct {
	ID                 string          `json:"id"`
	OwnerType          string          `json:"owner_type"`
	OwnerID            string          `json:"owner_id"`
	TargetType         string          `json:"target_type"`
	TargetID           string          `json:"target_id"`
	Subject            string          `json:"subject"`
	Body               string          `json:"body"`
	DeepLink           string          `json:"deep_link,omitempty"`
	AudienceDefinition json.RawMessage `json:"audience_definition"`
	ConsentScopeIDs    []string        `json:"consent_scope_ids,omitempty"`
}

func (h *SignalHandlers) CreateDraft(w http.ResponseWriter, r *http.Request) {
	did := middleware.GetUserDID(r.Context())
	if did == "" {
		writeSignalError(w, r, http.StatusUnauthorized, ErrCodeAuthFailed, "authentication required")
		return
	}
	var req CreateSignalRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeSignalError(w, r, http.StatusBadRequest, ErrCodeBadRequest, "invalid request body")
		return
	}
	revision, err := h.service.CreateDraft(r.Context(), signal.Signal{ID: req.ID, OwnerType: req.OwnerType, OwnerID: req.OwnerID, TargetType: req.TargetType, TargetID: req.TargetID, ConsentScopeIDs: req.ConsentScopeIDs}, signal.Content{Subject: req.Subject, Body: req.Body, DeepLink: req.DeepLink}, string(req.AudienceDefinition), did, nil)
	if err != nil {
		writeSignalServiceError(w, r, err)
		return
	}
	writeJSON(w, http.StatusCreated, revision)
}
func (h *SignalHandlers) Publish(w http.ResponseWriter, r *http.Request) {
	if middleware.GetUserDID(r.Context()) == "" {
		writeSignalError(w, r, http.StatusUnauthorized, ErrCodeAuthFailed, "authentication required")
		return
	}
	id := signalPathID(r.URL.Path)
	id = strings.TrimSuffix(id, "/publish")
	id = strings.Trim(id, "/")
	if id == "" {
		writeSignalError(w, r, http.StatusBadRequest, ErrCodeBadRequest, "signal ID is required")
		return
	}
	revision, err := h.service.Publish(r.Context(), id)
	if err != nil {
		writeSignalServiceError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, revision)
}
func (h *SignalHandlers) Get(w http.ResponseWriter, r *http.Request) {
	id := signalPathID(r.URL.Path)
	value, err := h.service.Get(r.Context(), id)
	if err != nil {
		writeSignalServiceError(w, r, err)
		return
	}
	if value.Signal.State != signal.StatePublished {
		writeSignalError(w, r, http.StatusNotFound, ErrCodeNotFound, "signal not found")
		return
	}
	response := publicSignalDetail{Signal: publicSignal{
		ID: value.Signal.ID, Title: value.Revision.Content.Subject, Body: value.Revision.Content.Body,
		State: string(value.Signal.State), Sender: publicSignalReference{ID: value.Signal.OwnerID, Name: value.Signal.OwnerID, Type: value.Signal.OwnerType},
		Target: &publicSignalTarget{ID: value.Signal.TargetID, Title: value.Signal.TargetID, Type: value.Signal.TargetType},
	}}
	if h.audience != nil {
		response.ConsentScopes = h.consentStates(r, value.Signal.ConsentScopeIDs)
	}
	writeJSON(w, http.StatusOK, response)
}

type publicSignalReference struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Type string `json:"type"`
}
type publicSignalTarget struct {
	ID    string `json:"id"`
	Title string `json:"title,omitempty"`
	Type  string `json:"type"`
}
type publicSignal struct {
	ID     string                `json:"id"`
	Title  string                `json:"title"`
	Body   string                `json:"body,omitempty"`
	State  string                `json:"state"`
	Sender publicSignalReference `json:"sender"`
	Target *publicSignalTarget   `json:"target,omitempty"`
}
type publicConsentScope struct {
	ID                string                 `json:"id"`
	Sender            publicSignalReference  `json:"sender"`
	Channel           string                 `json:"channel"`
	Purpose           string                 `json:"purpose"`
	DisclosureVersion string                 `json:"disclosure_version"`
	Region            string                 `json:"region,omitempty"`
	Tour              *publicSignalReference `json:"tour,omitempty"`
	Place             *publicSignalReference `json:"place,omitempty"`
}
type publicConsentState struct {
	Scope             publicConsentScope `json:"scope"`
	Status            string             `json:"status"`
	VerificationState string             `json:"verification_state"`
}
type publicSignalDetail struct {
	Signal        publicSignal         `json:"signal"`
	ConsentScopes []publicConsentState `json:"consent_scopes,omitempty"`
}

type consentMutationRequest struct {
	ScopeID string `json:"scope_id"`
	Action  string `json:"action"`
}

// MutateConsent resolves the authenticated DID through verified, revocable
// link evidence. The request never accepts a caller-supplied contact ID.
func (h *SignalHandlers) MutateConsent(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost || h.audience == nil {
		writeSignalError(w, r, http.StatusMethodNotAllowed, ErrCodeBadRequest, "method not allowed")
		return
	}
	did := middleware.GetUserDID(r.Context())
	if did == "" {
		writeSignalError(w, r, http.StatusUnauthorized, ErrCodeAuthFailed, "authentication required")
		return
	}
	var request consentMutationRequest
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil || request.ScopeID == "" || (request.Action != "grant" && request.Action != "revoke") {
		writeSignalError(w, r, http.StatusBadRequest, ErrCodeValidation, "invalid consent mutation")
		return
	}
	contacts, err := h.audience.ActiveContactsForDID(r.Context(), did)
	if err != nil || len(contacts) == 0 {
		writeSignalError(w, r, http.StatusForbidden, ErrCodeForbidden, "verified contact link required")
		return
	}
	evidence := map[string]string{"request_id": middleware.GetRequestID(r.Context())}
	now := time.Now().UTC()
	if request.Action == "grant" {
		err = h.audience.Grant(r.Context(), contacts[0].ID, request.ScopeID, "preference_center", evidence, now)
	} else {
		err = h.audience.Revoke(r.Context(), contacts[0].ID, request.ScopeID, "preference_center", evidence, now)
	}
	if err != nil {
		writeSignalError(w, r, http.StatusBadRequest, ErrCodeValidation, "consent scope unavailable")
		return
	}
	state, ok := h.consentState(r, contacts[0], request.ScopeID)
	if !ok {
		writeSignalError(w, r, http.StatusInternalServerError, ErrCodeInternal, "consent state unavailable")
		return
	}
	writeJSON(w, http.StatusOK, map[string]publicConsentState{"consent": state})
}

func (h *SignalHandlers) consentStates(r *http.Request, scopeIDs []string) []publicConsentState {
	did := middleware.GetUserDID(r.Context())
	contacts, _ := h.audience.ActiveContactsForDID(r.Context(), did)
	var contact audience.ContactPoint
	if len(contacts) > 0 {
		contact = contacts[0]
	}
	states := make([]publicConsentState, 0, len(scopeIDs))
	for _, scopeID := range scopeIDs {
		if state, ok := h.consentState(r, contact, scopeID); ok {
			states = append(states, state)
		}
	}
	return states
}

func (h *SignalHandlers) consentState(r *http.Request, contact audience.ContactPoint, scopeID string) (publicConsentState, bool) {
	scope, err := h.audience.GetScope(r.Context(), scopeID)
	if err != nil {
		return publicConsentState{}, false
	}
	status := "not_granted"
	if contact.ID != "" {
		if action, exists, err := h.audience.ConsentStatus(r.Context(), contact.ID, scopeID); err == nil && exists {
			if action == audience.ConsentGrant {
				status = "granted"
			} else {
				status = "revoked"
			}
		}
	}
	verification := "unverified"
	if contact.VerifiedAt != nil {
		verification = "verified"
	}
	state := publicConsentState{Status: status, VerificationState: verification, Scope: publicConsentScope{
		ID: scopeID, Sender: publicSignalReference{ID: scope.SenderID, Name: scope.SenderID, Type: scope.SenderType},
		Channel: scope.Channel, Purpose: scope.Purpose, DisclosureVersion: scope.DisclosureVersion, Region: scope.Region,
	}}
	if scope.TourID != "" {
		state.Scope.Tour = &publicSignalReference{ID: scope.TourID, Name: scope.TourID, Type: "tour"}
	}
	if scope.PlaceID != "" {
		state.Scope.Place = &publicSignalReference{ID: scope.PlaceID, Name: scope.PlaceID, Type: "place"}
	}
	return state, true
}

func signalPathID(path string) string {
	path = strings.TrimPrefix(path, "/api")
	return strings.Trim(strings.TrimPrefix(path, "/signals/"), "/")
}
func writeSignalServiceError(w http.ResponseWriter, r *http.Request, err error) {
	if status, code, message, ok := MapDomainError(err); ok {
		ctx := middleware.SetErrorCode(r.Context(), code)
		WriteError(w, ctx, status, code, message)
		return
	}
	writeSignalError(w, r, http.StatusBadRequest, ErrCodeValidation, "invalid signal")
}
func writeSignalError(w http.ResponseWriter, r *http.Request, status int, code, message string) {
	ctx := middleware.SetErrorCode(r.Context(), code)
	WriteError(w, ctx, status, code, message)
}
func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

// RegisterSignalRoutes registers all signal-related routes on the given mux.
func RegisterSignalRoutes(mux *http.ServeMux, deps *RouteDeps, h *SignalHandlers) {
	registerPrefix := func(prefix string) {
		mux.HandleFunc(prefix, func(w http.ResponseWriter, r *http.Request) {
			if r.Method == http.MethodPost {
				h.CreateDraft(w, r)
				return
			}
			ctx := middleware.SetErrorCode(r.Context(), ErrCodeBadRequest)
			WriteError(w, ctx, http.StatusMethodNotAllowed, ErrCodeBadRequest, "Method not allowed")
		})
		mux.HandleFunc(prefix+"/", func(w http.ResponseWriter, r *http.Request) {
			if strings.HasSuffix(strings.TrimRight(r.URL.Path, "/"), "/publish") && r.Method == http.MethodPost {
				h.Publish(w, r)
				return
			}
			if r.Method == http.MethodGet {
				h.Get(w, r)
				return
			}
			ctx := middleware.SetErrorCode(r.Context(), ErrCodeBadRequest)
			WriteError(w, ctx, http.StatusMethodNotAllowed, ErrCodeBadRequest, "Method not allowed")
		})
	}
	registerPrefix("/signals")
	registerPrefix("/api/signals")

	mux.HandleFunc("/audience/consent", h.MutateConsent)
	mux.HandleFunc("/api/audience/consent", h.MutateConsent)
}
