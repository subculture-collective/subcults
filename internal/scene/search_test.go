package scene

import (
	"fmt"
	"testing"
	"time"

	"github.com/google/uuid"
)

// TestSearchByBboxAndTime_Pagination tests that cursor pagination works correctly.
func TestSearchByBboxAndTime_Pagination(t *testing.T) {
	repo := NewInMemoryEventRepository()
	
	baseTime := time.Now().Add(24 * time.Hour)
	
	// Create 3 events at different times
	events := make([]*Event, 3)
	for i := 0; i < 3; i++ {
		event := &Event{
			ID:            uuid.New().String(),
			SceneID:       uuid.New().String(),
			Title:         fmt.Sprintf("Event %d", i+1),
			AllowPrecise:  true,
			PrecisePoint:  &Point{Lat: 40.7128, Lng: -74.0060},
			CoarseGeohash: "dr5regw",
			Status:        "scheduled",
			StartsAt:      baseTime.Add(time.Duration(i) * time.Hour),
			CreatedAt:     &baseTime,
			UpdatedAt:     &baseTime,
		}
		if err := repo.Insert(event); err != nil {
			t.Fatalf("failed to insert event: %v", err)
		}
		events[i] = event
	}
	
	from := baseTime.Add(-1 * time.Hour)
	to := baseTime.Add(6 * time.Hour)
	
	// Get first page (limit=2)
	results1, cursor1, err := repo.SearchByBboxAndTime(-74.1, 40.6, -73.9, 40.8, from, to, 2, "")
	if err != nil {
		t.Fatalf("failed to search: %v", err)
	}
	
	t.Logf("Page 1: %d events, cursor=%s", len(results1), cursor1)
	for i, e := range results1 {
		t.Logf("  Event %d: ID=%s, StartsAt=%s", i, e.ID, e.StartsAt.Format(time.RFC3339))
	}
	
	if len(results1) != 2 {
		t.Errorf("expected 2 events in page 1, got %d", len(results1))
	}
	
	if cursor1 == "" {
		t.Fatal("expected cursor1 to be set")
	}
	
	// Get second page with cursor
	results2, cursor2, err := repo.SearchByBboxAndTime(-74.1, 40.6, -73.9, 40.8, from, to, 2, cursor1)
	if err != nil {
		t.Fatalf("failed to search with cursor: %v", err)
	}
	
	t.Logf("Page 2: %d events, cursor=%s", len(results2), cursor2)
	for i, e := range results2 {
		t.Logf("  Event %d: ID=%s, StartsAt=%s", i, e.ID, e.StartsAt.Format(time.RFC3339))
	}
	
	if len(results2) != 1 {
		t.Errorf("expected 1 event in page 2, got %d", len(results2))
	}
	
	// Check for duplicates between pages
	seenIDs := make(map[string]bool)
	for _, e := range results1 {
		seenIDs[e.ID] = true
	}
	for _, e := range results2 {
		if seenIDs[e.ID] {
			t.Errorf("duplicate event ID %s found in both pages", e.ID)
		}
	}
	
	// Verify ordering
	allResults := append(results1, results2...)
	for i := 0; i < len(allResults)-1; i++ {
		if allResults[i].StartsAt.After(allResults[i+1].StartsAt) {
			t.Error("events are not sorted by starts_at")
		}
	}
}

// TestSearchEvents_TextSearch tests text search filtering.
func TestSearchEvents_TextSearch(t *testing.T) {
repo := NewInMemoryEventRepository()

baseTime := time.Now().Add(24 * time.Hour)

// Create events with different titles
event1 := &Event{
ID:            uuid.New().String(),
SceneID:       uuid.New().String(),
Title:         "Electronic Music Night",
Description:   "Amazing techno party",
AllowPrecise:  true,
PrecisePoint:  &Point{Lat: 40.7128, Lng: -74.0060},
CoarseGeohash: "dr5regw",
Status:        "scheduled",
StartsAt:      baseTime.Add(1 * time.Hour),
CreatedAt:     &baseTime,
UpdatedAt:     &baseTime,
}

event2 := &Event{
ID:            uuid.New().String(),
SceneID:       uuid.New().String(),
Title:         "Jazz Festival",
Description:   "Live jazz performances",
AllowPrecise:  true,
PrecisePoint:  &Point{Lat: 40.7128, Lng: -74.0060},
CoarseGeohash: "dr5regw",
Status:        "scheduled",
StartsAt:      baseTime.Add(2 * time.Hour),
CreatedAt:     &baseTime,
UpdatedAt:     &baseTime,
}

event3 := &Event{
ID:            uuid.New().String(),
SceneID:       uuid.New().String(),
Title:         "Rock Concert",
Description:   "Electronic rock fusion",
AllowPrecise:  true,
PrecisePoint:  &Point{Lat: 40.7128, Lng: -74.0060},
CoarseGeohash: "dr5regw",
Status:        "scheduled",
StartsAt:      baseTime.Add(3 * time.Hour),
CreatedAt:     &baseTime,
UpdatedAt:     &baseTime,
}

// Insert events
if err := repo.Insert(event1); err != nil {
t.Fatalf("failed to insert event1: %v", err)
}
if err := repo.Insert(event2); err != nil {
t.Fatalf("failed to insert event2: %v", err)
}
if err := repo.Insert(event3); err != nil {
t.Fatalf("failed to insert event3: %v", err)
}

from := baseTime.Add(-1 * time.Hour)
to := baseTime.Add(6 * time.Hour)

// Search with query "electronic" - should match event1 (title) and event3 (description)
results, _, err := repo.SearchEvents(EventSearchOptions{
MinLng: -74.1,
MinLat: 40.6,
MaxLng: -73.9,
MaxLat: 40.8,
From:   from,
To:     to,
Query:  "electronic",
Limit:  10,
})
if err != nil {
t.Fatalf("failed to search: %v", err)
}

// Should return 2 events (event1 and event3)
if len(results) != 2 {
t.Errorf("expected 2 events matching 'electronic', got %d", len(results))
}

// Verify the matched events
foundIDs := make(map[string]bool)
for _, e := range results {
foundIDs[e.ID] = true
}

if !foundIDs[event1.ID] {
t.Error("expected event1 (Electronic Music Night) in results")
}
if !foundIDs[event3.ID] {
t.Error("expected event3 (Rock Concert with electronic in description) in results")
}
if foundIDs[event2.ID] {
t.Error("event2 (Jazz Festival) should not match 'electronic'")
}
}

