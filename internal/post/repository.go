// Package post provides models and repository for managing posts
// with AT Protocol record tracking for idempotent ingestion.
package post

import (
	"errors"
	"sort"
	"sync"
	"time"

	"github.com/google/uuid"
)

// Common errors for post operations.
var (
	ErrPostNotFound = errors.New("post not found")
	ErrPostDeleted  = errors.New("post has been deleted")
)

// Attachment represents a media attachment on a post.
type Attachment struct {
	URL  string `json:"url"`
	Type string `json:"type,omitempty"`
}

// Post represents a content post within scenes/events.
type Post struct {
	ID          string       `json:"id"`
	SceneID     *string      `json:"scene_id,omitempty"`
	EventID     *string      `json:"event_id,omitempty"`
	AuthorDID   string       `json:"author_did"`
	Text        string       `json:"text"`
	Attachments []Attachment `json:"attachments,omitempty"`
	Labels      []string     `json:"labels,omitempty"`

	// AT Protocol record tracking
	RecordDID  *string `json:"record_did,omitempty"`
	RecordRKey *string `json:"record_rkey,omitempty"`

	CreatedAt time.Time  `json:"created_at"`
	UpdatedAt time.Time  `json:"updated_at"`
	DeletedAt *time.Time `json:"deleted_at,omitempty"`
}

// UpsertResult tracks statistics for upsert operations.
type UpsertResult struct {
	Inserted bool   // True if new record was inserted
	ID       string // The UUID of the upserted record
}

// FeedCursor represents a cursor for paginating through a feed.
// Uses (created_at, id) for stable pagination with tie-breaking.
type FeedCursor struct {
	CreatedAt time.Time `json:"created_at"`
	ID        string    `json:"id"`
}

// PostRepository defines the interface for post data operations.
type PostRepository interface {
	// Upsert inserts a new post or updates existing one based on (record_did, record_rkey).
	// Returns UpsertResult indicating whether insert or update occurred.
	Upsert(post *Post) (*UpsertResult, error)

	// Create inserts a new post with a generated UUID.
	Create(post *Post) error

	// Update updates an existing post.
	Update(post *Post) error

	// Delete soft-deletes a post by setting deleted_at timestamp.
	Delete(id string) error

	// GetByID retrieves a post by its UUID, excluding soft-deleted posts.
	GetByID(id string) (*Post, error)

	// GetByRecordKey retrieves a post by its AT Protocol record key.
	GetByRecordKey(did, rkey string) (*Post, error)

	// ListByScene retrieves posts for a scene with cursor-based pagination.
	// Returns posts ordered by created_at DESC, id ASC (tie-breaker).
	// Excludes soft-deleted posts and posts with 'hidden' label.
	// If cursor is nil, starts from the most recent post.
	// Returns posts, next cursor (nil if no more), and error.
	ListByScene(sceneID string, limit int, cursor *FeedCursor) ([]*Post, *FeedCursor, error)

	// ListByEvent retrieves posts for an event with cursor-based pagination.
	// Returns posts ordered by created_at DESC, id ASC (tie-breaker).
	// Excludes soft-deleted posts and posts with 'hidden' label.
	// If cursor is nil, starts from the most recent post.
	// Returns posts, next cursor (nil if no more), and error.
	ListByEvent(eventID string, limit int, cursor *FeedCursor) ([]*Post, *FeedCursor, error)
}

// InMemoryPostRepository is an in-memory implementation of PostRepository.
// Thread-safe via RWMutex.
type InMemoryPostRepository struct {
	mu    sync.RWMutex
	posts map[string]*Post  // UUID -> Post
	keys  map[string]string // "did:rkey" -> UUID
}

// NewInMemoryPostRepository creates a new in-memory post repository.
func NewInMemoryPostRepository() *InMemoryPostRepository {
	return &InMemoryPostRepository{
		posts: make(map[string]*Post),
		keys:  make(map[string]string),
	}
}

// makeKey creates a composite key from DID and rkey using a null byte separator to avoid collisions.
// AT Protocol DIDs contain colons (e.g., "did:plc:abc123"), so using a null byte prevents
// collisions like did="a:b" + rkey="c" vs did="a" + rkey="b:c" both producing "a:b:c".
func makeKey(did, rkey string) string {
	return did + "\x00" + rkey
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
			existing.Attachments = post.Attachments
			existing.Labels = post.Labels
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

// Create inserts a new post with a generated UUID.
func (r *InMemoryPostRepository) Create(post *Post) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	now := time.Now()
	post.ID = uuid.New().String()
	post.CreatedAt = now
	post.UpdatedAt = now

	postCopy := *post
	r.posts[post.ID] = &postCopy

	// If record key is provided, track it
	if post.RecordDID != nil && post.RecordRKey != nil {
		key := makeKey(*post.RecordDID, *post.RecordRKey)
		r.keys[key] = post.ID
	}

	return nil
}

// Update updates an existing post.
func (r *InMemoryPostRepository) Update(post *Post) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	existing, ok := r.posts[post.ID]
	if !ok {
		return ErrPostNotFound
	}

	// Don't allow updating deleted posts
	if existing.DeletedAt != nil {
		return ErrPostDeleted
	}

	// Update mutable fields
	existing.Text = post.Text
	existing.Attachments = post.Attachments
	existing.Labels = post.Labels
	existing.UpdatedAt = time.Now()

	return nil
}

