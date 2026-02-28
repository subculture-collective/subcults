package scene

import (
	"testing"
	"time"
)

// --- SceneRepository: Delete ---

func TestSceneRepository_Delete_Success(t *testing.T) {
	repo := NewInMemorySceneRepository()

	scene := &Scene{
		ID:            "scene-del-1",
		Name:          "Delete Me",
		OwnerDID:      "did:plc:owner1",
		CoarseGeohash: "dr5regw",
		AllowPrecise:  false,
	}
	if err := repo.Insert(scene); err != nil {
		t.Fatalf("Insert failed: %v", err)
	}

	err := repo.Delete("scene-del-1")
	if err != nil {
		t.Fatalf("Delete failed: %v", err)
	}

	// GetByID should return ErrSceneDeleted for soft-deleted scene
	_, err = repo.GetByID("scene-del-1")
	if err != ErrSceneDeleted {
		t.Errorf("Expected ErrSceneDeleted after delete, got %v", err)
	}
}

func TestSceneRepository_Delete_NotFound(t *testing.T) {
	repo := NewInMemorySceneRepository()

	err := repo.Delete("nonexistent-id")
	if err != ErrSceneNotFound {
		t.Errorf("Expected ErrSceneNotFound, got %v", err)
	}
}

func TestSceneRepository_Delete_AlreadyDeleted(t *testing.T) {
	repo := NewInMemorySceneRepository()

	scene := &Scene{
		ID:            "scene-del-2",
		Name:          "Double Delete",
		OwnerDID:      "did:plc:owner1",
		CoarseGeohash: "dr5regw",
		AllowPrecise:  false,
	}
	if err := repo.Insert(scene); err != nil {
		t.Fatalf("Insert failed: %v", err)
	}

	// First delete
	if err := repo.Delete("scene-del-2"); err != nil {
		t.Fatalf("First delete failed: %v", err)
	}

	// Second delete should return ErrSceneDeleted
	err := repo.Delete("scene-del-2")
	if err != ErrSceneDeleted {
		t.Errorf("Expected ErrSceneDeleted on second delete, got %v", err)
	}
}

// --- SceneRepository: GetByRecordKey ---

func TestSceneRepository_GetByRecordKey_Success(t *testing.T) {
	repo := NewInMemorySceneRepository()

	did := "did:plc:alice123"
	rkey := "scene456"
	scene := &Scene{
		ID:            "scene-rk-1",
		Name:          "Record Key Scene",
		OwnerDID:      "did:plc:owner1",
		CoarseGeohash: "dr5regw",
		AllowPrecise:  true,
		PrecisePoint:  &Point{Lat: 40.7128, Lng: -74.006},
		RecordDID:     &did,
		RecordRKey:    &rkey,
	}

	// Must upsert to populate the keys map
	_, err := repo.Upsert(scene)
	if err != nil {
		t.Fatalf("Upsert failed: %v", err)
	}

	retrieved, err := repo.GetByRecordKey(did, rkey)
	if err != nil {
		t.Fatalf("GetByRecordKey failed: %v", err)
	}

	if retrieved.Name != "Record Key Scene" {
		t.Errorf("Expected name 'Record Key Scene', got %s", retrieved.Name)
	}
	if retrieved.PrecisePoint == nil {
		t.Error("Expected PrecisePoint to be set (consent is true)")
	}
}

func TestSceneRepository_GetByRecordKey_NotFound(t *testing.T) {
	repo := NewInMemorySceneRepository()

	_, err := repo.GetByRecordKey("did:plc:nobody", "no-rkey")
	if err != ErrSceneNotFound {
		t.Errorf("Expected ErrSceneNotFound, got %v", err)
	}
}

