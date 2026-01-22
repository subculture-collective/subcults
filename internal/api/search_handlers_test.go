package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/onnwee/subcults/internal/audit"
	"github.com/onnwee/subcults/internal/scene"
	"github.com/onnwee/subcults/internal/stream"
)

// TestSearchEvents_Success tests successful event search with bbox and time range.
func TestSearchEvents_Success(t *testing.T) {
	eventRepo := scene.NewInMemoryEventRepository()
	sceneRepo := scene.NewInMemorySceneRepository()
	auditRepo := audit.NewInMemoryRepository()
	rsvpRepo := scene.NewInMemoryRSVPRepository()
	streamRepo := stream.NewInMemorySessionRepository()
	handlers := NewEventHandlers(eventRepo, sceneRepo, auditRepo, rsvpRepo, streamRepo, nil)

	// Create test events at different locations and times
	baseTime := time.Now().Add(24 * time.Hour)
	
	// Event 1: Inside bbox, within time range
	event1 := &scene.Event{
		ID:            uuid.New().String(),
		SceneID:       uuid.New().String(),
		Title:         "Event 1",
		AllowPrecise:  true,
		PrecisePoint:  &scene.Point{Lat: 40.7128, Lng: -74.0060}, // NYC
		CoarseGeohash: "dr5regw",
		Status:        "scheduled",
		StartsAt:      baseTime.Add(1 * time.Hour),
		CreatedAt:     &baseTime,
		UpdatedAt:     &baseTime,
	}
	
	// Event 2: Inside bbox, within time range (later than event1)
	event2 := &scene.Event{
		ID:            uuid.New().String(),
		SceneID:       uuid.New().String(),
		Title:         "Event 2",
		AllowPrecise:  true,
		PrecisePoint:  &scene.Point{Lat: 40.7589, Lng: -73.9851}, // Times Square
		CoarseGeohash: "dr5regw",
		Status:        "scheduled",
		StartsAt:      baseTime.Add(2 * time.Hour),
		CreatedAt:     &baseTime,
		UpdatedAt:     &baseTime,
	}
	
	// Event 3: Outside bbox (should not be returned)
	event3 := &scene.Event{
		ID:            uuid.New().String(),
		SceneID:       uuid.New().String(),
		Title:         "Event 3",
		AllowPrecise:  true,
		PrecisePoint:  &scene.Point{Lat: 34.0522, Lng: -118.2437}, // LA
		CoarseGeohash: "9q5",
		Status:        "scheduled",
		StartsAt:      baseTime.Add(1 * time.Hour),
		CreatedAt:     &baseTime,
		UpdatedAt:     &baseTime,
	}
	
	// Event 4: Inside bbox, but cancelled (should not be returned)
	event4 := &scene.Event{
		ID:            uuid.New().String(),
		SceneID:       uuid.New().String(),
		Title:         "Event 4",
		AllowPrecise:  true,
		PrecisePoint:  &scene.Point{Lat: 40.7128, Lng: -74.0060},
		CoarseGeohash: "dr5regw",
		Status:        "cancelled",
		StartsAt:      baseTime.Add(1 * time.Hour),
		CreatedAt:     &baseTime,
		UpdatedAt:     &baseTime,
		CancelledAt:   &baseTime,
	}
	
	// Insert events
	if err := eventRepo.Insert(event1); err != nil {
		t.Fatalf("failed to insert event1: %v", err)
	}
	if err := eventRepo.Insert(event2); err != nil {
		t.Fatalf("failed to insert event2: %v", err)
	}
	if err := eventRepo.Insert(event3); err != nil {
		t.Fatalf("failed to insert event3: %v", err)
	}
	if err := eventRepo.Insert(event4); err != nil {
		t.Fatalf("failed to insert event4: %v", err)
	}
	
	// Search with bbox covering NYC area
	// minLng=-74.1, minLat=40.6, maxLng=-73.9, maxLat=40.8
	from := baseTime.Format(time.RFC3339)
	to := baseTime.Add(3 * time.Hour).Format(time.RFC3339)
	
	url := fmt.Sprintf("/search/events?bbox=-74.1,40.6,-73.9,40.8&from=%s&to=%s&limit=10", from, to)
	req := httptest.NewRequest(http.MethodGet, url, nil)
	w := httptest.NewRecorder()
	
	handlers.SearchEvents(w, req)
	
	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}
	
	var response SearchEventsResponse
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	
	// Should return 2 events (event1 and event2), not event3 (outside bbox) or event4 (cancelled)
	if len(response.Events) != 2 {
		t.Errorf("expected 2 events, got %d", len(response.Events))
	}
	
	// Verify sorting: event1 should come before event2 (earlier start time)
	if len(response.Events) >= 2 {
		if !response.Events[0].StartsAt.Before(response.Events[1].StartsAt) {
			t.Error("events should be sorted by starts_at ascending")
		}
	}
}

