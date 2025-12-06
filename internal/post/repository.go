// Package post provides models and repository for managing posts
// with AT Protocol record tracking for idempotent ingestion.
package post

import (
	"errors"
	"sync"
	"time"

	"github.com/google/uuid"
)

// Common errors for post operations.
var (
	ErrPostNotFound = errors.New("post not found")
)

// Post represents a content post within scenes/events.
type Post struct {
	ID        string    `json:"id"`
	SceneID   *string   `json:"scene_id,omitempty"`
	EventID   *string   `json:"event_id,omitempty"`
	AuthorDID string    `json:"author_did"`
	Text      string    `json:"text"`
	
	// AT Protocol record tracking
	RecordDID  *string `json:"record_did,omitempty"`
	RecordRKey *string `json:"record_rkey,omitempty"`
	
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// UpsertResult tracks statistics for upsert operations.
type UpsertResult struct {
	Inserted bool   // True if new record was inserted
	ID       string // The UUID of the upserted record
}

// PostRepository defines the interface for post data operations.
type PostRepository interface {
	// Upsert inserts a new post or updates existing one based on (record_did, record_rkey).
	// Returns UpsertResult indicating whether insert or update occurred.
	Upsert(post *Post) (*UpsertResult, error)

	// GetByID retrieves a post by its UUID.
	GetByID(id string) (*Post, error)

	// GetByRecordKey retrieves a post by its AT Protocol record key.
	GetByRecordKey(did, rkey string) (*Post, error)
}

// InMemoryPostRepository is an in-memory implementation of PostRepository.
// Thread-safe via RWMutex.
type InMemoryPostRepository struct {
	mu    sync.RWMutex
	posts map[string]*Post                    // UUID -> Post
	keys  map[string]string                   // "did:rkey" -> UUID
}

// NewInMemoryPostRepository creates a new in-memory post repository.
func NewInMemoryPostRepository() *InMemoryPostRepository {
	return &InMemoryPostRepository{
		posts: make(map[string]*Post),
		keys:  make(map[string]string),
	}
}

// makeKey creates a composite key from DID and rkey.
func makeKey(did, rkey string) string {
	return did + ":" + rkey
}

// Upsert inserts a new post or updates existing one based on (record_did, record_rkey).
func (r *InMemoryPostRepository) Upsert(post *Post) (*UpsertResult, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	now := time.Now()
	var inserted bool
	var id string

	// Check if post exists by record key
	if post.RecordDID != nil && post.RecordRKey != nil {
		key := makeKey(*post.RecordDID, *post.RecordRKey)
		existingID, exists := r.keys[key]
		
		if exists {
			// Update existing post
			existing := r.posts[existingID]
			existing.SceneID = post.SceneID
			existing.EventID = post.EventID
			existing.AuthorDID = post.AuthorDID
			existing.Text = post.Text
			existing.UpdatedAt = now
			inserted = false
			id = existingID
		} else {
			// Insert new post
			if post.ID == "" {
				post.ID = uuid.New().String()
			}
			post.CreatedAt = now
			post.UpdatedAt = now
			
			postCopy := *post
			r.posts[post.ID] = &postCopy
			r.keys[key] = post.ID
			inserted = true
			id = post.ID
		}
	} else {
		// No record key, always insert new with new UUID
		newID := uuid.New().String()
		post.ID = newID
		post.CreatedAt = now
		post.UpdatedAt = now
		
		postCopy := *post
		r.posts[newID] = &postCopy
		inserted = true
		id = newID
	}

	return &UpsertResult{
		Inserted: inserted,
		ID:       id,
	}, nil
}

// GetByID retrieves a post by its UUID.
func (r *InMemoryPostRepository) GetByID(id string) (*Post, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	post, ok := r.posts[id]
	if !ok {
		return nil, ErrPostNotFound
	}

	postCopy := *post
	return &postCopy, nil
}

// GetByRecordKey retrieves a post by its AT Protocol record key.
func (r *InMemoryPostRepository) GetByRecordKey(did, rkey string) (*Post, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	key := makeKey(did, rkey)
	id, ok := r.keys[key]
	if !ok {
		return nil, ErrPostNotFound
	}

	post := r.posts[id]
	postCopy := *post
	return &postCopy, nil
}
