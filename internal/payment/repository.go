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

// PaymentRepository defines methods for payment record persistence.
type PaymentRepository interface {
	Insert(record *PaymentRecord) error
	GetByID(id string) (*PaymentRecord, error)
	GetBySessionID(sessionID string) (*PaymentRecord, error)
	Update(record *PaymentRecord) error
}

// InMemoryPaymentRepository implements PaymentRepository with in-memory storage.
type InMemoryPaymentRepository struct {
	mu      sync.RWMutex
	records map[string]*PaymentRecord
}

// NewInMemoryPaymentRepository creates a new in-memory payment repository.
func NewInMemoryPaymentRepository() *InMemoryPaymentRepository {
	return &InMemoryPaymentRepository{
		records: make(map[string]*PaymentRecord),
	}
}

// Insert adds a new payment record.
func (r *InMemoryPaymentRepository) Insert(record *PaymentRecord) error {
	r.mu.Lock()
	defer r.mu.Unlock()

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