func TestSceneRepository_GetByRecordKey_DeepCopy(t *testing.T) {
	repo := NewInMemorySceneRepository()

	did := "did:plc:alice123"
	rkey := "scene789"
	scene := &Scene{
		ID:            "scene-rk-dc",
		Name:          "Deep Copy Scene",
		OwnerDID:      "did:plc:owner1",
		CoarseGeohash: "dr5regw",
		AllowPrecise:  true,
		PrecisePoint:  &Point{Lat: 40.7128, Lng: -74.006},
		RecordDID:     &did,
		RecordRKey:    &rkey,
	}

	if _, err := repo.Upsert(scene); err != nil {
		t.Fatalf("Upsert failed: %v", err)
	}

	retrieved, _ := repo.GetByRecordKey(did, rkey)

	// Mutate the returned copy
	retrieved.Name = "Mutated"
	retrieved.PrecisePoint.Lat = 0.0

	// Verify the repo's copy is unchanged
	original, _ := repo.GetByRecordKey(did, rkey)
	if original.Name != "Deep Copy Scene" {
		t.Error("Deep copy violated: mutation of returned scene affected repository")
	}
	if original.PrecisePoint.Lat != 40.7128 {
		t.Error("Deep copy violated: mutation of PrecisePoint affected repository")
	}
}

// --- SceneRepository: ExistsByOwnerAndName ---

func TestSceneRepository_ExistsByOwnerAndName_Exists(t *testing.T) {
	repo := NewInMemorySceneRepository()

	scene := &Scene{
		ID:            "scene-eon-1",
		Name:          "My Scene",
		OwnerDID:      "did:plc:owner1",
		CoarseGeohash: "dr5regw",
		AllowPrecise:  false,
	}
	if err := repo.Insert(scene); err != nil {
		t.Fatalf("Insert failed: %v", err)
	}

	exists, err := repo.ExistsByOwnerAndName("did:plc:owner1", "My Scene", "")
	if err != nil {
		t.Fatalf("ExistsByOwnerAndName failed: %v", err)
	}
	if !exists {
		t.Error("Expected scene to exist for owner")
	}
}

func TestSceneRepository_ExistsByOwnerAndName_CaseInsensitive(t *testing.T) {
	repo := NewInMemorySceneRepository()

	scene := &Scene{
		ID:            "scene-eon-ci",
		Name:          "My Scene",
		OwnerDID:      "did:plc:owner1",
		CoarseGeohash: "dr5regw",
		AllowPrecise:  false,
	}
	if err := repo.Insert(scene); err != nil {
		t.Fatalf("Insert failed: %v", err)
	}

	exists, err := repo.ExistsByOwnerAndName("did:plc:owner1", "MY SCENE", "")
	if err != nil {
		t.Fatalf("ExistsByOwnerAndName failed: %v", err)
	}
	if !exists {
		t.Error("Expected case-insensitive match")
	}

	exists, err = repo.ExistsByOwnerAndName("did:plc:owner1", "my scene", "")
	if err != nil {
		t.Fatalf("ExistsByOwnerAndName failed: %v", err)
	}
	if !exists {
		t.Error("Expected case-insensitive match (lowercase)")
	}
}

func TestSceneRepository_ExistsByOwnerAndName_DifferentOwner(t *testing.T) {
	repo := NewInMemorySceneRepository()

	scene := &Scene{
		ID:            "scene-eon-do",
		Name:          "Unique Scene",
		OwnerDID:      "did:plc:owner1",
		CoarseGeohash: "dr5regw",
		AllowPrecise:  false,
	}
	if err := repo.Insert(scene); err != nil {
		t.Fatalf("Insert failed: %v", err)
	}

	exists, err := repo.ExistsByOwnerAndName("did:plc:owner2", "Unique Scene", "")
	if err != nil {
		t.Fatalf("ExistsByOwnerAndName failed: %v", err)
	}
	if exists {
		t.Error("Expected scene to NOT exist for different owner")
	}
}

func TestSceneRepository_ExistsByOwnerAndName_ExcludeID(t *testing.T) {
	repo := NewInMemorySceneRepository()

	scene := &Scene{
		ID:            "scene-eon-ex",
		Name:          "Exclude Me",
		OwnerDID:      "did:plc:owner1",
		CoarseGeohash: "dr5regw",
		AllowPrecise:  false,
	}
	if err := repo.Insert(scene); err != nil {
		t.Fatalf("Insert failed: %v", err)
	}

	// Excluding its own ID should return false (useful for updates)
	exists, err := repo.ExistsByOwnerAndName("did:plc:owner1", "Exclude Me", "scene-eon-ex")
	if err != nil {
		t.Fatalf("ExistsByOwnerAndName failed: %v", err)
	}
	if exists {
		t.Error("Expected false when excludeID matches the scene")
	}
}

