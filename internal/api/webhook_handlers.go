package api

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"

	"github.com/onnwee/subcults/internal/middleware"
	"github.com/onnwee/subcults/internal/payment"
	"github.com/onnwee/subcults/internal/scene"
	"github.com/stripe/stripe-go/v81"
	"github.com/stripe/stripe-go/v81/webhook"
)

// WebhookHandlers holds dependencies for webhook-related HTTP handlers.
type WebhookHandlers struct {
	webhookSecret   string
	paymentRepo     payment.PaymentRepository
	webhookRepo     payment.WebhookRepository
	sceneRepo       scene.SceneRepository
}

// NewWebhookHandlers creates a new WebhookHandlers instance.
func NewWebhookHandlers(
	webhookSecret string,
	paymentRepo payment.PaymentRepository,
	webhookRepo payment.WebhookRepository,
	sceneRepo scene.SceneRepository,
) *WebhookHandlers {
	return &WebhookHandlers{
		webhookSecret: webhookSecret,
		paymentRepo:   paymentRepo,
		webhookRepo:   webhookRepo,
		sceneRepo:     sceneRepo,
	}
}

// HandleStripeWebhook processes Stripe webhook events with signature verification.
// POST /internal/stripe
func (h *WebhookHandlers) HandleStripeWebhook(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Read the request body
	body, err := io.ReadAll(r.Body)
	if err != nil {
		ctx = middleware.SetErrorCode(ctx, ErrCodeBadRequest)
		WriteError(w, ctx, http.StatusBadRequest, ErrCodeBadRequest, "failed to read request body")
		return
	}

	// Get the Stripe signature from the header
	signature := r.Header.Get("Stripe-Signature")
	if signature == "" {
		ctx = middleware.SetErrorCode(ctx, ErrCodeBadRequest)
		WriteError(w, ctx, http.StatusBadRequest, ErrCodeBadRequest, "missing Stripe-Signature header")
		return
	}

	// Verify the webhook signature
	event, err := webhook.ConstructEvent(body, signature, h.webhookSecret)
	if err != nil {
		slog.WarnContext(ctx, "webhook signature verification failed", "error", err)
		ctx = middleware.SetErrorCode(ctx, ErrCodeBadRequest)
		WriteError(w, ctx, http.StatusBadRequest, ErrCodeBadRequest, "invalid signature")
		return
	}

	// Log minimal event info (type and ID only, not full payload)
	slog.InfoContext(ctx, "webhook event received", "event_type", event.Type, "event_id", event.ID)

	// Check idempotency - has this event already been processed?
	if err := h.webhookRepo.RecordEvent(event.ID, string(event.Type)); err != nil {
		if err == payment.ErrEventAlreadyProcessed {
			slog.InfoContext(ctx, "webhook event already processed, ignoring", "event_id", event.ID)
			// Return 200 to acknowledge receipt even though we're ignoring it
			w.WriteHeader(http.StatusOK)
			return
		}
		// Other errors recording the event
		slog.ErrorContext(ctx, "failed to record webhook event", "event_id", event.ID, "error", err)
		ctx = middleware.SetErrorCode(ctx, ErrCodeInternal)
		WriteError(w, ctx, http.StatusInternalServerError, ErrCodeInternal, "failed to process webhook")
		return
	}

	// Route to appropriate handler based on event type
	switch event.Type {
	case "checkout.session.completed":
		h.handleCheckoutSessionCompleted(ctx, event)
	case "payment_intent.succeeded":
		h.handlePaymentIntentSucceeded(ctx, event)
	case "payment_intent.payment_failed":
		h.handlePaymentIntentFailed(ctx, event)
	case "account.updated":
		h.handleAccountUpdated(ctx, event)
	default:
		// Unknown event type - log and ignore
		slog.InfoContext(ctx, "ignoring unhandled webhook event type", "event_type", event.Type, "event_id", event.ID)
	}

	// Always return 200 to acknowledge receipt
	w.WriteHeader(http.StatusOK)
}

// handleCheckoutSessionCompleted processes checkout.session.completed events.
func (h *WebhookHandlers) handleCheckoutSessionCompleted(ctx context.Context, event stripe.Event) {
	var session stripe.CheckoutSession
	if err := json.Unmarshal(event.Data.Raw, &session); err != nil {
		slog.ErrorContext(ctx, "failed to parse checkout session", "event_id", event.ID, "error", err)
		return
	}

	// Get the payment record by session ID
	record, err := h.paymentRepo.GetBySessionID(session.ID)
	if err != nil {
		slog.ErrorContext(ctx, "failed to get payment record", "session_id", session.ID, "error", err)
		return
	}

	// Update payment intent ID if available
	// For payment mode, the payment intent is created immediately
	if session.PaymentIntent != nil && session.PaymentIntent.ID != "" {
		// Store the payment intent ID - we'll wait for payment_intent.succeeded for final status
		// Update the record in memory
		record.PaymentIntentID = &session.PaymentIntent.ID
		
		// Note: We're not marking as completed here - we wait for payment_intent.succeeded
		// This is provisional status tracking as per requirements
		slog.InfoContext(ctx, "checkout session completed, payment intent recorded",
			"session_id", session.ID,
			"payment_intent_id", session.PaymentIntent.ID)
	}

	// If the mode requires immediate finalization (e.g., for certain payment methods),
	// we could mark as completed here, but typically we wait for the payment_intent.succeeded event
}

