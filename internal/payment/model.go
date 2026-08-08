// Package payment provides models and services for payment processing.
package payment

import "time"

// PaymentStatus represents the status of a payment record.
const (
	StatusPending   = "pending"
	StatusSucceeded = "succeeded"
	StatusFailed    = "failed"
	StatusCanceled  = "canceled"
	StatusRefunded  = "refunded"
)

// PaymentRecord represents a provisional payment record for a Stripe Checkout Session.
type PaymentRecord struct {
	ID                 string        `json:"id"`
	SessionID          string        `json:"session_id"`                     // Stripe Checkout Session ID
	Status             string        `json:"status"`                         // pending, succeeded, failed, canceled, refunded
	Amount             int64         `json:"amount"`                         // Total amount in cents
	Fee                int64         `json:"fee"`                            // Platform fee in cents
	Currency           string        `json:"currency,omitempty"`             // ISO 4217 currency code (defaults to 'usd' if empty)
	UserDID            string        `json:"user_did"`                       // User making the payment
	SceneID            string        `json:"scene_id"`                       // Scene receiving payment
	EventID            *string       `json:"event_id,omitempty"`             // Optional event ID
	AppearanceID       *string       `json:"appearance_id,omitempty"`        // Optional billed appearance
	TourID             *string       `json:"tour_id,omitempty"`              // Optional tour context
	SignalID           *string       `json:"signal_id,omitempty"`            // Optional modeled acquisition signal
	DeliveryID         *string       `json:"delivery_id,omitempty"`          // Optional message delivery touchpoint
	ConnectedAccountID *string       `json:"connected_account_id,omitempty"` // Stripe Connect account ID
	PaymentIntentID    *string       `json:"payment_intent_id,omitempty"`    // Stripe Payment Intent ID
	FailureReason      *string       `json:"failure_reason,omitempty"`       // Reason for failure
	AttributionModel   string        `json:"attribution_model,omitempty"`    // Modeled relationship, never causal truth
	AttributionWindow  time.Duration `json:"attribution_window,omitempty"`
	ProviderEventID    string        `json:"provider_event_id,omitempty"`
	RawPayloadSHA256   string        `json:"raw_payload_sha256,omitempty"`
	ReceivedAt         *time.Time    `json:"received_at,omitempty"`
	CreatedAt          *time.Time    `json:"created_at,omitempty"`
	UpdatedAt          *time.Time    `json:"updated_at,omitempty"`
}

// DeepCopy creates a deep copy of the PaymentRecord, including all pointer fields.
func (p *PaymentRecord) DeepCopy() *PaymentRecord {
	if p == nil {
		return nil
	}

	copied := &PaymentRecord{
		ID:                p.ID,
		SessionID:         p.SessionID,
		Status:            p.Status,
		Amount:            p.Amount,
		Fee:               p.Fee,
		Currency:          p.Currency,
		UserDID:           p.UserDID,
		SceneID:           p.SceneID,
		AttributionModel:  p.AttributionModel,
		AttributionWindow: p.AttributionWindow,
		ProviderEventID:   p.ProviderEventID,
		RawPayloadSHA256:  p.RawPayloadSHA256,
	}

	if p.EventID != nil {
		eventID := *p.EventID
		copied.EventID = &eventID
	}
	if p.AppearanceID != nil {
		value := *p.AppearanceID
		copied.AppearanceID = &value
	}
	if p.TourID != nil {
		value := *p.TourID
		copied.TourID = &value
	}
	if p.SignalID != nil {
		value := *p.SignalID
		copied.SignalID = &value
	}
	if p.DeliveryID != nil {
		value := *p.DeliveryID
		copied.DeliveryID = &value
	}
	if p.ConnectedAccountID != nil {
		accountID := *p.ConnectedAccountID
		copied.ConnectedAccountID = &accountID
	}
	if p.PaymentIntentID != nil {
		intentID := *p.PaymentIntentID
		copied.PaymentIntentID = &intentID
	}
	if p.FailureReason != nil {
		reason := *p.FailureReason
		copied.FailureReason = &reason
	}
	if p.CreatedAt != nil {
		createdAt := *p.CreatedAt
		copied.CreatedAt = &createdAt
	}
	if p.UpdatedAt != nil {
		updatedAt := *p.UpdatedAt
		copied.UpdatedAt = &updatedAt
	}
	if p.ReceivedAt != nil {
		receivedAt := *p.ReceivedAt
		copied.ReceivedAt = &receivedAt
	}

	return copied
}
