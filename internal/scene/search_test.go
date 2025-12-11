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