// Delete soft-deletes a post by setting deleted_at timestamp.
func (r *InMemoryPostRepository) Delete(id string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	post, ok := r.posts[id]
	if !ok {
		return ErrPostNotFound
	}

	// Already deleted - treat as not found for idempotency
	if post.DeletedAt != nil {
		return ErrPostNotFound
	}

	now := time.Now()
	post.DeletedAt = &now

	return nil
}

// GetByID retrieves a post by its UUID, excluding soft-deleted posts.
func (r *InMemoryPostRepository) GetByID(id string) (*Post, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	post, ok := r.posts[id]
	if !ok {
		return nil, ErrPostNotFound
	}

	// Exclude soft-deleted posts
	if post.DeletedAt != nil {
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

// ListByScene retrieves posts for a scene with cursor-based pagination.
func (r *InMemoryPostRepository) ListByScene(sceneID string, limit int, cursor *FeedCursor) ([]*Post, *FeedCursor, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	// Collect all non-deleted posts for this scene
	var candidates []*Post
	for _, post := range r.posts {
		// Skip deleted posts
		if post.DeletedAt != nil {
			continue
		}
		
		// Skip posts not in this scene
		if post.SceneID == nil || *post.SceneID != sceneID {
			continue
		}
		
		// Skip hidden posts
		if post.HasLabel(LabelHidden) {
			continue
		}
		
		// Apply cursor filter if provided
		if cursor != nil {
			// Skip posts that are newer or equal (in terms of created_at) but have higher/equal ID
			if post.CreatedAt.After(cursor.CreatedAt) {
				continue
			}
			if post.CreatedAt.Equal(cursor.CreatedAt) && post.ID >= cursor.ID {
				continue
			}
		}
		
		candidates = append(candidates, post)
	}

	// Sort by created_at DESC, then by ID ASC for tie-breaking
	// This ensures stable pagination
	sortPostsByCreatedDesc(candidates)

	// Apply limit and determine next cursor
	var results []*Post
	var nextCursor *FeedCursor
	
	if len(candidates) > limit {
		results = candidates[:limit]
		// Next cursor points to the last returned post
		lastPost := results[len(results)-1]
		nextCursor = &FeedCursor{
			CreatedAt: lastPost.CreatedAt,
			ID:        lastPost.ID,
		}
	} else {
		results = candidates
		// No more posts, cursor is nil
	}

	// Return deep copies to prevent external mutation
	copies := make([]*Post, len(results))
	for i, p := range results {
		postCopy := *p
		copies[i] = &postCopy
	}

	return copies, nextCursor, nil
}

// ListByEvent retrieves posts for an event with cursor-based pagination.
func (r *InMemoryPostRepository) ListByEvent(eventID string, limit int, cursor *FeedCursor) ([]*Post, *FeedCursor, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	// Collect all non-deleted posts for this event
	var candidates []*Post
	for _, post := range r.posts {
		// Skip deleted posts
		if post.DeletedAt != nil {
			continue
		}
		
		// Skip posts not in this event
		if post.EventID == nil || *post.EventID != eventID {
			continue
		}
		
		// Skip hidden posts
		if post.HasLabel(LabelHidden) {
			continue
		}
		
		// Apply cursor filter if provided
		if cursor != nil {
			// Skip posts that are newer or equal (in terms of created_at) but have higher/equal ID
			if post.CreatedAt.After(cursor.CreatedAt) {
				continue
			}
			if post.CreatedAt.Equal(cursor.CreatedAt) && post.ID >= cursor.ID {
				continue
			}
		}
		
		candidates = append(candidates, post)
	}

	// Sort by created_at DESC, then by ID ASC for tie-breaking
	sortPostsByCreatedDesc(candidates)

	// Apply limit and determine next cursor
	var results []*Post
	var nextCursor *FeedCursor
	
	if len(candidates) > limit {
		results = candidates[:limit]
		// Next cursor points to the last returned post
		lastPost := results[len(results)-1]
		nextCursor = &FeedCursor{
			CreatedAt: lastPost.CreatedAt,
			ID:        lastPost.ID,
		}
	} else {
		results = candidates
		// No more posts, cursor is nil
	}

	// Return deep copies to prevent external mutation
	copies := make([]*Post, len(results))
	for i, p := range results {
		postCopy := *p
		copies[i] = &postCopy
	}

	return copies, nextCursor, nil
}

// sortPostsByCreatedDesc sorts posts by created_at DESC, then by ID ASC for tie-breaking.
// This provides stable ordering for cursor-based pagination.
// Uses sort.Slice with O(n log n) introsort for efficient sorting of large result sets.
func sortPostsByCreatedDesc(posts []*Post) {
	sort.Slice(posts, func(i, j int) bool {
		// Sort by created_at DESC (newer first)
		if posts[i].CreatedAt.After(posts[j].CreatedAt) {
			return true
		}
		if posts[i].CreatedAt.Before(posts[j].CreatedAt) {
			return false
		}
		// Tie-break by ID ASC (lexicographic order) when timestamps are equal
		return posts[i].ID < posts[j].ID
	})
}
