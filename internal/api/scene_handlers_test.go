package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/onnwee/subcults/internal/scene"
)

// TestCreateScene_Success tests successful scene creation.
func TestCreateScene_Success(t *testing.T) {
	repo := scene.NewInMemorySceneRepository()
	handlers := NewSceneHandlers(repo)

	reqBody := CreateSceneRequest{
		Name:          "Test Scene",
		Description:   "A test scene",
		OwnerDID:      "did:plc:test123",
		AllowPrecise:  true,
		PrecisePoint:  &scene.Point{Lat: 40.7128, Lng: -74.0060},
		CoarseGeohash: "dr5regw",
		Tags:          []string{"test", "example"},
		Visibility:    "public",
		Palette:       &scene.Palette{Primary: "#ff0000", Secondary: "#00ff00"},
	}

	body, err := json.Marshal(reqBody)
	if err != nil {
		t.Fatalf("failed to marshal request: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/scenes", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	handlers.CreateScene(w, req)

	if w.Code != http.StatusCreated {
		t.Errorf("expected status 201, got %d", w.Code)
	}

	var createdScene scene.Scene
	if err := json.NewDecoder(w.Body).Decode(&createdScene); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if createdScene.Name != "Test Scene" {
		t.Errorf("expected name 'Test Scene', got %s", createdScene.Name)
	}
	if createdScene.OwnerDID != "did:plc:test123" {
		t.Errorf("expected owner_did 'did:plc:test123', got %s", createdScene.OwnerDID)
	}
	if createdScene.Visibility != "public" {
		t.Errorf("expected visibility 'public', got %s", createdScene.Visibility)
	}
	if createdScene.PrecisePoint == nil {
		t.Error("expected precise_point to be set")
	}
	if createdScene.CreatedAt == nil {
		t.Error("expected created_at to be set")
	}
}

// TestCreateScene_DefaultVisibility tests that visibility defaults to "public".
func TestCreateScene_DefaultVisibility(t *testing.T) {
	repo := scene.NewInMemorySceneRepository()
	handlers := NewSceneHandlers(repo)

	reqBody := CreateSceneRequest{
		Name:          "Test Scene",
		OwnerDID:      "did:plc:test123",
		CoarseGeohash: "dr5regw",
	}

	body, err := json.Marshal(reqBody)
	if err != nil {
		t.Fatalf("failed to marshal request: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/scenes", bytes.NewReader(body))
	w := httptest.NewRecorder()

	handlers.CreateScene(w, req)

	if w.Code != http.StatusCreated {
		t.Errorf("expected status 201, got %d", w.Code)
	}

	var createdScene scene.Scene
	if err := json.NewDecoder(w.Body).Decode(&createdScene); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if createdScene.Visibility != "public" {
		t.Errorf("expected default visibility 'public', got %s", createdScene.Visibility)
	}
}

// TestCreateScene_PrivacyEnforcement tests that privacy is enforced on creation.
func TestCreateScene_PrivacyEnforcement(t *testing.T) {
	repo := scene.NewInMemorySceneRepository()
	handlers := NewSceneHandlers(repo)

	reqBody := CreateSceneRequest{
		Name:          "Private Scene",
		OwnerDID:      "did:plc:test123",
		AllowPrecise:  false, // Privacy not consented
		PrecisePoint:  &scene.Point{Lat: 40.7128, Lng: -74.0060}, // Should be cleared
		CoarseGeohash: "dr5regw",
	}

	body, err := json.Marshal(reqBody)
	if err != nil {
		t.Fatalf("failed to marshal request: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/scenes", bytes.NewReader(body))
	w := httptest.NewRecorder()

	handlers.CreateScene(w, req)

	if w.Code != http.StatusCreated {
		t.Errorf("expected status 201, got %d", w.Code)
	}

	var createdScene scene.Scene
	if err := json.NewDecoder(w.Body).Decode(&createdScene); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if createdScene.PrecisePoint != nil {
		t.Error("expected precise_point to be nil when allow_precise=false")
	}
}

// TestCreateScene_DuplicateName tests duplicate name rejection.
func TestCreateScene_DuplicateName(t *testing.T) {
	repo := scene.NewInMemorySceneRepository()
	handlers := NewSceneHandlers(repo)

	// Create first scene
	firstReq := CreateSceneRequest{
		Name:          "Duplicate Scene",
		OwnerDID:      "did:plc:test123",
		CoarseGeohash: "dr5regw",
	}

	body, _ := json.Marshal(firstReq)
	req := httptest.NewRequest(http.MethodPost, "/scenes", bytes.NewReader(body))
	w := httptest.NewRecorder()
	handlers.CreateScene(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("first creation failed with status %d", w.Code)
	}

	// Try to create second scene with same name and owner
	body, _ = json.Marshal(firstReq)
	req = httptest.NewRequest(http.MethodPost, "/scenes", bytes.NewReader(body))
	w = httptest.NewRecorder()
	handlers.CreateScene(w, req)

	if w.Code != http.StatusConflict {
		t.Errorf("expected status 409, got %d", w.Code)
	}

	var errResp ErrorResponse
	if err := json.NewDecoder(w.Body).Decode(&errResp); err != nil {
		t.Fatalf("failed to decode error response: %v", err)
	}

	if errResp.Error.Code != ErrCodeConflict {
		t.Errorf("expected error code %s, got %s", ErrCodeConflict, errResp.Error.Code)
	}
}

// TestCreateScene_InvalidName tests name validation.
func TestCreateScene_InvalidName(t *testing.T) {
	tests := []struct {
		name        string
		sceneName   string
		wantStatus  int
		wantErrCode string
	}{
		{
			name:        "too short",
			sceneName:   "ab",
			wantStatus:  http.StatusBadRequest,
			wantErrCode: ErrCodeValidation,
		},
		{
			name:        "too long",
			sceneName:   strings.Repeat("a", 65),
			wantStatus:  http.StatusBadRequest,
			wantErrCode: ErrCodeValidation,
		},
		{
			name:        "invalid characters",
			sceneName:   "Scene<script>alert('xss')</script>",
			wantStatus:  http.StatusBadRequest,
			wantErrCode: ErrCodeValidation,
		},
		{
			name:        "special chars not allowed",
			sceneName:   "Scene@#$%",
			wantStatus:  http.StatusBadRequest,
			wantErrCode: ErrCodeValidation,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := scene.NewInMemorySceneRepository()
			handlers := NewSceneHandlers(repo)

			reqBody := CreateSceneRequest{
				Name:          tt.sceneName,
				OwnerDID:      "did:plc:test123",
				CoarseGeohash: "dr5regw",
			}

			body, _ := json.Marshal(reqBody)
			req := httptest.NewRequest(http.MethodPost, "/scenes", bytes.NewReader(body))
			w := httptest.NewRecorder()

			handlers.CreateScene(w, req)

			if w.Code != tt.wantStatus {
				t.Errorf("expected status %d, got %d", tt.wantStatus, w.Code)
			}

			var errResp ErrorResponse
			if err := json.NewDecoder(w.Body).Decode(&errResp); err != nil {
				t.Fatalf("failed to decode error response: %v", err)
			}

			if errResp.Error.Code != tt.wantErrCode {
				t.Errorf("expected error code %s, got %s", tt.wantErrCode, errResp.Error.Code)
			}
		})
	}
}

// TestCreateScene_MissingRequiredFields tests validation of required fields.
func TestCreateScene_MissingRequiredFields(t *testing.T) {
	tests := []struct {
		name    string
		reqBody CreateSceneRequest
	}{
		{
			name: "missing owner_did",
			reqBody: CreateSceneRequest{
				Name:          "Test Scene",
				CoarseGeohash: "dr5regw",
			},
		},
		{
			name: "missing coarse_geohash",
			reqBody: CreateSceneRequest{
				Name:     "Test Scene",
				OwnerDID: "did:plc:test123",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := scene.NewInMemorySceneRepository()
			handlers := NewSceneHandlers(repo)

			body, _ := json.Marshal(tt.reqBody)
			req := httptest.NewRequest(http.MethodPost, "/scenes", bytes.NewReader(body))
			w := httptest.NewRecorder()

			handlers.CreateScene(w, req)

			if w.Code != http.StatusBadRequest {
				t.Errorf("expected status 400, got %d", w.Code)
			}

			var errResp ErrorResponse
			if err := json.NewDecoder(w.Body).Decode(&errResp); err != nil {
				t.Fatalf("failed to decode error response: %v", err)
			}

			if errResp.Error.Code != ErrCodeValidation {
				t.Errorf("expected error code %s, got %s", ErrCodeValidation, errResp.Error.Code)
			}
		})
	}
}

// TestUpdateScene_Success tests successful scene update.
func TestUpdateScene_Success(t *testing.T) {
	repo := scene.NewInMemorySceneRepository()
	handlers := NewSceneHandlers(repo)

	// Create a scene first
	now := time.Now()
	originalScene := &scene.Scene{
		ID:            "test-scene-id",
		Name:          "Original Name",
		Description:   "Original description",
		OwnerDID:      "did:plc:test123",
		CoarseGeohash: "dr5regw",
		Visibility:    "public",
		CreatedAt:     &now,
		UpdatedAt:     &now,
	}
	if err := repo.Insert(originalScene); err != nil {
		t.Fatalf("failed to insert scene: %v", err)
	}

	// Update the scene
	newName := "Updated Name"
	newDesc := "Updated description"
	newVis := "unlisted"
	updateReq := UpdateSceneRequest{
		Name:        &newName,
		Description: &newDesc,
		Visibility:  &newVis,
		Tags:        []string{"updated", "tags"},
	}

	body, _ := json.Marshal(updateReq)
	req := httptest.NewRequest(http.MethodPatch, "/scenes/test-scene-id", bytes.NewReader(body))
	w := httptest.NewRecorder()

	handlers.UpdateScene(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	var updatedScene scene.Scene
	if err := json.NewDecoder(w.Body).Decode(&updatedScene); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if updatedScene.Name != "Updated Name" {
		t.Errorf("expected name 'Updated Name', got %s", updatedScene.Name)
	}
	if updatedScene.Description != "Updated description" {
		t.Errorf("expected description 'Updated description', got %s", updatedScene.Description)
	}
	if updatedScene.Visibility != "unlisted" {
		t.Errorf("expected visibility 'unlisted', got %s", updatedScene.Visibility)
	}
	if len(updatedScene.Tags) != 2 {
		t.Errorf("expected 2 tags, got %d", len(updatedScene.Tags))
	}
	if updatedScene.OwnerDID != "did:plc:test123" {
		t.Errorf("owner_did should remain unchanged, got %s", updatedScene.OwnerDID)
	}
}

// TestUpdateScene_NotFound tests updating a non-existent scene.
func TestUpdateScene_NotFound(t *testing.T) {
	repo := scene.NewInMemorySceneRepository()
	handlers := NewSceneHandlers(repo)

	newName := "Updated Name"
	updateReq := UpdateSceneRequest{
		Name: &newName,
	}

	body, _ := json.Marshal(updateReq)
	req := httptest.NewRequest(http.MethodPatch, "/scenes/nonexistent-id", bytes.NewReader(body))
	w := httptest.NewRecorder()

	handlers.UpdateScene(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected status 404, got %d", w.Code)
	}

	var errResp ErrorResponse
	if err := json.NewDecoder(w.Body).Decode(&errResp); err != nil {
		t.Fatalf("failed to decode error response: %v", err)
	}

	if errResp.Error.Code != ErrCodeNotFound {
		t.Errorf("expected error code %s, got %s", ErrCodeNotFound, errResp.Error.Code)
	}
}

// TestUpdateScene_DuplicateName tests updating to a duplicate name.
func TestUpdateScene_DuplicateName(t *testing.T) {
	repo := scene.NewInMemorySceneRepository()
	handlers := NewSceneHandlers(repo)

	now := time.Now()

	// Create first scene
	scene1 := &scene.Scene{
		ID:            "scene-1",
		Name:          "Scene One",
		OwnerDID:      "did:plc:test123",
		CoarseGeohash: "dr5regw",
		CreatedAt:     &now,
		UpdatedAt:     &now,
	}
	repo.Insert(scene1)

	// Create second scene
	scene2 := &scene.Scene{
		ID:            "scene-2",
		Name:          "Scene Two",
		OwnerDID:      "did:plc:test123",
		CoarseGeohash: "dr5regw",
		CreatedAt:     &now,
		UpdatedAt:     &now,
	}
	repo.Insert(scene2)

	// Try to update scene-2 to have the same name as scene-1
	newName := "Scene One"
	updateReq := UpdateSceneRequest{
		Name: &newName,
	}

	body, _ := json.Marshal(updateReq)
	req := httptest.NewRequest(http.MethodPatch, "/scenes/scene-2", bytes.NewReader(body))
	w := httptest.NewRecorder()

	handlers.UpdateScene(w, req)

	if w.Code != http.StatusConflict {
		t.Errorf("expected status 409, got %d", w.Code)
	}

	var errResp ErrorResponse
	if err := json.NewDecoder(w.Body).Decode(&errResp); err != nil {
		t.Fatalf("failed to decode error response: %v", err)
	}

	if errResp.Error.Code != ErrCodeConflict {
		t.Errorf("expected error code %s, got %s", ErrCodeConflict, errResp.Error.Code)
	}
}

// TestDeleteScene_Success tests successful scene deletion.
func TestDeleteScene_Success(t *testing.T) {
	repo := scene.NewInMemorySceneRepository()
	handlers := NewSceneHandlers(repo)

	now := time.Now()
	testScene := &scene.Scene{
		ID:            "test-scene-id",
		Name:          "Test Scene",
		OwnerDID:      "did:plc:test123",
		CoarseGeohash: "dr5regw",
		CreatedAt:     &now,
		UpdatedAt:     &now,
	}
	repo.Insert(testScene)

	req := httptest.NewRequest(http.MethodDelete, "/scenes/test-scene-id", nil)
	w := httptest.NewRecorder()

	handlers.DeleteScene(w, req)

	if w.Code != http.StatusNoContent {
		t.Errorf("expected status 204, got %d", w.Code)
	}

	// Verify scene is soft-deleted (returns 404 on get)
	_, err := repo.GetByID("test-scene-id")
	if err != scene.ErrSceneNotFound {
		t.Error("expected scene to be soft-deleted and return ErrSceneNotFound")
	}
}

// TestDeleteScene_NotFound tests deleting a non-existent scene.
func TestDeleteScene_NotFound(t *testing.T) {
	repo := scene.NewInMemorySceneRepository()
	handlers := NewSceneHandlers(repo)

	req := httptest.NewRequest(http.MethodDelete, "/scenes/nonexistent-id", nil)
	w := httptest.NewRecorder()

	handlers.DeleteScene(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected status 404, got %d", w.Code)
	}

	var errResp ErrorResponse
	if err := json.NewDecoder(w.Body).Decode(&errResp); err != nil {
		t.Fatalf("failed to decode error response: %v", err)
	}

	if errResp.Error.Code != ErrCodeNotFound {
		t.Errorf("expected error code %s, got %s", ErrCodeNotFound, errResp.Error.Code)
	}
}

// TestDeleteScene_AlreadyDeleted tests deleting an already deleted scene.
func TestDeleteScene_AlreadyDeleted(t *testing.T) {
	repo := scene.NewInMemorySceneRepository()
	handlers := NewSceneHandlers(repo)

	now := time.Now()
	testScene := &scene.Scene{
		ID:            "test-scene-id",
		Name:          "Test Scene",
		OwnerDID:      "did:plc:test123",
		CoarseGeohash: "dr5regw",
		CreatedAt:     &now,
		UpdatedAt:     &now,
	}
	repo.Insert(testScene)

	// Delete once
	repo.Delete("test-scene-id")

	// Try to delete again
	req := httptest.NewRequest(http.MethodDelete, "/scenes/test-scene-id", nil)
	w := httptest.NewRecorder()

	handlers.DeleteScene(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected status 404, got %d", w.Code)
	}
}

// TestValidateSceneName tests scene name validation function.
func TestValidateSceneName(t *testing.T) {
	tests := []struct {
		name      string
		sceneName string
		wantErr   bool
	}{
		{"valid name", "Test Scene", false},
		{"valid with numbers", "Scene 123", false},
		{"valid with dash", "Test-Scene", false},
		{"valid with underscore", "Test_Scene", false},
		{"valid with apostrophe", "Mike's Scene", false},
		{"valid with period", "Scene v1.0", false},
		{"valid with ampersand", "Rock & Roll", false},
		{"too short", "ab", true},
		{"too long", strings.Repeat("a", 65), true},
		{"invalid chars", "Scene<>", true},
		{"invalid chars @", "Scene@email", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			errMsg := validateSceneName(tt.sceneName)
			hasErr := errMsg != ""
			if hasErr != tt.wantErr {
				t.Errorf("validateSceneName(%q) error = %v, wantErr %v", tt.sceneName, errMsg, tt.wantErr)
			}
		})
	}
}

// TestValidateVisibility tests visibility validation.
func TestValidateVisibility(t *testing.T) {
	tests := []struct {
		visibility string
		wantErr    bool
	}{
		{"public", false},
		{"private", false},
		{"unlisted", false},
		{"", false}, // Empty is OK
		{"invalid", true},
		{"PUBLIC", true}, // Case sensitive
	}

	for _, tt := range tests {
		t.Run(tt.visibility, func(t *testing.T) {
			errMsg := validateVisibility(tt.visibility)
			hasErr := errMsg != ""
			if hasErr != tt.wantErr {
				t.Errorf("validateVisibility(%q) error = %v, wantErr %v", tt.visibility, errMsg, tt.wantErr)
			}
		})
	}
}

// TestUpdateScenePalette_Success tests successful palette update.
func TestUpdateScenePalette_Success(t *testing.T) {
repo := scene.NewInMemorySceneRepository()
handlers := NewSceneHandlers(repo)

// Create a scene first
now := time.Now()
testScene := &scene.Scene{
ID:            "test-scene-id",
Name:          "Test Scene",
OwnerDID:      "did:plc:test123",
CoarseGeohash: "dr5regw",
Visibility:    "public",
CreatedAt:     &now,
UpdatedAt:     &now,
}
if err := repo.Insert(testScene); err != nil {
t.Fatalf("failed to insert test scene: %v", err)
}

// Update palette
reqBody := UpdateScenePaletteRequest{
Palette: scene.Palette{
Primary:    "#ff0000",
Secondary:  "#00ff00",
Accent:     "#0000ff",
Background: "#ffffff",
Text:       "#000000",
},
}

body, err := json.Marshal(reqBody)
if err != nil {
t.Fatalf("failed to marshal request: %v", err)
}

req := httptest.NewRequest(http.MethodPatch, "/scenes/test-scene-id/palette", bytes.NewReader(body))
req.Header.Set("Content-Type", "application/json")
w := httptest.NewRecorder()

handlers.UpdateScenePalette(w, req)

if w.Code != http.StatusOK {
t.Errorf("expected status 200, got %d: %s", w.Code, w.Body.String())
}

var updatedScene scene.Scene
if err := json.NewDecoder(w.Body).Decode(&updatedScene); err != nil {
t.Fatalf("failed to decode response: %v", err)
}

if updatedScene.Palette == nil {
t.Fatal("expected palette to be set")
}
if updatedScene.Palette.Primary != "#ff0000" {
t.Errorf("expected primary color #ff0000, got %s", updatedScene.Palette.Primary)
}
if updatedScene.Palette.Text != "#000000" {
t.Errorf("expected text color #000000, got %s", updatedScene.Palette.Text)
}
}

// TestUpdateScenePalette_InvalidHexColor tests rejection of invalid hex colors.
func TestUpdateScenePalette_InvalidHexColor(t *testing.T) {
repo := scene.NewInMemorySceneRepository()
handlers := NewSceneHandlers(repo)

// Create a scene first
now := time.Now()
testScene := &scene.Scene{
ID:            "test-scene-id",
Name:          "Test Scene",
OwnerDID:      "did:plc:test123",
CoarseGeohash: "dr5regw",
Visibility:    "public",
CreatedAt:     &now,
UpdatedAt:     &now,
}
if err := repo.Insert(testScene); err != nil {
t.Fatalf("failed to insert test scene: %v", err)
}

tests := []struct {
name    string
palette scene.Palette
wantErr string
}{
{
name: "invalid primary color",
palette: scene.Palette{
Primary:    "not-a-color",
Secondary:  "#00ff00",
Accent:     "#0000ff",
Background: "#ffffff",
Text:       "#000000",
},
wantErr: "primary color",
},
{
name: "missing hash in secondary",
palette: scene.Palette{
Primary:    "#ff0000",
Secondary:  "00ff00",
Accent:     "#0000ff",
Background: "#ffffff",
Text:       "#000000",
},
wantErr: "secondary color",
},
{
name: "too short accent color",
palette: scene.Palette{
Primary:    "#ff0000",
Secondary:  "#00ff00",
Accent:     "#00f",
Background: "#ffffff",
Text:       "#000000",
},
wantErr: "accent color",
},
{
name: "empty background color",
palette: scene.Palette{
Primary:    "#ff0000",
Secondary:  "#00ff00",
Accent:     "#0000ff",
Background: "",
Text:       "#000000",
},
wantErr: "background color is required",
},
{
name: "empty text color",
palette: scene.Palette{
Primary:    "#ff0000",
Secondary:  "#00ff00",
Accent:     "#0000ff",
Background: "#ffffff",
Text:       "",
},
wantErr: "text color is required",
},
}

for _, tt := range tests {
t.Run(tt.name, func(t *testing.T) {
reqBody := UpdateScenePaletteRequest{Palette: tt.palette}
body, err := json.Marshal(reqBody)
if err != nil {
t.Fatalf("failed to marshal request: %v", err)
}

req := httptest.NewRequest(http.MethodPatch, "/scenes/test-scene-id/palette", bytes.NewReader(body))
req.Header.Set("Content-Type", "application/json")
w := httptest.NewRecorder()

handlers.UpdateScenePalette(w, req)

if w.Code != http.StatusBadRequest {
t.Errorf("expected status 400, got %d", w.Code)
}

var errResp ErrorResponse
if err := json.NewDecoder(w.Body).Decode(&errResp); err != nil {
t.Fatalf("failed to decode error response: %v", err)
}

if errResp.Error.Code != ErrCodeInvalidPalette {
t.Errorf("expected error code %s, got %s", ErrCodeInvalidPalette, errResp.Error.Code)
}

if !strings.Contains(errResp.Error.Message, tt.wantErr) {
t.Errorf("expected error message to contain %q, got %q", tt.wantErr, errResp.Error.Message)
}
})
}
}

// TestUpdateScenePalette_InsufficientContrast tests rejection of palettes with poor contrast.
func TestUpdateScenePalette_InsufficientContrast(t *testing.T) {
repo := scene.NewInMemorySceneRepository()
handlers := NewSceneHandlers(repo)

// Create a scene first
now := time.Now()
testScene := &scene.Scene{
ID:            "test-scene-id",
Name:          "Test Scene",
OwnerDID:      "did:plc:test123",
CoarseGeohash: "dr5regw",
Visibility:    "public",
CreatedAt:     &now,
UpdatedAt:     &now,
}
if err := repo.Insert(testScene); err != nil {
t.Fatalf("failed to insert test scene: %v", err)
}

tests := []struct {
name string
text string
bg   string
}{
{
name: "light gray on white",
text: "#cccccc",
bg:   "#ffffff",
},
{
name: "yellow on white",
text: "#ffff00",
bg:   "#ffffff",
},
{
name: "light blue on white",
text: "#aaddff",
bg:   "#ffffff",
},
}

for _, tt := range tests {
t.Run(tt.name, func(t *testing.T) {
reqBody := UpdateScenePaletteRequest{
Palette: scene.Palette{
Primary:    "#ff0000",
Secondary:  "#00ff00",
Accent:     "#0000ff",
Background: tt.bg,
Text:       tt.text,
},
}

body, err := json.Marshal(reqBody)
if err != nil {
t.Fatalf("failed to marshal request: %v", err)
}

req := httptest.NewRequest(http.MethodPatch, "/scenes/test-scene-id/palette", bytes.NewReader(body))
req.Header.Set("Content-Type", "application/json")
w := httptest.NewRecorder()

handlers.UpdateScenePalette(w, req)

if w.Code != http.StatusBadRequest {
t.Errorf("expected status 400, got %d: %s", w.Code, w.Body.String())
}

var errResp ErrorResponse
if err := json.NewDecoder(w.Body).Decode(&errResp); err != nil {
t.Fatalf("failed to decode error response: %v", err)
}

if errResp.Error.Code != ErrCodeInvalidPalette {
t.Errorf("expected error code %s, got %s", ErrCodeInvalidPalette, errResp.Error.Code)
}

if !strings.Contains(errResp.Error.Message, "contrast") {
t.Errorf("expected error message to contain 'contrast', got %q", errResp.Error.Message)
}
})
}
}

// TestUpdateScenePalette_ScriptTagSanitization tests XSS prevention.
func TestUpdateScenePalette_ScriptTagSanitization(t *testing.T) {
repo := scene.NewInMemorySceneRepository()
handlers := NewSceneHandlers(repo)

// Create a scene first
now := time.Now()
testScene := &scene.Scene{
ID:            "test-scene-id",
Name:          "Test Scene",
OwnerDID:      "did:plc:test123",
CoarseGeohash: "dr5regw",
Visibility:    "public",
CreatedAt:     &now,
UpdatedAt:     &now,
}
if err := repo.Insert(testScene); err != nil {
t.Fatalf("failed to insert test scene: %v", err)
}

reqBody := UpdateScenePaletteRequest{
Palette: scene.Palette{
Primary:    "<script>alert(1)</script>",
Secondary:  "#00ff00",
Accent:     "#0000ff",
Background: "#ffffff",
Text:       "#000000",
},
}

body, err := json.Marshal(reqBody)
if err != nil {
t.Fatalf("failed to marshal request: %v", err)
}

req := httptest.NewRequest(http.MethodPatch, "/scenes/test-scene-id/palette", bytes.NewReader(body))
req.Header.Set("Content-Type", "application/json")
w := httptest.NewRecorder()

handlers.UpdateScenePalette(w, req)

if w.Code != http.StatusBadRequest {
t.Errorf("expected status 400, got %d", w.Code)
}

var errResp ErrorResponse
if err := json.NewDecoder(w.Body).Decode(&errResp); err != nil {
t.Fatalf("failed to decode error response: %v", err)
}

if errResp.Error.Code != ErrCodeInvalidPalette {
t.Errorf("expected error code %s, got %s", ErrCodeInvalidPalette, errResp.Error.Code)
}
}

// TestUpdateScenePalette_SceneNotFound tests handling of non-existent scene.
func TestUpdateScenePalette_SceneNotFound(t *testing.T) {
repo := scene.NewInMemorySceneRepository()
handlers := NewSceneHandlers(repo)

reqBody := UpdateScenePaletteRequest{
Palette: scene.Palette{
Primary:    "#ff0000",
Secondary:  "#00ff00",
Accent:     "#0000ff",
Background: "#ffffff",
Text:       "#000000",
},
}

body, err := json.Marshal(reqBody)
if err != nil {
t.Fatalf("failed to marshal request: %v", err)
}

req := httptest.NewRequest(http.MethodPatch, "/scenes/nonexistent-id/palette", bytes.NewReader(body))
req.Header.Set("Content-Type", "application/json")
w := httptest.NewRecorder()

handlers.UpdateScenePalette(w, req)

if w.Code != http.StatusNotFound {
t.Errorf("expected status 404, got %d", w.Code)
}

var errResp ErrorResponse
if err := json.NewDecoder(w.Body).Decode(&errResp); err != nil {
t.Fatalf("failed to decode error response: %v", err)
}

if errResp.Error.Code != ErrCodeNotFound {
t.Errorf("expected error code %s, got %s", ErrCodeNotFound, errResp.Error.Code)
}
}