func TestSceneRepository_ExistsByOwnerAndName_IgnoresDeleted(t *testing.T) {
	repo := NewInMemorySceneRepository()

	scene := &Scene{
		ID:            "scene-eon-del",
		Name:          "Deleted Scene",
		OwnerDID:      "did:plc:owner1",
		CoarseGeohash: "dr5regw",
		AllowPrecise:  false,
	}
	if err := repo.Insert(scene); err != nil {
		t.Fatalf("Insert failed: %v", err)
	}

	// Delete the scene
	if err := repo.Delete("scene-eon-del"); err != nil {
		t.Fatalf("Delete failed: %v", err)
	}

	// Should return false because deleted scenes are excluded
	exists, err := repo.ExistsByOwnerAndName("did:plc:owner1", "Deleted Scene", "")
	if err != nil {
		t.Fatalf("ExistsByOwnerAndName failed: %v", err)
	}
	if exists {
		t.Error("Expected false for deleted scene")
	}
}

// --- EventRepository: Cancel ---

func TestEventRepository_Cancel_Success(t *testing.T) {
	repo := NewInMemoryEventRepository()

	event := &Event{
		ID:            "event-cancel-1",
		SceneID:       "scene-1",
		Title:         "Cancel This",
		CoarseGeohash: "dr5regw",
		Status:        "scheduled",
		StartsAt:      time.Now().Add(24 * time.Hour),
	}
	if err := repo.Insert(event); err != nil {
		t.Fatalf("Insert failed: %v", err)
	}

	reason := "weather conditions"
	err := repo.Cancel("event-cancel-1", &reason)
	if err != nil {
		t.Fatalf("Cancel failed: %v", err)
	}

	retrieved, err := repo.GetByID("event-cancel-1")
	if err != nil {
		t.Fatalf("GetByID failed: %v", err)
	}

	if retrieved.Status != "cancelled" {
		t.Errorf("Expected status 'cancelled', got %s", retrieved.Status)
	}
	if retrieved.CancelledAt == nil {
		t.Error("Expected CancelledAt to be set")
	}
	if retrieved.CancellationReason == nil || *retrieved.CancellationReason != "weather conditions" {
		t.Errorf("Expected cancellation reason 'weather conditions', got %v", retrieved.CancellationReason)
	}
}

func TestEventRepository_Cancel_NilReason(t *testing.T) {
	repo := NewInMemoryEventRepository()

	event := &Event{
		ID:            "event-cancel-nr",
		SceneID:       "scene-1",
		Title:         "Cancel No Reason",
		CoarseGeohash: "dr5regw",
		Status:        "scheduled",
		StartsAt:      time.Now().Add(24 * time.Hour),
	}
	if err := repo.Insert(event); err != nil {
		t.Fatalf("Insert failed: %v", err)
	}

	err := repo.Cancel("event-cancel-nr", nil)
	if err != nil {
		t.Fatalf("Cancel failed: %v", err)
	}

	retrieved, _ := repo.GetByID("event-cancel-nr")
	if retrieved.Status != "cancelled" {
		t.Errorf("Expected status 'cancelled', got %s", retrieved.Status)
	}
	if retrieved.CancellationReason != nil {
		t.Error("Expected nil cancellation reason")
	}
}

func TestEventRepository_Cancel_NotFound(t *testing.T) {
	repo := NewInMemoryEventRepository()

	err := repo.Cancel("nonexistent-event", nil)
	if err != ErrEventNotFound {
		t.Errorf("Expected ErrEventNotFound, got %v", err)
	}
}

