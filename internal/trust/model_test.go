package trust

import (
	"math"
	"testing"
)

func TestComputeTrustScore(t *testing.T) {
	tests := []struct {
		name        string
		memberships []Membership
		alliances   []Alliance
		want        float64
	}{
		{
			name:        "no memberships returns zero",
			memberships: []Membership{},
			alliances:   []Alliance{},
			want:        0.0,
		},
		{
			name:        "nil memberships returns zero",
			memberships: nil,
			alliances:   nil,
			want:        0.0,
		},
		{
			name: "single member no alliances",
			memberships: []Membership{
				{SceneID: "s1", UserDID: "did:user1", Role: "member", TrustWeight: 0.8},
			},
			alliances: []Alliance{},
			// avg_alliance = 1.0 (default), avg_membership = 0.8 * 1.0 = 0.8
			// score = 1.0 * 0.8 = 0.8
			want: 0.8,
		},
		{
			name: "single member with curator role",
			memberships: []Membership{
				{SceneID: "s1", UserDID: "did:user1", Role: "curator", TrustWeight: 0.8},
			},
			alliances: []Alliance{},
			// avg_alliance = 1.0, avg_membership = 0.8 * 1.5 = 1.2
			// score = 1.0 * 1.2 = 1.2
			want: 1.2,
		},
		{
			name: "single member with admin role",
			memberships: []Membership{
				{SceneID: "s1", UserDID: "did:user1", Role: "admin", TrustWeight: 0.5},
			},
			alliances: []Alliance{},
			// avg_alliance = 1.0, avg_membership = 0.5 * 2.0 = 1.0
			// score = 1.0 * 1.0 = 1.0
			want: 1.0,
		},
		{
			name: "multiple members different roles",
			memberships: []Membership{
				{SceneID: "s1", UserDID: "did:user1", Role: "member", TrustWeight: 0.6},  // 0.6 * 1.0 = 0.6
				{SceneID: "s1", UserDID: "did:user2", Role: "curator", TrustWeight: 0.8}, // 0.8 * 1.5 = 1.2
				{SceneID: "s1", UserDID: "did:user3", Role: "admin", TrustWeight: 1.0},   // 1.0 * 2.0 = 2.0
			},
			alliances: []Alliance{},
			// avg_membership = (0.6 + 1.2 + 2.0) / 3 = 3.8 / 3 = 1.2666...
			// score = 1.0 * 1.2666... = 1.2666...
			want: 3.8 / 3.0,
		},
		{
			name: "single member single alliance",
			memberships: []Membership{
				{SceneID: "s1", UserDID: "did:user1", Role: "member", TrustWeight: 1.0},
			},
			alliances: []Alliance{
				{FromSceneID: "s1", ToSceneID: "s2", Weight: 0.5},
			},
			// avg_alliance = 0.5, avg_membership = 1.0 * 1.0 = 1.0
			// score = 0.5 * 1.0 = 0.5
			want: 0.5,
		},
		{
			name: "multiple alliances",
			memberships: []Membership{
				{SceneID: "s1", UserDID: "did:user1", Role: "member", TrustWeight: 1.0},
			},
			alliances: []Alliance{
				{FromSceneID: "s1", ToSceneID: "s2", Weight: 0.4},
				{FromSceneID: "s1", ToSceneID: "s3", Weight: 0.6},
				{FromSceneID: "s1", ToSceneID: "s4", Weight: 0.8},
			},
			// avg_alliance = (0.4 + 0.6 + 0.8) / 3 = 0.6
			// avg_membership = 1.0
			// score = 0.6 * 1.0 = 0.6
			want: 0.6,
		},
		{
			name: "complex scenario",
			memberships: []Membership{
				{SceneID: "s1", UserDID: "did:user1", Role: "admin", TrustWeight: 0.9},   // 0.9 * 2.0 = 1.8
				{SceneID: "s1", UserDID: "did:user2", Role: "curator", TrustWeight: 0.7}, // 0.7 * 1.5 = 1.05
				{SceneID: "s1", UserDID: "did:user3", Role: "member", TrustWeight: 0.5},  // 0.5 * 1.0 = 0.5
				{SceneID: "s1", UserDID: "did:user4", Role: "member", TrustWeight: 0.8},  // 0.8 * 1.0 = 0.8
			},
			alliances: []Alliance{
				{FromSceneID: "s1", ToSceneID: "s2", Weight: 0.9},
				{FromSceneID: "s1", ToSceneID: "s3", Weight: 0.7},
			},
			// avg_alliance = (0.9 + 0.7) / 2 = 0.8
			// avg_membership = (1.8 + 1.05 + 0.5 + 0.8) / 4 = 4.15 / 4 = 1.0375
			// score = 0.8 * 1.0375 = 0.83
			want: 0.8 * (4.15 / 4.0),
		},
		{
			name: "unknown role uses default multiplier",
			memberships: []Membership{
				{SceneID: "s1", UserDID: "did:user1", Role: "unknown_role", TrustWeight: 0.5},
			},
			alliances: []Alliance{},
			// avg_membership = 0.5 * 1.0 (default) = 0.5
			// score = 1.0 * 0.5 = 0.5
			want: 0.5,
		},
		{
			name: "zero trust weight member",
			memberships: []Membership{
				{SceneID: "s1", UserDID: "did:user1", Role: "member", TrustWeight: 0.0},
			},
			alliances: []Alliance{},
			want:      0.0,
		},
		{
			name: "zero weight alliance",
			memberships: []Membership{
				{SceneID: "s1", UserDID: "did:user1", Role: "member", TrustWeight: 1.0},
			},
			alliances: []Alliance{
				{FromSceneID: "s1", ToSceneID: "s2", Weight: 0.0},
			},
			want: 0.0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ComputeTrustScore(tt.memberships, tt.alliances)
			if math.Abs(got-tt.want) > 1e-9 {
				t.Errorf("ComputeTrustScore() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestRoleMultipliers(t *testing.T) {
	// Verify role multipliers are correctly defined
	tests := []struct {
		role string
		want float64
	}{
		{role: "member", want: 1.0},
		{role: "curator", want: 1.5},
		{role: "admin", want: 2.0},
	}

	for _, tt := range tests {
		t.Run(tt.role, func(t *testing.T) {
			got := RoleMultiplier[tt.role]
			if got != tt.want {
				t.Errorf("RoleMultiplier[%q] = %v, want %v", tt.role, got, tt.want)
			}
		})
	}
}

func TestDirtyTracker(t *testing.T) {
	t.Run("initial state is empty", func(t *testing.T) {
		tracker := NewDirtyTracker()
		if tracker.DirtyCount() != 0 {
			t.Errorf("initial DirtyCount() = %d, want 0", tracker.DirtyCount())
		}
		if len(tracker.GetDirtyScenes()) != 0 {
			t.Errorf("initial GetDirtyScenes() len = %d, want 0", len(tracker.GetDirtyScenes()))
		}
	})

	t.Run("mark and check dirty", func(t *testing.T) {
		tracker := NewDirtyTracker()
		tracker.MarkDirty("scene-1")

		if !tracker.IsDirty("scene-1") {
			t.Error("expected scene-1 to be dirty")
		}
		if tracker.IsDirty("scene-2") {
			t.Error("expected scene-2 to not be dirty")
		}
		if tracker.DirtyCount() != 1 {
			t.Errorf("DirtyCount() = %d, want 1", tracker.DirtyCount())
		}
	})

	t.Run("clear dirty", func(t *testing.T) {
		tracker := NewDirtyTracker()
		tracker.MarkDirty("scene-1")
		tracker.ClearDirty("scene-1")

		if tracker.IsDirty("scene-1") {
			t.Error("expected scene-1 to not be dirty after clear")
		}
		if tracker.DirtyCount() != 0 {
			t.Errorf("DirtyCount() = %d, want 0", tracker.DirtyCount())
		}
	})

	t.Run("get dirty scenes", func(t *testing.T) {
		tracker := NewDirtyTracker()
		tracker.MarkDirty("scene-1")
		tracker.MarkDirty("scene-2")
		tracker.MarkDirty("scene-3")

		scenes := tracker.GetDirtyScenes()
		if len(scenes) != 3 {
			t.Errorf("GetDirtyScenes() len = %d, want 3", len(scenes))
		}

		// Check all scenes are present
		sceneMap := make(map[string]bool)
		for _, s := range scenes {
			sceneMap[s] = true
		}
		for _, expected := range []string{"scene-1", "scene-2", "scene-3"} {
			if !sceneMap[expected] {
				t.Errorf("expected %s in dirty scenes", expected)
			}
		}
	})

	t.Run("marking same scene twice is idempotent", func(t *testing.T) {
		tracker := NewDirtyTracker()
		tracker.MarkDirty("scene-1")
		tracker.MarkDirty("scene-1")

		if tracker.DirtyCount() != 1 {
			t.Errorf("DirtyCount() = %d, want 1 after double mark", tracker.DirtyCount())
		}
	})

	t.Run("clearing non-existent scene is safe", func(t *testing.T) {
		tracker := NewDirtyTracker()
		tracker.ClearDirty("nonexistent") // Should not panic

		if tracker.DirtyCount() != 0 {
			t.Errorf("DirtyCount() = %d, want 0", tracker.DirtyCount())
		}
	})
}

func TestDirtyTracker_Concurrency(t *testing.T) {
	tracker := NewDirtyTracker()
	done := make(chan bool)

	// Run concurrent operations
	for i := 0; i < 10; i++ {
		go func(id int) {
			for j := 0; j < 100; j++ {
				sceneID := "scene-" + string(rune('a'+id))
				tracker.MarkDirty(sceneID)
				tracker.IsDirty(sceneID)
				tracker.GetDirtyScenes()
				tracker.DirtyCount()
				if j%10 == 0 {
					tracker.ClearDirty(sceneID)
				}
			}
			done <- true
		}(i)
	}

	// Wait for all goroutines
	for i := 0; i < 10; i++ {
		<-done
	}

	// Just verify no panic occurred and state is consistent
	count := tracker.DirtyCount()
	scenes := tracker.GetDirtyScenes()
	if count != len(scenes) {
		t.Errorf("DirtyCount() = %d, but GetDirtyScenes() len = %d", count, len(scenes))
	}
}
