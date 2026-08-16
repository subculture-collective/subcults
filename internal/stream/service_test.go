package stream

import (
	"context"
	"testing"
)

func TestNewService(t *testing.T) {
	sessionRepo := NewInMemorySessionRepository()
	participantRepo := NewInMemoryParticipantRepository(sessionRepo)
	analyticsRepo := NewInMemoryAnalyticsRepository(sessionRepo)
	metrics := NewMetrics()

	svc := NewService(sessionRepo, participantRepo, analyticsRepo, metrics)
	if svc == nil {
		t.Fatal("expected non-nil service")
	}
	if svc.sessionRepo == nil {
		t.Error("expected sessionRepo to be set")
	}
}

func TestCreateStream(t *testing.T) {
	sessionRepo := NewInMemorySessionRepository()
	participantRepo := NewInMemoryParticipantRepository(sessionRepo)
	analyticsRepo := NewInMemoryAnalyticsRepository(sessionRepo)
	metrics := NewMetrics()
	svc := NewService(sessionRepo, participantRepo, analyticsRepo, metrics)

	ctx := context.Background()
	sceneID := "scene-123"

	session, err := svc.CreateStream(ctx, &sceneID, nil, "did:plc:host123")
	if err != nil {
		t.Fatalf("CreateStream failed: %v", err)
	}
	if session.ID == "" {
		t.Error("expected non-empty session ID")
	}
	if session.RoomName == "" {
		t.Error("expected non-empty room name")
	}
	if session.HostDID != "did:plc:host123" {
		t.Errorf("expected host DID 'did:plc:host123', got %s", session.HostDID)
	}
	if session.EndedAt != nil {
		t.Error("new session should not be ended")
	}
}

func TestEndStream(t *testing.T) {
	sessionRepo := NewInMemorySessionRepository()
	participantRepo := NewInMemoryParticipantRepository(sessionRepo)
	analyticsRepo := NewInMemoryAnalyticsRepository(sessionRepo)
	metrics := NewMetrics()
	svc := NewService(sessionRepo, participantRepo, analyticsRepo, metrics)

	ctx := context.Background()
	sceneID := "scene-456"

	session, err := svc.CreateStream(ctx, &sceneID, nil, "did:plc:host789")
	if err != nil {
		t.Fatalf("CreateStream failed: %v", err)
	}

	err = svc.EndStream(ctx, session.ID)
	if err != nil {
		t.Fatalf("EndStream failed: %v", err)
	}

	got, err := svc.GetStream(ctx, session.ID)
	if err != nil {
		t.Fatalf("GetStream failed: %v", err)
	}
	if got.EndedAt == nil {
		t.Error("ended session should have EndedAt set")
	}
}

func TestGetStreamNotFound(t *testing.T) {
	sessionRepo := NewInMemorySessionRepository()
	participantRepo := NewInMemoryParticipantRepository(sessionRepo)
	analyticsRepo := NewInMemoryAnalyticsRepository(sessionRepo)
	svc := NewService(sessionRepo, participantRepo, analyticsRepo, nil)

	ctx := context.Background()
	_, err := svc.GetStream(ctx, "nonexistent")
	if err != ErrStreamNotFound {
		t.Errorf("expected ErrStreamNotFound, got %v", err)
	}
}

func TestJoinLeaveStream(t *testing.T) {
	sessionRepo := NewInMemorySessionRepository()
	participantRepo := NewInMemoryParticipantRepository(sessionRepo)
	analyticsRepo := NewInMemoryAnalyticsRepository(sessionRepo)
	metrics := NewMetrics()
	svc := NewService(sessionRepo, participantRepo, analyticsRepo, metrics)

	ctx := context.Background()
	sceneID := "scene-join-test"

	session, err := svc.CreateStream(ctx, &sceneID, nil, "did:plc:host123")
	if err != nil {
		t.Fatalf("CreateStream failed: %v", err)
	}

	// Join
	participant, isReconnection, err := svc.JoinStream(ctx, session.ID, "did:plc:user456", nil)
	if err != nil {
		t.Fatalf("JoinStream failed: %v", err)
	}
	if participant == nil {
		t.Error("expected non-nil participant")
	}
	if isReconnection {
		t.Error("first join should not be a reconnection")
	}

	// Verify join count
	got, err := svc.GetStream(ctx, session.ID)
	if err != nil {
		t.Fatalf("GetStream failed: %v", err)
	}
	if got.JoinCount != 1 {
		t.Errorf("expected JoinCount 1, got %d", got.JoinCount)
	}

	// Leave
	err = svc.LeaveStream(ctx, session.ID, "did:plc:user456")
	if err != nil {
		t.Fatalf("LeaveStream failed: %v", err)
	}

	// Verify leave count
	got, err = svc.GetStream(ctx, session.ID)
	if err != nil {
		t.Fatalf("GetStream failed: %v", err)
	}
	if got.LeaveCount != 1 {
		t.Errorf("expected LeaveCount 1, got %d", got.LeaveCount)
	}
}

