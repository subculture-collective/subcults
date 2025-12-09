package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/onnwee/subcults/internal/audit"
	"github.com/onnwee/subcults/internal/middleware"
	"github.com/onnwee/subcults/internal/scene"
)

// TestCreateEvent_Success tests successful event creation.
func TestCreateEvent_Success(t *testing.T) {
	eventRepo := scene.NewInMemoryEventRepository()
	sceneRepo := scene.NewInMemorySceneRepository()
	auditRepo := audit.NewInMemoryRepository()
	handlers := NewEventHandlers(eventRepo, sceneRepo, auditRepo)

	// Create a scene first
	testScene := &scene.Scene{
		ID:            uuid.New().String(),
		Name:          "Test Scene",
		OwnerDID:      "did:plc:test123",
		CoarseGeohash: "dr5regw",
		CreatedAt:     &time.Time{},
	}
	if err := sceneRepo.Insert(testScene); err != nil {
		t.Fatalf("failed to insert scene: %v", err)
	}

	startsAt := time.Now().Add(24 * time.Hour)
	endsAt := startsAt.Add(2 * time.Hour)

	reqBody := CreateEventRequest{
		SceneID:       testScene.ID,
		Title:         "Test Event",
		Description:   "A test event",
		AllowPrecise:  true,
		PrecisePoint:  &scene.Point{Lat: 40.7128, Lng: -74.0060},
		CoarseGeohash: "dr5regw",
		Tags:          []string{"test", "example"},
		StartsAt:      startsAt,
		EndsAt:        &endsAt,
	}

	body, err := json.Marshal(reqBody)
	if err != nil {
		t.Fatalf("failed to marshal request: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/events", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	// Set user DID in context
	ctx := middleware.SetUserDID(req.Context(), "did:plc:test123")
	req = req.WithContext(ctx)
	w := httptest.NewRecorder()

	handlers.CreateEvent(w, req)

	if w.Code != http.StatusCreated {
		t.Errorf("expected status 201, got %d: %s", w.Code, w.Body.String())
	}

	var createdEvent scene.Event
	if err := json.NewDecoder(w.Body).Decode(&createdEvent); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if createdEvent.Title != "Test Event" {
		t.Errorf("expected title 'Test Event', got %s", createdEvent.Title)
	}
	if createdEvent.SceneID != testScene.ID {
		t.Errorf("expected scene_id '%s', got %s", testScene.ID, createdEvent.SceneID)
	}
	if createdEvent.PrecisePoint == nil {
		t.Error("expected precise_point to be set")
	}
	if createdEvent.Status != "scheduled" {
		t.Errorf("expected status 'scheduled', got %s", createdEvent.Status)
	}
}

// TestCreateEvent_InvalidTimeWindow tests rejection of invalid time windows.
func TestCreateEvent_InvalidTimeWindow(t *testing.T) {
	now := time.Now()
	tests := []struct {
		name     string
		startsAt time.Time
		endsAt   *time.Time
		wantCode int
		wantErr  string
	}{
		{
			name:     "end before start",
			startsAt: now.Add(24 * time.Hour),
			endsAt:   func() *time.Time { t := now; return &t }(),
			wantCode: http.StatusBadRequest,
			wantErr:  ErrCodeInvalidTimeRange,
		},
		{
			name:     "same time",
			startsAt: now.Add(24 * time.Hour),
			endsAt:   func() *time.Time { t := now.Add(24 * time.Hour); return &t }(),
			wantCode: http.StatusBadRequest,
			wantErr:  ErrCodeInvalidTimeRange,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			eventRepo := scene.NewInMemoryEventRepository()
			sceneRepo := scene.NewInMemorySceneRepository()
			auditRepo := audit.NewInMemoryRepository()
	handlers := NewEventHandlers(eventRepo, sceneRepo, auditRepo)

			// Create a scene first
			testScene := &scene.Scene{
				ID:            uuid.New().String(),
				Name:          "Test Scene",
				OwnerDID:      "did:plc:test123",
				CoarseGeohash: "dr5regw",
			}
			if err := sceneRepo.Insert(testScene); err != nil {
				t.Fatalf("failed to insert scene: %v", err)
			}

			reqBody := CreateEventRequest{
				SceneID:       testScene.ID,
				Title:         "Test Event",
				CoarseGeohash: "dr5regw",
				StartsAt:      tt.startsAt,
				EndsAt:        tt.endsAt,
			}

			body, err := json.Marshal(reqBody)
			if err != nil {
				t.Fatalf("failed to marshal request: %v", err)
			}

			req := httptest.NewRequest(http.MethodPost, "/events", bytes.NewReader(body))
			req.Header.Set("Content-Type", "application/json")
			ctx := middleware.SetUserDID(req.Context(), "did:plc:test123")
			req = req.WithContext(ctx)
			w := httptest.NewRecorder()

			handlers.CreateEvent(w, req)

			if w.Code != tt.wantCode {
				t.Errorf("expected status %d, got %d: %s", tt.wantCode, w.Code, w.Body.String())
			}

			var errResp ErrorResponse
			if err := json.NewDecoder(w.Body).Decode(&errResp); err != nil {
				t.Fatalf("failed to decode error response: %v", err)
			}

			if errResp.Error.Code != tt.wantErr {
				t.Errorf("expected error code '%s', got '%s'", tt.wantErr, errResp.Error.Code)
			}
		})
	}
}

// TestCreateEvent_MissingCoarseGeohash tests rejection when coarse_geohash is missing.
func TestCreateEvent_MissingCoarseGeohash(t *testing.T) {
	eventRepo := scene.NewInMemoryEventRepository()
	sceneRepo := scene.NewInMemorySceneRepository()
	auditRepo := audit.NewInMemoryRepository()
	handlers := NewEventHandlers(eventRepo, sceneRepo, auditRepo)

	// Create a scene first
	testScene := &scene.Scene{
		ID:            uuid.New().String(),
		Name:          "Test Scene",
		OwnerDID:      "did:plc:test123",
		CoarseGeohash: "dr5regw",
	}
	if err := sceneRepo.Insert(testScene); err != nil {
		t.Fatalf("failed to insert scene: %v", err)
	}

	reqBody := CreateEventRequest{
		SceneID:       testScene.ID,
		Title:         "Test Event",
		CoarseGeohash: "", // Empty geohash
		StartsAt:      time.Now().Add(24 * time.Hour),
	}

	body, err := json.Marshal(reqBody)
	if err != nil {
		t.Fatalf("failed to marshal request: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/events", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	ctx := middleware.SetUserDID(req.Context(), "did:plc:test123")
	req = req.WithContext(ctx)
	w := httptest.NewRecorder()

	handlers.CreateEvent(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", w.Code)
	}

	var errResp ErrorResponse
	if err := json.NewDecoder(w.Body).Decode(&errResp); err != nil {
		t.Fatalf("failed to decode error response: %v", err)
	}

	if errResp.Error.Code != ErrCodeValidation {
		t.Errorf("expected error code '%s', got '%s'", ErrCodeValidation, errResp.Error.Code)
	}
}

// TestCreateEvent_UnauthorizedCreate tests rejection when user doesn't own the scene.
func TestCreateEvent_UnauthorizedCreate(t *testing.T) {
	eventRepo := scene.NewInMemoryEventRepository()
	sceneRepo := scene.NewInMemorySceneRepository()
	auditRepo := audit.NewInMemoryRepository()
	handlers := NewEventHandlers(eventRepo, sceneRepo, auditRepo)

	// Create a scene with different owner
	testScene := &scene.Scene{
		ID:            uuid.New().String(),
		Name:          "Test Scene",
		OwnerDID:      "did:plc:owner123",
		CoarseGeohash: "dr5regw",
	}
	if err := sceneRepo.Insert(testScene); err != nil {
		t.Fatalf("failed to insert scene: %v", err)
	}

	reqBody := CreateEventRequest{
		SceneID:       testScene.ID,
		Title:         "Test Event",
		CoarseGeohash: "dr5regw",
		StartsAt:      time.Now().Add(24 * time.Hour),
	}

	body, err := json.Marshal(reqBody)
	if err != nil {
		t.Fatalf("failed to marshal request: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/events", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	// Set different user DID
	ctx := middleware.SetUserDID(req.Context(), "did:plc:different123")
	req = req.WithContext(ctx)
	w := httptest.NewRecorder()

	handlers.CreateEvent(w, req)

	if w.Code != http.StatusForbidden {
		t.Errorf("expected status 403, got %d: %s", w.Code, w.Body.String())
	}

	var errResp ErrorResponse
	if err := json.NewDecoder(w.Body).Decode(&errResp); err != nil {
		t.Fatalf("failed to decode error response: %v", err)
	}

	if errResp.Error.Code != ErrCodeForbidden {
		t.Errorf("expected error code '%s', got '%s'", ErrCodeForbidden, errResp.Error.Code)
	}
}

// TestCreateEvent_PrivacyEnforcement tests that privacy is enforced on creation.
func TestCreateEvent_PrivacyEnforcement(t *testing.T) {
	eventRepo := scene.NewInMemoryEventRepository()
	sceneRepo := scene.NewInMemorySceneRepository()
	auditRepo := audit.NewInMemoryRepository()
	handlers := NewEventHandlers(eventRepo, sceneRepo, auditRepo)

	// Create a scene first
	testScene := &scene.Scene{
		ID:            uuid.New().String(),
		Name:          "Test Scene",
		OwnerDID:      "did:plc:test123",
		CoarseGeohash: "dr5regw",
	}
	if err := sceneRepo.Insert(testScene); err != nil {
		t.Fatalf("failed to insert scene: %v", err)
	}

	reqBody := CreateEventRequest{
		SceneID:       testScene.ID,
		Title:         "Private Event",
		AllowPrecise:  false, // Privacy not consented
		PrecisePoint:  &scene.Point{Lat: 40.7128, Lng: -74.0060}, // Should be cleared
		CoarseGeohash: "dr5regw",
		StartsAt:      time.Now().Add(24 * time.Hour),
	}

	body, err := json.Marshal(reqBody)
	if err != nil {
		t.Fatalf("failed to marshal request: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/events", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	ctx := middleware.SetUserDID(req.Context(), "did:plc:test123")
	req = req.WithContext(ctx)
	w := httptest.NewRecorder()

	handlers.CreateEvent(w, req)

	if w.Code != http.StatusCreated {
		t.Errorf("expected status 201, got %d: %s", w.Code, w.Body.String())
	}

	var createdEvent scene.Event
	if err := json.NewDecoder(w.Body).Decode(&createdEvent); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if createdEvent.PrecisePoint != nil {
		t.Error("expected precise_point to be nil when allow_precise=false")
	}
}

// TestCreateEvent_TitleValidation tests title length validation.
func TestCreateEvent_TitleValidation(t *testing.T) {
	tests := []struct {
		name     string
		title    string
		wantCode int
		wantErr  string
	}{
		{
			name:     "too short",
			title:    "ab",
			wantCode: http.StatusBadRequest,
			wantErr:  ErrCodeValidation,
		},
		{
			name:     "too long",
			title:    strings.Repeat("a", 81),
			wantCode: http.StatusBadRequest,
			wantErr:  ErrCodeValidation,
		},
		{
			name:     "valid minimum",
			title:    "abc",
			wantCode: http.StatusCreated,
			wantErr:  "",
		},
		{
			name:     "valid maximum",
			title:    strings.Repeat("a", 80),
			wantCode: http.StatusCreated,
			wantErr:  "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			eventRepo := scene.NewInMemoryEventRepository()
			sceneRepo := scene.NewInMemorySceneRepository()
			auditRepo := audit.NewInMemoryRepository()
	handlers := NewEventHandlers(eventRepo, sceneRepo, auditRepo)

			// Create a scene first
			testScene := &scene.Scene{
				ID:            uuid.New().String(),
				Name:          "Test Scene",
				OwnerDID:      "did:plc:test123",
				CoarseGeohash: "dr5regw",
			}
			if err := sceneRepo.Insert(testScene); err != nil {
				t.Fatalf("failed to insert scene: %v", err)
			}

			reqBody := CreateEventRequest{
				SceneID:       testScene.ID,
				Title:         tt.title,
				CoarseGeohash: "dr5regw",
				StartsAt:      time.Now().Add(24 * time.Hour),
			}

			body, err := json.Marshal(reqBody)
			if err != nil {
				t.Fatalf("failed to marshal request: %v", err)
			}

			req := httptest.NewRequest(http.MethodPost, "/events", bytes.NewReader(body))
			req.Header.Set("Content-Type", "application/json")
			ctx := middleware.SetUserDID(req.Context(), "did:plc:test123")
			req = req.WithContext(ctx)
			w := httptest.NewRecorder()

			handlers.CreateEvent(w, req)

			if w.Code != tt.wantCode {
				t.Errorf("expected status %d, got %d: %s", tt.wantCode, w.Code, w.Body.String())
			}

			if tt.wantErr != "" {
				var errResp ErrorResponse
				if err := json.NewDecoder(w.Body).Decode(&errResp); err != nil {
					t.Fatalf("failed to decode error response: %v", err)
				}

				if errResp.Error.Code != tt.wantErr {
					t.Errorf("expected error code '%s', got '%s'", tt.wantErr, errResp.Error.Code)
				}
			}
		})
	}
}

// TestUpdateEvent_Success tests successful event update.
func TestUpdateEvent_Success(t *testing.T) {
	eventRepo := scene.NewInMemoryEventRepository()
	sceneRepo := scene.NewInMemorySceneRepository()
	auditRepo := audit.NewInMemoryRepository()
	handlers := NewEventHandlers(eventRepo, sceneRepo, auditRepo)

	// Create a scene first
	testScene := &scene.Scene{
		ID:            uuid.New().String(),
		Name:          "Test Scene",
		OwnerDID:      "did:plc:test123",
		CoarseGeohash: "dr5regw",
	}
	if err := sceneRepo.Insert(testScene); err != nil {
		t.Fatalf("failed to insert scene: %v", err)
	}

	// Create an event
	now := time.Now()
	startsAt := now.Add(24 * time.Hour)
	existingEvent := &scene.Event{
		ID:            uuid.New().String(),
		SceneID:       testScene.ID,
		Title:         "Original Title",
		CoarseGeohash: "dr5regw",
		StartsAt:      startsAt,
		CreatedAt:     &now,
		UpdatedAt:     &now,
	}
	if err := eventRepo.Insert(existingEvent); err != nil {
		t.Fatalf("failed to insert event: %v", err)
	}

	newTitle := "Updated Title"
	newDesc := "Updated description"
	reqBody := UpdateEventRequest{
		Title:       &newTitle,
		Description: &newDesc,
	}

	body, err := json.Marshal(reqBody)
	if err != nil {
		t.Fatalf("failed to marshal request: %v", err)
	}

	req := httptest.NewRequest(http.MethodPatch, "/events/"+existingEvent.ID, bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	ctx := middleware.SetUserDID(req.Context(), "did:plc:test123")
	req = req.WithContext(ctx)
	w := httptest.NewRecorder()

	handlers.UpdateEvent(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	var updatedEvent scene.Event
	if err := json.NewDecoder(w.Body).Decode(&updatedEvent); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if updatedEvent.Title != "Updated Title" {
		t.Errorf("expected title 'Updated Title', got %s", updatedEvent.Title)
	}
	if updatedEvent.Description != "Updated description" {
		t.Errorf("expected description 'Updated description', got %s", updatedEvent.Description)
	}
}

// TestUpdateEvent_CannotUpdatePastEvent tests that past events cannot have time updated.
func TestUpdateEvent_CannotUpdatePastEvent(t *testing.T) {
	eventRepo := scene.NewInMemoryEventRepository()
	sceneRepo := scene.NewInMemorySceneRepository()
	auditRepo := audit.NewInMemoryRepository()
	handlers := NewEventHandlers(eventRepo, sceneRepo, auditRepo)

	// Create a scene first
	testScene := &scene.Scene{
		ID:            uuid.New().String(),
		Name:          "Test Scene",
		OwnerDID:      "did:plc:test123",
		CoarseGeohash: "dr5regw",
	}
	if err := sceneRepo.Insert(testScene); err != nil {
		t.Fatalf("failed to insert scene: %v", err)
	}

	// Create a past event
	now := time.Now()
	pastTime := now.Add(-24 * time.Hour)
	existingEvent := &scene.Event{
		ID:            uuid.New().String(),
		SceneID:       testScene.ID,
		Title:         "Past Event",
		CoarseGeohash: "dr5regw",
		StartsAt:      pastTime,
		CreatedAt:     &now,
		UpdatedAt:     &now,
	}
	if err := eventRepo.Insert(existingEvent); err != nil {
		t.Fatalf("failed to insert event: %v", err)
	}

	newStartsAt := now.Add(48 * time.Hour)
	reqBody := UpdateEventRequest{
		StartsAt: &newStartsAt,
	}

	body, err := json.Marshal(reqBody)
	if err != nil {
		t.Fatalf("failed to marshal request: %v", err)
	}

	req := httptest.NewRequest(http.MethodPatch, "/events/"+existingEvent.ID, bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	ctx := middleware.SetUserDID(req.Context(), "did:plc:test123")
	req = req.WithContext(ctx)
	w := httptest.NewRecorder()

	handlers.UpdateEvent(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d: %s", w.Code, w.Body.String())
	}

	var errResp ErrorResponse
	if err := json.NewDecoder(w.Body).Decode(&errResp); err != nil {
		t.Fatalf("failed to decode error response: %v", err)
	}

	if errResp.Error.Code != ErrCodeValidation {
		t.Errorf("expected error code '%s', got '%s'", ErrCodeValidation, errResp.Error.Code)
	}
}

// TestUpdateEvent_TimeWindowValidation tests time window validation on update.
func TestUpdateEvent_TimeWindowValidation(t *testing.T) {
	eventRepo := scene.NewInMemoryEventRepository()
	sceneRepo := scene.NewInMemorySceneRepository()
	auditRepo := audit.NewInMemoryRepository()
	handlers := NewEventHandlers(eventRepo, sceneRepo, auditRepo)

	// Create a scene first
	testScene := &scene.Scene{
		ID:            uuid.New().String(),
		Name:          "Test Scene",
		OwnerDID:      "did:plc:test123",
		CoarseGeohash: "dr5regw",
	}
	if err := sceneRepo.Insert(testScene); err != nil {
		t.Fatalf("failed to insert scene: %v", err)
	}

	// Create a future event
	now := time.Now()
	startsAt := now.Add(24 * time.Hour)
	endsAt := startsAt.Add(2 * time.Hour)
	existingEvent := &scene.Event{
		ID:            uuid.New().String(),
		SceneID:       testScene.ID,
		Title:         "Future Event",
		CoarseGeohash: "dr5regw",
		StartsAt:      startsAt,
		EndsAt:        &endsAt,
		CreatedAt:     &now,
		UpdatedAt:     &now,
	}
	if err := eventRepo.Insert(existingEvent); err != nil {
		t.Fatalf("failed to insert event: %v", err)
	}

	// Try to set end time before start time
	newEndsAt := startsAt.Add(-1 * time.Hour)
	reqBody := UpdateEventRequest{
		EndsAt: &newEndsAt,
	}

	body, err := json.Marshal(reqBody)
	if err != nil {
		t.Fatalf("failed to marshal request: %v", err)
	}

	req := httptest.NewRequest(http.MethodPatch, "/events/"+existingEvent.ID, bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	ctx := middleware.SetUserDID(req.Context(), "did:plc:test123")
	req = req.WithContext(ctx)
	w := httptest.NewRecorder()

	handlers.UpdateEvent(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d: %s", w.Code, w.Body.String())
	}

	var errResp ErrorResponse
	if err := json.NewDecoder(w.Body).Decode(&errResp); err != nil {
		t.Fatalf("failed to decode error response: %v", err)
	}

	if errResp.Error.Code != ErrCodeInvalidTimeRange {
		t.Errorf("expected error code '%s', got '%s'", ErrCodeInvalidTimeRange, errResp.Error.Code)
	}
}

// TestGetEvent_Success tests successful event retrieval.
func TestGetEvent_Success(t *testing.T) {
	eventRepo := scene.NewInMemoryEventRepository()
	sceneRepo := scene.NewInMemorySceneRepository()
	auditRepo := audit.NewInMemoryRepository()
	handlers := NewEventHandlers(eventRepo, sceneRepo, auditRepo)

	// Create an event
	now := time.Now()
	testEvent := &scene.Event{
		ID:            uuid.New().String(),
		SceneID:       uuid.New().String(),
		Title:         "Test Event",
		CoarseGeohash: "dr5regw",
		StartsAt:      now.Add(24 * time.Hour),
		AllowPrecise:  true,
		PrecisePoint:  &scene.Point{Lat: 40.7128, Lng: -74.0060},
		CreatedAt:     &now,
		UpdatedAt:     &now,
	}
	if err := eventRepo.Insert(testEvent); err != nil {
		t.Fatalf("failed to insert event: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/events/"+testEvent.ID, nil)
	w := httptest.NewRecorder()

	handlers.GetEvent(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	var foundEvent scene.Event
	if err := json.NewDecoder(w.Body).Decode(&foundEvent); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if foundEvent.ID != testEvent.ID {
		t.Errorf("expected ID '%s', got '%s'", testEvent.ID, foundEvent.ID)
	}
	if foundEvent.Title != "Test Event" {
		t.Errorf("expected title 'Test Event', got %s", foundEvent.Title)
	}
	if foundEvent.PrecisePoint == nil {
		t.Error("expected precise_point to be set")
	}
}

// TestGetEvent_NotFound tests 404 when event doesn't exist.
func TestGetEvent_NotFound(t *testing.T) {
	eventRepo := scene.NewInMemoryEventRepository()
	sceneRepo := scene.NewInMemorySceneRepository()
	auditRepo := audit.NewInMemoryRepository()
	handlers := NewEventHandlers(eventRepo, sceneRepo, auditRepo)

	req := httptest.NewRequest(http.MethodGet, "/events/"+uuid.New().String(), nil)
	w := httptest.NewRecorder()

	handlers.GetEvent(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected status 404, got %d", w.Code)
	}

	var errResp ErrorResponse
	if err := json.NewDecoder(w.Body).Decode(&errResp); err != nil {
		t.Fatalf("failed to decode error response: %v", err)
	}

	if errResp.Error.Code != ErrCodeNotFound {
		t.Errorf("expected error code '%s', got '%s'", ErrCodeNotFound, errResp.Error.Code)
	}
}

// TestGetEvent_PrivacyEnforcement tests that precise_point is hidden when not allowed.
func TestGetEvent_PrivacyEnforcement(t *testing.T) {
	eventRepo := scene.NewInMemoryEventRepository()
	sceneRepo := scene.NewInMemorySceneRepository()
	auditRepo := audit.NewInMemoryRepository()
	handlers := NewEventHandlers(eventRepo, sceneRepo, auditRepo)

	// Create an event without precise location consent
	now := time.Now()
	testEvent := &scene.Event{
		ID:            uuid.New().String(),
		SceneID:       uuid.New().String(),
		Title:         "Private Event",
		CoarseGeohash: "dr5regw",
		StartsAt:      now.Add(24 * time.Hour),
		AllowPrecise:  false,
		PrecisePoint:  &scene.Point{Lat: 40.7128, Lng: -74.0060}, // Should be cleared
		CreatedAt:     &now,
		UpdatedAt:     &now,
	}
	if err := eventRepo.Insert(testEvent); err != nil {
		t.Fatalf("failed to insert event: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/events/"+testEvent.ID, nil)
	w := httptest.NewRecorder()

	handlers.GetEvent(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	var foundEvent scene.Event
	if err := json.NewDecoder(w.Body).Decode(&foundEvent); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if foundEvent.PrecisePoint != nil {
		t.Error("expected precise_point to be nil when allow_precise=false")
	}
}

// TestCancelEvent_Success tests successful event cancellation.
func TestCancelEvent_Success(t *testing.T) {
	eventRepo := scene.NewInMemoryEventRepository()
	sceneRepo := scene.NewInMemorySceneRepository()
	auditRepo := audit.NewInMemoryRepository()
	handlers := NewEventHandlers(eventRepo, sceneRepo, auditRepo)

	// Create a scene first
	testScene := &scene.Scene{
		ID:            uuid.New().String(),
		Name:          "Test Scene",
		OwnerDID:      "did:plc:test123",
		CoarseGeohash: "dr5regw",
		CreatedAt:     &time.Time{},
	}
	if err := sceneRepo.Insert(testScene); err != nil {
		t.Fatalf("failed to insert scene: %v", err)
	}

	// Create an event
	now := time.Now()
	testEvent := &scene.Event{
		ID:            uuid.New().String(),
		SceneID:       testScene.ID,
		Title:         "Test Event",
		CoarseGeohash: "dr5regw",
		StartsAt:      now.Add(24 * time.Hour),
		Status:        "scheduled",
		CreatedAt:     &now,
		UpdatedAt:     &now,
	}
	if err := eventRepo.Insert(testEvent); err != nil {
		t.Fatalf("failed to insert event: %v", err)
	}

	// Cancel the event with a reason
	reason := "Venue unavailable"
	reqBody := CancelEventRequest{
		Reason: &reason,
	}
	body, err := json.Marshal(reqBody)
	if err != nil {
		t.Fatalf("failed to marshal request: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/events/"+testEvent.ID+"/cancel", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	ctx := middleware.SetUserDID(req.Context(), "did:plc:test123")
	req = req.WithContext(ctx)
	w := httptest.NewRecorder()

	handlers.CancelEvent(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	var cancelledEvent scene.Event
	if err := json.NewDecoder(w.Body).Decode(&cancelledEvent); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if cancelledEvent.Status != "cancelled" {
		t.Errorf("expected status 'cancelled', got %s", cancelledEvent.Status)
	}
	if cancelledEvent.CancelledAt == nil {
		t.Error("expected cancelled_at to be set")
	}
	if cancelledEvent.CancellationReason == nil || *cancelledEvent.CancellationReason != reason {
		t.Errorf("expected cancellation_reason '%s', got %v", reason, cancelledEvent.CancellationReason)
	}
}

// TestCancelEvent_WithoutReason tests cancellation without providing a reason.
func TestCancelEvent_WithoutReason(t *testing.T) {
	eventRepo := scene.NewInMemoryEventRepository()
	sceneRepo := scene.NewInMemorySceneRepository()
	auditRepo := audit.NewInMemoryRepository()
	handlers := NewEventHandlers(eventRepo, sceneRepo, auditRepo)

	// Create a scene first
	testScene := &scene.Scene{
		ID:            uuid.New().String(),
		Name:          "Test Scene",
		OwnerDID:      "did:plc:test123",
		CoarseGeohash: "dr5regw",
		CreatedAt:     &time.Time{},
	}
	if err := sceneRepo.Insert(testScene); err != nil {
		t.Fatalf("failed to insert scene: %v", err)
	}

	// Create an event
	now := time.Now()
	testEvent := &scene.Event{
		ID:            uuid.New().String(),
		SceneID:       testScene.ID,
		Title:         "Test Event",
		CoarseGeohash: "dr5regw",
		StartsAt:      now.Add(24 * time.Hour),
		Status:        "scheduled",
		CreatedAt:     &now,
		UpdatedAt:     &now,
	}
	if err := eventRepo.Insert(testEvent); err != nil {
		t.Fatalf("failed to insert event: %v", err)
	}

	// Cancel without reason
	reqBody := CancelEventRequest{}
	body, err := json.Marshal(reqBody)
	if err != nil {
		t.Fatalf("failed to marshal request: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/events/"+testEvent.ID+"/cancel", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	ctx := middleware.SetUserDID(req.Context(), "did:plc:test123")
	req = req.WithContext(ctx)
	w := httptest.NewRecorder()

	handlers.CancelEvent(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	var cancelledEvent scene.Event
	if err := json.NewDecoder(w.Body).Decode(&cancelledEvent); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if cancelledEvent.Status != "cancelled" {
		t.Errorf("expected status 'cancelled', got %s", cancelledEvent.Status)
	}
	if cancelledEvent.CancellationReason != nil {
		t.Errorf("expected no cancellation_reason, got %v", *cancelledEvent.CancellationReason)
	}
}

// TestCancelEvent_Unauthorized tests rejection of unauthorized cancellation.
func TestCancelEvent_Unauthorized(t *testing.T) {
	eventRepo := scene.NewInMemoryEventRepository()
	sceneRepo := scene.NewInMemorySceneRepository()
	auditRepo := audit.NewInMemoryRepository()
	handlers := NewEventHandlers(eventRepo, sceneRepo, auditRepo)

	// Create a scene with different owner
	testScene := &scene.Scene{
		ID:            uuid.New().String(),
		Name:          "Test Scene",
		OwnerDID:      "did:plc:owner123",
		CoarseGeohash: "dr5regw",
		CreatedAt:     &time.Time{},
	}
	if err := sceneRepo.Insert(testScene); err != nil {
		t.Fatalf("failed to insert scene: %v", err)
	}

	// Create an event
	now := time.Now()
	testEvent := &scene.Event{
		ID:            uuid.New().String(),
		SceneID:       testScene.ID,
		Title:         "Test Event",
		CoarseGeohash: "dr5regw",
		StartsAt:      now.Add(24 * time.Hour),
		Status:        "scheduled",
		CreatedAt:     &now,
		UpdatedAt:     &now,
	}
	if err := eventRepo.Insert(testEvent); err != nil {
		t.Fatalf("failed to insert event: %v", err)
	}

	// Try to cancel as different user
	reqBody := CancelEventRequest{}
	body, err := json.Marshal(reqBody)
	if err != nil {
		t.Fatalf("failed to marshal request: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/events/"+testEvent.ID+"/cancel", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	ctx := middleware.SetUserDID(req.Context(), "did:plc:different123")
	req = req.WithContext(ctx)
	w := httptest.NewRecorder()

	handlers.CancelEvent(w, req)

	if w.Code != http.StatusForbidden {
		t.Errorf("expected status 403, got %d: %s", w.Code, w.Body.String())
	}

	var errResp ErrorResponse
	if err := json.NewDecoder(w.Body).Decode(&errResp); err != nil {
		t.Fatalf("failed to decode error response: %v", err)
	}

	if errResp.Error.Code != ErrCodeForbidden {
		t.Errorf("expected error code '%s', got '%s'", ErrCodeForbidden, errResp.Error.Code)
	}
}

// TestCancelEvent_Idempotent tests that cancelling an already cancelled event is idempotent.
func TestCancelEvent_Idempotent(t *testing.T) {
	eventRepo := scene.NewInMemoryEventRepository()
	sceneRepo := scene.NewInMemorySceneRepository()
	auditRepo := audit.NewInMemoryRepository()
	handlers := NewEventHandlers(eventRepo, sceneRepo, auditRepo)

	// Create a scene first
	testScene := &scene.Scene{
		ID:            uuid.New().String(),
		Name:          "Test Scene",
		OwnerDID:      "did:plc:test123",
		CoarseGeohash: "dr5regw",
		CreatedAt:     &time.Time{},
	}
	if err := sceneRepo.Insert(testScene); err != nil {
		t.Fatalf("failed to insert scene: %v", err)
	}

	// Create an event
	now := time.Now()
	testEvent := &scene.Event{
		ID:            uuid.New().String(),
		SceneID:       testScene.ID,
		Title:         "Test Event",
		CoarseGeohash: "dr5regw",
		StartsAt:      now.Add(24 * time.Hour),
		Status:        "scheduled",
		CreatedAt:     &now,
		UpdatedAt:     &now,
	}
	if err := eventRepo.Insert(testEvent); err != nil {
		t.Fatalf("failed to insert event: %v", err)
	}

	// First cancellation
	reqBody := CancelEventRequest{}
	body, err := json.Marshal(reqBody)
	if err != nil {
		t.Fatalf("failed to marshal request: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/events/"+testEvent.ID+"/cancel", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	ctx := middleware.SetUserDID(req.Context(), "did:plc:test123")
	req = req.WithContext(ctx)
	w := httptest.NewRecorder()

	handlers.CancelEvent(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200 on first cancel, got %d: %s", w.Code, w.Body.String())
	}

	// Get first cancellation timestamp
	var firstCancel scene.Event
	if err := json.NewDecoder(w.Body).Decode(&firstCancel); err != nil {
		t.Fatalf("failed to decode first response: %v", err)
	}

	// Second cancellation (should be idempotent)
	body2, _ := json.Marshal(reqBody)
	req2 := httptest.NewRequest(http.MethodPost, "/events/"+testEvent.ID+"/cancel", bytes.NewReader(body2))
	req2.Header.Set("Content-Type", "application/json")
	ctx2 := middleware.SetUserDID(req2.Context(), "did:plc:test123")
	req2 = req2.WithContext(ctx2)
	w2 := httptest.NewRecorder()

	handlers.CancelEvent(w2, req2)

	if w2.Code != http.StatusOK {
		t.Errorf("expected status 200 on second cancel, got %d: %s", w2.Code, w2.Body.String())
	}

	var secondCancel scene.Event
	if err := json.NewDecoder(w2.Body).Decode(&secondCancel); err != nil {
		t.Fatalf("failed to decode second response: %v", err)
	}

	// Verify idempotency - cancelled_at should be the same
	if !firstCancel.CancelledAt.Equal(*secondCancel.CancelledAt) {
		t.Errorf("expected cancelled_at to remain unchanged on second cancel")
	}
}

// TestCancelEvent_AuditLog tests that cancellation emits audit log.
func TestCancelEvent_AuditLog(t *testing.T) {
	eventRepo := scene.NewInMemoryEventRepository()
	sceneRepo := scene.NewInMemorySceneRepository()
	auditRepo := audit.NewInMemoryRepository()
	handlers := NewEventHandlers(eventRepo, sceneRepo, auditRepo)

	// Create a scene first
	testScene := &scene.Scene{
		ID:            uuid.New().String(),
		Name:          "Test Scene",
		OwnerDID:      "did:plc:test123",
		CoarseGeohash: "dr5regw",
		CreatedAt:     &time.Time{},
	}
	if err := sceneRepo.Insert(testScene); err != nil {
		t.Fatalf("failed to insert scene: %v", err)
	}

	// Create an event
	now := time.Now()
	testEvent := &scene.Event{
		ID:            uuid.New().String(),
		SceneID:       testScene.ID,
		Title:         "Test Event",
		CoarseGeohash: "dr5regw",
		StartsAt:      now.Add(24 * time.Hour),
		Status:        "scheduled",
		CreatedAt:     &now,
		UpdatedAt:     &now,
	}
	if err := eventRepo.Insert(testEvent); err != nil {
		t.Fatalf("failed to insert event: %v", err)
	}

	// Cancel the event
	reqBody := CancelEventRequest{}
	body, err := json.Marshal(reqBody)
	if err != nil {
		t.Fatalf("failed to marshal request: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/events/"+testEvent.ID+"/cancel", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	ctx := middleware.SetUserDID(req.Context(), "did:plc:test123")
	req = req.WithContext(ctx)
	w := httptest.NewRecorder()

	handlers.CancelEvent(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	// Verify audit log was created
	logs, err := auditRepo.QueryByEntity("event", testEvent.ID, 0)
	if err != nil {
		t.Fatalf("failed to get audit logs: %v", err)
	}

	if len(logs) != 1 {
		t.Errorf("expected 1 audit log entry, got %d", len(logs))
	}

	if len(logs) > 0 {
		if logs[0].Action != "event_cancel" {
			t.Errorf("expected action 'event_cancel', got '%s'", logs[0].Action)
		}
		if logs[0].UserDID != "did:plc:test123" {
			t.Errorf("expected user_did 'did:plc:test123', got '%s'", logs[0].UserDID)
		}
	}
}

// TestCancelEvent_IdempotentNoAuditDuplicate tests that second cancel doesn't create duplicate audit log.
func TestCancelEvent_IdempotentNoAuditDuplicate(t *testing.T) {
	eventRepo := scene.NewInMemoryEventRepository()
	sceneRepo := scene.NewInMemorySceneRepository()
	auditRepo := audit.NewInMemoryRepository()
	handlers := NewEventHandlers(eventRepo, sceneRepo, auditRepo)

	// Create a scene first
	testScene := &scene.Scene{
		ID:            uuid.New().String(),
		Name:          "Test Scene",
		OwnerDID:      "did:plc:test123",
		CoarseGeohash: "dr5regw",
		CreatedAt:     &time.Time{},
	}
	if err := sceneRepo.Insert(testScene); err != nil {
		t.Fatalf("failed to insert scene: %v", err)
	}

	// Create an event
	now := time.Now()
	testEvent := &scene.Event{
		ID:            uuid.New().String(),
		SceneID:       testScene.ID,
		Title:         "Test Event",
		CoarseGeohash: "dr5regw",
		StartsAt:      now.Add(24 * time.Hour),
		Status:        "scheduled",
		CreatedAt:     &now,
		UpdatedAt:     &now,
	}
	if err := eventRepo.Insert(testEvent); err != nil {
		t.Fatalf("failed to insert event: %v", err)
	}

	// First cancellation
	reqBody := CancelEventRequest{}
	body, err := json.Marshal(reqBody)
	if err != nil {
		t.Fatalf("failed to marshal request: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/events/"+testEvent.ID+"/cancel", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	ctx := middleware.SetUserDID(req.Context(), "did:plc:test123")
	req = req.WithContext(ctx)
	w := httptest.NewRecorder()

	handlers.CancelEvent(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200 on first cancel, got %d: %s", w.Code, w.Body.String())
	}

	// Second cancellation
	body2, _ := json.Marshal(reqBody)
	req2 := httptest.NewRequest(http.MethodPost, "/events/"+testEvent.ID+"/cancel", bytes.NewReader(body2))
	req2.Header.Set("Content-Type", "application/json")
	ctx2 := middleware.SetUserDID(req2.Context(), "did:plc:test123")
	req2 = req2.WithContext(ctx2)
	w2 := httptest.NewRecorder()

	handlers.CancelEvent(w2, req2)

	if w2.Code != http.StatusOK {
		t.Errorf("expected status 200 on second cancel, got %d: %s", w2.Code, w2.Body.String())
	}

	// Verify only one audit log was created
	logs, err := auditRepo.QueryByEntity("event", testEvent.ID, 0)
	if err != nil {
		t.Fatalf("failed to get audit logs: %v", err)
	}

	if len(logs) != 1 {
		t.Errorf("expected exactly 1 audit log entry (no duplicate), got %d", len(logs))
	}
}

