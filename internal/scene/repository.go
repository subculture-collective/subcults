// Package scene provides models and repository for managing scenes and events
// with location privacy controls.
package scene

import (
	"errors"
	"sync"

	"github.com/google/uuid"
)

// Common errors for scene and event operations.
var (
	ErrSceneNotFound = errors.New("scene not found")
	ErrEventNotFound = errors.New("event not found")
)

// UpsertResult tracks statistics for upsert operations.
type UpsertResult struct {
	Inserted bool   // True if new record was inserted
	ID       string // The UUID of the upserted record
}

// SceneRepository defines the interface for scene data operations.
// All implementations must enforce location consent before persisting data.
type SceneRepository interface {
	// Insert stores a new scene, enforcing location consent.
	// If allow_precise is false, precise_point will be set to NULL.
	Insert(scene *Scene) error

	// Update modifies an existing scene, enforcing location consent.
	// If allow_precise is false, precise_point will be set to NULL.
	Update(scene *Scene) error

	// Upsert inserts a new scene or updates existing one based on (record_did, record_rkey).
	// Returns UpsertResult indicating whether insert or update occurred.
	// Enforces location consent before persisting.
	Upsert(scene *Scene) (*UpsertResult, error)

	// GetByID retrieves a scene by its ID.
	GetByID(id string) (*Scene, error)

	// GetByRecordKey retrieves a scene by its AT Protocol record key.
	GetByRecordKey(did, rkey string) (*Scene, error)
}

// EventRepository defines the interface for event data operations.
// All implementations must enforce location consent before persisting data.
type EventRepository interface {
	// Insert stores a new event, enforcing location consent.
	// If allow_precise is false, precise_point will be set to NULL.
	Insert(event *Event) error

	// Update modifies an existing event, enforcing location consent.
	// If allow_precise is false, precise_point will be set to NULL.
	Update(event *Event) error

	// Upsert inserts a new event or updates existing one based on (record_did, record_rkey).
	// Returns UpsertResult indicating whether insert or update occurred.
	// Enforces location consent before persisting.
	Upsert(event *Event) (*UpsertResult, error)

	// GetByID retrieves an event by its ID.
	GetByID(id string) (*Event, error)

	// GetByRecordKey retrieves an event by its AT Protocol record key.
	GetByRecordKey(did, rkey string) (*Event, error)
}

// InMemorySceneRepository is an in-memory implementation of SceneRepository.
// Used for testing and development. Thread-safe via RWMutex.
type InMemorySceneRepository struct {
	mu     sync.RWMutex
	scenes map[string]*Scene
	keys   map[string]string // "did:rkey" -> UUID
}

// NewInMemorySceneRepository creates a new in-memory scene repository.
func NewInMemorySceneRepository() *InMemorySceneRepository {
	return &InMemorySceneRepository{
		scenes: make(map[string]*Scene),
		keys:   make(map[string]string),
	}
}

// Insert stores a new scene, enforcing location consent.
// If allow_precise is false, precise_point will be set to NULL.
func (r *InMemorySceneRepository) Insert(scene *Scene) error {
	// Create a deep copy to avoid modifying the original
	sceneCopy := *scene
	if scene.PrecisePoint != nil {
		pointCopy := *scene.PrecisePoint
		sceneCopy.PrecisePoint = &pointCopy
	}

	// Enforce consent before storing - this is the critical privacy control
	sceneCopy.EnforceLocationConsent()

	r.mu.Lock()
	r.scenes[sceneCopy.ID] = &sceneCopy
	r.mu.Unlock()
	return nil
}

// Update modifies an existing scene, enforcing location consent.
// If allow_precise is false, precise_point will be set to NULL.
func (r *InMemorySceneRepository) Update(scene *Scene) error {
	// Create a deep copy to avoid modifying the original
	sceneCopy := *scene
	if scene.PrecisePoint != nil {
		pointCopy := *scene.PrecisePoint
		sceneCopy.PrecisePoint = &pointCopy
	}

	// Enforce consent before storing - this is the critical privacy control
	sceneCopy.EnforceLocationConsent()

	r.mu.Lock()
	r.scenes[sceneCopy.ID] = &sceneCopy
	r.mu.Unlock()
	return nil
}

