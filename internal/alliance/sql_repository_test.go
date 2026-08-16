//go:build integration

package alliance_test

import (
	"testing"

	"github.com/google/uuid"
	"github.com/onnwee/subcults/internal/alliance"
	"github.com/onnwee/subcults/internal/testutil"
)

func insertAllianceScenes(t *testing.T, tdb *testutil.TestDB) (string, string) {
	t.Helper()
	fromID := uuid.New().String()
	toID := uuid.New().String()
	tdb.DB.Exec(`DELETE FROM alliances`)
	for _, id := range []string{fromID, toID} {
		_, err := tdb.DB.Exec(`INSERT INTO scenes(id,owner_did,name,description,coarse_geohash,allow_precise) VALUES($1,$2,$3,$4,$5,FALSE)`,
			id, "did:plc:owner", id, "desc", "u4pruydqqvj")
		if err != nil {
			t.Fatalf("insert scene %s: %v", id, err)
		}
	}
	return fromID, toID
}

func TestSQLAlliance_UpsertInsert(t *testing.T) {
	tdb := testutil.NewTestDB(t)
	repo := alliance.NewSQLAllianceRepository(tdb.DB)
	from, to := insertAllianceScenes(t, tdb)

	a := &alliance.Alliance{
		FromSceneID: from,
		ToSceneID:   to,
		Weight:      0.7,
		Status:      "active",
		RecordDID:   strPtr("did:plc:record"),
		RecordRKey:  strPtr("alliance-key-1"),
	}
	result, err := repo.Upsert(a)
	if err != nil {
		t.Fatalf("Upsert: %v", err)
	}
	if !result.Inserted {
		t.Error("expected Inserted=true")
	}

	got, err := repo.GetByID(a.ID)
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if got.FromSceneID != from {
		t.Errorf("FromSceneID = %q", got.FromSceneID)
	}
	if got.Weight != 0.7 {
		t.Errorf("Weight = %f, want 0.7", got.Weight)
	}
}

func TestSQLAlliance_UpsertUpdate(t *testing.T) {
	tdb := testutil.NewTestDB(t)
	repo := alliance.NewSQLAllianceRepository(tdb.DB)
	from, to := insertAllianceScenes(t, tdb)

	a1 := &alliance.Alliance{
		FromSceneID: from, ToSceneID: to, Weight: 0.5, Status: "active",
		RecordDID: strPtr("did:plc:record"), RecordRKey: strPtr("alliance-key-2"),
	}
	repo.Upsert(a1)

	a2 := &alliance.Alliance{
		FromSceneID: from, ToSceneID: to, Weight: 0.9, Status: "active",
		RecordDID: strPtr("did:plc:record"), RecordRKey: strPtr("alliance-key-2"),
	}
	result, err := repo.Upsert(a2)
	if err != nil {
		t.Fatalf("second Upsert: %v", err)
	}
	if result.Inserted {
		t.Error("expected Inserted=false on update")
	}

	got, _ := repo.GetByID(a2.ID)
	if got.Weight != 0.9 {
		t.Errorf("Weight = %f, want 0.9", got.Weight)
	}
}

func TestSQLAlliance_Insert(t *testing.T) {
	tdb := testutil.NewTestDB(t)
	repo := alliance.NewSQLAllianceRepository(tdb.DB)
	from, to := insertAllianceScenes(t, tdb)

	a := &alliance.Alliance{
		FromSceneID: from, ToSceneID: to, Weight: 0.6, Status: "active",
	}
	if err := repo.Insert(a); err != nil {
		t.Fatalf("Insert: %v", err)
	}
	if a.ID == "" {
		t.Error("expected ID to be set")
	}
}

func TestSQLAlliance_Update(t *testing.T) {
	tdb := testutil.NewTestDB(t)
	repo := alliance.NewSQLAllianceRepository(tdb.DB)
	from, to := insertAllianceScenes(t, tdb)

	a := &alliance.Alliance{FromSceneID: from, ToSceneID: to, Weight: 0.6, Status: "active"}
	repo.Insert(a)

	a.Weight = 0.3
	a.Status = "dissolved"
	if err := repo.Update(a); err != nil {
		t.Fatalf("Update: %v", err)
	}

	got, _ := repo.GetByID(a.ID)
	if got.Weight != 0.3 {
		t.Errorf("Weight = %f", got.Weight)
	}
	if got.Status != "dissolved" {
		t.Errorf("Status = %q", got.Status)
	}
}

func TestSQLAlliance_Delete(t *testing.T) {
	tdb := testutil.NewTestDB(t)
	repo := alliance.NewSQLAllianceRepository(tdb.DB)
	from, to := insertAllianceScenes(t, tdb)

	a := &alliance.Alliance{FromSceneID: from, ToSceneID: to, Weight: 0.5, Status: "active"}
	repo.Insert(a)

	if err := repo.Delete(a.ID); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	_, err := repo.GetByID(a.ID)
	if err != alliance.ErrAllianceDeleted {
		t.Errorf("expected ErrAllianceDeleted, got %v", err)
	}
	if err := repo.Delete(a.ID); err != alliance.ErrAllianceDeleted {
		t.Errorf("expected ErrAllianceDeleted on second delete, got %v", err)
	}
}

func TestSQLAlliance_GetByIDNotFound(t *testing.T) {
	tdb := testutil.NewTestDB(t)
	repo := alliance.NewSQLAllianceRepository(tdb.DB)
	_, err := repo.GetByID("00000000-0000-0000-0000-000000000000")
	if err != alliance.ErrAllianceNotFound {
		t.Errorf("expected ErrAllianceNotFound, got %v", err)
	}
}

func TestSQLAlliance_GetByRecordKey(t *testing.T) {
	tdb := testutil.NewTestDB(t)
	repo := alliance.NewSQLAllianceRepository(tdb.DB)
	from, to := insertAllianceScenes(t, tdb)

	a := &alliance.Alliance{
		FromSceneID: from, ToSceneID: to, Weight: 0.8, Status: "active",
		RecordDID: strPtr("did:plc:record"), RecordRKey: strPtr("alliance-key-3"),
	}
	repo.Upsert(a)

	got, err := repo.GetByRecordKey("did:plc:record", "alliance-key-3")
	if err != nil {
		t.Fatalf("GetByRecordKey: %v", err)
	}
	if got.Weight != 0.8 {
		t.Errorf("Weight = %f", got.Weight)
	}
}

func TestSQLAlliance_UpdateNotFound(t *testing.T) {
	tdb := testutil.NewTestDB(t)
	repo := alliance.NewSQLAllianceRepository(tdb.DB)

	a := &alliance.Alliance{ID: "00000000-0000-0000-0000-000000000000", Weight: 0.5, Status: "active"}
	err := repo.Update(a)
	if err != alliance.ErrAllianceNotFound {
		t.Errorf("expected ErrAllianceNotFound, got %v", err)
	}
}

func strPtr(s string) *string { return &s }
