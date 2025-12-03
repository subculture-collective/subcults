package scene

import (
	"testing"
)

func TestScene_EnforceLocationConsent(t *testing.T) {
	tests := []struct {
		name                 string
		allowPrecise         bool
		hasPrecisePoint      bool
		wantPrecisePointNil  bool
	}{
		{
			name:                 "consent false with precise point - should clear",
			allowPrecise:         false,
			hasPrecisePoint:      true,
			wantPrecisePointNil:  true,
		},
		{
			name:                 "consent false without precise point - remains nil",
			allowPrecise:         false,
			hasPrecisePoint:      false,
			wantPrecisePointNil:  true,
		},
		{
			name:                 "consent true with precise point - should keep",
			allowPrecise:         true,
			hasPrecisePoint:      true,
			wantPrecisePointNil:  false,
		},
		{
			name:                 "consent true without precise point - remains nil",
			allowPrecise:         true,
			hasPrecisePoint:      false,
			wantPrecisePointNil:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			scene := &Scene{
				ID:           "scene-1",
				Name:         "Test Scene",
				AllowPrecise: tt.allowPrecise,
			}
			if tt.hasPrecisePoint {
				scene.PrecisePoint = &Point{Lat: 40.7128, Lng: -74.0060}
			}

			scene.EnforceLocationConsent()

			if (scene.PrecisePoint == nil) != tt.wantPrecisePointNil {
				t.Errorf("EnforceLocationConsent() PrecisePoint = %v, wantNil = %v", scene.PrecisePoint, tt.wantPrecisePointNil)
			}
		})
	}
}

func TestEvent_EnforceLocationConsent(t *testing.T) {
	tests := []struct {
		name                 string
		allowPrecise         bool
		hasPrecisePoint      bool
		wantPrecisePointNil  bool
	}{
		{
			name:                 "consent false with precise point - should clear",
			allowPrecise:         false,
			hasPrecisePoint:      true,
			wantPrecisePointNil:  true,
		},
		{
			name:                 "consent false without precise point - remains nil",
			allowPrecise:         false,
			hasPrecisePoint:      false,
			wantPrecisePointNil:  true,
		},
		{
			name:                 "consent true with precise point - should keep",
			allowPrecise:         true,
			hasPrecisePoint:      true,
			wantPrecisePointNil:  false,
		},
		{
			name:                 "consent true without precise point - remains nil",
			allowPrecise:         true,
			hasPrecisePoint:      false,
			wantPrecisePointNil:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			event := &Event{
				ID:           "event-1",
				SceneID:      "scene-1",
				Name:         "Test Event",
				AllowPrecise: tt.allowPrecise,
			}
			if tt.hasPrecisePoint {
				event.PrecisePoint = &Point{Lat: 40.7128, Lng: -74.0060}
			}

			event.EnforceLocationConsent()

			if (event.PrecisePoint == nil) != tt.wantPrecisePointNil {
				t.Errorf("EnforceLocationConsent() PrecisePoint = %v, wantNil = %v", event.PrecisePoint, tt.wantPrecisePointNil)
			}
		})
	}
}

func TestInMemorySceneRepository_Insert_WithoutConsent(t *testing.T) {
	repo := NewInMemorySceneRepository()

	// Insert a scene with precise point but without consent
	scene := &Scene{
		ID:           "scene-1",
		Name:         "Test Scene",
		AllowPrecise: false, // No consent
		PrecisePoint: &Point{Lat: 40.7128, Lng: -74.0060},
	}

	err := repo.Insert(scene)
	if err != nil {
		t.Fatalf("Insert() error = %v", err)
	}

	// Verify the original scene was not modified
	if scene.PrecisePoint == nil {
		t.Error("Insert() should not modify original scene's PrecisePoint")
	}

	// Retrieve and verify stored scene has nil precise point
	stored, err := repo.GetByID("scene-1")
	if err != nil {
		t.Fatalf("GetByID() error = %v", err)
	}
	if stored == nil {
		t.Fatal("GetByID() returned nil scene")
	}
	if stored.PrecisePoint != nil {
		t.Error("Insert() with AllowPrecise=false should set PrecisePoint to nil in storage")
	}
}

