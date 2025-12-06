// Package stream provides models and repository for managing stream sessions
// with AT Protocol record tracking for idempotent ingestion.
package stream

import (
	"errors"
	"sync"
	"time"

	"github.com/google/uuid"
)

// Common errors for stream session operations.
var (
	ErrStreamNotFound = errors.New("stream session not found")
)

// Session represents a LiveKit audio room streaming session.
type Session struct {
	ID               string    `json:"id"`
	SceneID          *string   `json:"scene_id,omitempty"`
	EventID          *string   `json:"event_id,omitempty"`
	RoomName         string    `json:"room_name"`
	HostDID          string    `json:"host_did"`
	ParticipantCount int       `json:"participant_count"`
	
	// AT Protocol record tracking
	RecordDID  *string `json:"record_did,omitempty"`
	RecordRKey *string `json:"record_rkey,omitempty"`
	
	StartedAt time.Time  `json:"started_at"`
	EndedAt   *time.Time `json:"ended_at,omitempty"`
}

// UpsertResult tracks statistics for upsert operations.
type UpsertResult struct {
	Inserted bool   // True if new record was inserted
	ID       string // The UUID of the upserted record
}

// SessionRepository defines the interface for stream session data operations.
type SessionRepository interface {
	// Upsert inserts a new session or updates existing one based on (record_did, record_rkey).
	// Returns UpsertResult indicating whether insert or update occurred.
	Upsert(session *Session) (*UpsertResult, error)

	// GetByID retrieves a session by its UUID.
	GetByID(id string) (*Session, error)

	// GetByRecordKey retrieves a session by its AT Protocol record key.
	GetByRecordKey(did, rkey string) (*Session, error)
}

// InMemorySessionRepository is an in-memory implementation of SessionRepository.
// Thread-safe via RWMutex.
type InMemorySessionRepository struct {
	mu       sync.RWMutex
	sessions map[string]*Session // UUID -> Session
	keys     map[string]string   // "did:rkey" -> UUID
}

// NewInMemorySessionRepository creates a new in-memory session repository.
func NewInMemorySessionRepository() *InMemorySessionRepository {
	return &InMemorySessionRepository{
		sessions: make(map[string]*Session),
		keys:     make(map[string]string),
	}
}

// makeKey creates a composite key from DID and rkey using a null byte separator to avoid collisions.
// AT Protocol DIDs contain colons (e.g., "did:plc:abc123"), so using a null byte prevents
// collisions like did="a:b" + rkey="c" vs did="a" + rkey="b:c" both producing "a:b:c".
func makeKey(did, rkey string) string {
	return did + "\x00" + rkey
}

// Upsert inserts a new session or updates existing one based on (record_did, record_rkey).
func (r *InMemorySessionRepository) Upsert(session *Session) (*UpsertResult, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	now := time.Now()
	var inserted bool
	var id string

	// Check if session exists by record key
	if session.RecordDID != nil && session.RecordRKey != nil {
		key := makeKey(*session.RecordDID, *session.RecordRKey)
		existingID, exists := r.keys[key]
		
		if exists {
			// Update existing session
			existing := r.sessions[existingID]
			existing.SceneID = session.SceneID
			existing.EventID = session.EventID
			existing.RoomName = session.RoomName
			existing.HostDID = session.HostDID
			existing.ParticipantCount = session.ParticipantCount
			existing.EndedAt = session.EndedAt
			inserted = false
			id = existingID
		} else {
			// Insert new session
			if session.ID == "" {
				session.ID = uuid.New().String()
			}
			if session.StartedAt.IsZero() {
				session.StartedAt = now
			}
			
			sessionCopy := *session
			r.sessions[session.ID] = &sessionCopy
			r.keys[key] = session.ID
			inserted = true
			id = session.ID
		}
	} else {
		// No record key, always insert new with new UUID
		newID := uuid.New().String()
		session.ID = newID
		if session.StartedAt.IsZero() {
			session.StartedAt = now
		}
		
		sessionCopy := *session
		r.sessions[newID] = &sessionCopy
		inserted = true
		id = newID
	}

	return &UpsertResult{
		Inserted: inserted,
		ID:       id,
	}, nil
}

// GetByID retrieves a session by its UUID.
func (r *InMemorySessionRepository) GetByID(id string) (*Session, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	session, ok := r.sessions[id]
	if !ok {
		return nil, ErrStreamNotFound
	}

	sessionCopy := *session
	return &sessionCopy, nil
}

// GetByRecordKey retrieves a session by its AT Protocol record key.
func (r *InMemorySessionRepository) GetByRecordKey(did, rkey string) (*Session, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	key := makeKey(did, rkey)
	id, ok := r.keys[key]
	if !ok {
		return nil, ErrStreamNotFound
	}

	session := r.sessions[id]
	sessionCopy := *session
	return &sessionCopy, nil
}