// GetByID retrieves a scene by its ID.
func (r *InMemorySceneRepository) GetByID(id string) (*Scene, error) {
	r.mu.RLock()
	scene, ok := r.scenes[id]
	r.mu.RUnlock()
	if !ok {
		return nil, ErrSceneNotFound
	}
	// Return a copy to avoid external modification
	sceneCopy := *scene
	if scene.PrecisePoint != nil {
		pointCopy := *scene.PrecisePoint
		sceneCopy.PrecisePoint = &pointCopy
	}
	return &sceneCopy, nil
}

// makeSceneKey creates a composite key from DID and rkey using a null byte separator to avoid collisions.
// AT Protocol DIDs contain colons (e.g., "did:plc:abc123"), so using a null byte prevents
// collisions like did="a:b" + rkey="c" vs did="a" + rkey="b:c" both producing "a:b:c".
func makeSceneKey(did, rkey string) string {
	return did + "\x00" + rkey
}

// Upsert inserts a new scene or updates existing one based on (record_did, record_rkey).
// Enforces location consent before persisting.
func (r *InMemorySceneRepository) Upsert(scene *Scene) (*UpsertResult, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	var inserted bool
	var id string

	// Create a deep copy to avoid modifying the original
	sceneCopy := *scene
	if scene.PrecisePoint != nil {
		pointCopy := *scene.PrecisePoint
		sceneCopy.PrecisePoint = &pointCopy
	}

	// Enforce consent before storing - this is the critical privacy control
	sceneCopy.EnforceLocationConsent()

	// Check if scene exists by record key
	if scene.RecordDID != nil && scene.RecordRKey != nil {
		key := makeSceneKey(*scene.RecordDID, *scene.RecordRKey)
		existingID, exists := r.keys[key]
		
		if exists {
			// Update existing scene
			sceneCopy.ID = existingID
			r.scenes[existingID] = &sceneCopy
			inserted = false
			id = existingID
		} else {
			// Insert new scene
			if sceneCopy.ID == "" {
				sceneCopy.ID = uuid.New().String()
			}
			r.scenes[sceneCopy.ID] = &sceneCopy
			r.keys[key] = sceneCopy.ID
			inserted = true
			id = sceneCopy.ID
		}
	} else {
		// No record key, always insert new with new UUID
		newID := uuid.New().String()
		sceneCopy.ID = newID
		r.scenes[newID] = &sceneCopy
		inserted = true
		id = newID
	}

	return &UpsertResult{
		Inserted: inserted,
		ID:       id,
	}, nil
}

// GetByRecordKey retrieves a scene by its AT Protocol record key.
func (r *InMemorySceneRepository) GetByRecordKey(did, rkey string) (*Scene, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	key := makeSceneKey(did, rkey)
	id, ok := r.keys[key]
	if !ok {
		return nil, ErrSceneNotFound
	}

	scene := r.scenes[id]
	sceneCopy := *scene
	if scene.PrecisePoint != nil {
		pointCopy := *scene.PrecisePoint
		sceneCopy.PrecisePoint = &pointCopy
	}
	return &sceneCopy, nil
}

// InMemoryEventRepository is an in-memory implementation of EventRepository.
// Used for testing and development. Thread-safe via RWMutex.
type InMemoryEventRepository struct {
	mu     sync.RWMutex
	events map[string]*Event
	keys   map[string]string // "did:rkey" -> UUID
}

// NewInMemoryEventRepository creates a new in-memory event repository.
func NewInMemoryEventRepository() *InMemoryEventRepository {
	return &InMemoryEventRepository{
		events: make(map[string]*Event),
		keys:   make(map[string]string),
	}
}

