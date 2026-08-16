// Package scene provides models, repository, and service layer for managing
// scenes, events, and RSVPs with location privacy controls.
package scene

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/onnwee/subcults/internal/color"
	"github.com/onnwee/subcults/internal/validate"
)

// Service validation sentinels used by handlers to produce HTTP-level error
// codes with WriteAPIError.
var (
	ErrInvalidSceneName   = errors.New("invalid scene name")
	ErrInvalidDescription = errors.New("invalid description")
	ErrInvalidVisibility  = errors.New("invalid visibility mode")
	ErrInvalidTimeRange   = errors.New("invalid time range")
	ErrInvalidEventTitle  = errors.New("invalid event title")
	ErrInvalidRSVPStatus  = errors.New("invalid RSVP status")
	ErrEmptyCoarseGeohash = errors.New("coarse_geohash is required")
	ErrEmptyOwnerDID      = errors.New("owner_did is required")
	ErrEmptySceneID       = errors.New("scene_id is required")
	ErrEventNotUpcoming   = errors.New("event has already started")
)

// Service holds all three repository interfaces and a clock for timestamping.
type Service struct {
	sceneRepo SceneRepository
	eventRepo EventRepository
	rsvpRepo  RSVPRepository
	now       func() time.Time
}

// NewService constructs a scene domain service.
func NewService(sceneRepo SceneRepository, eventRepo EventRepository, rsvpRepo RSVPRepository) *Service {
	return &Service{
		sceneRepo: sceneRepo,
		eventRepo: eventRepo,
		rsvpRepo:  rsvpRepo,
		now:       time.Now,
	}
}

// SceneRepo returns the underlying SceneRepository for handlers that need
// direct repository access.
func (s *Service) SceneRepo() SceneRepository { return s.sceneRepo }

// EventRepo returns the underlying EventRepository for handlers that need
// direct repository access.
func (s *Service) EventRepo() EventRepository { return s.eventRepo }

// RSVPRepo returns the underlying RSVPRepository for handlers that need
// direct repository access.
func (s *Service) RSVPRepo() RSVPRepository { return s.rsvpRepo }

// ---------------------------------------------------------------------------
// Scene methods
// ---------------------------------------------------------------------------

// CreateScene validates inputs, checks for duplicate names, builds a scene,
// persists it, and returns the privacy-enforced stored copy.
func (s *Service) CreateScene(
	ctx context.Context,
	name, description, ownerDID, coarseGeohash string,
	tags []string,
	visibility string,
	palette *Palette,
	allowPrecise bool,
	precisePoint *Point,
	publicationStatus string,
) (*Scene, error) {
	// Validate and sanitize name
	validatedName, err := validate.SceneName(name)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrInvalidSceneName, err)
	}

	// Validate and sanitize description
	validatedDesc, err := validate.Description(description)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrInvalidDescription, err)
	}

	// Validate owner DID
	if strings.TrimSpace(ownerDID) == "" {
		return nil, ErrEmptyOwnerDID
	}

	// Validate coarse_geohash
	if strings.TrimSpace(coarseGeohash) == "" {
		return nil, ErrEmptyCoarseGeohash
	}

	// Validate visibility
	if err := validateVisibility(visibility); err != nil {
		return nil, err
	}
	if visibility == "" {
		visibility = "public"
	}

	// Check for duplicate name
	exists, err := s.sceneRepo.ExistsByOwnerAndName(ownerDID, validatedName, "")
	if err != nil {
		return nil, err
	}
	if exists {
		return nil, ErrDuplicateSceneName
	}

	// Sanitize tags
	sanitizedTags := sanitizeTagsSlice(tags)

	// Build scene
	now := s.now()
	newScene := &Scene{
		ID:                uuid.New().String(),
		Name:              validatedName,
		Description:       validatedDesc,
		OwnerDID:          ownerDID,
		AllowPrecise:      allowPrecise,
		PrecisePoint:      precisePoint,
		CoarseGeohash:     coarseGeohash,
		Tags:              sanitizedTags,
		Visibility:        visibility,
		Palette:           palette,
		CreatedAt:         &now,
		UpdatedAt:         &now,
		PublicationStatus: publicationStatus,
	}

	if err := s.sceneRepo.Insert(newScene); err != nil {
		return nil, err
	}

	return s.sceneRepo.GetByID(newScene.ID)
}

