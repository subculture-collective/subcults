// Package trust provides trust score computation for scenes based on
// membership and alliance relationships.
package trust

import (
	"context"
	"log/slog"
	"sync"
	"time"
)

// DataSource provides membership and alliance data for trust score computation.
type DataSource interface {
	// GetMembershipsByScene returns all memberships for a scene.
	GetMembershipsByScene(sceneID string) ([]Membership, error)
	// GetAlliancesByScene returns all alliances where the scene is the source.
	GetAlliancesByScene(sceneID string) ([]Alliance, error)
}

// ScoreStore persists computed trust scores.
type ScoreStore interface {
	// SaveScore stores a computed trust score.
	SaveScore(score SceneTrustScore) error
	// GetScore retrieves a trust score by scene ID.
	GetScore(sceneID string) (*SceneTrustScore, error)
}

// RecomputeJobConfig configures the trust score recompute job.
type RecomputeJobConfig struct {
	// Interval is the duration between recompute cycles.
	Interval time.Duration
	// Logger for job activity.
	Logger *slog.Logger
}

// DefaultRecomputeInterval is the default interval between recompute cycles.
const DefaultRecomputeInterval = 30 * time.Second

// RecomputeJob periodically recalculates trust scores for dirty scenes.
type RecomputeJob struct {
	config       RecomputeJobConfig
	dirtyTracker *DirtyTracker
	dataSource   DataSource
	scoreStore   ScoreStore

	mu      sync.Mutex
	running bool
	stopCh  chan struct{}
	doneCh  chan struct{}
}

// NewRecomputeJob creates a new trust score recompute job.
func NewRecomputeJob(
	config RecomputeJobConfig,
	dirtyTracker *DirtyTracker,
	dataSource DataSource,
	scoreStore ScoreStore,
) *RecomputeJob {
	if config.Interval == 0 {
		config.Interval = DefaultRecomputeInterval
	}
	if config.Logger == nil {
		config.Logger = slog.Default()
	}

	return &RecomputeJob{
		config:       config,
		dirtyTracker: dirtyTracker,
		dataSource:   dataSource,
		scoreStore:   scoreStore,
	}
}

// Start begins the periodic recompute job.
// Returns immediately; the job runs in a background goroutine.
func (j *RecomputeJob) Start(ctx context.Context) error {
	j.mu.Lock()
	if j.running {
		j.mu.Unlock()
		return nil
	}
	j.running = true
	j.stopCh = make(chan struct{})
	j.doneCh = make(chan struct{})
	j.mu.Unlock()

	go j.run(ctx)
	return nil
}

// Stop signals the recompute job to stop and waits for it to finish.
func (j *RecomputeJob) Stop() {
	j.mu.Lock()
	if !j.running {
		j.mu.Unlock()
		return
	}
	j.mu.Unlock()

	close(j.stopCh)
	<-j.doneCh

	j.mu.Lock()
	j.running = false
	j.mu.Unlock()
}

// IsRunning returns whether the job is currently running.
func (j *RecomputeJob) IsRunning() bool {
	j.mu.Lock()
	defer j.mu.Unlock()
	return j.running
}

// run is the main loop for the recompute job.
func (j *RecomputeJob) run(ctx context.Context) {
	defer close(j.doneCh)

	ticker := time.NewTicker(j.config.Interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			j.config.Logger.Info("trust recompute job stopping due to context cancellation")
			return
		case <-j.stopCh:
			j.config.Logger.Info("trust recompute job stopping due to stop signal")
			return
		case <-ticker.C:
			j.recomputeDirtyScenes()
		}
	}
}

// recomputeDirtyScenes processes all dirty scenes and updates their trust scores.
func (j *RecomputeJob) recomputeDirtyScenes() {
	dirtyScenes := j.dirtyTracker.GetDirtyScenes()
	if len(dirtyScenes) == 0 {
		return
	}

	j.config.Logger.Info("recomputing trust scores",
		"dirty_count", len(dirtyScenes))

	for _, sceneID := range dirtyScenes {
		if err := j.recomputeScene(sceneID); err != nil {
			j.config.Logger.Error("failed to recompute trust score",
				"scene_id", sceneID,
				"error", err)
			continue
		}
		j.dirtyTracker.ClearDirty(sceneID)
	}
}

// recomputeScene calculates and stores the trust score for a single scene.
func (j *RecomputeJob) recomputeScene(sceneID string) error {
	memberships, err := j.dataSource.GetMembershipsByScene(sceneID)
	if err != nil {
		return err
	}

	alliances, err := j.dataSource.GetAlliancesByScene(sceneID)
	if err != nil {
		return err
	}

	score := ComputeTrustScore(memberships, alliances)

	trustScore := SceneTrustScore{
		SceneID:    sceneID,
		Score:      score,
		ComputedAt: time.Now(),
	}

	if err := j.scoreStore.SaveScore(trustScore); err != nil {
		return err
	}

	j.config.Logger.Debug("trust score recomputed",
		"scene_id", sceneID,
		"score", score,
		"memberships", len(memberships),
		"alliances", len(alliances))

	return nil
}

// RecomputeNow immediately recomputes all dirty scenes without waiting for the ticker.
// This is useful for testing or forcing immediate updates.
func (j *RecomputeJob) RecomputeNow() {
	j.recomputeDirtyScenes()
}
