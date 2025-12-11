// Package scene provides models and repository for managing scenes and events
// with location privacy controls.
package scene

import (
	"errors"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
)

// Common errors for scene and event operations.
var (
	ErrSceneNotFound      = errors.New("scene not found")
	ErrSceneDeleted       = errors.New("scene deleted")
	ErrEventNotFound      = errors.New("event not found")
	ErrDuplicateSceneName = errors.New("scene name already exists for this owner")
	ErrRSVPNotFound       = errors.New("rsvp not found")
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
	// Returns ErrSceneNotFound if scene doesn't exist or is soft-deleted.
	GetByID(id string) (*Scene, error)

	// GetByRecordKey retrieves a scene by its AT Protocol record key.
	GetByRecordKey(did, rkey string) (*Scene, error)
	
	// Delete soft-deletes a scene by setting deleted_at timestamp.
	// Returns ErrSceneNotFound if scene doesn't exist or is already deleted.
	Delete(id string) error
	
	// ExistsByOwnerAndName checks if a non-deleted scene with the given name
	// exists for the specified owner. Used for duplicate name validation.
	ExistsByOwnerAndName(ownerDID, name string, excludeID string) (bool, error)
	
	// ListByOwner retrieves all non-deleted scenes owned by the specified DID.
	// Returns empty slice if no scenes found.
	ListByOwner(ownerDID string) ([]*Scene, error)
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
	
	// Cancel marks an event as cancelled with an optional reason.
	// Sets status to "cancelled", stores cancelled_at timestamp, and cancellation_reason.
	// Returns ErrEventNotFound if event doesn't exist.
	// Idempotent: returns nil if event is already cancelled.
	Cancel(id string, reason *string) error

	// SearchByBboxAndTime searches for events within a bounding box and time range.
	// Filters out cancelled events and applies pagination.
	// Returns events sorted by starts_at ascending.
	SearchByBboxAndTime(minLng, minLat, maxLng, maxLat float64, from, to time.Time, limit int, cursor string) ([]*Event, string, error)
}

