package api

import (
	"crypto/subtle"
	"encoding/json"
	"errors"
	"net/http"
	"net/url"
	"strings"

	"github.com/bluesky-social/indigo/atproto/syntax"
	"github.com/google/uuid"
	"github.com/onnwee/subcults/internal/atprotocol"
	"github.com/onnwee/subcults/internal/middleware"
)

// ATProtoOAuthHandlers exposes metadata and authenticated linking operations.
type ATProtoOAuthHandlers struct {
	service      *atprotocol.OAuthService
	provisioning *atprotocol.ProvisioningService
	syncPassword string
}

// SetSyncPassword enables the private Tap webhook using HTTP Basic auth.
func (h *ATProtoOAuthHandlers) SetSyncPassword(password string) { h.syncPassword = password }

// Sync accepts authenticated Tap webhook events and acknowledges only after a
// durable observation or quarantine record has been committed.
func (h *ATProtoOAuthHandlers) Sync(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		methodNotAllowed(w, http.MethodPost)
		return
	}
	username, password, ok := r.BasicAuth()
	if h.syncPassword == "" || !ok || username != "admin" || subtle.ConstantTimeCompare([]byte(password), []byte(h.syncPassword)) != 1 {
		WriteError(w, r.Context(), http.StatusUnauthorized, ErrCodeUnauthorized, "Authentication required")
		return
	}
	var event atprotocol.TapEnvelope
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1024*1024)).Decode(&event); err != nil {
		WriteError(w, r.Context(), http.StatusBadRequest, ErrCodeBadRequest, "Invalid Tap event")
		return
	}
	result, err := h.service.IngestTap(r.Context(), event)
	if err != nil {
		WriteError(w, r.Context(), http.StatusInternalServerError, ErrCodeInternal, "Tap event was not durably recorded")
		return
	}
	writeData(w, http.StatusOK, result)
}

// Publish writes a server-serialized Studio entity to the linked creator PDS.
func (h *ATProtoOAuthHandlers) Publish(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		methodNotAllowed(w, http.MethodPost)
		return
	}
	userID := middleware.GetUserID(r.Context())
	if userID == "" {
		WriteError(w, r.Context(), http.StatusUnauthorized, ErrCodeUnauthorized, "Authentication required")
		return
	}
	var request struct {
		EntityType string `json:"entity_type"`
		EntityID   string `json:"entity_id"`
		SwapCID    string `json:"swap_cid,omitempty"`
	}
	if err := decodeLimitedJSON(w, r, &request); err != nil {
		WriteError(w, r.Context(), http.StatusBadRequest, ErrCodeBadRequest, "Invalid request body")
		return
	}
	if _, err := uuid.Parse(request.EntityID); err != nil {
		WriteError(w, r.Context(), http.StatusBadRequest, ErrCodeBadRequest, "entity_id must be a UUID")
		return
	}
	result, err := h.service.PublishEntity(r.Context(), userID, request.EntityType, request.EntityID, request.SwapCID)
	if err != nil {
		status, code, message, ok := MapDomainError(err)
		if ok {
			WriteError(w, r.Context(), status, code, message)
		} else {
			WriteError(w, r.Context(), http.StatusInternalServerError, ErrCodeInternal, "An internal error occurred")
		}
		return
	}
	writeData(w, http.StatusAccepted, result)
}

// Projection returns public indexing state for a canonical AT URI.
func (h *ATProtoOAuthHandlers) Projection(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		methodNotAllowed(w, http.MethodGet)
		return
	}
	uri := strings.TrimSpace(r.URL.Query().Get("uri"))
	parsed, err := syntax.ParseATURI(uri)
	if err != nil || !atprotocol.IsCanonicalCollection(parsed.Collection().String()) {
		WriteError(w, r.Context(), http.StatusBadRequest, ErrCodeBadRequest, "A canonical Subcults AT URI is required")
		return
	}
	mapping, err := h.service.Projection(r.Context(), uri)
	if err != nil {
		if status, code, message, ok := MapDomainError(err); ok {
			WriteError(w, r.Context(), status, code, message)
		} else {
			WriteError(w, r.Context(), http.StatusInternalServerError, ErrCodeInternal, "Could not load projection state")
		}
		return
	}
	writeData(w, http.StatusOK, mapping)
}