func TestInMemorySceneRepository_Insert_WithConsent(t *testing.T) {
	repo := NewInMemorySceneRepository()

	// Insert a scene with precise point and consent
	scene := &Scene{
		ID:           "scene-2",
		Name:         "Test Scene With Consent",
		AllowPrecise: true, // Has consent
		PrecisePoint: &Point{Lat: 40.7128, Lng: -74.0060},
	}

	err := repo.Insert(scene)
	if err != nil {
		t.Fatalf("Insert() error = %v", err)
	}

	// Retrieve and verify stored scene has precise point
	stored, err := repo.GetByID("scene-2")
	if err != nil {
		t.Fatalf("GetByID() error = %v", err)
	}
	if stored == nil {
		t.Fatal("GetByID() returned nil scene")
	}
	if stored.PrecisePoint == nil {
		t.Error("Insert() with AllowPrecise=true should preserve PrecisePoint")
	}
	if stored.PrecisePoint != nil && (stored.PrecisePoint.Lat != 40.7128 || stored.PrecisePoint.Lng != -74.0060) {
		t.Errorf("Insert() PrecisePoint = %+v, want Lat=40.7128, Lng=-74.0060", stored.PrecisePoint)
	}
}

func TestInMemorySceneRepository_Update_WithoutConsent(t *testing.T) {
	repo := NewInMemorySceneRepository()

	// First insert with consent
	scene := &Scene{
		ID:           "scene-3",
		Name:         "Test Scene",
		AllowPrecise: true,
		PrecisePoint: &Point{Lat: 40.7128, Lng: -74.0060},
	}
	_ = repo.Insert(scene)

	// Now update removing consent but keeping precise point in input
	updatedScene := &Scene{
		ID:           "scene-3",
		Name:         "Updated Scene",
		AllowPrecise: false, // Consent removed
		PrecisePoint: &Point{Lat: 40.7128, Lng: -74.0060}, // Still has point in input
	}

	err := repo.Update(updatedScene)
	if err != nil {
		t.Fatalf("Update() error = %v", err)
	}

	// Retrieve and verify stored scene has nil precise point
	stored, err := repo.GetByID("scene-3")
	if err != nil {
		t.Fatalf("GetByID() error = %v", err)
	}
	if stored == nil {
		t.Fatal("GetByID() returned nil scene")
	}
	if stored.PrecisePoint != nil {
		t.Error("Update() with AllowPrecise=false should set PrecisePoint to nil in storage")
	}
}

func TestInMemoryEventRepository_Insert_WithoutConsent(t *testing.T) {
	repo := NewInMemoryEventRepository()

	// Insert an event with precise point but without consent
	event := &Event{
		ID:           "event-1",
		SceneID:      "scene-1",
		Name:         "Test Event",
		AllowPrecise: false, // No consent
		PrecisePoint: &Point{Lat: 40.7128, Lng: -74.0060},
	}

	err := repo.Insert(event)
	if err != nil {
		t.Fatalf("Insert() error = %v", err)
	}

	// Verify the original event was not modified
	if event.PrecisePoint == nil {
		t.Error("Insert() should not modify original event's PrecisePoint")
	}

	// Retrieve and verify stored event has nil precise point
	stored, err := repo.GetByID("event-1")
	if err != nil {
		t.Fatalf("GetByID() error = %v", err)
	}
	if stored == nil {
		t.Fatal("GetByID() returned nil event")
	}
	if stored.PrecisePoint != nil {
		t.Error("Insert() with AllowPrecise=false should set PrecisePoint to nil in storage")
	}
}

func TestInMemoryEventRepository_Insert_WithConsent(t *testing.T) {
	repo := NewInMemoryEventRepository()

	// Insert an event with precise point and consent
	event := &Event{
		ID:           "event-2",
		SceneID:      "scene-1",
		Name:         "Test Event With Consent",
		AllowPrecise: true, // Has consent
		PrecisePoint: &Point{Lat: 40.7128, Lng: -74.0060},
	}

	err := repo.Insert(event)
	if err != nil {
		t.Fatalf("Insert() error = %v", err)
	}

	// Retrieve and verify stored event has precise point
	stored, err := repo.GetByID("event-2")
	if err != nil {
		t.Fatalf("GetByID() error = %v", err)
	}
	if stored == nil {
		t.Fatal("GetByID() returned nil event")
	}
	if stored.PrecisePoint == nil {
		t.Error("Insert() with AllowPrecise=true should preserve PrecisePoint")
	}
	if stored.PrecisePoint != nil && (stored.PrecisePoint.Lat != 40.7128 || stored.PrecisePoint.Lng != -74.0060) {
		t.Errorf("Insert() PrecisePoint = %+v, want Lat=40.7128, Lng=-74.0060", stored.PrecisePoint)
	}
}