// Insert stores a new event, enforcing location consent.
// If allow_precise is false, precise_point will be set to NULL.
func (r *InMemoryEventRepository) Insert(event *Event) error {
	// Create a deep copy to avoid modifying the original
	eventCopy := *event
	if event.PrecisePoint != nil {
		pointCopy := *event.PrecisePoint
		eventCopy.PrecisePoint = &pointCopy
	}

	// Enforce consent before storing - this is the critical privacy control
	eventCopy.EnforceLocationConsent()

	r.mu.Lock()
	r.events[eventCopy.ID] = &eventCopy
	r.mu.Unlock()
	return nil
}

// Update modifies an existing event, enforcing location consent.
// If allow_precise is false, precise_point will be set to NULL.
func (r *InMemoryEventRepository) Update(event *Event) error {
	// Create a deep copy to avoid modifying the original
	eventCopy := *event
	if event.PrecisePoint != nil {
		pointCopy := *event.PrecisePoint
		eventCopy.PrecisePoint = &pointCopy
	}

	// Enforce consent before storing - this is the critical privacy control
	eventCopy.EnforceLocationConsent()

	r.mu.Lock()
	r.events[eventCopy.ID] = &eventCopy
	r.mu.Unlock()
	return nil
}

// GetByID retrieves an event by its ID.
func (r *InMemoryEventRepository) GetByID(id string) (*Event, error) {
	r.mu.RLock()
	event, ok := r.events[id]
	r.mu.RUnlock()
	if !ok {
		return nil, ErrEventNotFound
	}
	// Return a copy to avoid external modification
	eventCopy := *event
	if event.PrecisePoint != nil {
		pointCopy := *event.PrecisePoint
		eventCopy.PrecisePoint = &pointCopy
	}
	return &eventCopy, nil
}

// makeEventKey creates a composite key from DID and rkey using a null byte separator to avoid collisions.
// AT Protocol DIDs contain colons (e.g., "did:plc:abc123"), so using a null byte prevents
// collisions like did="a:b" + rkey="c" vs did="a" + rkey="b:c" both producing "a:b:c".
func makeEventKey(did, rkey string) string {
	return did + "\x00" + rkey
}

// Upsert inserts a new event or updates existing one based on (record_did, record_rkey).
// Enforces location consent before persisting.
func (r *InMemoryEventRepository) Upsert(event *Event) (*UpsertResult, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	var inserted bool
	var id string

	// Create a deep copy to avoid modifying the original
	eventCopy := *event
	if event.PrecisePoint != nil {
		pointCopy := *event.PrecisePoint
		eventCopy.PrecisePoint = &pointCopy
	}

	// Enforce consent before storing - this is the critical privacy control
	eventCopy.EnforceLocationConsent()

	// Check if event exists by record key
	if event.RecordDID != nil && event.RecordRKey != nil {
		key := makeEventKey(*event.RecordDID, *event.RecordRKey)
		existingID, exists := r.keys[key]
		
		if exists {
			// Update existing event
			eventCopy.ID = existingID
			r.events[existingID] = &eventCopy
			inserted = false
			id = existingID
		} else {
			// Insert new event
			if eventCopy.ID == "" {
				eventCopy.ID = uuid.New().String()
			}
			r.events[eventCopy.ID] = &eventCopy
			r.keys[key] = eventCopy.ID
			inserted = true
			id = eventCopy.ID
		}
	} else {
		// No record key, always insert new with new UUID
		newID := uuid.New().String()
		eventCopy.ID = newID
		r.events[newID] = &eventCopy
		inserted = true
		id = newID
	}

	return &UpsertResult{
		Inserted: inserted,
		ID:       id,
	}, nil
}

// GetByRecordKey retrieves an event by its AT Protocol record key.
func (r *InMemoryEventRepository) GetByRecordKey(did, rkey string) (*Event, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	key := makeEventKey(did, rkey)
	id, ok := r.keys[key]
	if !ok {
		return nil, ErrEventNotFound
	}

	event := r.events[id]
	eventCopy := *event
	if event.PrecisePoint != nil {
		pointCopy := *event.PrecisePoint
		eventCopy.PrecisePoint = &pointCopy
	}
	return &eventCopy, nil
}