// NewATProtoOAuthHandlers creates AT Protocol OAuth HTTP handlers.
func NewATProtoOAuthHandlers(service *atprotocol.OAuthService, provisioning *atprotocol.ProvisioningService) *ATProtoOAuthHandlers {
	return &ATProtoOAuthHandlers{service: service, provisioning: provisioning}
}

// Provision issues a guarded single-use invitation without receiving a password.
func (h *ATProtoOAuthHandlers) Provision(w http.ResponseWriter, r *http.Request) {
	if h.provisioning == nil {
		WriteError(w, r.Context(), http.StatusServiceUnavailable, "atproto_provisioning_unavailable", "AT Protocol account provisioning is unavailable")
		return
	}
	userID := middleware.GetUserID(r.Context())
	if userID == "" {
		WriteError(w, r.Context(), http.StatusUnauthorized, ErrCodeUnauthorized, "Authentication required")
		return
	}
	switch r.Method {
	case http.MethodGet:
		status, err := h.provisioning.Status(r.Context(), userID)
		if errors.Is(err, atprotocol.ErrOAuthRequestNotFound) {
			writeData(w, http.StatusOK, nil)
			return
		}
		if err != nil {
			WriteError(w, r.Context(), http.StatusInternalServerError, ErrCodeInternal, "Could not load provisioning status")
			return
		}
		writeData(w, http.StatusOK, status)
	case http.MethodPost:
		var request struct {
			Handle         string `json:"handle"`
			TurnstileToken string `json:"turnstile_token"`
		}
		if err := decodeLimitedJSON(w, r, &request); err != nil {
			WriteError(w, r.Context(), http.StatusBadRequest, ErrCodeBadRequest, "Invalid request body")
			return
		}
		result, err := h.provisioning.Provision(r.Context(), userID, request.Handle, request.TurnstileToken, clientAddress(r))
		if err != nil {
			status, code, message, ok := MapDomainError(err)
			if ok {
				WriteError(w, r.Context(), status, code, message)
			} else {
				WriteError(w, r.Context(), http.StatusInternalServerError, ErrCodeInternal, "An internal error occurred")
			}
			return
		}
		writeData(w, http.StatusCreated, result)
	default:
		w.Header().Set("Allow", "GET, POST")
		WriteError(w, r.Context(), http.StatusMethodNotAllowed, ErrCodeBadRequest, "Method not allowed")
	}
}

// ClientMetadata serves the OAuth client metadata document.
func (h *ATProtoOAuthHandlers) ClientMetadata(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		methodNotAllowed(w, http.MethodGet)
		return
	}
	writeRawJSON(w, http.StatusOK, h.service.ClientMetadata())
}

// JWKS serves the confidential client's public signing key.
func (h *ATProtoOAuthHandlers) JWKS(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		methodNotAllowed(w, http.MethodGet)
		return
	}
	writeRawJSON(w, http.StatusOK, h.service.PublicJWKS())
}

// Start begins identity linking.
func (h *ATProtoOAuthHandlers) Start(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		methodNotAllowed(w, http.MethodPost)
		return
	}
	userID := middleware.GetUserID(r.Context())
	if userID == "" {
		WriteError(w, r.Context(), http.StatusUnauthorized, ErrCodeUnauthorized, "Authentication required")
		return
	}
	var request struct {
		Identifier string `json:"identifier"`
		ReturnPath string `json:"return_path"`
	}
	if err := decodeLimitedJSON(w, r, &request); err != nil {
		WriteError(w, r.Context(), http.StatusBadRequest, ErrCodeBadRequest, "Invalid request body")
		return
	}
	if request.ReturnPath == "" {
		request.ReturnPath = "/settings"
	}
	redirect, err := h.service.Start(r.Context(), userID, request.Identifier, request.ReturnPath, false)
	if err != nil {
		WriteError(w, r.Context(), http.StatusBadRequest, "atproto_oauth_start_failed", err.Error())
		return
	}
	writeData(w, http.StatusAccepted, map[string]string{"redirect_url": redirect})
}

