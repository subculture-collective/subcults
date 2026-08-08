package api

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/onnwee/subcults/internal/identity"
	"github.com/onnwee/subcults/internal/middleware"
)

const refreshCookieName = "subcult_refresh"

type IdentityAuthHandlers struct {
	service      *identity.Service
	secureCookie bool
}

func (h *IdentityAuthHandlers) CreatorAccess(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())
	if userID == "" {
		WriteError(w, r.Context(), http.StatusUnauthorized, ErrCodeUnauthorized, "Authentication required")
		return
	}
	switch r.Method {
	case http.MethodGet:
		request, err := h.service.GetCreatorAccess(r.Context(), userID)
		if err != nil {
			writeData(w, http.StatusOK, nil)
			return
		}
		writeData(w, http.StatusOK, request)
	case http.MethodPost:
		var body struct {
			Statement string `json:"statement"`
		}
		if err := decodeLimitedJSON(w, r, &body); err != nil {
			WriteError(w, r.Context(), http.StatusBadRequest, ErrCodeBadRequest, "Invalid request body")
			return
		}
		request, err := h.service.RequestCreatorAccess(r.Context(), userID, body.Statement)
		if err != nil {
			WriteError(w, r.Context(), http.StatusBadRequest, "invalid_creator_request", err.Error())
			return
		}
		writeData(w, http.StatusAccepted, request)
	default:
		w.Header().Set("Allow", "GET, POST")
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (h *IdentityAuthHandlers) ReviewCreatorAccess(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPatch {
		methodNotAllowed(w, http.MethodPatch)
		return
	}
	reviewerID := middleware.GetUserID(r.Context())
	if reviewerID == "" {
		WriteError(w, r.Context(), http.StatusUnauthorized, ErrCodeUnauthorized, "Authentication required")
		return
	}
	requestID := strings.Trim(strings.TrimPrefix(r.URL.Path, "/api/v1/admin/creator-access/"), "/")
	if requestID == "" || strings.Contains(requestID, "/") {
		WriteError(w, r.Context(), http.StatusNotFound, ErrCodeNotFound, "Creator request not found")
		return
	}
	var body struct {
		Status string `json:"status"`
		Note   string `json:"note"`
	}
	if err := decodeLimitedJSON(w, r, &body); err != nil {
		WriteError(w, r.Context(), http.StatusBadRequest, ErrCodeBadRequest, "Invalid request body")
		return
	}
	request, err := h.service.ReviewCreatorAccess(r.Context(), requestID, reviewerID, body.Status, body.Note)
	if err != nil {
		WriteError(w, r.Context(), http.StatusForbidden, ErrCodeForbidden, err.Error())
		return
	}
	writeData(w, http.StatusOK, request)
}

func NewIdentityAuthHandlers(service *identity.Service, secureCookie bool) *IdentityAuthHandlers {
	return &IdentityAuthHandlers{service: service, secureCookie: secureCookie}
}

func (h *IdentityAuthHandlers) RequestMagicLink(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		methodNotAllowed(w, http.MethodPost)
		return
	}
	var request struct {
		Email      string `json:"email"`
		ReturnPath string `json:"return_path"`
	}
	if err := decodeLimitedJSON(w, r, &request); err != nil {
		WriteError(w, r.Context(), http.StatusBadRequest, ErrCodeBadRequest, "Invalid request body")
		return
	}
	if err := h.service.RequestMagicLink(r.Context(), request.Email, request.ReturnPath); err != nil {
		if errors.Is(err, identity.ErrInvalidEmail) || errors.Is(err, identity.ErrInvalidReturnPath) {
			WriteError(w, r.Context(), http.StatusBadRequest, "invalid_magic_link_request", err.Error())
			return
		}
		WriteError(w, r.Context(), http.StatusServiceUnavailable, "email_unavailable", "Access email could not be sent")
		return
	}
	writeData(w, http.StatusAccepted, map[string]any{"accepted": true})
}

func (h *IdentityAuthHandlers) VerifyMagicLink(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		methodNotAllowed(w, http.MethodPost)
		return
	}
	var request struct {
		Token string `json:"token"`
	}
	if err := decodeLimitedJSON(w, r, &request); err != nil {
		WriteError(w, r.Context(), http.StatusBadRequest, ErrCodeBadRequest, "Invalid request body")
		return
	}
	result, err := h.service.VerifyMagicLink(r.Context(), request.Token, r.UserAgent())
	if err != nil {
		WriteError(w, r.Context(), http.StatusUnauthorized, "invalid_magic_link", "Access link is invalid or expired")
		return
	}
	h.setRefreshCookie(w, result.RefreshToken)
	writeData(w, http.StatusOK, authPayload(result))
}

