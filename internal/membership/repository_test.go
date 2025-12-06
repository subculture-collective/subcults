package membership

import (
	"testing"
)

func strPtr(s string) *string {
	return &s
}

func TestMembershipRepository_Upsert_Insert(t *testing.T) {
	repo := NewInMemoryMembershipRepository()
	did := "did:plc:alice123"
	rkey := "membership456"

	membership := &Membership{
		SceneID:     "scene-1",
		UserDID:     "did:plc:user789",
		Role:        "member",
		Status:      "active",
		TrustWeight: 0.8,
		RecordDID:   strPtr(did),
		RecordRKey:  strPtr(rkey),
	}

	result, err := repo.Upsert(membership)
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

	if retrieved.Role != "member" {
		t.Errorf("Expected role 'member', got %s", retrieved.Role)
	}
}

func TestMembershipRepository_Upsert_Update(t *testing.T) {
	repo := NewInMemoryMembershipRepository()
	did := "did:plc:alice123"
	rkey := "membership456"

	// First insert
	membership := &Membership{
		SceneID:     "scene-1",
		UserDID:     "did:plc:user789",
		Role:        "member",
		Status:      "active",
		TrustWeight: 0.5,
		RecordDID:   strPtr(did),
		RecordRKey:  strPtr(rkey),
	}

	result1, err := repo.Upsert(membership)
	if err != nil {
		t.Fatalf("First upsert failed: %v", err)
	}

	if !result1.Inserted {
		t.Error("Expected insert on first upsert")
	}

	// Second upsert with same record key
	membership2 := &Membership{
		SceneID:     "scene-2",
		UserDID:     "did:plc:user789",
		Role:        "curator",
		Status:      "active",
		TrustWeight: 0.9,
		RecordDID:   strPtr(did),
		RecordRKey:  strPtr(rkey),
	}

	result2, err := repo.Upsert(membership2)
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

	if retrieved.Role != "curator" {
		t.Errorf("Expected updated role 'curator', got %s", retrieved.Role)
	}

	if retrieved.TrustWeight != 0.9 {
		t.Errorf("Expected updated trust weight 0.9, got %f", retrieved.TrustWeight)
	}
}

func TestMembershipRepository_Upsert_Idempotent(t *testing.T) {
	repo := NewInMemoryMembershipRepository()
	did := "did:plc:alice123"
	rkey := "membership456"

	membership := &Membership{
		SceneID:     "scene-1",
		UserDID:     "did:plc:user789",
		Role:        "member",
		Status:      "active",
		TrustWeight: 0.5,
		RecordDID:   strPtr(did),
		RecordRKey:  strPtr(rkey),
	}

	// First upsert
	result1, err := repo.Upsert(membership)
	if err != nil {
		t.Fatalf("First upsert failed: %v", err)
	}

	// Second upsert with same content
	result2, err := repo.Upsert(membership)
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

func TestMembershipRepository_Upsert_WithoutRecordKey(t *testing.T) {
	repo := NewInMemoryMembershipRepository()

	membership := &Membership{
		SceneID:     "scene-1",
		UserDID:     "did:plc:user789",
		Role:        "member",
		Status:      "active",
		TrustWeight: 0.5,
	}

	result, err := repo.Upsert(membership)
	if err != nil {
		t.Fatalf("Upsert failed: %v", err)
	}

	if !result.Inserted {
		t.Error("Expected insert when no record key provided")
	}

	// Second upsert without record key should also insert
	result2, err := repo.Upsert(membership)
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

func TestMembershipRepository_GetByRecordKey_NotFound(t *testing.T) {
	repo := NewInMemoryMembershipRepository()

	membership, err := repo.GetByRecordKey("did:plc:alice123", "nonexistent")
	if err != ErrMembershipNotFound {
		t.Errorf("Expected ErrMembershipNotFound, got %v", err)
	}

	if membership != nil {
		t.Error("Expected nil membership for non-existent record")
	}
}

func TestMembershipRepository_GetByID_AfterUpsert(t *testing.T) {
	repo := NewInMemoryMembershipRepository()
	did := "did:plc:alice123"
	rkey := "membership456"

	membership := &Membership{
		SceneID:     "scene-1",
		UserDID:     "did:plc:user789",
		Role:        "member",
		Status:      "active",
		TrustWeight: 0.5,
		RecordDID:   strPtr(did),
		RecordRKey:  strPtr(rkey),
	}

	result, err := repo.Upsert(membership)
	if err != nil {
		t.Fatalf("Upsert failed: %v", err)
	}

	// Retrieve by UUID
	retrieved, err := repo.GetByID(result.ID)
	if err != nil {
		t.Fatalf("GetByID failed: %v", err)
	}

	if retrieved.Role != "member" {
		t.Errorf("Expected role 'member', got %s", retrieved.Role)
	}
}
