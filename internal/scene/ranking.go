// Package scene provides models and repository for managing scenes and events
// with location privacy controls.
package scene

import (
	"math"
	"strings"
	"time"
)

// EventRankingWeights defines the weights for different ranking components.
// These weights determine how much each factor contributes to the final composite score.
//
// The default formula is optimized for event discovery:
// - Text match (40%): Prioritizes query relevance for targeted search
// - Recency (30%): Favors events happening sooner for timely discovery
// - Proximity (20%): Considers geographic convenience
// - Trust (10%): Adds scene reputation signal without dominating results
//
// Formula: composite_score = (recency * 0.3) + (text * 0.4) + (proximity * 0.2) + (trust * 0.1)
//
// Note: When trust ranking is disabled via feature flag, trust weight is effectively 0,
// and the remaining components are normalized to sum to 0.9.
type EventRankingWeights struct {
	Recency   float64 // Time recency weight (default: 0.3)
	TextMatch float64 // Text match weight (default: 0.4)
	Proximity float64 // Proximity/geo weight (default: 0.2)
	Trust     float64 // Trust score weight (default: 0.1)
}

// DefaultEventRankingWeights returns the default ranking weights for event search.
var DefaultEventRankingWeights = EventRankingWeights{
	Recency:   0.3,
	TextMatch: 0.4,
	Proximity: 0.2,
	Trust:     0.1,
}

// CalculateRecencyWeight computes the time recency weight for an event.
// Formula: 1 - ((event_start - now) / window_span) clamped to [0, 1]
// 
// Parameters:
//   - eventStart: The start time of the event
//   - now: Current time (reference point)
//   - windowSpan: The total time window duration (from - to)
//
// Returns a value between 0.0 (furthest in future) and 1.0 (happening now/past)
func CalculateRecencyWeight(eventStart time.Time, now time.Time, windowSpan time.Duration) float64 {
	if windowSpan <= 0 {
		return 1.0 // If no window span, consider all events equally recent
	}

	// Calculate time difference from now to event start
	timeDiff := eventStart.Sub(now)
	
	// If event is in the past or happening now, it's maximally recent
	if timeDiff <= 0 {
		return 1.0
	}

	// Calculate weight: 1 - (timeDiff / windowSpan)
	weight := 1.0 - (float64(timeDiff) / float64(windowSpan))
	
	// Clamp to [0, 1] range
	if weight < 0.0 {
		return 0.0
	}
	if weight > 1.0 {
		return 1.0
	}
	
	return weight
}

// CalculateTextMatchScore computes a simple text match score based on query presence.
// Returns 1.0 if query matches, 0.0 otherwise.
// For in-memory implementation, we use simple case-insensitive substring matching.
// In a real PostGIS implementation, this would use ts_rank with tsvector.
func CalculateTextMatchScore(event *Event, query string) float64 {
	if query == "" {
		return 1.0 // No query means all events match equally
	}
	
	queryLower := strings.ToLower(query)
	
	// Check title
	if strings.Contains(strings.ToLower(event.Title), queryLower) {
		return 1.0
	}
	
	// Check description
	if strings.Contains(strings.ToLower(event.Description), queryLower) {
		return 0.8 // Slightly lower weight for description match
	}
	
	// Check tags
	for _, tag := range event.Tags {
		if strings.Contains(strings.ToLower(tag), queryLower) {
			return 0.6 // Lower weight for tag match
		}
	}
	
	return 0.0 // No match
}

// CalculateProximityScore computes a distance-based proximity score.
// Returns a value between 0.0 (far) and 1.0 (close) based on distance from reference point.
// For simplicity in in-memory implementation, we use the center of the bbox as reference.
// In a real PostGIS implementation, this would use ST_Distance with proper geo calculations.
func CalculateProximityScore(event *Event, centerLat, centerLng float64) float64 {
	if event.PrecisePoint == nil {
		return 0.5 // Default score if no location
	}
	
	// Calculate simple Euclidean distance (not great circle, but good enough for in-memory)
	// For production with PostGIS, use ST_Distance with proper geodesic calculations
	latDiff := event.PrecisePoint.Lat - centerLat
	lngDiff := event.PrecisePoint.Lng - centerLng
	distance := math.Sqrt(latDiff*latDiff + lngDiff*lngDiff)
	
	// Normalize distance to [0, 1] range
	// Use a simple decay function: 1 / (1 + distance)
	// This gives 1.0 for distance=0, 0.5 for distance=1, etc.
	score := 1.0 / (1.0 + distance)
	
	return score
}

// CalculateCompositeScore computes the final composite ranking score for an event.
// Combines recency, text match, proximity, and optional trust scores using configured weights.
func CalculateCompositeScore(
	recencyWeight float64,
	textMatchScore float64,
	proximityScore float64,
	trustScore float64,
	weights EventRankingWeights,
	includeTrust bool,
) float64 {
	score := (recencyWeight * weights.Recency) +
		(textMatchScore * weights.TextMatch) +
		(proximityScore * weights.Proximity)
	
	if includeTrust {
		score += trustScore * weights.Trust
	}
	
	return score
}
