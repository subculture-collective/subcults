package scene

import (
	"math"
	"testing"
	"time"
)

// TestCalculateRecencyWeight tests the recency weight calculation.
func TestCalculateRecencyWeight(t *testing.T) {
	now := time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC)
	windowSpan := 24 * time.Hour // 24 hour window

	tests := []struct {
		name        string
		eventStart  time.Time
		expectedMin float64
		expectedMax float64
	}{
		{
			name:        "event in the past",
			eventStart:  now.Add(-1 * time.Hour),
			expectedMin: 1.0,
			expectedMax: 1.0,
		},
		{
			name:        "event happening now",
			eventStart:  now,
			expectedMin: 1.0,
			expectedMax: 1.0,
		},
		{
			name:        "event 6 hours in future (75% through window)",
			eventStart:  now.Add(6 * time.Hour),
			expectedMin: 0.74,
			expectedMax: 0.76,
		},
		{
			name:        "event 12 hours in future (50% through window)",
			eventStart:  now.Add(12 * time.Hour),
			expectedMin: 0.49,
			expectedMax: 0.51,
		},
		{
			name:        "event 18 hours in future (25% through window)",
			eventStart:  now.Add(18 * time.Hour),
			expectedMin: 0.24,
			expectedMax: 0.26,
		},
		{
			name:        "event 24 hours in future (end of window)",
			eventStart:  now.Add(24 * time.Hour),
			expectedMin: 0.0,
			expectedMax: 0.01,
		},
		{
			name:        "event beyond window",
			eventStart:  now.Add(30 * time.Hour),
			expectedMin: 0.0,
			expectedMax: 0.0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			weight := CalculateRecencyWeight(tt.eventStart, now, windowSpan)

			if weight < tt.expectedMin || weight > tt.expectedMax {
				t.Errorf("expected weight in range [%f, %f], got %f",
					tt.expectedMin, tt.expectedMax, weight)
			}

			// Verify weight is clamped to [0, 1]
			if weight < 0.0 || weight > 1.0 {
				t.Errorf("weight %f is outside valid range [0.0, 1.0]", weight)
			}
		})
	}
}

// TestCalculateRecencyWeight_ZeroWindowSpan tests edge case of zero window span.
func TestCalculateRecencyWeight_ZeroWindowSpan(t *testing.T) {
	now := time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC)
	eventStart := now.Add(1 * time.Hour)

	weight := CalculateRecencyWeight(eventStart, now, 0)

	if weight != 1.0 {
		t.Errorf("expected weight 1.0 for zero window span, got %f", weight)
	}
}

// TestCalculateTextMatchScore tests text matching for events.
func TestCalculateTextMatchScore(t *testing.T) {
	event := &Event{
		Title:       "Electronic Music Night",
		Description: "Join us for an amazing techno party",
		Tags:        []string{"electronic", "techno", "underground"},
	}

	tests := []struct {
		name          string
		query         string
		expectedScore float64
	}{
		{
			name:          "empty query",
			query:         "",
			expectedScore: 1.0,
		},
		{
			name:          "exact title match",
			query:         "electronic music",
			expectedScore: 1.0,
		},
		{
			name:          "case insensitive title match",
			query:         "ELECTRONIC",
			expectedScore: 1.0,
		},
		{
			name:          "partial title match",
			query:         "music",
			expectedScore: 1.0,
		},
		{
			name:          "description match",
			query:         "techno party",
			expectedScore: 0.8,
		},
		{
			name:          "tag match",
			query:         "underground",
			expectedScore: 0.6,
		},
		{
			name:          "no match",
			query:         "jazz",
			expectedScore: 0.0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			score := CalculateTextMatchScore(event, tt.query)
			if score != tt.expectedScore {
				t.Errorf("expected score %f, got %f", tt.expectedScore, score)
			}
		})
	}
}

