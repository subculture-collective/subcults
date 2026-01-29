package stream

import (
"testing"
"time"
)

// TestSession_Structure tests the Session structure.
func TestSession_Structure(t *testing.T) {
now := time.Now()
endedTime := now.Add(-1 * time.Hour)
sceneID := "scene-123"
participantID := "participant-featured"

tests := []struct {
name    string
session *Session
wantActive bool
}{
{
name: "active_session",
session: &Session{
ID:        "session-123",
SceneID:   &sceneID,
RoomName:  "test-room",
HostDID:   "did:plc:host",
StartedAt: now,
EndedAt:   nil,
},
wantActive: true,
},
{
name: "ended_session",
session: &Session{
ID:        "session-456",
RoomName:  "ended-room",
HostDID:   "did:plc:host",
StartedAt: now.Add(-2 * time.Hour),
EndedAt:   &endedTime,
},
wantActive: false,
},
{
name: "locked_session",
session: &Session{
ID:        "session-locked",
RoomName:  "locked-room",
HostDID:   "did:plc:host",
StartedAt: now,
IsLocked:  true,
},
wantActive: true,
},
{
name: "session_with_featured_participant",
session: &Session{
ID:                  "session-featured",
RoomName:            "featured-room",
HostDID:             "did:plc:host",
StartedAt:           now,
FeaturedParticipant: &participantID,
},
wantActive: true,
},
}

for _, tt := range tests {
t.Run(tt.name, func(t *testing.T) {
// Check if session is active based on EndedAt
isActive := tt.session.EndedAt == nil
if isActive != tt.wantActive {
t.Errorf("Session active = %v, want %v", isActive, tt.wantActive)
}

// Validate required fields
if tt.session.ID == "" {
t.Error("ID should not be empty")
}
if tt.session.RoomName == "" {
t.Error("RoomName should not be empty")
}
if tt.session.HostDID == "" {
t.Error("HostDID should not be empty")
}
if tt.session.StartedAt.IsZero() {
t.Error("StartedAt should not be zero")
}
})
}
}

// TestActiveStreamInfo_Structure tests the ActiveStreamInfo structure.
func TestActiveStreamInfo_Structure(t *testing.T) {
now := time.Now()

info := &ActiveStreamInfo{
StreamSessionID: "session-456",
RoomName:        "live-room",
StartedAt:       now,
}

if info.StreamSessionID == "" {
t.Error("StreamSessionID should not be empty")
}
if info.RoomName != "live-room" {
t.Errorf("RoomName = %s, want live-room", info.RoomName)
}
if info.StartedAt.IsZero() {
t.Error("StartedAt should not be zero")
}
}

// TestUpsertResult_Structure tests the UpsertResult structure.
func TestUpsertResult_Structure(t *testing.T) {
tests := []struct {
name     string
result   *UpsertResult
wantType string
}{
{
name: "insert_result",
result: &UpsertResult{
ID:       "new-id",
Inserted: true,
},
wantType: "insert",
},
{
name: "update_result",
result: &UpsertResult{
ID:       "existing-id",
Inserted: false,
},
wantType: "update",
},
}

for _, tt := range tests {
t.Run(tt.name, func(t *testing.T) {
if tt.result.ID == "" {
t.Error("ID should not be empty")
}
if tt.wantType == "insert" && !tt.result.Inserted {
t.Error("Inserted should be true for insert results")
}
if tt.wantType == "update" && tt.result.Inserted {
t.Error("Inserted should be false for update results")
}
})
}
}