// TestSearchEvents_Ranking tests that events are ranked by composite score.
func TestSearchEvents_Ranking(t *testing.T) {
repo := NewInMemoryEventRepository()

now := time.Now()
baseTime := now.Add(24 * time.Hour)

// Create events with different characteristics for ranking
// Event 1: Happening sooner (higher recency), exact title match
event1 := &Event{
ID:            uuid.New().String(),
SceneID:       "scene1",
Title:         "Music Festival",
AllowPrecise:  true,
PrecisePoint:  &Point{Lat: 40.7128, Lng: -74.0060}, // Center of bbox
CoarseGeohash: "dr5regw",
Status:        "scheduled",
StartsAt:      baseTime.Add(1 * time.Hour), // Soon
CreatedAt:     &baseTime,
UpdatedAt:     &baseTime,
}

// Event 2: Happening later (lower recency), title match
event2 := &Event{
ID:            uuid.New().String(),
SceneID:       "scene2",
Title:         "Music Concert",
AllowPrecise:  true,
PrecisePoint:  &Point{Lat: 40.7128, Lng: -74.0060},
CoarseGeohash: "dr5regw",
Status:        "scheduled",
StartsAt:      baseTime.Add(10 * time.Hour), // Later
CreatedAt:     &baseTime,
UpdatedAt:     &baseTime,
}

// Insert events
if err := repo.Insert(event1); err != nil {
t.Fatalf("failed to insert event1: %v", err)
}
if err := repo.Insert(event2); err != nil {
t.Fatalf("failed to insert event2: %v", err)
}

from := baseTime.Add(-1 * time.Hour)
to := baseTime.Add(24 * time.Hour)

// Search with query "music"
results, _, err := repo.SearchEvents(EventSearchOptions{
MinLng: -74.1,
MinLat: 40.6,
MaxLng: -73.9,
MaxLat: 40.8,
From:   from,
To:     to,
Query:  "music",
Limit:  10,
})
if err != nil {
t.Fatalf("failed to search: %v", err)
}

if len(results) != 2 {
t.Fatalf("expected 2 events, got %d", len(results))
}

// Event 1 should rank higher due to better recency
if results[0].ID != event1.ID {
t.Errorf("expected event1 to rank first (better recency), but got %s", results[0].ID)
}
}

