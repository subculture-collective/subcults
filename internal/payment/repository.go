// Package payment provides repository for payment record persistence.
package payment

import (
	"errors"
	"sync"
	"time"

	"github.com/google/uuid"
)

// ErrPaymentRecordNotFound is returned when a payment record is not found.
var ErrPaymentRecordNotFound = errors.New("payment record not found")

// ErrInvalidStatusTransition is returned when an invalid status transition is attempted.
var ErrInvalidStatusTransition = errors.New("invalid status transition")

// ErrDuplicateSessionID is returned when attempting to create a payment with a duplicate session ID.
var ErrDuplicateSessionID = errors.New("duplicate session ID")

// PaymentRepository defines methods for payment record persistence.
type PaymentRepository interface {
	Insert(record *PaymentRecord) error
	GetByID(id string) (*PaymentRecord, error)
	GetBySessionID(sessionID string) (*PaymentRecord, error)
	Update(record *PaymentRecord) error
	
	// CreatePending creates a new payment record in pending status.
	// Returns ErrDuplicateSessionID if a record with the same session_id already exists.
	CreatePending(record *PaymentRecord) error
	
	// MarkCompleted transitions a payment from pending to succeeded status.
	// Returns ErrPaymentRecordNotFound if the session doesn't exist.
	// Returns ErrInvalidStatusTransition if the payment is not in pending status.
	// Idempotent: returns nil if already in succeeded status.
	MarkCompleted(sessionID, paymentIntentID string) error
	
	// MarkFailed transitions a payment from pending to failed status.
	// Returns ErrPaymentRecordNotFound if the session doesn't exist.
	// Returns ErrInvalidStatusTransition if the payment is not in pending status.
	// Idempotent: returns nil if already in failed status with the same reason.
	MarkFailed(sessionID, reason string) error
}

// InMemoryPaymentRepository implements PaymentRepository with in-memory storage.
type InMemoryPaymentRepository struct {
	mu      sync.RWMutex
	records map[string]*PaymentRecord
	sessions map[string]string // Maps session_id -> record ID for uniqueness
}

// NewInMemoryPaymentRepository creates a new in-memory payment repository.
func NewInMemoryPaymentRepository() *InMemoryPaymentRepository {
	return &InMemoryPaymentRepository{
		records:  make(map[string]*PaymentRecord),
		sessions: make(map[string]string),
	}
}

// isValidStatusTransition validates if a status transition is allowed.
// Valid transitions:
//   pending -> succeeded, failed, canceled
//   succeeded -> refunded
//   All other transitions are invalid (e.g., succeeded -> failed, failed -> succeeded)
func isValidStatusTransition(from, to string) bool {
	switch from {
	case StatusPending:
		return to == StatusSucceeded || to == StatusFailed || to == StatusCanceled
	case StatusSucceeded:
		return to == StatusRefunded
	default:
		return false
	}
}

// Insert adds a new payment record.
func (r *InMemoryPaymentRepository) Insert(record *PaymentRecord) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	// Check for duplicate session ID
	if _, exists := r.sessions[record.SessionID]; exists {
		return ErrDuplicateSessionID
	}

	if record.ID == "" {
		record.ID = uuid.New().String()
	}

	// Set timestamps for new record
	now := time.Now()
	if record.CreatedAt == nil {
		record.CreatedAt = &now
	}
	if record.UpdatedAt == nil {
		record.UpdatedAt = &now
	}

	// Deep copy to prevent external mutation
	copied := *record
	r.records[record.ID] = &copied
	r.sessions[record.SessionID] = record.ID

	return nil
}

// GetByID retrieves a payment record by ID.
func (r *InMemoryPaymentRepository) GetByID(id string) (*PaymentRecord, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	record, ok := r.records[id]
	if !ok {
		return nil, ErrPaymentRecordNotFound
	}

	// Deep copy to prevent external mutation
	copied := *record
	return &copied, nil
}

// GetBySessionID retrieves a payment record by session ID.
func (r *InMemoryPaymentRepository) GetBySessionID(sessionID string) (*PaymentRecord, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	for _, record := range r.records {
		if record.SessionID == sessionID {
			// Deep copy to prevent external mutation
			copied := *record
			return &copied, nil
		}
	}

	return nil, ErrPaymentRecordNotFound
}

// Update updates an existing payment record.
func (r *InMemoryPaymentRepository) Update(record *PaymentRecord) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, ok := r.records[record.ID]; !ok {
		return ErrPaymentRecordNotFound
	}

	// Update the UpdatedAt timestamp
	now := time.Now()
	record.UpdatedAt = &now

	// Deep copy to prevent external mutation
	copied := *record
	r.records[record.ID] = &copied

	return nil
}

// CreatePending creates a new payment record in pending status.
// Returns ErrDuplicateSessionID if a record with the same session_id already exists.
func (r *InMemoryPaymentRepository) CreatePending(record *PaymentRecord) error {
	// Force status to pending for safety
	record.Status = StatusPending
	return r.Insert(record)
}

// MarkCompleted transitions a payment from pending to succeeded status.
// Returns ErrPaymentRecordNotFound if the session doesn't exist.
// Returns ErrInvalidStatusTransition if the payment is not in pending status.
// Idempotent: returns nil if already in succeeded status.
func (r *InMemoryPaymentRepository) MarkCompleted(sessionID, paymentIntentID string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	recordID, ok := r.sessions[sessionID]
	if !ok {
		return ErrPaymentRecordNotFound
	}

	record := r.records[recordID]
	
	// Idempotent: if already succeeded, return success
	if record.Status == StatusSucceeded {
		return nil
	}

	// Validate status transition
	if !isValidStatusTransition(record.Status, StatusSucceeded) {
		return ErrInvalidStatusTransition
	}

	// Update status and payment intent ID
	record.Status = StatusSucceeded
	record.PaymentIntentID = &paymentIntentID
	now := time.Now()
	record.UpdatedAt = &now

	return nil
}

// MarkFailed transitions a payment from pending to failed status.
// Returns ErrPaymentRecordNotFound if the session doesn't exist.
// Returns ErrInvalidStatusTransition if the payment is not in pending status.
// Idempotent: returns nil if already in failed status with the same reason.
func (r *InMemoryPaymentRepository) MarkFailed(sessionID, reason string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	recordID, ok := r.sessions[sessionID]
	if !ok {
		return ErrPaymentRecordNotFound
	}

	record := r.records[recordID]
	
	// Idempotent: if already failed with same reason, return success
	if record.Status == StatusFailed {
		if record.FailureReason != nil && *record.FailureReason == reason {
			return nil
		}
		// If already failed but with different reason, update the reason
		record.FailureReason = &reason
		now := time.Now()
		record.UpdatedAt = &now
		return nil
	}

	// Validate status transition
	if !isValidStatusTransition(record.Status, StatusFailed) {
		return ErrInvalidStatusTransition
	}

	// Update status and failure reason
	record.Status = StatusFailed
	record.FailureReason = &reason
	now := time.Now()
	record.UpdatedAt = &now

	return nil
}