// TestSearchEvents_BboxValidation tests bbox parameter validation.
func TestSearchEvents_BboxValidation(t *testing.T) {
	eventRepo := scene.NewInMemoryEventRepository()
	sceneRepo := scene.NewInMemorySceneRepository()
	auditRepo := audit.NewInMemoryRepository()
	rsvpRepo := scene.NewInMemoryRSVPRepository()
	streamRepo := stream.NewInMemorySessionRepository()
	handlers := NewEventHandlers(eventRepo, sceneRepo, auditRepo, rsvpRepo, streamRepo, nil)
	
	baseTime := time.Now().Add(24 * time.Hour)
	from := baseTime.Format(time.RFC3339)
	to := baseTime.Add(1 * time.Hour).Format(time.RFC3339)
	
	tests := []struct {
		name       string
		bbox       string
		wantStatus int
		wantCode   string
	}{
		{
			name:       "missing bbox",
			bbox:       "",
			wantStatus: http.StatusBadRequest,
			wantCode:   ErrCodeValidation,
		},
		{
			name:       "invalid bbox format",
			bbox:       "-74.1,40.6,-73.9", // missing one coordinate
			wantStatus: http.StatusBadRequest,
			wantCode:   ErrCodeValidation,
		},
		{
			name:       "invalid longitude",
			bbox:       "-200,40.6,-73.9,40.8", // lng out of range
			wantStatus: http.StatusBadRequest,
			wantCode:   ErrCodeValidation,
		},
		{
			name:       "invalid latitude",
			bbox:       "-74.1,100,-73.9,40.8", // lat out of range
			wantStatus: http.StatusBadRequest,
			wantCode:   ErrCodeValidation,
		},
		{
			name:       "minLng >= maxLng",
			bbox:       "-73.9,40.6,-74.1,40.8", // minLng > maxLng
			wantStatus: http.StatusBadRequest,
			wantCode:   ErrCodeValidation,
		},
		{
			name:       "minLat >= maxLat",
			bbox:       "-74.1,40.8,-73.9,40.6", // minLat > maxLat
			wantStatus: http.StatusBadRequest,
			wantCode:   ErrCodeValidation,
		},
		{
			name:       "non-numeric coordinate",
			bbox:       "invalid,40.6,-73.9,40.8",
			wantStatus: http.StatusBadRequest,
			wantCode:   ErrCodeValidation,
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			url := fmt.Sprintf("/search/events?bbox=%s&from=%s&to=%s", tt.bbox, from, to)
			req := httptest.NewRequest(http.MethodGet, url, nil)
			w := httptest.NewRecorder()
			
			handlers.SearchEvents(w, req)
			
			if w.Code != tt.wantStatus {
				t.Errorf("expected status %d, got %d", tt.wantStatus, w.Code)
			}
			
			var errResp ErrorResponse
			if err := json.NewDecoder(w.Body).Decode(&errResp); err != nil {
				t.Fatalf("failed to decode error response: %v", err)
			}
			
			if errResp.Error.Code != tt.wantCode {
				t.Errorf("expected error code %s, got %s", tt.wantCode, errResp.Error.Code)
			}
		})
	}
}

