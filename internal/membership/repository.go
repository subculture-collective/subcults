// Package membership provides models and repository for managing scene memberships
// with AT Protocol record tracking for idempotent ingestion.
package membership

import (
	"errors"
	"sync"
	"time"

	"github.com/google/uuid"
)

// Common errors for membership operations.
var (
	ErrMembershipNotFound = errors.New("membership not found")
)

// Membership represents a user's participation in a scene.
type Membership struct {
	ID        string  `json:"id"`
	SceneID   string  `json:"scene_id"`
	UserDID   string  `json:"user_did"`
	Role      string  `json:"role"`
	Status    string  `json:"status"`
	TrustWeight float64 `json:"trust_weight"`
	
	// AT Protocol record tracking
	RecordDID  *string `json:"record_did,omitempty"`
	RecordRKey *string `json:"record_rkey,omitempty"`
	
	Since     time.Time `json:"since"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// UpsertResult tracks statistics for upsert operations.
type UpsertResult struct {
	Inserted bool   // True if new record was inserted
	ID       string // The UUID of the upserted record
}

// MembershipRepository defines the interface for membership data operations.
type MembershipRepository interface {
	// Upsert inserts a new membership or updates existing one based on (record_did, record_rkey).
	// Returns UpsertResult indicating whether insert or update occurred.
	Upsert(membership *Membership) (*UpsertResult, error)

	// GetByID retrieves a membership by its UUID.
	GetByID(id string) (*Membership, error)

	// GetByRecordKey retrieves a membership by its AT Protocol record key.
	GetByRecordKey(did, rkey string) (*Membership, error)
}

// InMemoryMembershipRepository is an in-memory implementation of MembershipRepository.
// Thread-safe via RWMutex.
type InMemoryMembershipRepository struct {
	mu          sync.RWMutex
	memberships map[string]*Membership // UUID -> Membership
	keys        map[string]string      // "did:rkey" -> UUID
}

// NewInMemoryMembershipRepository creates a new in-memory membership repository.
func NewInMemoryMembershipRepository() *InMemoryMembershipRepository {
	return &InMemoryMembershipRepository{
		memberships: make(map[string]*Membership),
		keys:        make(map[string]string),
	}
}

// makeKey creates a composite key from DID and rkey.
func makeKey(did, rkey string) string {
	return did + ":" + rkey
}

// Upsert inserts a new membership or updates existing one based on (record_did, record_rkey).
func (r *InMemoryMembershipRepository) Upsert(membership *Membership) (*UpsertResult, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	now := time.Now()
	var inserted bool
	var id string

	// Check if membership exists by record key
	if membership.RecordDID != nil && membership.RecordRKey != nil {
		key := makeKey(*membership.RecordDID, *membership.RecordRKey)
		existingID, exists := r.keys[key]
		
		if exists {
			// Update existing membership
			existing := r.memberships[existingID]
			existing.SceneID = membership.SceneID
			existing.UserDID = membership.UserDID
			existing.Role = membership.Role
			existing.Status = membership.Status
			existing.TrustWeight = membership.TrustWeight
			existing.UpdatedAt = now
			inserted = false
			id = existingID
		} else {
			// Insert new membership
			if membership.ID == "" {
				membership.ID = uuid.New().String()
			}
			if membership.Since.IsZero() {
				membership.Since = now
			}
			membership.CreatedAt = now
			membership.UpdatedAt = now
			
			membershipCopy := *membership
			r.memberships[membership.ID] = &membershipCopy
			r.keys[key] = membership.ID
			inserted = true
			id = membership.ID
		}
	} else {
		// No record key, always insert new with new UUID
		newID := uuid.New().String()
		membership.ID = newID
		if membership.Since.IsZero() {
			membership.Since = now
		}
		membership.CreatedAt = now
		membership.UpdatedAt = now
		
		membershipCopy := *membership
		r.memberships[newID] = &membershipCopy
		inserted = true
		id = newID
	}

	return &UpsertResult{
		Inserted: inserted,
		ID:       id,
	}, nil
}

// GetByID retrieves a membership by its UUID.
func (r *InMemoryMembershipRepository) GetByID(id string) (*Membership, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	membership, ok := r.memberships[id]
	if !ok {
		return nil, ErrMembershipNotFound
	}

	membershipCopy := *membership
	return &membershipCopy, nil
}

// GetByRecordKey retrieves a membership by its AT Protocol record key.
func (r *InMemoryMembershipRepository) GetByRecordKey(did, rkey string) (*Membership, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	key := makeKey(did, rkey)
	id, ok := r.keys[key]
	if !ok {
		return nil, ErrMembershipNotFound
	}

	membership := r.memberships[id]
	membershipCopy := *membership
	return &membershipCopy, nil
}