// UpdateSceneParams holds the partial-update fields for UpdateScene.
type UpdateSceneParams struct {
	Version     int64
	Name        *string
	Description *string
	Tags        []string
	Visibility  *string
	Palette     *Palette
	AllowPrecise *bool
	PrecisePoint *Point
}

// UpdateScene retrieves the existing scene, applies partial updates with
// validation, checks for version conflicts, and persists the result.
func (s *Service) UpdateScene(ctx context.Context, sceneID string, params UpdateSceneParams) (*Scene, error) {
	existing, err := s.sceneRepo.GetByID(sceneID)
	if err != nil {
		return nil, err
	}

	if existing.Version > 0 {
		if params.Version == 0 {
			return nil, fmt.Errorf("version is required: %w", ErrVersionConflict)
		}
		existing.Version = params.Version
	}

	if params.Name != nil {
		newName, err := validate.SceneName(*params.Name)
		if err != nil {
			return nil, fmt.Errorf("%w: %v", ErrInvalidSceneName, err)
		}
		exists, err := s.sceneRepo.ExistsByOwnerAndName(existing.OwnerDID, newName, sceneID)
		if err != nil {
			return nil, err
		}
		if exists {
			return nil, ErrDuplicateSceneName
		}
		existing.Name = newName
	}

	if params.Description != nil {
		validatedDesc, err := validate.Description(*params.Description)
		if err != nil {
			return nil, fmt.Errorf("%w: %v", ErrInvalidDescription, err)
		}
		existing.Description = validatedDesc
	}

	if params.Tags != nil {
		existing.Tags = sanitizeTagsSlice(params.Tags)
	}

	if params.Visibility != nil {
		if err := validateVisibility(*params.Visibility); err != nil {
			return nil, err
		}
		existing.Visibility = *params.Visibility
	}

	if params.Palette != nil {
		existing.Palette = params.Palette
	}

	if params.AllowPrecise != nil {
		existing.AllowPrecise = *params.AllowPrecise
	}

	if params.PrecisePoint != nil {
		existing.PrecisePoint = params.PrecisePoint
	}

	now := s.now()
	existing.UpdatedAt = &now

	if err := s.sceneRepo.Update(existing); err != nil {
		return nil, err
	}

	return s.sceneRepo.GetByID(sceneID)
}

// GetScene retrieves a scene by ID (delegates to repository).
func (s *Service) GetScene(ctx context.Context, id string) (*Scene, error) {
	return s.sceneRepo.GetByID(id)
}

// ListScenesByOwner retrieves all non-deleted scenes owned by the given DID.
func (s *Service) ListScenesByOwner(ctx context.Context, ownerDID string) ([]*Scene, error) {
	return s.sceneRepo.ListByOwner(ownerDID)
}

// DeleteScene soft-deletes a scene by ID.
func (s *Service) DeleteScene(ctx context.Context, id string) error {
	return s.sceneRepo.Delete(id)
}