func TestEventRepository_Cancel_Idempotent(t *testing.T) {
	repo := NewInMemoryEventRepository()

	event := &Event{
		ID:            "event-cancel-idem",
		SceneID:       "scene-1",
		Title:         "Idempotent Cancel",
		CoarseGeohash: "dr5regw",
		Status:        "scheduled",
		StartsAt:      time.Now().Add(24 * time.Hour),
	}
	if err := repo.Insert(event); err != nil {
		t.Fatalf("Insert failed: %v", err)
	}

	reason := "first cancel"
	if err := repo.Cancel("event-cancel-idem", &reason); err != nil {
		t.Fatalf("First cancel failed: %v", err)
	}

	retrieved1, _ := repo.GetByID("event-cancel-idem")
	cancelledAt1 := retrieved1.CancelledAt

	// Second cancel should be idempotent
	reason2 := "second cancel"
	if err := repo.Cancel("event-cancel-idem", &reason2); err != nil {
		t.Fatalf("Second cancel failed: %v", err)
	}

	retrieved2, _ := repo.GetByID("event-cancel-idem")
	if !retrieved2.CancelledAt.Equal(*cancelledAt1) {
		t.Error("Expected CancelledAt to remain unchanged on idempotent call")
	}
}

// --- EventRepository: GetByRecordKey ---

func TestEventRepository_GetByRecordKey_Success(t *testing.T) {
	repo := NewInMemoryEventRepository()

	did := "did:plc:alice123"
	rkey := "event456"
	event := &Event{
		ID:            "event-rk-1",
		SceneID:       "scene-1",
		Title:         "Record Key Event",
		CoarseGeohash: "dr5regw",
		AllowPrecise:  true,
		PrecisePoint:  &Point{Lat: 40.7128, Lng: -74.006},
		Status:        "scheduled",
		StartsAt:      time.Now().Add(24 * time.Hour),
		RecordDID:     &did,
		RecordRKey:    &rkey,
	}

	// Must upsert to populate the keys map
	if _, err := repo.Upsert(event); err != nil {
		t.Fatalf("Upsert failed: %v", err)
	}

	retrieved, err := repo.GetByRecordKey(did, rkey)
	if err != nil {
		t.Fatalf("GetByRecordKey failed: %v", err)
	}

	if retrieved.Title != "Record Key Event" {
		t.Errorf("Expected title 'Record Key Event', got %s", retrieved.Title)
	}
	if retrieved.PrecisePoint == nil {
		t.Error("Expected PrecisePoint to be set (consent is true)")
	}
}

func TestEventRepository_GetByRecordKey_NotFound(t *testing.T) {
	repo := NewInMemoryEventRepository()

	_, err := repo.GetByRecordKey("did:plc:nobody", "no-rkey")
	if err != ErrEventNotFound {
		t.Errorf("Expected ErrEventNotFound, got %v", err)
	}
}

func TestEventRepository_GetByRecordKey_DeepCopy(t *testing.T) {
	repo := NewInMemoryEventRepository()

	did := "did:plc:alice123"
	rkey := "event789"
	event := &Event{
		ID:            "event-rk-dc",
		SceneID:       "scene-1",
		Title:         "Deep Copy Event",
		CoarseGeohash: "dr5regw",
		AllowPrecise:  true,
		PrecisePoint:  &Point{Lat: 40.7128, Lng: -74.006},
		Status:        "scheduled",
		StartsAt:      time.Now().Add(24 * time.Hour),
		RecordDID:     &did,
		RecordRKey:    &rkey,
	}

	if _, err := repo.Upsert(event); err != nil {
		t.Fatalf("Upsert failed: %v", err)
	}

	retrieved, _ := repo.GetByRecordKey(did, rkey)

	// Mutate the returned copy
	retrieved.Title = "Mutated"
	retrieved.PrecisePoint.Lat = 0.0

	// Verify the repo's copy is unchanged
	original, _ := repo.GetByRecordKey(did, rkey)
	if original.Title != "Deep Copy Event" {
		t.Error("Deep copy violated: mutation of returned event affected repository")
	}
	if original.PrecisePoint.Lat != 40.7128 {
		t.Error("Deep copy violated: mutation of PrecisePoint affected repository")
	}
}
