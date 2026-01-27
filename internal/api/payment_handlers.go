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
	sceneRepo             scene.SceneRepository
	paymentRepo           payment.PaymentRepository
	stripeClient          payment.Client
	returnURL             string
	refreshURL            string
	applicationFeePercent float64
}

// NewPaymentHandlers creates a new PaymentHandlers instance.
func NewPaymentHandlers(
	sceneRepo scene.SceneRepository,
	paymentRepo payment.PaymentRepository,
	stripeClient payment.Client,
	returnURL string,
	refreshURL string,
	applicationFeePercent float64,
) *PaymentHandlers {
	return &PaymentHandlers{
		sceneRepo:           sceneRepo,
		paymentRepo:         paymentRepo,
		stripeClient:        stripeClient,
		returnURL:           returnURL,
		refreshURL:          refreshURL,
		applicationFeePercent: applicationFeePercent,
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
		slog.ErrorContext(ctx, "failed to get scene", "scene_id", req.SceneID, "error", err)
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
		slog.ErrorContext(ctx, "failed to create Stripe Connect account", "scene_id", req.SceneID, "error", err)
		ctx = middleware.SetErrorCode(ctx, ErrCodeInternal)
		WriteError(w, ctx, http.StatusInternalServerError, ErrCodeInternal, "failed to create payment account")
		return
	}

	// Create onboarding link
	link, err := h.stripeClient.CreateAccountLink(account.ID, h.returnURL, h.refreshURL)
	if err != nil {
		slog.ErrorContext(ctx, "failed to create account link", "account_id", account.ID, "error", err)
		ctx = middleware.SetErrorCode(ctx, ErrCodeInternal)
		WriteError(w, ctx, http.StatusInternalServerError, ErrCodeInternal, "failed to create onboarding link")
		return
	}

	// Update scene with connected account ID
	// Note: Per requirements, this is persisted here. Full onboarding completion
	// will be tracked via webhook (separate task).
	existingScene.ConnectedAccountID = &account.ID
	if err := h.sceneRepo.Update(existingScene); err != nil {
		slog.ErrorContext(ctx, "failed to update scene with connected account", "scene_id", req.SceneID, "error", err)
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

// CheckoutSessionRequest represents the request body for creating a Stripe Checkout Session.
type CheckoutSessionRequest struct {
	SceneID    string                 `json:"scene_id"`
	EventID    *string                `json:"event_id,omitempty"`
	Items      []CheckoutItemRequest  `json:"items"`
	SuccessURL string                 `json:"success_url"`
	CancelURL  string                 `json:"cancel_url"`
}

// CheckoutItemRequest represents a line item in the checkout.
type CheckoutItemRequest struct {
	PriceID  string `json:"price_id"`
	Quantity int64  `json:"quantity"`
}

// CheckoutSessionResponse represents the response for a successful checkout session creation.
type CheckoutSessionResponse struct {
	SessionURL string `json:"session_url"`
	SessionID  string `json:"session_id"`
}

// CreateCheckoutSession creates a Stripe Checkout Session for event ticket or merch with application fee.
// POST /payments/checkout
func (h *PaymentHandlers) CreateCheckoutSession(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Get authenticated user DID from context
	userDID := middleware.GetUserDID(ctx)
	if userDID == "" {
		ctx = middleware.SetErrorCode(ctx, ErrCodeUnauthorized)
		WriteError(w, ctx, http.StatusUnauthorized, ErrCodeUnauthorized, "authentication required")
		return
	}

	// Parse request body
	var req CheckoutSessionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		ctx = middleware.SetErrorCode(ctx, ErrCodeBadRequest)
		WriteError(w, ctx, http.StatusBadRequest, ErrCodeBadRequest, "invalid request body")
		return
	}

	// Validate required fields
	if req.SceneID == "" {
		ctx = middleware.SetErrorCode(ctx, ErrCodeBadRequest)
		WriteError(w, ctx, http.StatusBadRequest, ErrCodeBadRequest, "scene_id is required")
		return
	}
	if len(req.Items) == 0 {
		ctx = middleware.SetErrorCode(ctx, ErrCodeBadRequest)
		WriteError(w, ctx, http.StatusBadRequest, ErrCodeBadRequest, "items list cannot be empty")
		return
	}
	if req.SuccessURL == "" {
		ctx = middleware.SetErrorCode(ctx, ErrCodeBadRequest)
		WriteError(w, ctx, http.StatusBadRequest, ErrCodeBadRequest, "success_url is required")
		return
	}
	if req.CancelURL == "" {
		ctx = middleware.SetErrorCode(ctx, ErrCodeBadRequest)
		WriteError(w, ctx, http.StatusBadRequest, ErrCodeBadRequest, "cancel_url is required")
		return
	}

	// Validate items
	for i, item := range req.Items {
		if item.PriceID == "" {
			ctx = middleware.SetErrorCode(ctx, ErrCodeBadRequest)
			WriteError(w, ctx, http.StatusBadRequest, ErrCodeBadRequest, "price_id is required for all items")
			return
		}
		if item.Quantity <= 0 {
			ctx = middleware.SetErrorCode(ctx, ErrCodeBadRequest)
			WriteError(w, ctx, http.StatusBadRequest, ErrCodeBadRequest, "quantity must be positive")
			return
		}
		// Enforce reasonable quantity limit to prevent abuse
		if item.Quantity > 100 {
			ctx = middleware.SetErrorCode(ctx, ErrCodeBadRequest)
			WriteError(w, ctx, http.StatusBadRequest, ErrCodeBadRequest, "quantity cannot exceed 100 per item")
			return
		}
		slog.InfoContext(ctx, "checkout item", "index", i, "price_id", item.PriceID, "quantity", item.Quantity)
	}

	// Get scene from repository
	existingScene, err := h.sceneRepo.GetByID(req.SceneID)
	if err != nil {
		slog.ErrorContext(ctx, "failed to get scene", "scene_id", req.SceneID, "error", err)
		ctx = middleware.SetErrorCode(ctx, ErrCodeNotFound)
		WriteError(w, ctx, http.StatusNotFound, ErrCodeNotFound, "scene not found")
		return
	}

	// Validate scene has connected account
	if existingScene.ConnectedAccountID == nil || *existingScene.ConnectedAccountID == "" {
		ctx = middleware.SetErrorCode(ctx, "not_onboarded")
		WriteError(w, ctx, http.StatusBadRequest, "not_onboarded", "scene must be onboarded for payments before creating checkout session")
		return
	}

	// Convert items to payment client format
	items := make([]payment.CheckoutItem, len(req.Items))
	for i, item := range req.Items {
		items[i] = payment.CheckoutItem{
			PriceID:  item.PriceID,
			Quantity: item.Quantity,
		}
	}

	// Note: In a real implementation, we would fetch the total amount from Stripe Price API
	// For now, we'll compute the fee based on a placeholder amount of $100 (10000 cents)
	// This will be properly calculated when Stripe processes the actual prices
	placeholderAmount := int64(10000) // $100 in cents as placeholder
	applicationFee := int64(float64(placeholderAmount) * h.applicationFeePercent / 100.0)

	// Create Stripe Checkout Session
	sessionParams := &payment.CheckoutSessionParams{
		ConnectedAccountID: *existingScene.ConnectedAccountID,
		Items:              items,
		SuccessURL:         req.SuccessURL,
		CancelURL:          req.CancelURL,
		ApplicationFee:     applicationFee,
		UserDID:            userDID,
	}

	session, err := h.stripeClient.CreateCheckoutSession(sessionParams)
	if err != nil {
		slog.ErrorContext(ctx, "failed to create checkout session", "scene_id", req.SceneID, "error", err)
		ctx = middleware.SetErrorCode(ctx, ErrCodeInternal)
		WriteError(w, ctx, http.StatusInternalServerError, ErrCodeInternal, "failed to create checkout session")
		return
	}

	// Create provisional payment record
	paymentRecord := &payment.PaymentRecord{
		SessionID: session.ID,
		Amount:    placeholderAmount,
		Fee:       applicationFee,
		Currency:  "usd", // Default currency
		UserDID:   userDID,
		SceneID:   req.SceneID,
		EventID:   req.EventID,
	}

	if err := h.paymentRepo.CreatePending(paymentRecord); err != nil {
		slog.ErrorContext(ctx, "failed to insert payment record", "session_id", session.ID, "error", err)
		// Not a critical failure; continue and return session URL
	}

	// Return session URL
	response := CheckoutSessionResponse{
		SessionURL: session.URL,
		SessionID:  session.ID,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(response); err != nil {
		slog.Error("failed to encode response", "error", err)
	}
}
