//go:build integration

package stream_test

import (
	"testing"

	"github.com/google/uuid"
	"github.com/onnwee/subcults/internal/stream"
	"github.com/onnwee/subcults/internal/testutil"
)

func insertStreamScene(t *testing.T, tdb *testutil.TestDB) string {
	t.Helper()
	sceneID := uuid.New().String()
	tdb.DB.Exec(`DELETE FROM stream_analytics`)
	tdb.DB.Exec(`DELETE FROM stream_participant_events`)
	tdb.DB.Exec(`DELETE FROM stream_participants`)
	tdb.DB.Exec(`DELETE FROM stream_sessions`)
	_, err := tdb.DB.Exec(`INSERT INTO scenes(id,owner_did,name,description,coarse_geohash,allow_precise) VALUES($1,$2,$3,$4,$5,FALSE)`,
		sceneID, "did:plc:streamer", "Test Stream Scene", "Scene for stream tests", "u4pruydqqvj")
	if err != nil {
		t.Fatalf("insert scene: %v", err)
	}
	return sceneID
}

// ─────────────────────────────────────
// SQLSessionRepository tests
// ─────────────────────────────────────

func TestSQLSession_CreateAndGet(t *testing.T) {
	tdb := testutil.NewTestDB(t)
	repo := stream.NewSQLSessionRepository(tdb.DB)
	sceneID := insertStreamScene(t, tdb)

	id, room, err := repo.CreateStreamSession(&sceneID, nil, "did:plc:host")
	if err != nil {
		t.Fatalf("CreateStreamSession: %v", err)
	}
	if id == "" {
		t.Error("expected non-empty id")
	}
	if room == "" {
		t.Error("expected non-empty room name")
	}

	s, err := repo.GetByID(id)
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if s.HostDID != "did:plc:host" {
		t.Errorf("HostDID = %q, want %q", s.HostDID, "did:plc:host")
	}
}

func TestSQLSession_GetByIDNotFound(t *testing.T) {
	tdb := testutil.NewTestDB(t)
	repo := stream.NewSQLSessionRepository(tdb.DB)
	_, err := repo.GetByID("00000000-0000-0000-0000-000000000000")
	if err != stream.ErrStreamNotFound {
		t.Errorf("expected ErrStreamNotFound, got %v", err)
	}
}

func TestSQLSession_Upsert(t *testing.T) {
	tdb := testutil.NewTestDB(t)
	repo := stream.NewSQLSessionRepository(tdb.DB)
	sceneID := insertStreamScene(t, tdb)

	s := &stream.Session{
		SceneID:    &sceneID,
		RoomName:   "upsert-room",
		HostDID:    "did:plc:upsert",
		RecordDID:  strPtr("did:plc:upsert-did"),
		RecordRKey: strPtr("test-upsert-key"),
	}
	result, err := repo.Upsert(s)
	if err != nil {
		t.Fatalf("first Upsert: %v", err)
	}
	if !result.Inserted {
		t.Error("expected Inserted=true on first upsert")
	}
	originalID := s.ID

	s2 := &stream.Session{
		RoomName:   "upsert-room-updated",
		HostDID:    "did:plc:upsert2",
		RecordDID:  strPtr("did:plc:upsert-did"),
		RecordRKey: strPtr("test-upsert-key"),
	}
	result2, err := repo.Upsert(s2)
	if err != nil {
		t.Fatalf("second Upsert: %v", err)
	}
	if result2.Inserted {
		t.Error("expected Inserted=false on second upsert")
	}
	if s2.ID != originalID {
		t.Errorf("expected same ID, got %s != %s", s2.ID, originalID)
	}

	got, err := repo.GetByID(s2.ID)
	if err != nil {
		t.Fatalf("GetByID after upsert: %v", err)
	}
	if got.RoomName != "upsert-room-updated" {
		t.Errorf("RoomName = %q", got.RoomName)
	}
}

func TestSQLSession_EndStreamSession(t *testing.T) {
	tdb := testutil.NewTestDB(t)
	repo := stream.NewSQLSessionRepository(tdb.DB)
	sceneID := insertStreamScene(t, tdb)

	id, _, err := repo.CreateStreamSession(&sceneID, nil, "did:plc:host")
	if err != nil {
		t.Fatalf("CreateStreamSession: %v", err)
	}

	if err := repo.EndStreamSession(id); err != nil {
		t.Fatalf("EndStreamSession: %v", err)
	}
	// Idempotent: second call should not error
	if err := repo.EndStreamSession(id); err != nil {
		t.Fatalf("EndStreamSession idempotent: %v", err)
	}
	// Unknown stream
	if err := repo.EndStreamSession("00000000-0000-0000-0000-000000000000"); err != stream.ErrStreamNotFound {
		t.Errorf("expected ErrStreamNotFound, got %v", err)
	}
}

