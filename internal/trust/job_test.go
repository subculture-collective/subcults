package trust

import (
	"context"
	"log/slog"
	"math"
	"os"
	"testing"
	"time"
)

func TestRecomputeJob_StartStop(t *testing.T) {
	dataSource := NewInMemoryDataSource()
	scoreStore := NewInMemoryScoreStore()
	dirtyTracker := NewDirtyTracker()

	job := NewRecomputeJob(
		RecomputeJobConfig{
			Interval: 100 * time.Millisecond,
			Logger:   slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError})),
		},
		dirtyTracker,
		dataSource,
		scoreStore,
	)

	// Job should not be running initially
	if job.IsRunning() {
		t.Error("job should not be running before Start")
	}

	// Start the job
	ctx := context.Background()
	if err := job.Start(ctx); err != nil {
		t.Fatalf("Start() error = %v", err)
	}

	if !job.IsRunning() {
		t.Error("job should be running after Start")
	}

	// Starting again should be safe (idempotent)
	if err := job.Start(ctx); err != nil {
		t.Fatalf("Start() second call error = %v", err)
	}

	// Stop the job
	job.Stop()

	if job.IsRunning() {
		t.Error("job should not be running after Stop")
	}

	// Stopping again should be safe
	job.Stop()
}

func TestRecomputeJob_RecomputesOnlyDirtyScenes(t *testing.T) {
	dataSource := NewInMemoryDataSource()
	scoreStore := NewInMemoryScoreStore()
	dirtyTracker := NewDirtyTracker()

	// Setup data for two scenes
	dataSource.AddMembership(Membership{
		SceneID:     "scene-1",
		UserDID:     "did:user1",
		Role:        "member",
		TrustWeight: 0.8,
	})
	dataSource.AddMembership(Membership{
		SceneID:     "scene-2",
		UserDID:     "did:user2",
		Role:        "admin",
		TrustWeight: 1.0,
	})

	job := NewRecomputeJob(
		RecomputeJobConfig{
			Interval: 100 * time.Millisecond,
			Logger:   slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError})),
		},
		dirtyTracker,
		dataSource,
		scoreStore,
	)

	// Mark only scene-1 as dirty
	dirtyTracker.MarkDirty("scene-1")

	// Trigger immediate recompute
	job.RecomputeNow()

	// Check scene-1 was recomputed
	score1, err := scoreStore.GetScore("scene-1")
	if err != nil {
		t.Fatalf("GetScore(scene-1) error = %v", err)
	}
	if score1 == nil {
		t.Fatal("expected score for scene-1")
	}
	expectedScore1 := 0.8 // 0.8 * 1.0 (member multiplier) * 1.0 (no alliances)
	if score1.Score != expectedScore1 {
		t.Errorf("scene-1 score = %v, want %v", score1.Score, expectedScore1)
	}

	// Check scene-2 was NOT recomputed (not dirty)
	score2, err := scoreStore.GetScore("scene-2")
	if err != nil {
		t.Fatalf("GetScore(scene-2) error = %v", err)
	}
	if score2 != nil {
		t.Error("expected no score for scene-2 (not dirty)")
	}

	// Check scene-1 is no longer dirty
	if dirtyTracker.IsDirty("scene-1") {
		t.Error("scene-1 should not be dirty after recompute")
	}
}

func TestRecomputeJob_RecomputesWithAlliances(t *testing.T) {
	dataSource := NewInMemoryDataSource()
	scoreStore := NewInMemoryScoreStore()
	dirtyTracker := NewDirtyTracker()

	// Setup scene with memberships and alliances
	dataSource.AddMembership(Membership{
		SceneID:     "scene-1",
		UserDID:     "did:user1",
		Role:        "curator",
		TrustWeight: 0.8,
	})
	dataSource.AddAlliance(Alliance{
		FromSceneID: "scene-1",
		ToSceneID:   "scene-2",
		Weight:      0.6,
	})
	dataSource.AddAlliance(Alliance{
		FromSceneID: "scene-1",
		ToSceneID:   "scene-3",
		Weight:      0.8,
	})

	job := NewRecomputeJob(
		RecomputeJobConfig{
			Interval: 100 * time.Millisecond,
			Logger:   slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError})),
		},
		dirtyTracker,
		dataSource,
		scoreStore,
	)

	dirtyTracker.MarkDirty("scene-1")
	job.RecomputeNow()

	score, err := scoreStore.GetScore("scene-1")
	if err != nil {
		t.Fatalf("GetScore error = %v", err)
	}
	if score == nil {
		t.Fatal("expected score for scene-1")
	}

	// Expected: avg_alliance = (0.6 + 0.8) / 2 = 0.7
	//           avg_membership = 0.8 * 1.5 (curator) = 1.2
	//           score = 0.7 * 1.2 = 0.84
	expectedScore := 0.7 * 1.2
	if math.Abs(score.Score-expectedScore) > 1e-9 {
		t.Errorf("score = %v, want %v", score.Score, expectedScore)
	}
}