// UpdateScenePalette validates colors and contrast, then persists the palette.
func (s *Service) UpdateScenePalette(ctx context.Context, sceneID string, palette *Palette) (*Scene, error) {
	existing, err := s.sceneRepo.GetByID(sceneID)
	if err != nil {
		return nil, err
	}

	// Define color fields in deterministic order for consistent validation
	type colorField struct {
		name  string
		value *string
	}
	colorFields := []colorField{
		{"primary", &palette.Primary},
		{"secondary", &palette.Secondary},
		{"accent", &palette.Accent},
		{"background", &palette.Background},
		{"text", &palette.Text},
	}

	for _, field := range colorFields {
		if strings.TrimSpace(*field.value) == "" {
			return nil, fmt.Errorf("%s color is required", field.name)
		}
		sanitized := color.SanitizeColor(*field.value)
		if sanitized == "" {
			return nil, fmt.Errorf("%s color: invalid hex color format, expected #RRGGBB", field.name)
		}
		*field.value = sanitized
	}

	// Validate contrast ratio (WCAG AA minimum 4.5:1)
	if _, err := color.ValidateContrast(palette.Text, palette.Background); err != nil {
		return nil, err
	}

	existing.Palette = palette
	now := s.now()
	existing.UpdatedAt = &now

	if err := s.sceneRepo.Update(existing); err != nil {
		return nil, err
	}

	return existing, nil
}

// ---------------------------------------------------------------------------
// Event methods
// ---------------------------------------------------------------------------

// CreateEvent validates inputs, checks the scene exists, creates and
// returns the privacy-enforced event.
func (s *Service) CreateEvent(
	ctx context.Context,
	sceneID, title, description, coarseGeohash string,
	allowPrecise bool,
	precisePoint *Point,
	tags []string,
	startsAt time.Time,
	endsAt *time.Time,
	locationAccess string,
	placeID, venueID *string,
	kind string,
	publicationStatus string,
) (*Event, error) {
	// Validate title
	validatedTitle, err := validate.EventTitle(title)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrInvalidEventTitle, err)
	}

	// Validate scene_id
	if strings.TrimSpace(sceneID) == "" {
		return nil, ErrEmptySceneID
	}

	// Validate coarse_geohash
	if strings.TrimSpace(coarseGeohash) == "" {
		return nil, ErrEmptyCoarseGeohash
	}

	// Validate time window
	if err := validateTimeWindow(startsAt, endsAt); err != nil {
		return nil, err
	}

	// Validate description
	validatedDesc, err := validate.Description(description)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrInvalidDescription, err)
	}

	if locationAccess == "" {
		locationAccess = "public"
	}
	if locationAccess != "public" && locationAccess != "protected" {
		return nil, fmt.Errorf("location_access must be public or protected")
	}

	// Build event
	now := s.now()
	newEvent := &Event{
		ID:                uuid.New().String(),
		SceneID:           sceneID,
		Title:             validatedTitle,
		Description:       validatedDesc,
		AllowPrecise:      allowPrecise,
		PrecisePoint:      precisePoint,
		CoarseGeohash:     coarseGeohash,
		Tags:              sanitizeTagsSlice(tags),
		Status:            "scheduled",
		StartsAt:          startsAt,
		EndsAt:            endsAt,
		LocationAccess:    locationAccess,
		PlaceID:           placeID,
		VenueID:           venueID,
		Kind:              kind,
		CreatedAt:         &now,
		UpdatedAt:         &now,
		PublicationStatus: publicationStatus,
	}

	if err := s.eventRepo.Insert(newEvent); err != nil {
		return nil, err
	}

	return s.eventRepo.GetByID(newEvent.ID)
}

// UpdateEventParams holds the partial-update fields for UpdateEvent.
type UpdateEventParams struct {
	Version        int64
	Title          *string
	Description    *string
	Tags           []string
	AllowPrecise   *bool
	PrecisePoint   *Point
	CoarseGeohash  *string
	StartsAt       *time.Time
	EndsAt         *time.Time
	LocationAccess *string
}