// TestSearchEvents_TimeRangeValidation tests time range parameter validation.
func TestSearchEvents_TimeRangeValidation(t *testing.T) {
	eventRepo := scene.NewInMemoryEventRepository()
	sceneRepo := scene.NewInMemorySceneRepository()
	auditRepo := audit.NewInMemoryRepository()
	rsvpRepo := scene.NewInMemoryRSVPRepository()
	streamRepo := stream.NewInMemorySessionRepository()
	handlers := NewEventHandlers(eventRepo, sceneRepo, auditRepo, rsvpRepo, streamRepo, nil)
	
	baseTime := time.Now().Add(24 * time.Hour)
	
	tests := []struct {
		name       string
		from       string
		to         string
		wantStatus int
		wantCode   string
	}{
		{
			name:       "missing from",
			from:       "",
			to:         baseTime.Format(time.RFC3339),
			wantStatus: http.StatusBadRequest,
			wantCode:   ErrCodeValidation,
		},
		{
			name:       "missing to",
			from:       baseTime.Format(time.RFC3339),
			to:         "",
			wantStatus: http.StatusBadRequest,
			wantCode:   ErrCodeValidation,
		},
		{
			name:       "invalid from format",
			from:       "2024-01-01",
			to:         baseTime.Format(time.RFC3339),
			wantStatus: http.StatusBadRequest,
			wantCode:   ErrCodeValidation,
		},
		{
			name:       "invalid to format",
			from:       baseTime.Format(time.RFC3339),
			to:         "invalid",
			wantStatus: http.StatusBadRequest,
			wantCode:   ErrCodeValidation,
		},
		{
			name:       "from after to",
			from:       baseTime.Add(2 * time.Hour).Format(time.RFC3339),
			to:         baseTime.Format(time.RFC3339),
			wantStatus: http.StatusBadRequest,
			wantCode:   ErrCodeInvalidTimeRange,
		},
		{
			name:       "from equals to",
			from:       baseTime.Format(time.RFC3339),
			to:         baseTime.Format(time.RFC3339),
			wantStatus: http.StatusBadRequest,
			wantCode:   ErrCodeInvalidTimeRange,
		},
		{
			name:       "window exceeds 30 days",
			from:       baseTime.Format(time.RFC3339),
			to:         baseTime.Add(31 * 24 * time.Hour).Format(time.RFC3339),
			wantStatus: http.StatusBadRequest,
			wantCode:   ErrCodeValidation,
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			url := fmt.Sprintf("/search/events?bbox=-74.1,40.6,-73.9,40.8&from=%s&to=%s", tt.from, tt.to)
			req := httptest.NewRequest(http.MethodGet, url, nil)
			w := httptest.NewRecorder()
			
			handlers.SearchEvents(w, req)
			
			if w.Code != tt.wantStatus {
				t.Errorf("expected status %d, got %d: %s", tt.wantStatus, w.Code, w.Body.String())
			}
			
			var errResp ErrorResponse
			if err := json.NewDecoder(w.Body).Decode(&errResp); err != nil {
				t.Fatalf("failed to decode error response: %v", err)
			}
			
			if errResp.Error.Code != tt.wantCode {
				t.Errorf("expected error code %s, got %s", tt.wantCode, errResp.Error.Code)
			}
		})
	}
}

// TestSearchEvents_Pagination tests cursor-based pagination.
func TestSearchEvents_Pagination(t *testing.T) {
	eventRepo := scene.NewInMemoryEventRepository()
	sceneRepo := scene.NewInMemorySceneRepository()
	auditRepo := audit.NewInMemoryRepository()
	rsvpRepo := scene.NewInMemoryRSVPRepository()
	streamRepo := stream.NewInMemorySessionRepository()
	handlers := NewEventHandlers(eventRepo, sceneRepo, auditRepo, rsvpRepo, streamRepo, nil)
	
	baseTime := time.Now().Add(24 * time.Hour)
	
	// Create 5 events
	for i := 0; i < 5; i++ {
		event := &scene.Event{
			ID:            uuid.New().String(),
			SceneID:       uuid.New().String(),
			Title:         fmt.Sprintf("Event %d", i+1),
			AllowPrecise:  true,
			PrecisePoint:  &scene.Point{Lat: 40.7128, Lng: -74.0060},
			CoarseGeohash: "dr5regw",
			Status:        "scheduled",
			StartsAt:      baseTime.Add(time.Duration(i) * time.Hour),
			CreatedAt:     &baseTime,
			UpdatedAt:     &baseTime,
		}
		if err := eventRepo.Insert(event); err != nil {
			t.Fatalf("failed to insert event: %v", err)
		}
	}
	
	from := baseTime.Format(time.RFC3339)
	to := baseTime.Add(6 * time.Hour).Format(time.RFC3339)
	
	// First page: limit=2
	url := fmt.Sprintf("/search/events?bbox=-74.1,40.6,-73.9,40.8&from=%s&to=%s&limit=2", from, to)
	req := httptest.NewRequest(http.MethodGet, url, nil)
	w := httptest.NewRecorder()
	
	handlers.SearchEvents(w, req)
	
	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}
	
	var response1 SearchEventsResponse
	if err := json.NewDecoder(w.Body).Decode(&response1); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	
	if len(response1.Events) != 2 {
		t.Errorf("expected 2 events in first page, got %d", len(response1.Events))
	}
	
	if response1.NextCursor == "" {
		t.Error("expected next_cursor to be set")
	}
	
	// Second page: use cursor
	url2 := fmt.Sprintf("/search/events?bbox=-74.1,40.6,-73.9,40.8&from=%s&to=%s&limit=2&cursor=%s", from, to, response1.NextCursor)
	req2 := httptest.NewRequest(http.MethodGet, url2, nil)
	w2 := httptest.NewRecorder()
	
	handlers.SearchEvents(w2, req2)
	
	if w2.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", w2.Code, w2.Body.String())
	}
	
	var response2 SearchEventsResponse
	if err := json.NewDecoder(w2.Body).Decode(&response2); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	
	if len(response2.Events) != 2 {
		t.Errorf("expected 2 events in second page, got %d", len(response2.Events))
	}
	
	// Events from page 2 should have later start times than page 1
	if len(response1.Events) > 0 && len(response2.Events) > 0 {
		lastFromPage1 := response1.Events[len(response1.Events)-1]
		firstFromPage2 := response2.Events[0]
		if !lastFromPage1.StartsAt.Before(firstFromPage2.StartsAt) {
			t.Error("pagination should maintain ordering: page 2 events should have later start times")
		}
	}
	
	// Verify no duplicate events between pages
	seenIDs := make(map[string]bool)
	for _, event := range response1.Events {
		seenIDs[event.ID] = true
	}
	for _, event := range response2.Events {
		if seenIDs[event.ID] {
			t.Errorf("duplicate event ID %s found across pages", event.ID)
		}
	}
}