// handlePaymentIntentSucceeded processes payment_intent.succeeded events.
func (h *WebhookHandlers) handlePaymentIntentSucceeded(ctx context.Context, event stripe.Event) {
	var paymentIntent stripe.PaymentIntent
	if err := json.Unmarshal(event.Data.Raw, &paymentIntent); err != nil {
		slog.ErrorContext(ctx, "failed to parse payment intent", "event_id", event.ID, "error", err)
		return
	}

	// Get the checkout session ID from metadata or charges
	// Stripe attaches the checkout session to the payment intent
	sessionID := ""
	if paymentIntent.Metadata != nil {
		sessionID = paymentIntent.Metadata["session_id"]
	}
	
	// If not in metadata, we cannot process this event without a database query
	// In a real implementation with a database, we could query:
	// SELECT * FROM payment_records WHERE payment_intent_id = paymentIntent.ID
	// For now, we'll skip this event if we can't find the session
	if sessionID == "" {
		slog.WarnContext(ctx, "payment intent succeeded but session ID not found",
			"payment_intent_id", paymentIntent.ID,
			"event_id", event.ID)
		return
	}

	// Mark payment as completed
	if err := h.paymentRepo.MarkCompleted(sessionID, paymentIntent.ID); err != nil {
		if err == payment.ErrPaymentRecordNotFound {
			slog.WarnContext(ctx, "payment record not found for payment intent",
				"session_id", sessionID,
				"payment_intent_id", paymentIntent.ID)
			return
		}
		slog.ErrorContext(ctx, "failed to mark payment completed",
			"session_id", sessionID,
			"payment_intent_id", paymentIntent.ID,
			"error", err)
		return
	}

	slog.InfoContext(ctx, "payment marked as completed",
		"session_id", sessionID,
		"payment_intent_id", paymentIntent.ID,
		"amount", paymentIntent.Amount,
		"currency", paymentIntent.Currency)
}

// handlePaymentIntentFailed processes payment_intent.payment_failed events.
func (h *WebhookHandlers) handlePaymentIntentFailed(ctx context.Context, event stripe.Event) {
	var paymentIntent stripe.PaymentIntent
	if err := json.Unmarshal(event.Data.Raw, &paymentIntent); err != nil {
		slog.ErrorContext(ctx, "failed to parse payment intent", "event_id", event.ID, "error", err)
		return
	}

	// Get the session ID (same logic as succeeded)
	sessionID := ""
	if paymentIntent.Metadata != nil {
		sessionID = paymentIntent.Metadata["session_id"]
	}

	if sessionID == "" {
		slog.WarnContext(ctx, "payment intent failed but session ID not found",
			"payment_intent_id", paymentIntent.ID,
			"event_id", event.ID)
		return
	}

	// Extract failure reason
	failureReason := "unknown"
	if paymentIntent.LastPaymentError != nil {
		if paymentIntent.LastPaymentError.Code != "" {
			failureReason = string(paymentIntent.LastPaymentError.Code)
		} else if paymentIntent.LastPaymentError.Msg != "" {
			failureReason = paymentIntent.LastPaymentError.Msg
		}
	}

	// Mark payment as failed
	if err := h.paymentRepo.MarkFailed(sessionID, failureReason); err != nil {
		if err == payment.ErrPaymentRecordNotFound {
			slog.WarnContext(ctx, "payment record not found for failed payment intent",
				"session_id", sessionID,
				"payment_intent_id", paymentIntent.ID)
			return
		}
		slog.ErrorContext(ctx, "failed to mark payment as failed",
			"session_id", sessionID,
			"payment_intent_id", paymentIntent.ID,
			"error", err)
		return
	}

	slog.InfoContext(ctx, "payment marked as failed",
		"session_id", sessionID,
		"payment_intent_id", paymentIntent.ID,
		"reason", failureReason)
}

// handleAccountUpdated processes account.updated events for Connect onboarding completion.
func (h *WebhookHandlers) handleAccountUpdated(ctx context.Context, event stripe.Event) {
	var account stripe.Account
	if err := json.Unmarshal(event.Data.Raw, &account); err != nil {
		slog.ErrorContext(ctx, "failed to parse account", "event_id", event.ID, "error", err)
		return
	}

	// Check if capabilities are now active
	// For Express accounts, we check if transfers capability is active
	transfersActive := account.Capabilities != nil && 
		account.Capabilities.Transfers == stripe.AccountCapabilityStatusActive

	if !transfersActive {
		// Capabilities not yet active, no action needed
		slog.InfoContext(ctx, "account capabilities not yet active",
			"account_id", account.ID,
			"transfers_active", transfersActive)
		return
	}

	// Capabilities are active - log for now
	// In a full implementation with a connected_account_status field, we would update it here
	// For now, the presence of ConnectedAccountID in the scene indicates onboarding started,
	// and this event confirms capabilities are active
	slog.InfoContext(ctx, "account capabilities activated",
		"account_id", account.ID,
		"details_submitted", account.DetailsSubmitted,
		"charges_enabled", account.ChargesEnabled)

	// Note: We don't have a connected_account_status field in the Scene model yet,
	// so we're just logging this for observability. When that field is added,
	// we would query for scenes with this connected_account_id and update their status.
}
