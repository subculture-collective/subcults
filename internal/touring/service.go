package touring

import (
	"context"
	"strings"
	"time"

	"github.com/google/uuid"
)

// Service holds the touring repository and a clock for timestamping.
type Service struct {
	repo Repository
	now  func() time.Time
}

// NewService constructs a touring domain service.
func NewService(repo Repository) *Service {
	return &Service{repo: repo, now: time.Now}
}

// CreateProfile sets defaults, stores the profile, and when kind is "artist"
// creates and stores an Act with a fresh UUID.
func (s *Service) CreateProfile(ctx context.Context, kind, canonicalName, visibility, createdByUserID string) (Profile, *Act, error) {
	if visibility == "" {
		visibility = VisibilityPublic
	}
	profile := Profile{
		ID:                uuid.NewString(),
		Kind:              kind,
		CanonicalName:     strings.TrimSpace(canonicalName),
		Visibility:        visibility,
		Version:           1,
		CreatedByUserID:   createdByUserID,
		PublicationStatus: "draft",
	}
	if err := s.repo.StoreProfile(profile); err != nil {
		return Profile{}, nil, err
	}
	if kind == "artist" {
		act := Act{ID: uuid.NewString(), ProfileID: profile.ID, PublicationStatus: "draft"}
		if err := s.repo.StoreAct(act); err != nil {
			return Profile{}, nil, err
		}
		return profile, &act, nil
	}
	return profile, nil, nil
}

// CreatePlace sets defaults and stores the place.
func (s *Service) CreatePlace(ctx context.Context, place Place, createdByUserID string) (Place, error) {
	if place.ID == "" {
		place.ID = uuid.NewString()
	}
	if place.Version == 0 {
		place.Version = 1
	}
	place.CreatedByUserID = createdByUserID
	place.PublicationStatus = "draft"
	if err := s.repo.StorePlace(place); err != nil {
		return Place{}, err
	}
	return place, nil
}

// CreateVenue sets defaults (fresh UUID, version 1, allow_precise false,
// draft status) and stores the venue.
func (s *Service) CreateVenue(ctx context.Context, venue Venue) (Venue, error) {
	venue.ID = uuid.NewString()
	venue.Version = 1
	venue.PublicationStatus = "draft"
	venue.AllowPrecise = false
	venue.PrecisePoint = nil
	if err := s.repo.StoreVenue(venue); err != nil {
		return Venue{}, err
	}
	return venue, nil
}

// CreateTour sets defaults, validates the primary act exists, and stores the
// tour together with its primary TourAct.
func (s *Service) CreateTour(ctx context.Context, tour Tour, createdByUserID, primaryAddedByDID string) (Tour, error) {
	if tour.ID == "" {
		tour.ID = uuid.NewString()
	}
	if tour.Status == "" {
		tour.Status = TourStatusDraft
	}
	if tour.Version == 0 {
		tour.Version = 1
	}
	tour.CreatedByUserID = createdByUserID
	tour.PublicationStatus = "draft"
	if _, err := s.repo.GetAct(tour.PrimaryActID); err != nil {
		return Tour{}, err
	}
	if err := s.repo.CreateTour(tour, primaryAddedByDID); err != nil {
		return Tour{}, err
	}
	return tour, nil
}

// CreateAppearance sets defaults, validates the act exists, and stores the
// appearance. Event validation is left to the caller via its own repository.
func (s *Service) CreateAppearance(ctx context.Context, appearance Appearance, createdByUserID string) (Appearance, error) {
	if appearance.ID == "" {
		appearance.ID = uuid.NewString()
	}
	if appearance.Status == "" {
		appearance.Status = AppearanceStatusAnnounced
	}
	if appearance.Version == 0 {
		appearance.Version = 1
	}
	appearance.CreatedByUserID = createdByUserID
	appearance.PublicationStatus = "draft"
	if _, err := s.repo.GetAct(appearance.ActID); err != nil {
		return Appearance{}, err
	}
	if err := s.repo.CreateAppearance(appearance); err != nil {
		return Appearance{}, err
	}
	return appearance, nil
}

// UpdateProfile delegates to the repository.
func (s *Service) UpdateProfile(update Profile) error {
	return s.repo.UpdateProfile(&update)
}

// UpdatePlace delegates to the repository.
func (s *Service) UpdatePlace(update Place) error {
	return s.repo.UpdatePlace(&update)
}

// UpdateVenue delegates to the repository.
func (s *Service) UpdateVenue(update Venue) error {
	return s.repo.UpdateVenue(&update)
}

// UpdateTour delegates to the repository.
func (s *Service) UpdateTour(update Tour) error {
	return s.repo.UpdateTour(&update)
}

// UpdateAppearance delegates to the repository.
func (s *Service) UpdateAppearance(update Appearance) error {
	return s.repo.UpdateAppearance(&update)
}

// GetProfile delegates to the repository.
func (s *Service) GetProfile(id string) (Profile, error) {
	return s.repo.GetProfile(id)
}

// GetPlace delegates to the repository.
func (s *Service) GetPlace(id string) (Place, error) {
	return s.repo.GetPlace(id)
}

// GetVenue delegates to the repository.
func (s *Service) GetVenue(id string) (Venue, error) {
	return s.repo.GetVenue(id)
}

// GetTour delegates to the repository.
func (s *Service) GetTour(id string) (Tour, error) {
	return s.repo.GetTour(id)
}

// GetAppearance delegates to the repository.
func (s *Service) GetAppearance(id string) (Appearance, error) {
	return s.repo.GetAppearance(id)
}

// GetAct delegates to the repository.
func (s *Service) GetAct(id string) (Act, error) {
	return s.repo.GetAct(id)
}

// ListAppearances delegates to the repository.
func (s *Service) ListAppearances() ([]Appearance, error) {
	return s.repo.ListAppearances()
}

// ListAppearancesForTour delegates to the repository.
func (s *Service) ListAppearancesForTour(tourID string) ([]Appearance, error) {
	return s.repo.ListAppearancesForTour(tourID)
}

// ListAppearancesForAct delegates to the repository.
func (s *Service) ListAppearancesForAct(actID string) ([]Appearance, error) {
	return s.repo.ListAppearancesForAct(actID)
}

// FindActByProfile delegates to the repository.
func (s *Service) FindActByProfile(profileID string) (Act, error) {
	return s.repo.FindActByProfile(profileID)
}

// ListHomeTerritories delegates to the repository.
func (s *Service) ListHomeTerritories(actID string) ([]HomeTerritory, error) {
	return s.repo.ListHomeTerritories(actID)
}

// ListEventHosts delegates to the repository.
func (s *Service) ListEventHosts(eventID string) ([]EventHost, error) {
	return s.repo.ListEventHosts(eventID)
}

// VerificationForEntity delegates to the repository.
func (s *Service) VerificationForEntity(entityType, entityID string) string {
	return s.repo.VerificationForEntity(entityType, entityID)
}

// UpdateAct delegates to the repository.
func (s *Service) UpdateAct(ctx context.Context, update Act) error {
	return s.repo.StoreAct(update)
}