func TestRecomputeJob_PeriodicExecution(t *testing.T) {
	dataSource := NewInMemoryDataSource()
	scoreStore := NewInMemoryScoreStore()
	dirtyTracker := NewDirtyTracker()

	// Setup initial data
	dataSource.AddMembership(Membership{
		SceneID:     "scene-1",
		UserDID:     "did:user1",
		Role:        "member",
		TrustWeight: 0.5,
	})

	job := NewRecomputeJob(
		RecomputeJobConfig{
			Interval: 50 * time.Millisecond, // Short interval for testing
			Logger:   slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError})),
		},
		dirtyTracker,
		dataSource,
		scoreStore,
	)

	ctx := context.Background()
	if err := job.Start(ctx); err != nil {
		t.Fatalf("Start() error = %v", err)
	}
	defer job.Stop()

	// Mark scene dirty
	dirtyTracker.MarkDirty("scene-1")

	// Wait for at least one tick
	time.Sleep(100 * time.Millisecond)

	// Check score was computed
	score, err := scoreStore.GetScore("scene-1")
	if err != nil {
		t.Fatalf("GetScore error = %v", err)
	}
	if score == nil {
		t.Fatal("expected score to be computed after periodic tick")
	}
	if score.Score != 0.5 {
		t.Errorf("score = %v, want 0.5", score.Score)
	}

	// Scene should no longer be dirty
	if dirtyTracker.IsDirty("scene-1") {
		t.Error("scene-1 should not be dirty after recompute")
	}
}

func TestRecomputeJob_ContextCancellation(t *testing.T) {
	dataSource := NewInMemoryDataSource()
	scoreStore := NewInMemoryScoreStore()
	dirtyTracker := NewDirtyTracker()

	job := NewRecomputeJob(
		RecomputeJobConfig{
			Interval: 100 * time.Millisecond,
			Logger:   slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError})),
		},
		dirtyTracker,
		dataSource,
		scoreStore,
	)

	ctx, cancel := context.WithCancel(context.Background())
	if err := job.Start(ctx); err != nil {
		t.Fatalf("Start() error = %v", err)
	}

	if !job.IsRunning() {
		t.Error("job should be running")
	}

	// Cancel context
	cancel()

	// Give job time to notice cancellation
	time.Sleep(50 * time.Millisecond)

	// Job should have stopped - wait for doneCh via Stop()
	job.Stop()

	if job.IsRunning() {
		t.Error("job should have stopped after context cancellation")
	}
}

func TestRecomputeJob_DefaultInterval(t *testing.T) {
	dataSource := NewInMemoryDataSource()
	scoreStore := NewInMemoryScoreStore()
	dirtyTracker := NewDirtyTracker()

	// Create job with zero interval (should use default)
	job := NewRecomputeJob(
		RecomputeJobConfig{
			Interval: 0,
		},
		dirtyTracker,
		dataSource,
		scoreStore,
	)

	// The job should use default interval internally
	// We can't easily verify the interval value, but we verify it doesn't panic
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if err := job.Start(ctx); err != nil {
		t.Fatalf("Start() error = %v", err)
	}
	defer job.Stop()

	if !job.IsRunning() {
		t.Error("job should be running with default interval")
	}
}

func TestRecomputeJob_EmptyDirtyScenes(t *testing.T) {
	dataSource := NewInMemoryDataSource()
	scoreStore := NewInMemoryScoreStore()
	dirtyTracker := NewDirtyTracker()

	job := NewRecomputeJob(
		RecomputeJobConfig{
			Interval: 100 * time.Millisecond,
			Logger:   slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError})),
		},
		dirtyTracker,
		dataSource,
		scoreStore,
	)

	// No dirty scenes
	job.RecomputeNow()

	// Should complete without error and no scores stored
	allScores := scoreStore.AllScores()
	if len(allScores) != 0 {
		t.Errorf("expected no scores, got %d", len(allScores))
	}
}

