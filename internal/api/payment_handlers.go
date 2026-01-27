// Package api provides HTTP handlers for the Subcults API.
package api

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"time"

	"github.com/onnwee/subcults/internal/middleware"
	"github.com/onnwee/subcults/internal/payment"
	"github.com/onnwee/subcults/internal/scene"
)

// PaymentHandlers holds dependencies for payment-related HTTP handlers.
type PaymentHandlers struct {
	sceneRepo     scene.SceneRepository
	stripeClient  payment.Client
	returnURL     string
	refreshURL    string
}

// NewPaymentHandlers creates a new PaymentHandlers instance.
func NewPaymentHandlers(
	sceneRepo scene.SceneRepository,
	stripeClient payment.Client,
	returnURL string,
	refreshURL string,
) *PaymentHandlers {
	return &PaymentHandlers{
		sceneRepo:    sceneRepo,
		stripeClient: stripeClient,
		returnURL:    returnURL,
		refreshURL:   refreshURL,
	}
}

// OnboardSceneRequest represents the request body for creating a Stripe onboarding link.
type OnboardSceneRequest struct {
	SceneID string `json:"scene_id"`
}

// OnboardSceneResponse represents the response for a successful onboarding link creation.
type OnboardSceneResponse struct {
	URL       string `json:"url"`
	ExpiresAt string `json:"expires_at"`
}

// OnboardScene creates a Stripe Connect onboarding link for a scene owner.
// POST /payments/onboard
func (h *PaymentHandlers) OnboardScene(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Get authenticated user DID from context
	userDID := middleware.GetUserDID(ctx)
	if userDID == "" {
		ctx = middleware.SetErrorCode(ctx, ErrCodeUnauthorized)
		WriteError(w, ctx, http.StatusUnauthorized, ErrCodeUnauthorized, "authentication required")
		return
	}

	// Parse request body
	var req OnboardSceneRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		ctx = middleware.SetErrorCode(ctx, ErrCodeBadRequest)
		WriteError(w, ctx, http.StatusBadRequest, ErrCodeBadRequest, "invalid request body")
		return
	}

	// Validate scene ID is not empty
	if req.SceneID == "" {
		ctx = middleware.SetErrorCode(ctx, ErrCodeBadRequest)
		WriteError(w, ctx, http.StatusBadRequest, ErrCodeBadRequest, "scene_id is required")
		return
	}

	// Get scene from repository
	existingScene, err := h.sceneRepo.GetByID(req.SceneID)
	if err != nil {
		slog.Error("failed to get scene", "scene_id", req.SceneID, "error", err)
		ctx = middleware.SetErrorCode(ctx, ErrCodeNotFound)
		WriteError(w, ctx, http.StatusNotFound, ErrCodeNotFound, "scene not found")
		return
	}

	// Verify requesting user owns the scene
	if !existingScene.IsOwner(userDID) {
		ctx = middleware.SetErrorCode(ctx, ErrCodeForbidden)
		WriteError(w, ctx, http.StatusForbidden, ErrCodeForbidden, "only scene owner can onboard for payments")
		return
	}

	// Check if scene already has a connected account
	if existingScene.ConnectedAccountID != nil && *existingScene.ConnectedAccountID != "" {
		ctx = middleware.SetErrorCode(ctx, "already_onboarded")
		WriteError(w, ctx, http.StatusBadRequest, "already_onboarded", "scene is already onboarded for payments")
		return
	}

	// Create Stripe Connect account
	account, err := h.stripeClient.CreateConnectAccount()
	if err != nil {
		slog.Error("failed to create Stripe Connect account", "scene_id", req.SceneID, "error", err)
		ctx = middleware.SetErrorCode(ctx, ErrCodeInternal)
		WriteError(w, ctx, http.StatusInternalServerError, ErrCodeInternal, "failed to create payment account")
		return
	}

	// Create onboarding link
	link, err := h.stripeClient.CreateAccountLink(account.ID, h.returnURL, h.refreshURL)
	if err != nil {
		slog.Error("failed to create account link", "account_id", account.ID, "error", err)
		ctx = middleware.SetErrorCode(ctx, ErrCodeInternal)
		WriteError(w, ctx, http.StatusInternalServerError, ErrCodeInternal, "failed to create onboarding link")
		return
	}

	// Update scene with connected account ID
	// Note: Per requirements, this is persisted here. Full onboarding completion
	// will be tracked via webhook (separate task).
	existingScene.ConnectedAccountID = &account.ID
	if err := h.sceneRepo.Update(existingScene); err != nil {
		slog.Error("failed to update scene with connected account", "scene_id", req.SceneID, "error", err)
		ctx = middleware.SetErrorCode(ctx, ErrCodeInternal)
		WriteError(w, ctx, http.StatusInternalServerError, ErrCodeInternal, "failed to save payment account")
		return
	}

	// Return onboarding URL and expiry
	// Stripe account links typically expire in 30 minutes
	expiresAt := time.Now().Add(30 * time.Minute).Format(time.RFC3339)
	
	response := OnboardSceneResponse{
		URL:       link.URL,
		ExpiresAt: expiresAt,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(response); err != nil {
		slog.Error("failed to encode response", "error", err)
	}
}
