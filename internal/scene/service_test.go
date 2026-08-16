package scene

import (
	"context"
	"strings"
	"testing"
	"time"
)

// ---------------------------------------------------------------------------
// Scene Service Tests
// ---------------------------------------------------------------------------

func newTestService() (*Service, *InMemorySceneRepository, *InMemoryEventRepository, *InMemoryRSVPRepository) {
	sceneRepo := NewInMemorySceneRepository()
	eventRepo := NewInMemoryEventRepository()
	rsvpRepo := NewInMemoryRSVPRepository()
	svc := NewService(sceneRepo, eventRepo, rsvpRepo)
	return svc, sceneRepo, eventRepo, rsvpRepo
}

func TestServiceCreateScene_Success(t *testing.T) {
	svc, _, _, _ := newTestService()
	ctx := context.Background()

	scene, err := svc.CreateScene(ctx,
		"Test Scene", "A test description",
		"did:plc:test123", "dr5regw",
		[]string{"test", "example"},
		"public",
		&Palette{Primary: "#ff0000", Secondary: "#00ff00"},
		true,
		&Point{Lat: 40.7128, Lng: -74.0060},
		"published",
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if scene.Name != "Test Scene" {
		t.Errorf("expected name 'Test Scene', got %s", scene.Name)
	}
	if scene.OwnerDID != "did:plc:test123" {
		t.Errorf("expected ownerDID 'did:plc:test123', got %s", scene.OwnerDID)
	}
	if scene.Visibility != "public" {
		t.Errorf("expected visibility 'public', got %s", scene.Visibility)
	}
	if scene.PrecisePoint == nil {
		t.Error("expected precise_point to be set")
	}
	if scene.CreatedAt == nil {
		t.Error("expected created_at to be set")
	}
}

func TestServiceCreateScene_DefaultVisibility(t *testing.T) {
	svc, _, _, _ := newTestService()
	ctx := context.Background()

	scene, err := svc.CreateScene(ctx,
		"Test Scene", "",
		"did:plc:test123", "dr5regw",
		nil, "", nil, false, nil, "published",
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if scene.Visibility != "public" {
		t.Errorf("expected default visibility 'public', got %s", scene.Visibility)
	}
}

func TestServiceCreateScene_PrivacyEnforcement(t *testing.T) {
	svc, _, _, _ := newTestService()
	ctx := context.Background()

	scene, err := svc.CreateScene(ctx,
		"Private Scene", "",
		"did:plc:test123", "dr5regw",
		nil, "", nil,
		false,
		&Point{Lat: 40.7128, Lng: -74.0060},
		"published",
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if scene.PrecisePoint != nil {
		t.Error("expected precise_point to be nil when allow_precise=false")
	}
}

func TestServiceCreateScene_DuplicateName(t *testing.T) {
	svc, _, _, _ := newTestService()
	ctx := context.Background()

	_, err := svc.CreateScene(ctx,
		"Duplicate Scene", "",
		"did:plc:test123", "dr5regw",
		nil, "", nil, false, nil, "published",
	)
	if err != nil {
		t.Fatalf("first creation failed: %v", err)
	}

	_, err = svc.CreateScene(ctx,
		"Duplicate Scene", "",
		"did:plc:test123", "dr5regw",
		nil, "", nil, false, nil, "published",
	)
	if err != ErrDuplicateSceneName {
		t.Errorf("expected ErrDuplicateSceneName, got %v", err)
	}
}

func TestServiceCreateScene_InvalidName(t *testing.T) {
	svc, _, _, _ := newTestService()
	ctx := context.Background()

	tests := []string{
		"ab",                                                                          // too short
		strings.Repeat("a", 65),                                                       // too long
		"Scene<script>alert('xss')</script>",                                          // invalid chars
	}

	for _, name := range tests {
		_, err := svc.CreateScene(ctx,
			name, "",
			"did:plc:test123", "dr5regw",
			nil, "", nil, false, nil, "published",
		)
		if !strings.Contains(err.Error(), ErrInvalidSceneName.Error()) {
			t.Errorf("name %q: expected ErrInvalidSceneName, got %v", name, err)
		}
	}
}

func TestServiceCreateScene_MissingFields(t *testing.T) {
	svc, _, _, _ := newTestService()
	ctx := context.Background()

	// Missing owner_did
	_, err := svc.CreateScene(ctx,
		"Test Scene", "",
		"", "dr5regw",
		nil, "", nil, false, nil, "published",
	)
	if err != ErrEmptyOwnerDID {
		t.Errorf("expected ErrEmptyOwnerDID, got %v", err)
	}

	// Missing coarse_geohash
	_, err = svc.CreateScene(ctx,
		"Test Scene", "",
		"did:plc:test123", "",
		nil, "", nil, false, nil, "published",
	)
	if err != ErrEmptyCoarseGeohash {
		t.Errorf("expected ErrEmptyCoarseGeohash, got %v", err)
	}
}

func TestServiceCreateScene_InvalidVisibility(t *testing.T) {
	svc, _, _, _ := newTestService()
	ctx := context.Background()

	_, err := svc.CreateScene(ctx,
		"Test Scene", "",
		"did:plc:test123", "dr5regw",
		nil, "invalid", nil, false, nil, "published",
	)
	if err != ErrInvalidVisibility {
		t.Errorf("expected ErrInvalidVisibility, got %v", err)
	}
}

func TestServiceUpdateScene_Success(t *testing.T) {
	svc, sceneRepo, _, _ := newTestService()
	ctx := context.Background()

	// Create a scene first
	now := time.Now()
	original := &Scene{
		ID:            "test-scene-id",
		Name:          "Original Name",
		Description:   "Original description",
		OwnerDID:      "did:plc:test123",
		CoarseGeohash: "dr5regw",
		Visibility:    "public",
		CreatedAt:     &now,
		UpdatedAt:     &now,
	}
	if err := sceneRepo.Insert(original); err != nil {
		t.Fatalf("failed to insert scene: %v", err)
	}

	newName := "Updated Name"
	newDesc := "Updated description"
	newVis := "unlisted"
	updated, err := svc.UpdateScene(ctx, "test-scene-id", UpdateSceneParams{
		Name:        &newName,
		Description: &newDesc,
		Visibility:  &newVis,
		Tags:        []string{"updated", "tags"},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if updated.Name != "Updated Name" {
		t.Errorf("expected name 'Updated Name', got %s", updated.Name)
	}
	if updated.Description != "Updated description" {
		t.Errorf("expected description 'Updated description', got %s", updated.Description)
	}
	if updated.Visibility != "unlisted" {
		t.Errorf("expected visibility 'unlisted', got %s", updated.Visibility)
	}
}

func TestServiceUpdateScene_NotFound(t *testing.T) {
	svc, _, _, _ := newTestService()
	ctx := context.Background()

	newName := "New Name"
	_, err := svc.UpdateScene(ctx, "nonexistent-id", UpdateSceneParams{
		Name: &newName,
	})
	if err != ErrSceneNotFound {
		t.Errorf("expected ErrSceneNotFound, got %v", err)
	}
}

func TestServiceUpdateScene_DuplicateName(t *testing.T) {
	svc, sceneRepo, _, _ := newTestService()
	ctx := context.Background()

	now := time.Now()
	scene1 := &Scene{ID: "scene-1", Name: "Scene One", OwnerDID: "did:plc:test123", CoarseGeohash: "dr5regw", CreatedAt: &now, UpdatedAt: &now}
	scene2 := &Scene{ID: "scene-2", Name: "Scene Two", OwnerDID: "did:plc:test123", CoarseGeohash: "dr5regw", CreatedAt: &now, UpdatedAt: &now}
	sceneRepo.Insert(scene1)
	sceneRepo.Insert(scene2)

	newName := "Scene One"
	_, err := svc.UpdateScene(ctx, "scene-2", UpdateSceneParams{
		Name: &newName,
	})
	if err != ErrDuplicateSceneName {
		t.Errorf("expected ErrDuplicateSceneName, got %v", err)
	}
}

func TestServiceGetScene(t *testing.T) {
	svc, sceneRepo, _, _ := newTestService()
	ctx := context.Background()

	now := time.Now()
	scene := &Scene{ID: "test-id", Name: "Test", OwnerDID: "did:plc:test123", CoarseGeohash: "dr5regw", CreatedAt: &now, UpdatedAt: &now}
	sceneRepo.Insert(scene)

	got, err := svc.GetScene(ctx, "test-id")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.ID != "test-id" {
		t.Errorf("expected ID 'test-id', got %s", got.ID)
	}
}

func TestServiceDeleteScene(t *testing.T) {
	svc, sceneRepo, _, _ := newTestService()
	ctx := context.Background()

	now := time.Now()
	scene := &Scene{ID: "test-id", Name: "Test", OwnerDID: "did:plc:test123", CoarseGeohash: "dr5regw", CreatedAt: &now, UpdatedAt: &now}
	sceneRepo.Insert(scene)

	if err := svc.DeleteScene(ctx, "test-id"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify deleted
	_, err := sceneRepo.GetByID("test-id")
	if err != ErrSceneDeleted {
		t.Errorf("expected ErrSceneDeleted, got %v", err)
	}
}

// ---------------------------------------------------------------------------
// Event Service Tests
// ---------------------------------------------------------------------------

func TestServiceCreateEvent_Success(t *testing.T) {
	svc, sceneRepo, _, _ := newTestService()
	ctx := context.Background()

	// Create a scene first
	now := time.Now()
	sc := &Scene{ID: "scene-1", Name: "Test Scene", OwnerDID: "did:plc:test123", CoarseGeohash: "dr5regw", CreatedAt: &now, UpdatedAt: &now}
	sceneRepo.Insert(sc)

	startsAt := time.Now().Add(24 * time.Hour)
	endsAt := startsAt.Add(2 * time.Hour)

	event, err := svc.CreateEvent(ctx,
		"scene-1", "Test Event", "A test event", "dr5regw",
		true, &Point{Lat: 40.7128, Lng: -74.0060},
		[]string{"test", "example"},
		startsAt, &endsAt,
		"", nil, nil, "",
		"published",
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if event.Title != "Test Event" {
		t.Errorf("expected title 'Test Event', got %s", event.Title)
	}
	if event.SceneID != "scene-1" {
		t.Errorf("expected scene_id 'scene-1', got %s", event.SceneID)
	}
	if event.Status != "scheduled" {
		t.Errorf("expected status 'scheduled', got %s", event.Status)
	}
}

func TestServiceCreateEvent_InvalidTimeRange(t *testing.T) {
	svc, sceneRepo, _, _ := newTestService()
	ctx := context.Background()

	now := time.Now()
	sc := &Scene{ID: "scene-1", Name: "Test", OwnerDID: "did:plc:test123", CoarseGeohash: "dr5regw", CreatedAt: &now, UpdatedAt: &now}
	sceneRepo.Insert(sc)

	endsAt := time.Now()
	_, err := svc.CreateEvent(ctx,
		"scene-1", "Test Event", "", "dr5regw",
		false, nil, nil,
		time.Now().Add(24*time.Hour), &endsAt, // end before start
		"", nil, nil, "",
		"published",
	)
	if err != ErrInvalidTimeRange {
		t.Errorf("expected ErrInvalidTimeRange, got %v", err)
	}
}

// ---------------------------------------------------------------------------
// RSVP Service Tests
// ---------------------------------------------------------------------------

func TestServiceCreateOrUpdateRSVP_Success(t *testing.T) {
	svc, _, eventRepo, rsvpRepo := newTestService()
	ctx := context.Background()

	// Create an upcoming event
	now := time.Now()
	event := &Event{
		ID:            "event-1",
		SceneID:       "scene-1",
		Title:         "Test Event",
		CoarseGeohash: "dr5regw",
		StartsAt:      now.Add(24 * time.Hour),
		CreatedAt:     &now,
		UpdatedAt:     &now,
	}
	eventRepo.Insert(event)

	err := svc.CreateOrUpdateRSVP(ctx, "did:plc:user1", "event-1", "going")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	rsvp, err := rsvpRepo.GetByEventAndUser("event-1", "did:plc:user1")
	if err != nil {
		t.Fatalf("failed to get RSVP: %v", err)
	}
	if rsvp.Status != "going" {
		t.Errorf("expected status 'going', got %s", rsvp.Status)
	}
}

func TestServiceCreateOrUpdateRSVP_InvalidStatus(t *testing.T) {
	svc, _, eventRepo, _ := newTestService()
	ctx := context.Background()

	now := time.Now()
	event := &Event{
		ID:            "event-1",
		SceneID:       "scene-1",
		Title:         "Test Event",
		CoarseGeohash: "dr5regw",
		StartsAt:      now.Add(24 * time.Hour),
		CreatedAt:     &now,
		UpdatedAt:     &now,
	}
	eventRepo.Insert(event)

	err := svc.CreateOrUpdateRSVP(ctx, "did:plc:user1", "event-1", "invalid")
	if !strings.Contains(err.Error(), ErrInvalidRSVPStatus.Error()) {
		t.Errorf("expected ErrInvalidRSVPStatus, got %v", err)
	}
}

func TestServiceCreateOrUpdateRSVP_PastEvent(t *testing.T) {
	svc, _, eventRepo, _ := newTestService()
	ctx := context.Background()

	now := time.Now()
	event := &Event{
		ID:            "event-1",
		SceneID:       "scene-1",
		Title:         "Past Event",
		CoarseGeohash: "dr5regw",
		StartsAt:      now.Add(-24 * time.Hour),
		CreatedAt:     &now,
		UpdatedAt:     &now,
	}
	eventRepo.Insert(event)

	err := svc.CreateOrUpdateRSVP(ctx, "did:plc:user1", "event-1", "going")
	if err != ErrEventNotUpcoming {
		t.Errorf("expected ErrEventNotUpcoming, got %v", err)
	}
}

func TestServiceCreateOrUpdateRSVP_EventNotFound(t *testing.T) {
	svc, _, _, _ := newTestService()
	ctx := context.Background()

	err := svc.CreateOrUpdateRSVP(ctx, "did:plc:user1", "nonexistent", "going")
	if err != ErrEventNotFound {
		t.Errorf("expected ErrEventNotFound, got %v", err)
	}
}