func TestInMemoryDataSource(t *testing.T) {
	t.Run("add and get memberships", func(t *testing.T) {
		ds := NewInMemoryDataSource()

		ds.AddMembership(Membership{SceneID: "s1", UserDID: "u1", Role: "member", TrustWeight: 0.5})
		ds.AddMembership(Membership{SceneID: "s1", UserDID: "u2", Role: "admin", TrustWeight: 0.8})
		ds.AddMembership(Membership{SceneID: "s2", UserDID: "u3", Role: "curator", TrustWeight: 0.6})

		memberships, err := ds.GetMembershipsByScene("s1")
		if err != nil {
			t.Fatalf("GetMembershipsByScene error = %v", err)
		}
		if len(memberships) != 2 {
			t.Errorf("expected 2 memberships for s1, got %d", len(memberships))
		}

		memberships2, err := ds.GetMembershipsByScene("s2")
		if err != nil {
			t.Fatalf("GetMembershipsByScene error = %v", err)
		}
		if len(memberships2) != 1 {
			t.Errorf("expected 1 membership for s2, got %d", len(memberships2))
		}

		// Non-existent scene
		memberships3, err := ds.GetMembershipsByScene("s3")
		if err != nil {
			t.Fatalf("GetMembershipsByScene error = %v", err)
		}
		if len(memberships3) != 0 {
			t.Errorf("expected 0 memberships for s3, got %d", len(memberships3))
		}
	})

	t.Run("add and get alliances", func(t *testing.T) {
		ds := NewInMemoryDataSource()

		ds.AddAlliance(Alliance{FromSceneID: "s1", ToSceneID: "s2", Weight: 0.5})
		ds.AddAlliance(Alliance{FromSceneID: "s1", ToSceneID: "s3", Weight: 0.7})
		ds.AddAlliance(Alliance{FromSceneID: "s2", ToSceneID: "s1", Weight: 0.6})

		alliances, err := ds.GetAlliancesByScene("s1")
		if err != nil {
			t.Fatalf("GetAlliancesByScene error = %v", err)
		}
		if len(alliances) != 2 {
			t.Errorf("expected 2 alliances from s1, got %d", len(alliances))
		}
	})

	t.Run("clear memberships", func(t *testing.T) {
		ds := NewInMemoryDataSource()

		ds.AddMembership(Membership{SceneID: "s1", UserDID: "u1", Role: "member", TrustWeight: 0.5})
		ds.ClearMemberships("s1")

		memberships, _ := ds.GetMembershipsByScene("s1")
		if len(memberships) != 0 {
			t.Errorf("expected 0 memberships after clear, got %d", len(memberships))
		}
	})

	t.Run("clear alliances", func(t *testing.T) {
		ds := NewInMemoryDataSource()

		ds.AddAlliance(Alliance{FromSceneID: "s1", ToSceneID: "s2", Weight: 0.5})
		ds.ClearAlliances("s1")

		alliances, _ := ds.GetAlliancesByScene("s1")
		if len(alliances) != 0 {
			t.Errorf("expected 0 alliances after clear, got %d", len(alliances))
		}
	})
}

func TestInMemoryScoreStore(t *testing.T) {
	t.Run("save and get score", func(t *testing.T) {
		store := NewInMemoryScoreStore()

		now := time.Now()
		score := SceneTrustScore{
			SceneID:    "s1",
			Score:      0.75,
			ComputedAt: now,
		}

		if err := store.SaveScore(score); err != nil {
			t.Fatalf("SaveScore error = %v", err)
		}

		retrieved, err := store.GetScore("s1")
		if err != nil {
			t.Fatalf("GetScore error = %v", err)
		}
		if retrieved == nil {
			t.Fatal("expected score to be retrieved")
		}
		if retrieved.Score != 0.75 {
			t.Errorf("score = %v, want 0.75", retrieved.Score)
		}
	})

	t.Run("get non-existent score", func(t *testing.T) {
		store := NewInMemoryScoreStore()

		score, err := store.GetScore("nonexistent")
		if err != nil {
			t.Fatalf("GetScore error = %v", err)
		}
		if score != nil {
			t.Error("expected nil for non-existent score")
		}
	})

	t.Run("update score", func(t *testing.T) {
		store := NewInMemoryScoreStore()

		store.SaveScore(SceneTrustScore{SceneID: "s1", Score: 0.5, ComputedAt: time.Now()})
		store.SaveScore(SceneTrustScore{SceneID: "s1", Score: 0.8, ComputedAt: time.Now()})

		retrieved, _ := store.GetScore("s1")
		if retrieved.Score != 0.8 {
			t.Errorf("score = %v, want 0.8 (updated)", retrieved.Score)
		}
	})

	t.Run("all scores", func(t *testing.T) {
		store := NewInMemoryScoreStore()

		store.SaveScore(SceneTrustScore{SceneID: "s1", Score: 0.5, ComputedAt: time.Now()})
		store.SaveScore(SceneTrustScore{SceneID: "s2", Score: 0.6, ComputedAt: time.Now()})

		allScores := store.AllScores()
		if len(allScores) != 2 {
			t.Errorf("expected 2 scores, got %d", len(allScores))
		}
	})
}