// TestSearchEvents_LimitValidation tests limit parameter validation.
func TestSearchEvents_LimitValidation(t *testing.T) {
	eventRepo := scene.NewInMemoryEventRepository()
	sceneRepo := scene.NewInMemorySceneRepository()
	auditRepo := audit.NewInMemoryRepository()
	rsvpRepo := scene.NewInMemoryRSVPRepository()
	streamRepo := stream.NewInMemorySessionRepository()
	handlers := NewEventHandlers(eventRepo, sceneRepo, auditRepo, rsvpRepo, streamRepo, nil)
	
	baseTime := time.Now().Add(24 * time.Hour)
	from := baseTime.Format(time.RFC3339)
	to := baseTime.Add(1 * time.Hour).Format(time.RFC3339)
	
	tests := []struct {
		name       string
		limit      string
		wantStatus int
	}{
		{
			name:       "limit below minimum",
			limit:      "0",
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "limit above maximum",
			limit:      "101",
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "invalid limit",
			limit:      "invalid",
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "valid limit",
			limit:      "25",
			wantStatus: http.StatusOK,
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			url := fmt.Sprintf("/search/events?bbox=-74.1,40.6,-73.9,40.8&from=%s&to=%s&limit=%s", from, to, tt.limit)
			req := httptest.NewRequest(http.MethodGet, url, nil)
			w := httptest.NewRecorder()
			
			handlers.SearchEvents(w, req)
			
			if w.Code != tt.wantStatus {
				t.Errorf("expected status %d, got %d: %s", tt.wantStatus, w.Code, w.Body.String())
			}
		})
	}
}

// TestSearchEvents_EmptyResults tests search with no matching events.
func TestSearchEvents_EmptyResults(t *testing.T) {
	eventRepo := scene.NewInMemoryEventRepository()
	sceneRepo := scene.NewInMemorySceneRepository()
	auditRepo := audit.NewInMemoryRepository()
	rsvpRepo := scene.NewInMemoryRSVPRepository()
	streamRepo := stream.NewInMemorySessionRepository()
	handlers := NewEventHandlers(eventRepo, sceneRepo, auditRepo, rsvpRepo, streamRepo, nil)
	
	baseTime := time.Now().Add(24 * time.Hour)
	from := baseTime.Format(time.RFC3339)
	to := baseTime.Add(1 * time.Hour).Format(time.RFC3339)
	
	url := fmt.Sprintf("/search/events?bbox=-74.1,40.6,-73.9,40.8&from=%s&to=%s", from, to)
	req := httptest.NewRequest(http.MethodGet, url, nil)
	w := httptest.NewRecorder()
	
	handlers.SearchEvents(w, req)
	
	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}
	
	var response SearchEventsResponse
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	
	if response.Events == nil {
		t.Error("events array should not be nil, even when empty")
	}
	
	if len(response.Events) != 0 {
		t.Errorf("expected 0 events, got %d", len(response.Events))
	}
	
	if response.NextCursor != "" {
		t.Error("next_cursor should be empty when there are no more results")
	}
}
