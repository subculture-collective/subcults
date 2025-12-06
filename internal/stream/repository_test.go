package stream

import (
	"testing"
)

func strPtr(s string) *string {
	return &s
}

func TestSessionRepository_Upsert_Insert(t *testing.T) {
	repo := NewInMemorySessionRepository()
	did := "did:plc:alice123"
	rkey := "stream456"
	sceneID := "scene-1"

	session := &Session{
		SceneID:          &sceneID,
		RoomName:         "test-room",
		HostDID:          "did:plc:host789",
		ParticipantCount: 5,
		RecordDID:        strPtr(did),
		RecordRKey:       strPtr(rkey),
	}

	result, err := repo.Upsert(session)
	if err != nil {
		t.Fatalf("Upsert failed: %v", err)
	}

	if !result.Inserted {
		t.Error("Expected insert, got update")
	}

	if result.ID == "" {
		t.Error("Expected non-empty ID")
	}

	// Verify we can retrieve it
	retrieved, err := repo.GetByRecordKey(did, rkey)
	if err != nil {
		t.Fatalf("GetByRecordKey failed: %v", err)
	}

	if retrieved.RoomName != "test-room" {
		t.Errorf("Expected room name 'test-room', got %s", retrieved.RoomName)
	}
}

func TestSessionRepository_Upsert_Update(t *testing.T) {
	repo := NewInMemorySessionRepository()
	did := "did:plc:alice123"
	rkey := "stream456"
	sceneID := "scene-1"

	// First insert
	session := &Session{
		SceneID:          &sceneID,
		RoomName:         "test-room",
		HostDID:          "did:plc:host789",
		ParticipantCount: 5,
		RecordDID:        strPtr(did),
		RecordRKey:       strPtr(rkey),
	}

	result1, err := repo.Upsert(session)
	if err != nil {
		t.Fatalf("First upsert failed: %v", err)
	}

	if !result1.Inserted {
		t.Error("Expected insert on first upsert")
	}

	// Second upsert with same record key
	session2 := &Session{
		SceneID:          &sceneID,
		RoomName:         "updated-room",
		HostDID:          "did:plc:host789",
		ParticipantCount: 10,
		RecordDID:        strPtr(did),
		RecordRKey:       strPtr(rkey),
	}

	result2, err := repo.Upsert(session2)
	if err != nil {
		t.Fatalf("Second upsert failed: %v", err)
	}

	if result2.Inserted {
		t.Error("Expected update, got insert")
	}

	if result1.ID != result2.ID {
		t.Errorf("Expected same ID, got %s and %s", result1.ID, result2.ID)
	}

	// Verify update was persisted
	retrieved, err := repo.GetByRecordKey(did, rkey)
	if err != nil {
		t.Fatalf("GetByRecordKey failed: %v", err)
	}

	if retrieved.RoomName != "updated-room" {
		t.Errorf("Expected updated room name 'updated-room', got %s", retrieved.RoomName)
	}

	if retrieved.ParticipantCount != 10 {
		t.Errorf("Expected participant count 10, got %d", retrieved.ParticipantCount)
	}
}

func TestSessionRepository_Upsert_Idempotent(t *testing.T) {
	repo := NewInMemorySessionRepository()
	did := "did:plc:alice123"
	rkey := "stream456"
	sceneID := "scene-1"

	session := &Session{
		SceneID:          &sceneID,
		RoomName:         "test-room",
		HostDID:          "did:plc:host789",
		ParticipantCount: 5,
		RecordDID:        strPtr(did),
		RecordRKey:       strPtr(rkey),
	}

	// First upsert
	result1, err := repo.Upsert(session)
	if err != nil {
		t.Fatalf("First upsert failed: %v", err)
	}

	// Second upsert with same content
	result2, err := repo.Upsert(session)
	if err != nil {
		t.Fatalf("Second upsert failed: %v", err)
	}

	// Should update, not insert
	if result2.Inserted {
		t.Error("Expected update (idempotent), got insert")
	}

	if result1.ID != result2.ID {
		t.Error("Idempotent upserts should return same ID")
	}
}

func TestSessionRepository_Upsert_WithoutRecordKey(t *testing.T) {
	repo := NewInMemorySessionRepository()
	sceneID := "scene-1"

	session := &Session{
		SceneID:          &sceneID,
		RoomName:         "test-room",
		HostDID:          "did:plc:host789",
		ParticipantCount: 5,
	}

	result, err := repo.Upsert(session)
	if err != nil {
		t.Fatalf("Upsert failed: %v", err)
	}

	if !result.Inserted {
		t.Error("Expected insert when no record key provided")
	}

	// Second upsert without record key should also insert
	result2, err := repo.Upsert(session)
	if err != nil {
		t.Fatalf("Second upsert failed: %v", err)
	}

	if !result2.Inserted {
		t.Error("Expected insert when no record key provided")
	}

	// IDs should be different
	if result.ID == result2.ID {
		t.Error("Expected different IDs for separate inserts")
	}
}

func TestSessionRepository_GetByRecordKey_NotFound(t *testing.T) {
	repo := NewInMemorySessionRepository()

	session, err := repo.GetByRecordKey("did:plc:alice123", "nonexistent")
	if err != ErrStreamNotFound {
		t.Errorf("Expected ErrStreamNotFound, got %v", err)
	}

	if session != nil {
		t.Error("Expected nil session for non-existent record")
	}
}

func TestSessionRepository_GetByID_AfterUpsert(t *testing.T) {
	repo := NewInMemorySessionRepository()
	did := "did:plc:alice123"
	rkey := "stream456"
	sceneID := "scene-1"

	session := &Session{
		SceneID:          &sceneID,
		RoomName:         "test-room",
		HostDID:          "did:plc:host789",
		ParticipantCount: 5,
		RecordDID:        strPtr(did),
		RecordRKey:       strPtr(rkey),
	}

	result, err := repo.Upsert(session)
	if err != nil {
		t.Fatalf("Upsert failed: %v", err)
	}

	// Retrieve by UUID
	retrieved, err := repo.GetByID(result.ID)
	if err != nil {
		t.Fatalf("GetByID failed: %v", err)
	}

	if retrieved.RoomName != "test-room" {
		t.Errorf("Expected room name 'test-room', got %s", retrieved.RoomName)
	}
}