func TestInMemoryEventRepository_Update_WithoutConsent(t *testing.T) {
	repo := NewInMemoryEventRepository()

	// First insert with consent
	event := &Event{
		ID:           "event-3",
		SceneID:      "scene-1",
		Name:         "Test Event",
		AllowPrecise: true,
		PrecisePoint: &Point{Lat: 40.7128, Lng: -74.0060},
	}
	_ = repo.Insert(event)

	// Now update removing consent but keeping precise point in input
	updatedEvent := &Event{
		ID:           "event-3",
		SceneID:      "scene-1",
		Name:         "Updated Event",
		AllowPrecise: false, // Consent removed
		PrecisePoint: &Point{Lat: 40.7128, Lng: -74.0060}, // Still has point in input
	}

	err := repo.Update(updatedEvent)
	if err != nil {
		t.Fatalf("Update() error = %v", err)
	}

	// Retrieve and verify stored event has nil precise point
	stored, err := repo.GetByID("event-3")
	if err != nil {
		t.Fatalf("GetByID() error = %v", err)
	}
	if stored == nil {
		t.Fatal("GetByID() returned nil event")
	}
	if stored.PrecisePoint != nil {
		t.Error("Update() with AllowPrecise=false should set PrecisePoint to nil in storage")
	}
}

func TestInMemorySceneRepository_GetByID_ReturnsNilForNonexistent(t *testing.T) {
	repo := NewInMemorySceneRepository()

	stored, err := repo.GetByID("nonexistent")
	if err != nil {
		t.Fatalf("GetByID() error = %v", err)
	}
	if stored != nil {
		t.Error("GetByID() should return nil for nonexistent scene")
	}
}

func TestInMemoryEventRepository_GetByID_ReturnsNilForNonexistent(t *testing.T) {
	repo := NewInMemoryEventRepository()

	stored, err := repo.GetByID("nonexistent")
	if err != nil {
		t.Fatalf("GetByID() error = %v", err)
	}
	if stored != nil {
		t.Error("GetByID() should return nil for nonexistent event")
	}
}

func TestInMemorySceneRepository_Insert_DeepCopyProtection(t *testing.T) {
	repo := NewInMemorySceneRepository()

	// Insert a scene with precise point and consent
	originalPoint := &Point{Lat: 40.7128, Lng: -74.0060}
	scene := &Scene{
		ID:           "scene-deep-copy",
		Name:         "Test Scene",
		AllowPrecise: true,
		PrecisePoint: originalPoint,
	}

	err := repo.Insert(scene)
	if err != nil {
		t.Fatalf("Insert() error = %v", err)
	}

	// Modify the original point after insertion
	originalPoint.Lat = 0.0
	originalPoint.Lng = 0.0

	// Retrieve and verify stored scene was NOT affected by the modification
	stored, err := repo.GetByID("scene-deep-copy")
	if err != nil {
		t.Fatalf("GetByID() error = %v", err)
	}
	if stored == nil {
		t.Fatal("GetByID() returned nil scene")
	}
	if stored.PrecisePoint == nil {
		t.Fatal("stored scene PrecisePoint should not be nil")
	}
	if stored.PrecisePoint.Lat != 40.7128 || stored.PrecisePoint.Lng != -74.0060 {
		t.Errorf("Insert() should create deep copy; stored PrecisePoint = %+v, want Lat=40.7128, Lng=-74.0060", stored.PrecisePoint)
	}
}

func TestInMemoryEventRepository_Insert_DeepCopyProtection(t *testing.T) {
	repo := NewInMemoryEventRepository()

	// Insert an event with precise point and consent
	originalPoint := &Point{Lat: 40.7128, Lng: -74.0060}
	event := &Event{
		ID:           "event-deep-copy",
		SceneID:      "scene-1",
		Name:         "Test Event",
		AllowPrecise: true,
		PrecisePoint: originalPoint,
	}

	err := repo.Insert(event)
	if err != nil {
		t.Fatalf("Insert() error = %v", err)
	}

	// Modify the original point after insertion
	originalPoint.Lat = 0.0
	originalPoint.Lng = 0.0

	// Retrieve and verify stored event was NOT affected by the modification
	stored, err := repo.GetByID("event-deep-copy")
	if err != nil {
		t.Fatalf("GetByID() error = %v", err)
	}
	if stored == nil {
		t.Fatal("GetByID() returned nil event")
	}
	if stored.PrecisePoint == nil {
		t.Fatal("stored event PrecisePoint should not be nil")
	}
	if stored.PrecisePoint.Lat != 40.7128 || stored.PrecisePoint.Lng != -74.0060 {
		t.Errorf("Insert() should create deep copy; stored PrecisePoint = %+v, want Lat=40.7128, Lng=-74.0060", stored.PrecisePoint)
	}
}
