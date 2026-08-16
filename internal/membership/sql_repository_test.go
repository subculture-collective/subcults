//go:build integration

package membership_test

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/onnwee/subcults/internal/membership"
	"github.com/onnwee/subcults/internal/testutil"
)

func insertMembershipScene(t *testing.T, tdb *testutil.TestDB) string {
	t.Helper()
	sceneID := uuid.New().String()
	tdb.DB.Exec(`DELETE FROM memberships`)
	_, err := tdb.DB.Exec(`INSERT INTO scenes(id,owner_did,name,description,coarse_geohash,allow_precise) VALUES($1,$2,$3,$4,$5,FALSE)`,
		sceneID, "did:plc:owner", "Test Mem Scene", "Scene for membership tests", "u4pruydqqvj")
	if err != nil {
		t.Fatalf("insert scene: %v", err)
	}
	return sceneID
}

func TestSQLMembership_UpsertInsert(t *testing.T) {
	tdb := testutil.NewTestDB(t)
	repo := membership.NewSQLMembershipRepository(tdb.DB)
	sceneID := insertMembershipScene(t, tdb)

	m := &membership.Membership{
		SceneID:     sceneID,
		UserDID:     "did:plc:member",
		Role:        "member",
		Status:      "active",
		TrustWeight: 0.5,
		RecordDID:   strPtr("did:plc:record"),
		RecordRKey:  strPtr("mem-key-1"),
	}
	result, err := repo.Upsert(m)
	if err != nil {
		t.Fatalf("Upsert: %v", err)
	}
	if !result.Inserted {
		t.Error("expected Inserted=true")
	}

	got, err := repo.GetByID(m.ID)
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if got.Role != "member" {
		t.Errorf("Role = %q", got.Role)
	}
}

func TestSQLMembership_UpsertUpdate(t *testing.T) {
	tdb := testutil.NewTestDB(t)
	repo := membership.NewSQLMembershipRepository(tdb.DB)
	sceneID := insertMembershipScene(t, tdb)

	m1 := &membership.Membership{
		SceneID:     sceneID,
		UserDID:     "did:plc:member",
		Role:        "member",
		Status:      "active",
		TrustWeight: 0.5,
		RecordDID:   strPtr("did:plc:record"),
		RecordRKey:  strPtr("mem-key-2"),
	}
	repo.Upsert(m1)

	m2 := &membership.Membership{
		SceneID:     sceneID,
		UserDID:     "did:plc:member",
		Role:        "curator",
		Status:      "active",
		TrustWeight: 0.8,
		RecordDID:   strPtr("did:plc:record"),
		RecordRKey:  strPtr("mem-key-2"),
	}
	result, err := repo.Upsert(m2)
	if err != nil {
		t.Fatalf("second Upsert: %v", err)
	}
	if result.Inserted {
		t.Error("expected Inserted=false on update")
	}

	got, _ := repo.GetByID(m2.ID)
	if got.Role != "curator" {
		t.Errorf("Role = %q, want 'curator'", got.Role)
	}
	if got.TrustWeight != 0.8 {
		t.Errorf("TrustWeight = %f, want 0.8", got.TrustWeight)
	}
}

func TestSQLMembership_GetBySceneAndUser(t *testing.T) {
	tdb := testutil.NewTestDB(t)
	repo := membership.NewSQLMembershipRepository(tdb.DB)
	sceneID := insertMembershipScene(t, tdb)

	repo.Upsert(&membership.Membership{
		SceneID: sceneID, UserDID: "did:plc:user1", Role: "member", Status: "active", TrustWeight: 0.5,
	})

	got, err := repo.GetBySceneAndUser(sceneID, "did:plc:user1")
	if err != nil {
		t.Fatalf("GetBySceneAndUser: %v", err)
	}
	if got.UserDID != "did:plc:user1" {
		t.Errorf("UserDID = %q", got.UserDID)
	}

	_, err = repo.GetBySceneAndUser(sceneID, "did:plc:nonexistent")
	if err != membership.ErrMembershipNotFound {
		t.Errorf("expected ErrMembershipNotFound, got %v", err)
	}
}