func TestSQLSession_JoinAndLeave(t *testing.T) {
	tdb := testutil.NewTestDB(t)
	repo := stream.NewSQLSessionRepository(tdb.DB)
	sceneID := insertStreamScene(t, tdb)

	id, _, err := repo.CreateStreamSession(&sceneID, nil, "did:plc:host")
	if err != nil {
		t.Fatalf("CreateStreamSession: %v", err)
	}

	if err := repo.RecordJoin(id); err != nil {
		t.Fatalf("RecordJoin: %v", err)
	}
	if err := repo.RecordLeave(id); err != nil {
		t.Fatalf("RecordLeave: %v", err)
	}
	s, err := repo.GetByID(id)
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if s.JoinCount != 1 {
		t.Errorf("JoinCount = %d, want 1", s.JoinCount)
	}
	if s.LeaveCount != 1 {
		t.Errorf("LeaveCount = %d, want 1", s.LeaveCount)
	}
}

func TestSQLSession_LockAndFeature(t *testing.T) {
	tdb := testutil.NewTestDB(t)
	repo := stream.NewSQLSessionRepository(tdb.DB)
	sceneID := insertStreamScene(t, tdb)

	id, _, _ := repo.CreateStreamSession(&sceneID, nil, "did:plc:host")

	if err := repo.SetLockStatus(id, true); err != nil {
		t.Fatalf("SetLockStatus: %v", err)
	}
	feat := "participant-featured-1"
	if err := repo.SetFeaturedParticipant(id, &feat); err != nil {
		t.Fatalf("SetFeaturedParticipant: %v", err)
	}
	s, _ := repo.GetByID(id)
	if !s.IsLocked {
		t.Error("expected IsLocked=true")
	}
	if s.FeaturedParticipant == nil || *s.FeaturedParticipant != feat {
		t.Errorf("FeaturedParticipant = %v", s.FeaturedParticipant)
	}
}

func TestSQLSession_HasActiveStream(t *testing.T) {
	tdb := testutil.NewTestDB(t)
	repo := stream.NewSQLSessionRepository(tdb.DB)
	sceneID := insertStreamScene(t, tdb)

	has, err := repo.HasActiveStreamForScene(sceneID)
	if err != nil {
		t.Fatalf("HasActiveStreamForScene: %v", err)
	}
	if has {
		t.Error("expected no active stream for fresh scene")
	}

	repo.CreateStreamSession(&sceneID, nil, "did:plc:host")
	has, err = repo.HasActiveStreamForScene(sceneID)
	if err != nil {
		t.Fatalf("HasActiveStreamForScene after create: %v", err)
	}
	if !has {
		t.Error("expected active stream after creation")
	}
}

// ─────────────────────────────────────
// SQLParticipantRepository tests
// ─────────────────────────────────────

func TestSQLParticipant_JoinAndLeave(t *testing.T) {
	tdb := testutil.NewTestDB(t)
	sessionRepo := stream.NewSQLSessionRepository(tdb.DB)
	partRepo := stream.NewSQLParticipantRepository(tdb.DB)
	sceneID := insertStreamScene(t, tdb)

	sid, _, _ := sessionRepo.CreateStreamSession(&sceneID, nil, "did:plc:host")

	p, reconn, err := partRepo.RecordJoin(sid, "livekit-p1", "did:plc:user1")
	if err != nil {
		t.Fatalf("RecordJoin: %v", err)
	}
	if reconn {
		t.Error("expected reconn=false for first join")
	}
	if p.LeftAt != nil {
		t.Error("expected LeftAt=nil")
	}

	active, err := partRepo.GetActiveParticipants(sid)
	if err != nil {
		t.Fatalf("GetActiveParticipants: %v", err)
	}
	if len(active) != 1 {
		t.Fatalf("expected 1 active, got %d", len(active))
	}

	if err := partRepo.RecordLeave(sid, "livekit-p1"); err != nil {
		t.Fatalf("RecordLeave: %v", err)
	}

	active, _ = partRepo.GetActiveParticipants(sid)
	if len(active) != 0 {
		t.Errorf("expected 0 active after leave, got %d", len(active))
	}
}

func TestSQLParticipant_DuplicateActive(t *testing.T) {
	tdb := testutil.NewTestDB(t)
	sessionRepo := stream.NewSQLSessionRepository(tdb.DB)
	partRepo := stream.NewSQLParticipantRepository(tdb.DB)
	sceneID := insertStreamScene(t, tdb)

	sid, _, _ := sessionRepo.CreateStreamSession(&sceneID, nil, "did:plc:host")
	partRepo.RecordJoin(sid, "livekit-p2", "did:plc:user2")
	_, _, err := partRepo.RecordJoin(sid, "livekit-p2", "did:plc:user2")
	if err != stream.ErrParticipantAlreadyActive {
		t.Errorf("expected ErrParticipantAlreadyActive, got %v", err)
	}
}

