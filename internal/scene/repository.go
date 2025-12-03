// Package scene provides models and repository for managing scenes and events
// with location privacy controls.
package scene

// SceneRepository defines the interface for scene data operations.
// All implementations must enforce location consent before persisting data.
type SceneRepository interface {
	// Insert stores a new scene, enforcing location consent.
	// If allow_precise is false, precise_point will be set to NULL.
	Insert(scene *Scene) error

	// Update modifies an existing scene, enforcing location consent.
	// If allow_precise is false, precise_point will be set to NULL.
	Update(scene *Scene) error

	// GetByID retrieves a scene by its ID.
	GetByID(id string) (*Scene, error)
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

	// GetByID retrieves an event by its ID.
	GetByID(id string) (*Event, error)
}

// InMemorySceneRepository is an in-memory implementation of SceneRepository.
// Used for testing and development.
type InMemorySceneRepository struct {
	scenes map[string]*Scene
}

// NewInMemorySceneRepository creates a new in-memory scene repository.
func NewInMemorySceneRepository() *InMemorySceneRepository {
	return &InMemorySceneRepository{
		scenes: make(map[string]*Scene),
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

	r.scenes[sceneCopy.ID] = &sceneCopy
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

	r.scenes[sceneCopy.ID] = &sceneCopy
	return nil
}

// GetByID retrieves a scene by its ID.
func (r *InMemorySceneRepository) GetByID(id string) (*Scene, error) {
	scene, ok := r.scenes[id]
	if !ok {
		return nil, nil
	}
	// Return a copy to avoid external modification
	sceneCopy := *scene
	if scene.PrecisePoint != nil {
		pointCopy := *scene.PrecisePoint
		sceneCopy.PrecisePoint = &pointCopy
	}
	return &sceneCopy, nil
}

// InMemoryEventRepository is an in-memory implementation of EventRepository.
// Used for testing and development.
type InMemoryEventRepository struct {
	events map[string]*Event
}

// NewInMemoryEventRepository creates a new in-memory event repository.
func NewInMemoryEventRepository() *InMemoryEventRepository {
	return &InMemoryEventRepository{
		events: make(map[string]*Event),
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

	r.events[eventCopy.ID] = &eventCopy
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

	r.events[eventCopy.ID] = &eventCopy
	return nil
}

// GetByID retrieves an event by its ID.
func (r *InMemoryEventRepository) GetByID(id string) (*Event, error) {
	event, ok := r.events[id]
	if !ok {
		return nil, nil
	}
	// Return a copy to avoid external modification
	eventCopy := *event
	if event.PrecisePoint != nil {
		pointCopy := *event.PrecisePoint
		eventCopy.PrecisePoint = &pointCopy
	}
	return &eventCopy, nil
}
