// Package trust provides trust score computation for scenes based on
// membership and alliance relationships.
package trust

import (
	"sync"
	"time"
)

// RoleMultiplier defines the trust weight multiplier for different membership roles.
// Higher roles contribute more to the scene's overall trust score.
var RoleMultiplier = map[string]float64{
	"member":  1.0,
	"curator": 1.5,
	"admin":   2.0,
}

// DefaultRoleMultiplier is used when a role is not found in the RoleMultiplier map.
const DefaultRoleMultiplier = 1.0

// Membership represents a user's membership in a scene with their role.
type Membership struct {
	SceneID   string  `json:"scene_id"`
	UserDID   string  `json:"user_did"`
	Role      string  `json:"role"`
	TrustWeight float64 `json:"trust_weight"` // Base trust weight (0.0-1.0)
}

// Alliance represents a trust relationship between two scenes.
type Alliance struct {
	FromSceneID string  `json:"from_scene_id"`
	ToSceneID   string  `json:"to_scene_id"`
	Weight      float64 `json:"weight"` // Trust weight (0.0-1.0)
}

// SceneTrustScore represents the computed trust score for a scene.
type SceneTrustScore struct {
	SceneID     string    `json:"scene_id"`
	Score       float64   `json:"score"`
	ComputedAt  time.Time `json:"computed_at"`
}

// ComputeTrustScore calculates the trust score for a scene using the formula:
// score = avg(alliance weights) * avg(membership trust weights * role multipliers)
//
// If there are no alliances, alliance average defaults to 1.0.
// If there are no memberships, the score is 0.0.
func ComputeTrustScore(memberships []Membership, alliances []Alliance) float64 {
	if len(memberships) == 0 {
		return 0.0
	}

	// Calculate average alliance weight
	allianceAvg := 1.0
	if len(alliances) > 0 {
		var allianceSum float64
		for _, a := range alliances {
			allianceSum += a.Weight
		}
		allianceAvg = allianceSum / float64(len(alliances))
	}

	// Calculate average membership trust weight (with role multipliers)
	var membershipSum float64
	for _, m := range memberships {
		multiplier := RoleMultiplier[m.Role]
		if multiplier == 0 {
			multiplier = DefaultRoleMultiplier
		}
		membershipSum += m.TrustWeight * multiplier
	}
	membershipAvg := membershipSum / float64(len(memberships))

	return allianceAvg * membershipAvg
}

// DirtyTracker tracks which scenes have pending changes that require
// trust score recomputation. Thread-safe via RWMutex.
type DirtyTracker struct {
	mu         sync.RWMutex
	dirtyFlags map[string]time.Time // sceneID -> time marked dirty
}

// NewDirtyTracker creates a new DirtyTracker instance.
func NewDirtyTracker() *DirtyTracker {
	return &DirtyTracker{
		dirtyFlags: make(map[string]time.Time),
	}
}

// MarkDirty marks a scene as needing trust score recomputation.
func (t *DirtyTracker) MarkDirty(sceneID string) {
	t.mu.Lock()
	t.dirtyFlags[sceneID] = time.Now()
	t.mu.Unlock()
}

// ClearDirty removes the dirty flag for a scene after recomputation.
func (t *DirtyTracker) ClearDirty(sceneID string) {
	t.mu.Lock()
	delete(t.dirtyFlags, sceneID)
	t.mu.Unlock()
}

// GetDirtyScenes returns a list of scene IDs that are marked dirty.
// Returns a copy to avoid external modification.
func (t *DirtyTracker) GetDirtyScenes() []string {
	t.mu.RLock()
	defer t.mu.RUnlock()

	scenes := make([]string, 0, len(t.dirtyFlags))
	for sceneID := range t.dirtyFlags {
		scenes = append(scenes, sceneID)
	}
	return scenes
}

// IsDirty checks if a specific scene is marked as dirty.
func (t *DirtyTracker) IsDirty(sceneID string) bool {
	t.mu.RLock()
	defer t.mu.RUnlock()
	_, exists := t.dirtyFlags[sceneID]
	return exists
}

// DirtyCount returns the number of scenes marked as dirty.
func (t *DirtyTracker) DirtyCount() int {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return len(t.dirtyFlags)
}
