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
	// Metrics for performance tracking.
	Metrics *Metrics
	// JobMetrics for centralized background job tracking.
	JobMetrics JobMetrics
	// Timeout for each recompute cycle.
	Timeout time.Duration
}

// JobMetrics provides centralized background job metrics tracking.
// This interface allows the job to report to the centralized job metrics system.
type JobMetrics interface {
	IncJobsTotal(jobType, status string)
	ObserveJobDuration(jobType string, seconds float64)
	IncJobErrors(jobType, errorType string)
}

// DefaultRecomputeInterval is the default interval between recompute cycles.
const DefaultRecomputeInterval = 30 * time.Second

// DefaultRecomputeTimeout is the default timeout for a single recompute cycle.
const DefaultRecomputeTimeout = 30 * time.Second

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
	if config.Timeout == 0 {
		config.Timeout = DefaultRecomputeTimeout
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
	stopCh := j.stopCh
	doneCh := j.doneCh
	j.mu.Unlock()

	close(stopCh)
	<-doneCh

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
			j.recomputeDirtyScenes(ctx)
		}
	}
}

// recomputeDirtyScenes processes all dirty scenes and updates their trust scores.
func (j *RecomputeJob) recomputeDirtyScenes(parentCtx context.Context) {
	dirtyScenes := j.dirtyTracker.GetDirtyScenes()
	if len(dirtyScenes) == 0 {
		return
	}

	// Create context with timeout derived from parent
	ctx, cancel := context.WithTimeout(parentCtx, j.config.Timeout)
	defer cancel()

	// Track metrics
	startTime := time.Now()
	sceneCount := len(dirtyScenes)
	var successCount int
	var varianceSum float64
	var varianceCount int

	j.config.Logger.Info("recomputing trust scores",
		"dirty_count", sceneCount)

	// Process each scene
	for i, sceneID := range dirtyScenes {
		// Check timeout
		select {
		case <-ctx.Done():
			j.config.Logger.Error("trust recompute timeout exceeded",
				"processed", i,
				"total", sceneCount,
				"timeout", j.config.Timeout)
			if j.config.Metrics != nil {
				j.config.Metrics.IncRecomputeErrors()
			}
			if j.config.JobMetrics != nil {
				j.config.JobMetrics.IncJobErrors("trust_recompute", "timeout")
			}
			
			// Record job completion metrics even for timeout
			duration := time.Since(startTime).Seconds()
			if j.config.Metrics != nil {
				j.config.Metrics.ObserveRecomputeDuration(duration)
			}
			if j.config.JobMetrics != nil {
				j.config.JobMetrics.IncJobsTotal("trust_recompute", "failure")
				j.config.JobMetrics.ObserveJobDuration("trust_recompute", duration)
			}
			return
		default:
		}

		// Get previous score for variance calculation
		var previousScore float64
		var hasPreviousScore bool
		if prevScoreObj, err := j.scoreStore.GetScore(sceneID); err == nil && prevScoreObj != nil {
			previousScore = prevScoreObj.Score
			hasPreviousScore = true
		}

		// Recompute scene
		newScore, err := j.recomputeSceneWithScore(sceneID)
		if err != nil {
			j.config.Logger.Error("failed to recompute trust score",
				"scene_id", sceneID,
				"error", err)
			if j.config.Metrics != nil {
				j.config.Metrics.IncRecomputeErrors()
			}
			if j.config.JobMetrics != nil {
				j.config.JobMetrics.IncJobErrors("trust_recompute", "recompute_error")
			}
			continue
		}

		// Calculate variance for this scene
		if hasPreviousScore {
			variance := abs(newScore - previousScore)
			varianceSum += variance
			varianceCount++
		}

		j.dirtyTracker.ClearDirty(sceneID)
		successCount++

		// Log batch progress every 10 scenes
		if (i+1)%10 == 0 {
			j.config.Logger.Debug("recompute progress",
				"processed", i+1,
				"total", sceneCount)
		}
	}

	// Calculate metrics
	duration := time.Since(startTime).Seconds()
	avgVariance := 0.0
	if varianceCount > 0 {
		avgVariance = varianceSum / float64(varianceCount)
	}

	// Determine job status
	status := "success"
	if successCount < sceneCount {
		status = "failure"
	}

	// Update metrics
	if j.config.Metrics != nil {
		j.config.Metrics.IncRecomputeTotal()
		j.config.Metrics.ObserveRecomputeDuration(duration)
		j.config.Metrics.SetLastRecomputeTimestamp(float64(time.Now().Unix()))
		j.config.Metrics.SetLastRecomputeSceneCount(float64(successCount))
	}

	// Update centralized job metrics
	if j.config.JobMetrics != nil {
		j.config.JobMetrics.IncJobsTotal("trust_recompute", status)
		j.config.JobMetrics.ObserveJobDuration("trust_recompute", duration)
	}

	// Completion log with required fields
	j.config.Logger.Info("trust recompute completed",
		"duration_seconds", duration,
		"scenes_processed", successCount,
		"scenes_failed", sceneCount-successCount,
		"avg_weight_variance", avgVariance)
}

// abs returns the absolute value of a float64.
func abs(x float64) float64 {
	if x < 0 {
		return -x
	}
	return x
}

// recomputeSceneWithScore calculates and stores the trust score for a single scene.
// Returns the new score for variance calculation.
func (j *RecomputeJob) recomputeSceneWithScore(sceneID string) (float64, error) {
	memberships, err := j.dataSource.GetMembershipsByScene(sceneID)
	if err != nil {
		return 0, err
	}

	alliances, err := j.dataSource.GetAlliancesByScene(sceneID)
	if err != nil {
		return 0, err
	}

	score := ComputeTrustScore(memberships, alliances)

	trustScore := SceneTrustScore{
		SceneID:    sceneID,
		Score:      score,
		ComputedAt: time.Now(),
	}

	if err := j.scoreStore.SaveScore(trustScore); err != nil {
		return 0, err
	}

	j.config.Logger.Debug("trust score recomputed",
		"scene_id", sceneID,
		"score", score,
		"memberships", len(memberships),
		"alliances", len(alliances))

	return score, nil
}

// RecomputeNow immediately recomputes all dirty scenes without waiting for the ticker.
// This is useful for testing or forcing immediate updates.
func (j *RecomputeJob) RecomputeNow() {
	j.recomputeDirtyScenes(context.Background())
}
