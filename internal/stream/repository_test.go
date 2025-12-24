package stream

import (
	"strings"
	"testing"
	"time"
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

func TestSessionRepository_HasActiveStreamForScene(t *testing.T) {
repo := NewInMemorySessionRepository()
sceneID1 := "scene-1"
sceneID2 := "scene-2"

// Insert active stream for scene-1
session1 := &Session{
ID:               "stream-1",
SceneID:          &sceneID1,
RoomName:         "room-1",
HostDID:          "did:plc:host1",
ParticipantCount: 5,
EndedAt:          nil, // Active stream
}
if _, err := repo.Upsert(session1); err != nil {
t.Fatalf("Upsert session1 failed: %v", err)
}

// Insert ended stream for scene-1
endTime := timePtr()
session2 := &Session{
ID:               "stream-2",
SceneID:          &sceneID1,
RoomName:         "room-2",
HostDID:          "did:plc:host2",
ParticipantCount: 3,
EndedAt:          endTime, // Ended stream
}
if _, err := repo.Upsert(session2); err != nil {
t.Fatalf("Upsert session2 failed: %v", err)
}

// Test: scene-1 should have active stream
hasActive, err := repo.HasActiveStreamForScene(sceneID1)
if err != nil {
t.Fatalf("HasActiveStreamForScene failed: %v", err)
}
if !hasActive {
t.Error("Expected scene-1 to have active stream")
}

// Test: scene-2 should not have active stream
hasActive, err = repo.HasActiveStreamForScene(sceneID2)
if err != nil {
t.Fatalf("HasActiveStreamForScene failed: %v", err)
}
if hasActive {
t.Error("Expected scene-2 to not have active stream")
}
}

func TestSessionRepository_HasActiveStreamForScene_AllEnded(t *testing.T) {
repo := NewInMemorySessionRepository()
sceneID := "scene-1"
endTime := timePtr()

// Insert only ended streams
session1 := &Session{
ID:               "stream-1",
SceneID:          &sceneID,
RoomName:         "room-1",
HostDID:          "did:plc:host1",
ParticipantCount: 5,
EndedAt:          endTime,
}
if _, err := repo.Upsert(session1); err != nil {
t.Fatalf("Upsert failed: %v", err)
}

// Test: should not have active stream
hasActive, err := repo.HasActiveStreamForScene(sceneID)
if err != nil {
t.Fatalf("HasActiveStreamForScene failed: %v", err)
}
if hasActive {
t.Error("Expected no active stream when all streams are ended")
}
}

func timePtr() *time.Time {
t := time.Now()
return &t
}

func TestSessionRepository_HasActiveStreamsForScenes(t *testing.T) {
repo := NewInMemorySessionRepository()

scene1 := "scene-1"
scene2 := "scene-2"
scene3 := "scene-3"

// Scene 1: has active stream
session1 := &Session{
ID:               "stream-1",
SceneID:          &scene1,
RoomName:         "room-1",
HostDID:          "did:plc:host1",
ParticipantCount: 5,
EndedAt:          nil, // Active
}

// Scene 2: has ended stream only
endTime := timePtr()
session2 := &Session{
ID:               "stream-2",
SceneID:          &scene2,
RoomName:         "room-2",
HostDID:          "did:plc:host2",
ParticipantCount: 3,
EndedAt:          endTime, // Ended
}

// Scene 3: no streams

if _, err := repo.Upsert(session1); err != nil {
t.Fatalf("Upsert session1 failed: %v", err)
}
if _, err := repo.Upsert(session2); err != nil {
t.Fatalf("Upsert session2 failed: %v", err)
}

// Test: Check active streams for all scenes
activeStreams, err := repo.HasActiveStreamsForScenes([]string{scene1, scene2, scene3})
if err != nil {
t.Fatalf("HasActiveStreamsForScenes failed: %v", err)
}

if !activeStreams[scene1] {
t.Error("Expected scene1 to have active stream")
}
if activeStreams[scene2] {
t.Error("Expected scene2 to not have active stream (ended)")
}
if activeStreams[scene3] {
t.Error("Expected scene3 to not have active stream (no streams)")
}
}

func TestSessionRepository_HasActiveStreamsForScenes_EmptyInput(t *testing.T) {
repo := NewInMemorySessionRepository()

activeStreams, err := repo.HasActiveStreamsForScenes([]string{})
if err != nil {
t.Fatalf("HasActiveStreamsForScenes failed: %v", err)
}

if len(activeStreams) != 0 {
t.Errorf("Expected empty map, got %d entries", len(activeStreams))
}
}

func TestSessionRepository_CreateStreamSession_WithSceneID(t *testing.T) {
repo := NewInMemorySessionRepository()
sceneID := "scene-123"
hostDID := "did:plc:host456"

id, roomName, err := repo.CreateStreamSession(&sceneID, nil, hostDID)
if err != nil {
t.Fatalf("CreateStreamSession failed: %v", err)
}

if id == "" {
t.Error("Expected non-empty session ID")
}

// Verify room name format: scene-{sceneId}-{timestamp}
if !strings.Contains(roomName, "scene-scene-123-") {
t.Errorf("Expected room name to contain 'scene-scene-123-', got %s", roomName)
}

// Verify session was created
session, err := repo.GetByID(id)
if err != nil {
t.Fatalf("GetByID failed: %v", err)
}

if session.SceneID == nil || *session.SceneID != sceneID {
t.Errorf("Expected scene_id %s, got %v", sceneID, session.SceneID)
}

if session.HostDID != hostDID {
t.Errorf("Expected host_did %s, got %s", hostDID, session.HostDID)
}

if session.EndedAt != nil {
t.Error("Expected ended_at to be nil (active stream)")
}

if session.RoomName != roomName {
t.Errorf("Expected room_name %s, got %s", roomName, session.RoomName)
}
}

func TestSessionRepository_CreateStreamSession_WithEventID(t *testing.T) {
repo := NewInMemorySessionRepository()
eventID := "event-789"
hostDID := "did:plc:host456"

id, roomName, err := repo.CreateStreamSession(nil, &eventID, hostDID)
if err != nil {
t.Fatalf("CreateStreamSession failed: %v", err)
}

if id == "" {
t.Error("Expected non-empty session ID")
}

// Verify room name format: event-{eventId}-{timestamp}
if !strings.Contains(roomName, "event-event-789-") {
t.Errorf("Expected room name to contain 'event-event-789-', got %s", roomName)
}

// Verify session was created
session, err := repo.GetByID(id)
if err != nil {
t.Fatalf("GetByID failed: %v", err)
}

if session.EventID == nil || *session.EventID != eventID {
t.Errorf("Expected event_id %s, got %v", eventID, session.EventID)
}
}

func TestSessionRepository_CreateStreamSession_NoSceneOrEvent(t *testing.T) {
repo := NewInMemorySessionRepository()
hostDID := "did:plc:host456"

_, _, err := repo.CreateStreamSession(nil, nil, hostDID)
if err == nil {
t.Error("Expected error when neither scene_id nor event_id provided")
}

// Test with empty strings
emptyScene := ""
emptyEvent := ""
_, _, err = repo.CreateStreamSession(&emptyScene, &emptyEvent, hostDID)
if err == nil {
t.Error("Expected error when both scene_id and event_id are empty")
}
}

func TestSessionRepository_EndStreamSession_Success(t *testing.T) {
repo := NewInMemorySessionRepository()
sceneID := "scene-123"
hostDID := "did:plc:host456"

// Create a session
id, _, err := repo.CreateStreamSession(&sceneID, nil, hostDID)
if err != nil {
t.Fatalf("CreateStreamSession failed: %v", err)
}

// Verify it's active
session, _ := repo.GetByID(id)
if session.EndedAt != nil {
t.Error("Expected session to be active before ending")
}

// End the session
err = repo.EndStreamSession(id)
if err != nil {
t.Fatalf("EndStreamSession failed: %v", err)
}

// Verify it's now ended
session, err = repo.GetByID(id)
if err != nil {
t.Fatalf("GetByID failed: %v", err)
}

if session.EndedAt == nil {
t.Error("Expected ended_at to be set after ending session")
}
}

func TestSessionRepository_EndStreamSession_NotFound(t *testing.T) {
repo := NewInMemorySessionRepository()

err := repo.EndStreamSession("nonexistent-id")
if err != ErrStreamNotFound {
t.Errorf("Expected ErrStreamNotFound, got %v", err)
}
}

func TestSessionRepository_EndStreamSession_Idempotent(t *testing.T) {
repo := NewInMemorySessionRepository()
sceneID := "scene-123"
hostDID := "did:plc:host456"

// Create and end a session
id, _, err := repo.CreateStreamSession(&sceneID, nil, hostDID)
if err != nil {
t.Fatalf("CreateStreamSession failed: %v", err)
}

err = repo.EndStreamSession(id)
if err != nil {
t.Fatalf("First EndStreamSession failed: %v", err)
}

// Get the ended_at timestamp
session1, _ := repo.GetByID(id)
endedAt1 := session1.EndedAt

// End it again (idempotent)
err = repo.EndStreamSession(id)
if err != nil {
t.Fatalf("Second EndStreamSession failed: %v", err)
}

// Verify ended_at didn't change
session2, _ := repo.GetByID(id)
if !session2.EndedAt.Equal(*endedAt1) {
t.Error("Expected ended_at to remain unchanged on idempotent call")
}
}

func TestSessionRepository_CreateStreamSession_EmptyHostDID(t *testing.T) {
repo := NewInMemorySessionRepository()
sceneID := "scene-123"

_, _, err := repo.CreateStreamSession(&sceneID, nil, "")
if err == nil {
t.Error("Expected error when hostDID is empty")
}

expectedErrMsg := "hostDID must not be empty"
if err != nil && err.Error() != expectedErrMsg {
t.Errorf("Expected error message '%s', got '%s'", expectedErrMsg, err.Error())
}
}

// TestSessionRepository_GetActiveStreamForEvent_MultipleStreams tests most recent stream selection for single query.
func TestSessionRepository_GetActiveStreamForEvent_MultipleStreams(t *testing.T) {
repo := NewInMemorySessionRepository()
eventID := "event-multi-single"

// Create first active stream
stream1ID, _, err := repo.CreateStreamSession(nil, &eventID, "did:plc:host1")
if err != nil {
t.Fatalf("CreateStreamSession 1 failed: %v", err)
}

// Wait to ensure different timestamps
time.Sleep(10 * time.Millisecond)

// Create second active stream (should be more recent)
stream2ID, room2, err := repo.CreateStreamSession(nil, &eventID, "did:plc:host2")
if err != nil {
t.Fatalf("CreateStreamSession 2 failed: %v", err)
}

// Single query should return the most recent stream
activeStream, err := repo.GetActiveStreamForEvent(eventID)
if err != nil {
t.Fatalf("GetActiveStreamForEvent failed: %v", err)
}

if activeStream == nil {
t.Fatal("Expected active stream, got nil")
}

// Should return the most recent stream (stream2)
if activeStream.StreamSessionID != stream2ID {
t.Errorf("Expected most recent stream_session_id '%s', got '%s' (older: '%s')", stream2ID, activeStream.StreamSessionID, stream1ID)
}

if activeStream.RoomName != room2 {
t.Errorf("Expected room_name '%s', got '%s'", room2, activeStream.RoomName)
}
}

func TestSessionRepository_RecordJoin_Success(t *testing.T) {
repo := NewInMemorySessionRepository()
sceneID := "scene-123"

// Create a stream session
id, _, err := repo.CreateStreamSession(&sceneID, nil, "did:plc:host")
if err != nil {
t.Fatalf("CreateStreamSession failed: %v", err)
}

// Get initial join count
session, err := repo.GetByID(id)
if err != nil {
t.Fatalf("GetByID failed: %v", err)
}
if session.JoinCount != 0 {
t.Errorf("Initial join_count = %d, want 0", session.JoinCount)
}

// Record multiple joins
for i := 0; i < 5; i++ {
if err := repo.RecordJoin(id); err != nil {
t.Fatalf("RecordJoin failed: %v", err)
}
}

// Verify join count
session, err = repo.GetByID(id)
if err != nil {
t.Fatalf("GetByID failed: %v", err)
}
if session.JoinCount != 5 {
t.Errorf("join_count = %d, want 5", session.JoinCount)
}
}

func TestSessionRepository_RecordJoin_NotFound(t *testing.T) {
repo := NewInMemorySessionRepository()

err := repo.RecordJoin("nonexistent-id")
if err != ErrStreamNotFound {
t.Errorf("Expected ErrStreamNotFound, got %v", err)
}
}

func TestSessionRepository_RecordLeave_Success(t *testing.T) {
repo := NewInMemorySessionRepository()
sceneID := "scene-123"

// Create a stream session
id, _, err := repo.CreateStreamSession(&sceneID, nil, "did:plc:host")
if err != nil {
t.Fatalf("CreateStreamSession failed: %v", err)
}

// Get initial leave count
session, err := repo.GetByID(id)
if err != nil {
t.Fatalf("GetByID failed: %v", err)
}
if session.LeaveCount != 0 {
t.Errorf("Initial leave_count = %d, want 0", session.LeaveCount)
}

// Record multiple leaves
for i := 0; i < 3; i++ {
if err := repo.RecordLeave(id); err != nil {
t.Fatalf("RecordLeave failed: %v", err)
}
}

// Verify leave count
session, err = repo.GetByID(id)
if err != nil {
t.Fatalf("GetByID failed: %v", err)
}
if session.LeaveCount != 3 {
t.Errorf("leave_count = %d, want 3", session.LeaveCount)
}
}

func TestSessionRepository_RecordLeave_NotFound(t *testing.T) {
repo := NewInMemorySessionRepository()

err := repo.RecordLeave("nonexistent-id")
if err != ErrStreamNotFound {
t.Errorf("Expected ErrStreamNotFound, got %v", err)
}
}

func TestSessionRepository_JoinLeave_Combined(t *testing.T) {
repo := NewInMemorySessionRepository()
sceneID := "scene-123"

// Create a stream session
id, _, err := repo.CreateStreamSession(&sceneID, nil, "did:plc:host")
if err != nil {
t.Fatalf("CreateStreamSession failed: %v", err)
}

// Simulate join/leave events
if err := repo.RecordJoin(id); err != nil {
t.Fatalf("RecordJoin 1 failed: %v", err)
}
if err := repo.RecordJoin(id); err != nil {
t.Fatalf("RecordJoin 2 failed: %v", err)
}
if err := repo.RecordLeave(id); err != nil {
t.Fatalf("RecordLeave 1 failed: %v", err)
}
if err := repo.RecordJoin(id); err != nil {
t.Fatalf("RecordJoin 3 failed: %v", err)
}
if err := repo.RecordLeave(id); err != nil {
t.Fatalf("RecordLeave 2 failed: %v", err)
}
if err := repo.RecordLeave(id); err != nil {
t.Fatalf("RecordLeave 3 failed: %v", err)
}

// Verify counts
session, err := repo.GetByID(id)
if err != nil {
t.Fatalf("GetByID failed: %v", err)
}
if session.JoinCount != 3 {
t.Errorf("join_count = %d, want 3", session.JoinCount)
}
if session.LeaveCount != 3 {
t.Errorf("leave_count = %d, want 3", session.LeaveCount)
}
}