func TestSQLMembership_UpdateStatus(t *testing.T) {
	tdb := testutil.NewTestDB(t)
	repo := membership.NewSQLMembershipRepository(tdb.DB)
	sceneID := insertMembershipScene(t, tdb)

	m := &membership.Membership{
		SceneID: sceneID, UserDID: "did:plc:user", Role: "member", Status: "pending", TrustWeight: 0.5,
	}
	repo.Upsert(m)

	if err := repo.UpdateStatus(m.ID, "active", nil); err != nil {
		t.Fatalf("UpdateStatus: %v", err)
	}
	got, _ := repo.GetByID(m.ID)
	if got.Status != "active" {
		t.Errorf("Status = %q, want 'active'", got.Status)
	}

	now := time.Now()
	if err := repo.UpdateStatus(m.ID, "rejected", &now); err != nil {
		t.Fatalf("UpdateStatus with since: %v", err)
	}
	got, _ = repo.GetByID(m.ID)
	if got.Status != "rejected" {
		t.Errorf("Status = %q", got.Status)
	}
}

func TestSQLMembership_UpdateRole(t *testing.T) {
	tdb := testutil.NewTestDB(t)
	repo := membership.NewSQLMembershipRepository(tdb.DB)
	sceneID := insertMembershipScene(t, tdb)

	m := &membership.Membership{
		SceneID: sceneID, UserDID: "did:plc:user", Role: "member", Status: "active", TrustWeight: 0.5,
	}
	repo.Upsert(m)

	if err := repo.UpdateRole(m.ID, "admin"); err != nil {
		t.Fatalf("UpdateRole: %v", err)
	}
	got, _ := repo.GetByID(m.ID)
	if got.Role != "admin" {
		t.Errorf("Role = %q, want 'admin'", got.Role)
	}
}

func TestSQLMembership_ListByScene(t *testing.T) {
	tdb := testutil.NewTestDB(t)
	repo := membership.NewSQLMembershipRepository(tdb.DB)
	sceneID := insertMembershipScene(t, tdb)

	repo.Upsert(&membership.Membership{SceneID: sceneID, UserDID: "did:plc:u1", Role: "member", Status: "active", TrustWeight: 0.5})
	repo.Upsert(&membership.Membership{SceneID: sceneID, UserDID: "did:plc:u2", Role: "curator", Status: "pending", TrustWeight: 0.8})

	all, err := repo.ListByScene(sceneID, "")
	if err != nil {
		t.Fatalf("ListByScene all: %v", err)
	}
	if len(all) != 2 {
		t.Errorf("expected 2 memberships, got %d", len(all))
	}

	active, err := repo.ListByScene(sceneID, "active")
	if err != nil {
		t.Fatalf("ListByScene active: %v", err)
	}
	if len(active) != 1 {
		t.Errorf("expected 1 active, got %d", len(active))
	}
}

func TestSQLMembership_CountByScenes(t *testing.T) {
	tdb := testutil.NewTestDB(t)
	repo := membership.NewSQLMembershipRepository(tdb.DB)
	sceneID := insertMembershipScene(t, tdb)

	sceneID2 := uuid.New().String()
	tdb.DB.Exec(`DELETE FROM memberships`)
	tdb.DB.Exec(`INSERT INTO scenes(id,owner_did,name,description,coarse_geohash,allow_precise) VALUES($1,$2,$3,$4,$5,FALSE)`,
		sceneID2, "did:plc:owner2", "Scene 2", "desc", "u4pruydqqvj")

	repo.Upsert(&membership.Membership{SceneID: sceneID, UserDID: "did:plc:u1", Role: "member", Status: "active", TrustWeight: 0.5})
	repo.Upsert(&membership.Membership{SceneID: sceneID, UserDID: "did:plc:u2", Role: "curator", Status: "pending", TrustWeight: 0.8})
	repo.Upsert(&membership.Membership{SceneID: sceneID2, UserDID: "did:plc:u3", Role: "member", Status: "active", TrustWeight: 0.5})

	counts, err := repo.CountByScenes([]string{sceneID, sceneID2}, "active")
	if err != nil {
		t.Fatalf("CountByScenes: %v", err)
	}
	if counts[sceneID] != 1 {
		t.Errorf("scene 1 active count = %d, want 1", counts[sceneID])
	}
	if counts[sceneID2] != 1 {
		t.Errorf("scene 2 active count = %d, want 1", counts[sceneID2])
	}
}

func strPtr(s string) *string { return &s }
