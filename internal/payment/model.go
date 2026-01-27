// Package payment provides models and services for payment processing.
package payment

import "time"

// PaymentStatus represents the status of a payment record.
const (
	StatusPending   = "pending"
	StatusSucceeded = "succeeded"
	StatusFailed    = "failed"
	StatusCanceled  = "canceled"
)

// PaymentRecord represents a provisional payment record for a Stripe Checkout Session.
type PaymentRecord struct {
	ID        string     `json:"id"`
	SessionID string     `json:"session_id"`         // Stripe Checkout Session ID
	Status    string     `json:"status"`             // pending, succeeded, failed, canceled
	Amount    int64      `json:"amount"`             // Total amount in cents
	Fee       int64      `json:"fee"`                // Platform fee in cents
	UserDID   string     `json:"user_did"`           // User making the payment
	SceneID   string     `json:"scene_id"`           // Scene receiving payment
	EventID   *string    `json:"event_id,omitempty"` // Optional event ID
	CreatedAt *time.Time `json:"created_at,omitempty"`
	UpdatedAt *time.Time `json:"updated_at,omitempty"`
}
