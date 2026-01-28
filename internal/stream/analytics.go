// Package stream provides analytics models and computation for stream sessions.
package stream

import (
	"time"
)

// ParticipantEvent represents a single join or leave event by a participant.
type ParticipantEvent struct {
	ID              string    `json:"id"`
	StreamSessionID string    `json:"stream_session_id"`
	ParticipantDID  string    `json:"participant_did"`
	EventType       string    `json:"event_type"` // "join" or "leave"
	GeohashPrefix   *string   `json:"geohash_prefix,omitempty"` // 4-char prefix for privacy-safe geo distribution
	OccurredAt      time.Time `json:"occurred_at"`
}

// Analytics represents computed analytics for a stream session.
// All geographic data is aggregated to preserve privacy (no precise locations or PII).
type Analytics struct {
	ID                          string             `json:"id"`
	StreamSessionID             string             `json:"stream_session_id"`
	
	// Core engagement metrics
	PeakConcurrentListeners     int                `json:"peak_concurrent_listeners"`
	TotalUniqueParticipants     int                `json:"total_unique_participants"`
	TotalJoinAttempts           int                `json:"total_join_attempts"`
	
	// Timing metrics
	StreamDurationSeconds       int                `json:"stream_duration_seconds"`
	EngagementLagSeconds        *int               `json:"engagement_lag_seconds,omitempty"` // NULL if no joins
	
	// Retention metrics
	AvgListenDurationSeconds    *float64           `json:"avg_listen_duration_seconds,omitempty"`
	MedianListenDurationSeconds *float64           `json:"median_listen_duration_seconds,omitempty"`
	
	// Geographic distribution (privacy-safe aggregate)
	// Map of 4-char geohash prefix -> count
	GeographicDistribution      map[string]int     `json:"geographic_distribution"`
	
	ComputedAt                  time.Time          `json:"computed_at"`
}

// AnalyticsRepository defines the interface for analytics data operations.
type AnalyticsRepository interface {
	// RecordParticipantEvent records a join or leave event for a participant.
	// geohashPrefix should be a 4-character prefix for privacy-safe geographic tracking (optional).
	RecordParticipantEvent(streamSessionID, participantDID, eventType string, geohashPrefix *string) error
	
	// GetParticipantEvents retrieves all participant events for a stream session, ordered by occurred_at.
	GetParticipantEvents(streamSessionID string) ([]*ParticipantEvent, error)
	
	// ComputeAnalytics calculates and stores analytics for a stream session.
	// Should be called when a stream ends. Returns the computed analytics.
	ComputeAnalytics(streamSessionID string) (*Analytics, error)
	
	// GetAnalytics retrieves the computed analytics for a stream session.
	// Returns nil if analytics have not been computed yet.
	GetAnalytics(streamSessionID string) (*Analytics, error)
}