func (h *IdentityAuthHandlers) Refresh(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		methodNotAllowed(w, http.MethodPost)
		return
	}
	cookie, err := r.Cookie(refreshCookieName)
	if err != nil {
		WriteError(w, r.Context(), http.StatusUnauthorized, ErrCodeUnauthorized, "Refresh session is missing")
		return
	}
	result, err := h.service.Refresh(r.Context(), cookie.Value, r.UserAgent())
	if err != nil {
		h.clearRefreshCookie(w)
		WriteError(w, r.Context(), http.StatusUnauthorized, ErrCodeUnauthorized, "Refresh session is invalid or expired")
		return
	}
	h.setRefreshCookie(w, result.RefreshToken)
	writeData(w, http.StatusOK, authPayload(result))
}

func (h *IdentityAuthHandlers) Logout(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		methodNotAllowed(w, http.MethodPost)
		return
	}
	if cookie, err := r.Cookie(refreshCookieName); err == nil {
		_ = h.service.Logout(r.Context(), cookie.Value)
	}
	h.clearRefreshCookie(w)
	w.WriteHeader(http.StatusNoContent)
}

func (h *IdentityAuthHandlers) Me(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		methodNotAllowed(w, http.MethodGet)
		return
	}
	userID := middleware.GetUserID(r.Context())
	if userID == "" {
		WriteError(w, r.Context(), http.StatusUnauthorized, ErrCodeUnauthorized, "Authentication required")
		return
	}
	user, err := h.service.GetUser(r.Context(), userID)
	if err != nil {
		WriteError(w, r.Context(), http.StatusUnauthorized, ErrCodeUnauthorized, "Account is unavailable")
		return
	}
	writeData(w, http.StatusOK, publicUser(user))
}

func (h *IdentityAuthHandlers) CompleteProfile(w http.ResponseWriter, r *http.Request) {
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
		Handle      string `json:"handle"`
		DisplayName string `json:"display_name"`
	}
	if err := decodeLimitedJSON(w, r, &request); err != nil {
		WriteError(w, r.Context(), http.StatusBadRequest, ErrCodeBadRequest, "Invalid request body")
		return
	}
	user, err := h.service.CompleteProfile(r.Context(), userID, request.Handle, request.DisplayName)
	if err != nil {
		WriteError(w, r.Context(), http.StatusConflict, "handle_unavailable", "Choose another handle")
		return
	}
	writeData(w, http.StatusOK, publicUser(user))
}

func (h *IdentityAuthHandlers) setRefreshCookie(w http.ResponseWriter, token string) {
	http.SetCookie(w, &http.Cookie{Name: refreshCookieName, Value: token, Path: "/api/v1/auth", MaxAge: int((30 * 24 * time.Hour).Seconds()), HttpOnly: true, Secure: h.secureCookie, SameSite: http.SameSiteLaxMode})
}

func (h *IdentityAuthHandlers) clearRefreshCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{Name: refreshCookieName, Value: "", Path: "/api/v1/auth", MaxAge: -1, HttpOnly: true, Secure: h.secureCookie, SameSite: http.SameSiteLaxMode})
}

func authPayload(result identity.AuthResult) map[string]any {
	return map[string]any{"access_token": result.AccessToken, "user": publicUser(result.User), "return_path": result.ReturnPath}
}

func publicUser(user identity.User) map[string]any {
	return map[string]any{"id": user.ID, "did": user.InternalDID, "handle": user.Handle, "display_name": user.DisplayName, "role": user.Role, "onboarding_complete": user.OnboardingComplete}
}

func decodeLimitedJSON(w http.ResponseWriter, r *http.Request, destination any) error {
	r.Body = http.MaxBytesReader(w, r.Body, 16*1024)
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	return decoder.Decode(destination)
}

func writeData(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]any{"data": data})
}

func methodNotAllowed(w http.ResponseWriter, allowed string) {
	w.Header().Set("Allow", allowed)
	http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
}