// TestCalculateProximityScore tests distance-based proximity scoring.
func TestCalculateProximityScore(t *testing.T) {
	// Center point (NYC)
	centerLat := 40.7128
	centerLng := -74.0060

	tests := []struct {
		name        string
		event       *Event
		expectedMin float64
	}{
		{
			name: "event at center point",
			event: &Event{
				PrecisePoint: &Point{Lat: centerLat, Lng: centerLng},
			},
			expectedMin: 0.99, // Should be very close to 1.0
		},
		{
			name: "event nearby (0.01 degrees away)",
			event: &Event{
				PrecisePoint: &Point{Lat: centerLat + 0.01, Lng: centerLng + 0.01},
			},
			expectedMin: 0.95, // Still very high score
		},
		{
			name: "event further away (0.1 degrees away)",
			event: &Event{
				PrecisePoint: &Point{Lat: centerLat + 0.1, Lng: centerLng + 0.1},
			},
			expectedMin: 0.8, // Moderate score
		},
		{
			name: "event without location",
			event: &Event{
				PrecisePoint: nil,
			},
			expectedMin: 0.49, // Default score
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			score := CalculateProximityScore(tt.event, centerLat, centerLng)

			if score < tt.expectedMin {
				t.Errorf("expected score >= %f, got %f", tt.expectedMin, score)
			}

			// Verify score is in [0, 1]
			if score < 0.0 || score > 1.0 {
				t.Errorf("score %f is outside valid range [0.0, 1.0]", score)
			}
		})
	}
}

// TestCalculateCompositeScore tests the composite ranking score calculation.
func TestCalculateCompositeScore(t *testing.T) {
	weights := DefaultEventRankingWeights

	tests := []struct {
		name         string
		recency      float64
		textMatch    float64
		proximity    float64
		trust        float64
		includeTrust bool
		expected     float64
	}{
		{
			name:         "all perfect scores without trust",
			recency:      1.0,
			textMatch:    1.0,
			proximity:    1.0,
			trust:        1.0,
			includeTrust: false,
			expected:     0.9, // 0.3 + 0.4 + 0.2 = 0.9
		},
		{
			name:         "all perfect scores with trust",
			recency:      1.0,
			textMatch:    1.0,
			proximity:    1.0,
			trust:        1.0,
			includeTrust: true,
			expected:     1.0, // 0.3 + 0.4 + 0.2 + 0.1 = 1.0
		},
		{
			name:         "all zero scores",
			recency:      0.0,
			textMatch:    0.0,
			proximity:    0.0,
			trust:        0.0,
			includeTrust: false,
			expected:     0.0,
		},
		{
			name:         "mixed scores without trust",
			recency:      0.5,
			textMatch:    0.8,
			proximity:    0.6,
			trust:        0.7,
			includeTrust: false,
			expected:     0.59, // (0.5*0.3) + (0.8*0.4) + (0.6*0.2) = 0.59
		},
		{
			name:         "mixed scores with trust",
			recency:      0.5,
			textMatch:    0.8,
			proximity:    0.6,
			trust:        0.7,
			includeTrust: true,
			expected:     0.66, // (0.5*0.3) + (0.8*0.4) + (0.6*0.2) + (0.7*0.1) = 0.66
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			score := CalculateCompositeScore(
				tt.recency,
				tt.textMatch,
				tt.proximity,
				tt.trust,
				weights,
				tt.includeTrust,
			)

			// Allow small floating point tolerance
			if math.Abs(score-tt.expected) > 0.001 {
				t.Errorf("expected score %f, got %f", tt.expected, score)
			}
		})
	}
}

// TestCalculateCompositeScore_CustomWeights tests custom weight configuration.
func TestCalculateCompositeScore_CustomWeights(t *testing.T) {
	// Custom weights that sum to 1.0
	customWeights := EventRankingWeights{
		Recency:   0.5,
		TextMatch: 0.3,
		Proximity: 0.1,
		Trust:     0.1,
	}

	score := CalculateCompositeScore(1.0, 1.0, 1.0, 1.0, customWeights, true)

	if math.Abs(score-1.0) > 0.001 {
		t.Errorf("expected score 1.0 with custom weights, got %f", score)
	}
}