// TestSearchEvents_TrustScoreIntegration tests trust score weighting in ranking.
func TestSearchEvents_TrustScoreIntegration(t *testing.T) {
repo := NewInMemoryEventRepository()

now := time.Now()
baseTime := now.Add(24 * time.Hour)

// Create two identical events except for scene
event1 := &Event{
ID:            uuid.New().String(),
SceneID:       "scene-low-trust",
Title:         "Music Event",
AllowPrecise:  true,
PrecisePoint:  &Point{Lat: 40.7128, Lng: -74.0060},
CoarseGeohash: "dr5regw",
Status:        "scheduled",
StartsAt:      baseTime.Add(1 * time.Hour),
CreatedAt:     &baseTime,
UpdatedAt:     &baseTime,
}

event2 := &Event{
ID:            uuid.New().String(),
SceneID:       "scene-high-trust",
Title:         "Music Event",
AllowPrecise:  true,
PrecisePoint:  &Point{Lat: 40.7128, Lng: -74.0060},
CoarseGeohash: "dr5regw",
Status:        "scheduled",
StartsAt:      baseTime.Add(1 * time.Hour), // Same time
CreatedAt:     &baseTime,
UpdatedAt:     &baseTime,
}

// Insert events
if err := repo.Insert(event1); err != nil {
t.Fatalf("failed to insert event1: %v", err)
}
if err := repo.Insert(event2); err != nil {
t.Fatalf("failed to insert event2: %v", err)
}

from := baseTime.Add(-1 * time.Hour)
to := baseTime.Add(24 * time.Hour)

// Search WITHOUT trust scores - should be ordered by ID (stable sort)
resultsWithoutTrust, _, err := repo.SearchEvents(EventSearchOptions{
MinLng: -74.1,
MinLat: 40.6,
MaxLng: -73.9,
MaxLat: 40.8,
From:   from,
To:     to,
Query:  "music",
Limit:  10,
})
if err != nil {
t.Fatalf("failed to search: %v", err)
}

if len(resultsWithoutTrust) != 2 {
t.Fatalf("expected 2 events, got %d", len(resultsWithoutTrust))
}

// Search WITH trust scores - high trust scene should rank first
trustScores := map[string]float64{
"scene-low-trust":  0.3,
"scene-high-trust": 0.9,
}

resultsWithTrust, _, err := repo.SearchEvents(EventSearchOptions{
MinLng:      -74.1,
MinLat:      40.6,
MaxLng:      -73.9,
MaxLat:      40.8,
From:        from,
To:          to,
Query:       "music",
Limit:       10,
TrustScores: trustScores,
})
if err != nil {
t.Fatalf("failed to search with trust: %v", err)
}

if len(resultsWithTrust) != 2 {
t.Fatalf("expected 2 events, got %d", len(resultsWithTrust))
}

// High trust scene should rank first
if resultsWithTrust[0].SceneID != "scene-high-trust" {
t.Errorf("expected high-trust scene to rank first with trust scores, got %s", resultsWithTrust[0].SceneID)
}
}

// TestSearchEvents_CursorPrecision tests that cursor maintains full score precision.
func TestSearchEvents_CursorPrecision(t *testing.T) {
repo := NewInMemoryEventRepository()

now := time.Now()
baseTime := now.Add(24 * time.Hour)

// Create events that will have very similar composite scores
// that might round to the same value with limited precision
for i := 0; i < 5; i++ {
event := &Event{
ID:            uuid.New().String(),
SceneID:       "scene1",
Title:         "Music Event",
AllowPrecise:  true,
PrecisePoint:  &Point{Lat: 40.7128, Lng: -74.0060},
CoarseGeohash: "dr5regw",
Status:        "scheduled",
// Very small time differences to create similar recency scores
StartsAt:      baseTime.Add(time.Duration(i) * time.Millisecond),
CreatedAt:     &baseTime,
UpdatedAt:     &baseTime,
}
if err := repo.Insert(event); err != nil {
t.Fatalf("failed to insert event: %v", err)
}
}

from := baseTime.Add(-1 * time.Hour)
to := baseTime.Add(24 * time.Hour)

// Get first page with limit 2
results1, cursor1, err := repo.SearchEvents(EventSearchOptions{
MinLng: -74.1,
MinLat: 40.6,
MaxLng: -73.9,
MaxLat: 40.8,
From:   from,
To:     to,
Query:  "music",
Limit:  2,
})
if err != nil {
t.Fatalf("failed to search: %v", err)
}

if len(results1) != 2 {
t.Fatalf("expected 2 events in first page, got %d", len(results1))
}

if cursor1 == "" {
t.Fatal("expected cursor1 to be set")
}

// Get second page with cursor
results2, cursor2, err := repo.SearchEvents(EventSearchOptions{
MinLng: -74.1,
MinLat: 40.6,
MaxLng: -73.9,
MaxLat: 40.8,
From:   from,
To:     to,
Query:  "music",
Limit:  2,
Cursor: cursor1,
})
if err != nil {
t.Fatalf("failed to search with cursor: %v", err)
}

if len(results2) != 2 {
t.Fatalf("expected 2 events in second page, got %d", len(results2))
}

// Get third page
results3, _, err := repo.SearchEvents(EventSearchOptions{
MinLng: -74.1,
MinLat: 40.6,
MaxLng: -73.9,
MaxLat: 40.8,
From:   from,
To:     to,
Query:  "music",
Limit:  2,
Cursor: cursor2,
})
if err != nil {
t.Fatalf("failed to search with cursor2: %v", err)
}

if len(results3) != 1 {
t.Fatalf("expected 1 event in third page, got %d", len(results3))
}

// Verify no duplicates across pages
seenIDs := make(map[string]bool)
allResults := append(append(results1, results2...), results3...)
for _, event := range allResults {
if seenIDs[event.ID] {
t.Errorf("duplicate event ID %s found across pages", event.ID)
}
seenIDs[event.ID] = true
}

// Should have all 5 events
if len(seenIDs) != 5 {
t.Errorf("expected 5 unique events across all pages, got %d", len(seenIDs))
}
}