// Callback completes the PDS authorization redirect.
func (h *ATProtoOAuthHandlers) Callback(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		methodNotAllowed(w, http.MethodGet)
		return
	}
	_, returnPath, err := h.service.Callback(r.Context(), r.URL.Query())
	if err != nil {
		target := "/settings?atproto=error"
		if strings.HasPrefix(returnPath, "/") && !strings.HasPrefix(returnPath, "//") {
			target = returnPath + querySeparator(returnPath) + "atproto=error"
		}
		http.Redirect(w, r, target, http.StatusSeeOther)
		return
	}
	http.Redirect(w, r, returnPath+querySeparator(returnPath)+"atproto=linked", http.StatusSeeOther)
}

// Status returns the current link without exposing session secrets.
func (h *ATProtoOAuthHandlers) Status(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		methodNotAllowed(w, http.MethodGet)
		return
	}
	userID := middleware.GetUserID(r.Context())
	if userID == "" {
		WriteError(w, r.Context(), http.StatusUnauthorized, ErrCodeUnauthorized, "Authentication required")
		return
	}
	link, err := h.service.Status(r.Context(), userID)
	if errors.Is(err, atprotocol.ErrOAuthSessionNotFound) {
		writeData(w, http.StatusOK, map[string]any{"linked": false})
		return
	}
	if err != nil {
		WriteError(w, r.Context(), http.StatusInternalServerError, ErrCodeInternal, "Could not load AT Protocol status")
		return
	}
	writeData(w, http.StatusOK, publicATProtoLink(link))
}

// Upgrade requests the exact canonical repository scopes.
func (h *ATProtoOAuthHandlers) Upgrade(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		methodNotAllowed(w, http.MethodPost)
		return
	}
	userID := middleware.GetUserID(r.Context())
	if userID == "" {
		WriteError(w, r.Context(), http.StatusUnauthorized, ErrCodeUnauthorized, "Authentication required")
		return
	}
	var request struct {
		ReturnPath string `json:"return_path"`
	}
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil && !errors.Is(err, http.ErrBodyReadAfterClose) {
		WriteError(w, r.Context(), http.StatusBadRequest, ErrCodeBadRequest, "Invalid request body")
		return
	}
	if request.ReturnPath == "" {
		request.ReturnPath = "/studio"
	}
	redirect, err := h.service.Upgrade(r.Context(), userID, request.ReturnPath)
	if err != nil {
		WriteError(w, r.Context(), http.StatusBadRequest, "atproto_oauth_upgrade_failed", err.Error())
		return
	}
	writeData(w, http.StatusAccepted, map[string]string{"redirect_url": redirect})
}

// Unlink revokes the OAuth session while preserving local drafts.
func (h *ATProtoOAuthHandlers) Unlink(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete {
		methodNotAllowed(w, http.MethodDelete)
		return
	}
	userID := middleware.GetUserID(r.Context())
	if userID == "" {
		WriteError(w, r.Context(), http.StatusUnauthorized, ErrCodeUnauthorized, "Authentication required")
		return
	}
	if err := h.service.Unlink(r.Context(), userID); err != nil {
		WriteError(w, r.Context(), http.StatusBadRequest, "atproto_unlink_failed", "Could not unlink AT Protocol account")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func publicATProtoLink(link atprotocol.Link) map[string]any {
	return map[string]any{
		"linked": true, "did": link.AccountDID, "handle": link.Handle,
		"host_url": link.HostURL, "status": link.Status,
		"granted_scopes": link.GrantedScopes, "linked_at": link.LinkedAt,
	}
}

func querySeparator(path string) string {
	if parsed, err := url.Parse(path); err == nil && parsed.RawQuery != "" {
		return "&"
	}
	return "?"
}

func clientAddress(r *http.Request) string {
	if value := strings.TrimSpace(r.Header.Get("CF-Connecting-IP")); value != "" {
		return value
	}
	if value := strings.TrimSpace(r.Header.Get("X-Forwarded-For")); value != "" {
		return value
	}
	return r.RemoteAddr
}

func writeRawJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}