func TestSQLParticipant_History(t *testing.T) {
	tdb := testutil.NewTestDB(t)
	sessionRepo := stream.NewSQLSessionRepository(tdb.DB)
	partRepo := stream.NewSQLParticipantRepository(tdb.DB)
	sceneID := insertStreamScene(t, tdb)

	sid, _, _ := sessionRepo.CreateStreamSession(&sceneID, nil, "did:plc:host")
	partRepo.RecordJoin(sid, "livekit-p3", "did:plc:user3")
	partRepo.RecordLeave(sid, "livekit-p3")
	partRepo.RecordJoin(sid, "livekit-p3", "did:plc:user3")

	history, err := partRepo.GetParticipantHistory(sid)
	if err != nil {
		t.Fatalf("GetParticipantHistory: %v", err)
	}
	if len(history) != 2 {
		t.Fatalf("expected 2 history entries, got %d", len(history))
	}
}

// ─────────────────────────────────────
// SQLAnalyticsRepository tests
// ─────────────────────────────────────

func TestSQLAnalytics_RecordAndGetEvents(t *testing.T) {
	tdb := testutil.NewTestDB(t)
	sessionRepo := stream.NewSQLSessionRepository(tdb.DB)
	analyticsRepo := stream.NewSQLAnalyticsRepository(tdb.DB)
	sceneID := insertStreamScene(t, tdb)

	sid, _, _ := sessionRepo.CreateStreamSession(&sceneID, nil, "did:plc:host")

	geo := "u4pr"
	if err := analyticsRepo.RecordParticipantEvent(sid, "did:plc:user1", "join", &geo); err != nil {
		t.Fatalf("RecordParticipantEvent join: %v", err)
	}
	if err := analyticsRepo.RecordParticipantEvent(sid, "did:plc:user1", "leave", nil); err != nil {
		t.Fatalf("RecordParticipantEvent leave: %v", err)
	}

	events, err := analyticsRepo.GetParticipantEvents(sid)
	if err != nil {
		t.Fatalf("GetParticipantEvents: %v", err)
	}
	if len(events) != 2 {
		t.Fatalf("expected 2 events, got %d", len(events))
	}
}

func TestSQLAnalytics_ComputeAndGet(t *testing.T) {
	tdb := testutil.NewTestDB(t)
	sessionRepo := stream.NewSQLSessionRepository(tdb.DB)
	analyticsRepo := stream.NewSQLAnalyticsRepository(tdb.DB)
	sceneID := insertStreamScene(t, tdb)

	sid, _, _ := sessionRepo.CreateStreamSession(&sceneID, nil, "did:plc:host")
	sessionRepo.EndStreamSession(sid)

	geo := "u4pr"
	analyticsRepo.RecordParticipantEvent(sid, "did:plc:user1", "join", &geo)
	analyticsRepo.RecordParticipantEvent(sid, "did:plc:user1", "leave", nil)

	a, err := analyticsRepo.ComputeAnalytics(sid)
	if err != nil {
		t.Fatalf("ComputeAnalytics: %v", err)
	}
	if a.TotalUniqueParticipants != 1 {
		t.Errorf("TotalUniqueParticipants = %d, want 1", a.TotalUniqueParticipants)
	}
	if a.StreamDurationSeconds < 0 {
		t.Errorf("StreamDurationSeconds = %d", a.StreamDurationSeconds)
	}

	got, err := analyticsRepo.GetAnalytics(sid)
	if err != nil {
		t.Fatalf("GetAnalytics: %v", err)
	}
	if got.ID != a.ID {
		t.Errorf("ID mismatch")
	}
}

func TestSQLAnalytics_GetNotFound(t *testing.T) {
	tdb := testutil.NewTestDB(t)
	repo := stream.NewSQLAnalyticsRepository(tdb.DB)
	_, err := repo.GetAnalytics("00000000-0000-0000-0000-000000000000")
	if err != stream.ErrAnalyticsNotFound {
		t.Errorf("expected ErrAnalyticsNotFound, got %v", err)
	}
}

func TestSQLAnalytics_InvalidEventType(t *testing.T) {
	tdb := testutil.NewTestDB(t)
	sessionRepo := stream.NewSQLSessionRepository(tdb.DB)
	analyticsRepo := stream.NewSQLAnalyticsRepository(tdb.DB)
	sceneID := insertStreamScene(t, tdb)

	sid, _, _ := sessionRepo.CreateStreamSession(&sceneID, nil, "did:plc:host")
	err := analyticsRepo.RecordParticipantEvent(sid, "did:plc:user1", "invalid", nil)
	if err == nil {
		t.Error("expected error for invalid event type")
	}
}

// ─────────────────────────────────────
// Helpers
// ─────────────────────────────────────

func strPtr(s string) *string { return &s }