// UpdateEvent retrieves the existing event, applies partial updates with
// validation, checks for version conflicts, and persists the result.
func (s *Service) UpdateEvent(ctx context.Context, eventID string, params UpdateEventParams) (*Event, error) {
	existing, err := s.eventRepo.GetByID(eventID)
	if err != nil {
		return nil, err
	}

	if existing.Version > 0 {
		if params.Version == 0 {
			return nil, fmt.Errorf("version is required: %w", ErrVersionConflict)
		}
		existing.Version = params.Version
	}

	if params.Title != nil {
		validatedTitle, err := validate.EventTitle(*params.Title)
		if err != nil {
			return nil, fmt.Errorf("%w: %v", ErrInvalidEventTitle, err)
		}
		existing.Title = validatedTitle
	}

	if params.Description != nil {
		validatedDesc, err := validate.Description(*params.Description)
		if err != nil {
			return nil, fmt.Errorf("%w: %v", ErrInvalidDescription, err)
		}
		existing.Description = validatedDesc
	}

	if params.Tags != nil {
		existing.Tags = sanitizeTagsSlice(params.Tags)
	}

	if params.AllowPrecise != nil {
		existing.AllowPrecise = *params.AllowPrecise
	}

	if params.PrecisePoint != nil {
		existing.PrecisePoint = params.PrecisePoint
	}

	if params.CoarseGeohash != nil {
		if strings.TrimSpace(*params.CoarseGeohash) == "" {
			return nil, ErrEmptyCoarseGeohash
		}
		existing.CoarseGeohash = *params.CoarseGeohash
	}

	if params.LocationAccess != nil {
		if *params.LocationAccess != "public" && *params.LocationAccess != "protected" {
			return nil, fmt.Errorf("location_access must be public or protected")
		}
		existing.LocationAccess = *params.LocationAccess
	}

	// Handle time updates with validation
	startsAt := existing.StartsAt
	endsAt := existing.EndsAt

	if params.StartsAt != nil {
		startsAt = *params.StartsAt
	}

	if params.EndsAt != nil {
		endsAt = params.EndsAt
	}

	if err := validateTimeWindow(startsAt, endsAt); err != nil {
		return nil, err
	}

	existing.StartsAt = startsAt
	existing.EndsAt = endsAt

	now := s.now()
	existing.UpdatedAt = &now

	if err := s.eventRepo.Update(existing); err != nil {
		return nil, err
	}

	return s.eventRepo.GetByID(eventID)
}

// GetEvent retrieves an event by ID (delegates to repository).
func (s *Service) GetEvent(ctx context.Context, id string) (*Event, error) {
	return s.eventRepo.GetByID(id)
}

// CancelEvent marks an event as cancelled.
func (s *Service) CancelEvent(ctx context.Context, id string, reason *string) error {
	return s.eventRepo.Cancel(id, reason)
}

// ---------------------------------------------------------------------------
// RSVP methods
// ---------------------------------------------------------------------------

// CreateOrUpdateRSVP validates status, checks the event is upcoming,
// and upserts the RSVP.
func (s *Service) CreateOrUpdateRSVP(ctx context.Context, userDID, eventID, status string) error {
	status = strings.TrimSpace(status)
	if status != "going" && status != "maybe" {
		return fmt.Errorf("%w: %s", ErrInvalidRSVPStatus, status)
	}

	// Verify event exists and is upcoming
	event, err := s.eventRepo.GetByID(eventID)
	if err != nil {
		return err
	}

	if !event.StartsAt.After(s.now()) {
		return ErrEventNotUpcoming
	}

	rsvp := &RSVP{
		EventID: eventID,
		UserID:  userDID,
		Status:  status,
	}

	return s.rsvpRepo.Upsert(rsvp)
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func validateVisibility(visibility string) error {
	if visibility == "" {
		return nil
	}
	if visibility != "public" && visibility != "private" && visibility != "unlisted" {
		return ErrInvalidVisibility
	}
	return nil
}

func validateTimeWindow(startsAt time.Time, endsAt *time.Time) error {
	if endsAt != nil && !startsAt.Before(*endsAt) {
		return ErrInvalidTimeRange
	}
	return nil
}

func sanitizeTagsSlice(tags []string) []string {
	sanitized := make([]string, len(tags))
	for i, tag := range tags {
		sanitized[i] = validate.SanitizeHTML(tag)
	}
	return sanitized
}