func TestLockStream(t *testing.T) {
	sessionRepo := NewInMemorySessionRepository()
	participantRepo := NewInMemoryParticipantRepository(sessionRepo)
	analyticsRepo := NewInMemoryAnalyticsRepository(sessionRepo)
	svc := NewService(sessionRepo, participantRepo, analyticsRepo, nil)

	ctx := context.Background()
	sceneID := "scene-lock-test"

	session, err := svc.CreateStream(ctx, &sceneID, nil, "did:plc:host123")
	if err != nil {
		t.Fatalf("CreateStream failed: %v", err)
	}

	err = svc.LockStream(ctx, session.ID, true)
	if err != nil {
		t.Fatalf("LockStream failed: %v", err)
	}

	got, err := svc.GetStream(ctx, session.ID)
	if err != nil {
		t.Fatalf("GetStream failed: %v", err)
	}
	if !got.IsLocked {
		t.Error("expected IsLocked to be true")
	}

	// Unlock
	err = svc.LockStream(ctx, session.ID, false)
	if err != nil {
		t.Fatalf("LockStream unlock failed: %v", err)
	}
	got, err = svc.GetStream(ctx, session.ID)
	if err != nil {
		t.Fatalf("GetStream failed: %v", err)
	}
	if got.IsLocked {
		t.Error("expected IsLocked to be false")
	}
}

func TestSetFeaturedParticipant(t *testing.T) {
	sessionRepo := NewInMemorySessionRepository()
	participantRepo := NewInMemoryParticipantRepository(sessionRepo)
	analyticsRepo := NewInMemoryAnalyticsRepository(sessionRepo)
	svc := NewService(sessionRepo, participantRepo, analyticsRepo, nil)

	ctx := context.Background()
	sceneID := "scene-featured-test"

	session, err := svc.CreateStream(ctx, &sceneID, nil, "did:plc:host123")
	if err != nil {
		t.Fatalf("CreateStream failed: %v", err)
	}

	pid := "user-participant-1"
	err = svc.SetFeaturedParticipant(ctx, session.ID, &pid)
	if err != nil {
		t.Fatalf("SetFeaturedParticipant failed: %v", err)
	}

	got, err := svc.GetStream(ctx, session.ID)
	if err != nil {
		t.Fatalf("GetStream failed: %v", err)
	}
	if got.FeaturedParticipant == nil || *got.FeaturedParticipant != pid {
		t.Errorf("expected featured participant %s, got %v", pid, got.FeaturedParticipant)
	}

	// Clear
	err = svc.SetFeaturedParticipant(ctx, session.ID, nil)
	if err != nil {
		t.Fatalf("SetFeaturedParticipant clear failed: %v", err)
	}
	got, err = svc.GetStream(ctx, session.ID)
	if err != nil {
		t.Fatalf("GetStream failed: %v", err)
	}
	if got.FeaturedParticipant != nil {
		t.Error("expected featured participant to be nil")
	}
}

func TestGetStreamAnalyticsNotFound(t *testing.T) {
	sessionRepo := NewInMemorySessionRepository()
	participantRepo := NewInMemoryParticipantRepository(sessionRepo)
	analyticsRepo := NewInMemoryAnalyticsRepository(sessionRepo)
	svc := NewService(sessionRepo, participantRepo, analyticsRepo, nil)

	ctx := context.Background()
	_, err := svc.GetStreamAnalytics(ctx, "nonexistent")
	if err != ErrAnalyticsNotFound {
		t.Errorf("expected ErrAnalyticsNotFound, got %v", err)
	}
}

func TestGetActiveParticipants(t *testing.T) {
	sessionRepo := NewInMemorySessionRepository()
	participantRepo := NewInMemoryParticipantRepository(sessionRepo)
	analyticsRepo := NewInMemoryAnalyticsRepository(sessionRepo)
	metrics := NewMetrics()
	svc := NewService(sessionRepo, participantRepo, analyticsRepo, metrics)

	ctx := context.Background()
	sceneID := "scene-count-test"

	session, err := svc.CreateStream(ctx, &sceneID, nil, "did:plc:host123")
	if err != nil {
		t.Fatalf("CreateStream failed: %v", err)
	}

	count, err := svc.GetActiveParticipants(ctx, session.ID)
	if err != nil {
		t.Fatalf("GetActiveParticipants failed: %v", err)
	}
	if count != 0 {
		t.Errorf("expected 0 active participants, got %d", count)
	}

	_, _, err = svc.JoinStream(ctx, session.ID, "did:plc:user1", nil)
	if err != nil {
		t.Fatalf("JoinStream failed: %v", err)
	}

	count, err = svc.GetActiveParticipants(ctx, session.ID)
	if err != nil {
		t.Fatalf("GetActiveParticipants failed: %v", err)
	}
	if count != 1 {
		t.Errorf("expected 1 active participant, got %d", count)
	}
}

func TestEndStreamIdempotent(t *testing.T) {
	sessionRepo := NewInMemorySessionRepository()
	participantRepo := NewInMemoryParticipantRepository(sessionRepo)
	analyticsRepo := NewInMemoryAnalyticsRepository(sessionRepo)
	svc := NewService(sessionRepo, participantRepo, analyticsRepo, nil)

	ctx := context.Background()
	sceneID := "scene-idempotent-test"

	session, err := svc.CreateStream(ctx, &sceneID, nil, "did:plc:host123")
	if err != nil {
		t.Fatalf("CreateStream failed: %v", err)
	}

	err = svc.EndStream(ctx, session.ID)
	if err != nil {
		t.Fatalf("first EndStream failed: %v", err)
	}

	// Second end should be idempotent
	err = svc.EndStream(ctx, session.ID)
	if err != nil {
		t.Fatalf("second EndStream failed: %v", err)
	}
}