// RSVPRepository defines the interface for RSVP data operations.
type RSVPRepository interface {
	// Upsert inserts or updates an RSVP for an event.
	// Idempotent: if RSVP exists with same status, returns without error.
	Upsert(rsvp *RSVP) error

	// Delete removes an RSVP for a user and event.
	// Returns ErrRSVPNotFound if RSVP doesn't exist.
	Delete(eventID, userID string) error

	// GetByEventAndUser retrieves an RSVP for a specific user and event.
	// Returns ErrRSVPNotFound if RSVP doesn't exist.
	GetByEventAndUser(eventID, userID string) (*RSVP, error)

	// GetCountsByEvent returns aggregated RSVP counts by status for an event.
	GetCountsByEvent(eventID string) (*RSVPCounts, error)
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
// Returns ErrSceneNotFound if scene doesn't exist.
// Returns ErrSceneDeleted if scene exists but is soft-deleted.
func (r *InMemorySceneRepository) GetByID(id string) (*Scene, error) {
	r.mu.RLock()
	scene, ok := r.scenes[id]
	r.mu.RUnlock()
	if !ok {
		return nil, ErrSceneNotFound
	}
	if scene.DeletedAt != nil {
		return nil, ErrSceneDeleted
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

// Delete soft-deletes a scene by setting deleted_at timestamp.
// Returns ErrSceneNotFound if scene doesn't exist.
// Returns ErrSceneDeleted if scene is already deleted.
func (r *InMemorySceneRepository) Delete(id string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	scene, ok := r.scenes[id]
	if !ok {
		return ErrSceneNotFound
	}
	if scene.DeletedAt != nil {
		return ErrSceneDeleted
	}

	now := time.Now()
	scene.DeletedAt = &now
	return nil
}

// ExistsByOwnerAndName checks if a non-deleted scene with the given name
// exists for the specified owner. Used for duplicate name validation.
// Performs case-insensitive comparison to prevent names differing only by case.
// excludeID allows checking for duplicates while excluding a specific scene
// (useful when updating a scene's name).
func (r *InMemorySceneRepository) ExistsByOwnerAndName(ownerDID, name string, excludeID string) (bool, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	// Normalize name to lowercase for case-insensitive comparison
	normalizedName := strings.ToLower(name)
	for id, scene := range r.scenes {
		if id == excludeID {
			continue
		}
		if scene.DeletedAt == nil && scene.OwnerDID == ownerDID && strings.ToLower(scene.Name) == normalizedName {
			return true, nil
		}
	}
	return false, nil
}

// ListByOwner retrieves all non-deleted scenes owned by the specified DID.
// Returns empty slice if no scenes found.
func (r *InMemorySceneRepository) ListByOwner(ownerDID string) ([]*Scene, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var result []*Scene
	for _, scene := range r.scenes {
		if scene.DeletedAt == nil && scene.OwnerDID == ownerDID {
			// Return a copy to avoid external modification
			sceneCopy := *scene
			if scene.PrecisePoint != nil {
				pointCopy := *scene.PrecisePoint
				sceneCopy.PrecisePoint = &pointCopy
			}
			result = append(result, &sceneCopy)
		}
	}

	return result, nil
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

// copyEvent creates a deep copy of an event to avoid external modification.
func copyEvent(event *Event) *Event {
	eventCopy := *event
	if event.PrecisePoint != nil {
		pointCopy := *event.PrecisePoint
		eventCopy.PrecisePoint = &pointCopy
	}
	return &eventCopy
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

// Cancel marks an event as cancelled with an optional reason.
// Sets status to "cancelled", stores cancelled_at timestamp, and cancellation_reason.
// Returns ErrEventNotFound if event doesn't exist.
// Idempotent: returns nil if event is already cancelled.
func (r *InMemoryEventRepository) Cancel(id string, reason *string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	event, ok := r.events[id]
	if !ok {
		return ErrEventNotFound
	}

	// Idempotent: if already cancelled, return success
	if event.Status == "cancelled" && event.CancelledAt != nil {
		return nil
	}

	// Update event status and cancellation metadata
	now := time.Now()
	event.Status = "cancelled"
	event.CancelledAt = &now
	event.CancellationReason = reason
	event.UpdatedAt = &now

	return nil
}

// SearchByBboxAndTime searches for events within a bounding box and time range.
// Filters out cancelled events and applies pagination.
// Returns events sorted by starts_at ascending.
func (r *InMemoryEventRepository) SearchByBboxAndTime(minLng, minLat, maxLng, maxLat float64, from, to time.Time, limit int, cursor string) ([]*Event, string, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	// Initialize empty slice to ensure non-nil return
	results := make([]*Event, 0)
	var cursorTime time.Time
	var cursorID string

	// Parse cursor if provided (format: "RFC3339|ID")
	// Using | as separator to avoid conflicts with colons in RFC3339 timestamps
	if cursor != "" {
		parts := strings.Split(cursor, "|")
		if len(parts) == 2 {
			parsedTime, err := time.Parse(time.RFC3339, parts[0])
			if err == nil {
				// Truncate to second precision to match RFC3339 format
				// This ensures comparison works correctly
				cursorTime = parsedTime.Truncate(time.Second)
				cursorID = parts[1]
			}
		}
	}

	// Collect matching events
	for _, event := range r.events {
		// Skip cancelled events
		if event.Status == "cancelled" {
			continue
		}

		// Skip deleted events
		if event.DeletedAt != nil {
			continue
		}

		// Check time range: event starts_at must be between from and to
		if event.StartsAt.Before(from) || event.StartsAt.After(to) {
			continue
		}

		// Check bounding box
		// For in-memory implementation, check if precise_point is within bbox
		// In a real PostGIS implementation, this would use ST_MakeEnvelope and geohash intersection
		if event.PrecisePoint != nil {
			lat := event.PrecisePoint.Lat
			lng := event.PrecisePoint.Lng
			if lng >= minLng && lng <= maxLng && lat >= minLat && lat <= maxLat {
				results = append(results, copyEvent(event))
			}
		}
		// Events without precise_point are currently excluded from search results.
		// Coarse geohash intersection support will be added in a future update.
	}

	// Sort by starts_at ascending, then by ID for stable ordering
	// Using stdlib sort for O(n log n) performance
	sort.Slice(results, func(i, j int) bool {
		if results[i].StartsAt.Equal(results[j].StartsAt) {
			return results[i].ID < results[j].ID
		}
		return results[i].StartsAt.Before(results[j].StartsAt)
	})

	// Apply cursor filter AFTER sorting for stable pagination
	if !cursorTime.IsZero() {
		filtered := make([]*Event, 0)
		for _, event := range results {
			// Truncate event time to second precision for comparison
			eventTime := event.StartsAt.Truncate(time.Second)
			
			// Skip events before cursor time
			if eventTime.Before(cursorTime) {
				continue
			}
			// If same time as cursor, skip events with ID <= cursor ID (for stable ordering)
			if eventTime.Equal(cursorTime) && event.ID <= cursorID {
				continue
			}
			// Event is after cursor position, include it
			filtered = append(filtered, event)
		}
		results = filtered
	}

	// Apply limit and generate next cursor
	var nextCursor string
	if len(results) > limit {
		// We have more results than requested, truncate and set cursor
		if limit > 0 {
			lastEvent := results[limit-1]
			// Use | as separator to avoid conflicts with colons in RFC3339 timestamps
			nextCursor = lastEvent.StartsAt.Format(time.RFC3339) + "|" + lastEvent.ID
			results = results[:limit]
		}
	}

	return results, nextCursor, nil
}

// InMemoryRSVPRepository is an in-memory implementation of RSVPRepository.
// Used for testing and development. Thread-safe via RWMutex.
type InMemoryRSVPRepository struct {
	mu    sync.RWMutex
	rsvps map[string]*RSVP // key: "eventID:userID"
}

// NewInMemoryRSVPRepository creates a new in-memory RSVP repository.
func NewInMemoryRSVPRepository() *InMemoryRSVPRepository {
	return &InMemoryRSVPRepository{
		rsvps: make(map[string]*RSVP),
	}
}

// makeRSVPKey creates a composite key from event ID and user ID.
func makeRSVPKey(eventID, userID string) string {
	return eventID + ":" + userID
}

// Upsert inserts or updates an RSVP for an event.
// Idempotent: if RSVP exists with same status, returns without error.
func (r *InMemoryRSVPRepository) Upsert(rsvp *RSVP) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	key := makeRSVPKey(rsvp.EventID, rsvp.UserID)
	now := time.Now()

	// Check if RSVP already exists
	existing, exists := r.rsvps[key]
	if exists {
		// Update existing RSVP
		existing.Status = rsvp.Status
		existing.UpdatedAt = &now
	} else {
		// Create new RSVP
		rsvpCopy := *rsvp
		rsvpCopy.CreatedAt = &now
		rsvpCopy.UpdatedAt = &now
		r.rsvps[key] = &rsvpCopy
	}

	return nil
}

// Delete removes an RSVP for a user and event.
// Returns ErrRSVPNotFound if RSVP doesn't exist.
func (r *InMemoryRSVPRepository) Delete(eventID, userID string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	key := makeRSVPKey(eventID, userID)
	if _, exists := r.rsvps[key]; !exists {
		return ErrRSVPNotFound
	}

	delete(r.rsvps, key)
	return nil
}

// GetByEventAndUser retrieves an RSVP for a specific user and event.
// Returns ErrRSVPNotFound if RSVP doesn't exist.
func (r *InMemoryRSVPRepository) GetByEventAndUser(eventID, userID string) (*RSVP, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	key := makeRSVPKey(eventID, userID)
	rsvp, exists := r.rsvps[key]
	if !exists {
		return nil, ErrRSVPNotFound
	}

	// Return a copy to avoid external modification
	rsvpCopy := *rsvp
	return &rsvpCopy, nil
}

// GetCountsByEvent returns aggregated RSVP counts by status for an event.
func (r *InMemoryRSVPRepository) GetCountsByEvent(eventID string) (*RSVPCounts, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	counts := &RSVPCounts{
		Going: 0,
		Maybe: 0,
	}

	for _, rsvp := range r.rsvps {
		if rsvp.EventID == eventID {
			switch rsvp.Status {
			case "going":
				counts.Going++
			case "maybe":
				counts.Maybe++
			}
		}
	}

	return counts, nil
}
