package touring

import (
	"errors"
	"sort"
	"strings"
	"sync"
	"time"
)

var (
	ErrTourNotFound        = errors.New("tour not found")
	ErrAppearanceNotFound  = errors.New("appearance not found")
	ErrSourceNotFound      = errors.New("source not found")
	ErrAssertionNotFound   = errors.New("entity assertion not found")
	ErrDuplicateTour       = errors.New("duplicate tour")
	ErrDuplicateHost       = errors.New("duplicate event host")
	ErrDuplicateTourAct    = errors.New("duplicate tour act")
	ErrDuplicateAppearance = errors.New("duplicate appearance")
	ErrDuplicateSource     = errors.New("duplicate source")
	ErrDuplicateAssertion  = errors.New("duplicate entity assertion")
	ErrVersionConflict     = errors.New("stale touring aggregate version")
)

// Repository is the touring persistence boundary. This in-memory implementation
// is used only for domain tests and local fixtures; it is not a Postgres adapter.
type Repository interface {
	CreateTour(tour Tour, primaryAddedByDID string) error
	GetTour(id string) (Tour, error)
	AddTourAct(tourAct TourAct) error
	ListTourActs(tourID string) ([]TourAct, error)
	CreateAppearance(appearance Appearance) error
	GetAppearance(id string) (Appearance, error)
	ListAppearancesForTour(tourID string) ([]Appearance, error)
	AddEventHost(host EventHost) error
	ListEventHosts(eventID string) ([]EventHost, error)
	UpsertSource(source Source) (Source, error)
	CreateAssertion(assertion EntityAssertion) error
	GetAssertion(id string) (EntityAssertion, error)
	StorePlace(place Place) error
	StoreVenue(venue Venue) error
	StoreProfile(profile Profile) error
	StoreAct(act Act) error
	UpdatePlace(place *Place) error
	UpdateVenue(venue *Venue) error
	UpdateProfile(profile *Profile) error
	UpdateTour(tour *Tour) error
	UpdateAppearance(appearance *Appearance) error
	AddHomeTerritory(territory HomeTerritory) error
	GetPlace(id string) (Place, error)
	GetVenue(id string) (Venue, error)
	GetProfile(id string) (Profile, error)
	GetAct(id string) (Act, error)
	FindActByProfile(profileID string) (Act, error)
	ListHomeTerritories(actID string) ([]HomeTerritory, error)
	ListAppearances() ([]Appearance, error)
	ListAppearancesForAct(actID string) ([]Appearance, error)
	VerificationForEntity(entityType, entityID string) string
}

// InMemoryRepository implements Repository with the same business invariants
// expected of the future durable adapter.
type InMemoryRepository struct {
	mu              sync.RWMutex
	tours           map[string]Tour
	tourActs        map[string][]TourAct
	appearances     map[string]Appearance
	eventHosts      map[string][]EventHost
	sources         map[string]Source
	sourceKeys      map[string]string
	assertions      map[string]EntityAssertion
	places          map[string]Place
	venues          map[string]Venue
	profiles        map[string]Profile
	acts            map[string]Act
	homeTerritories map[string][]HomeTerritory
}

func NewInMemoryRepository() *InMemoryRepository {
	return &InMemoryRepository{
		tours:           make(map[string]Tour),
		tourActs:        make(map[string][]TourAct),
		appearances:     make(map[string]Appearance),
		eventHosts:      make(map[string][]EventHost),
		sources:         make(map[string]Source),
		sourceKeys:      make(map[string]string),
		assertions:      make(map[string]EntityAssertion),
		places:          make(map[string]Place),
		venues:          make(map[string]Venue),
		profiles:        make(map[string]Profile),
		acts:            make(map[string]Act),
		homeTerritories: make(map[string][]HomeTerritory),
	}
}

func (r *InMemoryRepository) StoreVenue(venue Venue) error {
	venue.EnforceLocationConsent()
	if err := venue.Validate(); err != nil {
		return err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.places[venue.PlaceID]; !ok {
		return ErrInvalidPlace
	}
	r.venues[venue.ID] = venue
	return nil
}

func (r *InMemoryRepository) StorePlace(place Place) error {
	if err := place.Validate(); err != nil || strings.TrimSpace(place.ID) == "" {
		return ErrInvalidPlace
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.places[place.ID] = place
	return nil
}

func (r *InMemoryRepository) StoreProfile(profile Profile) error {
	if err := profile.Validate(); err != nil || strings.TrimSpace(profile.ID) == "" {
		return ErrInvalidProfile
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.profiles[profile.ID] = profile
	return nil
}

func (r *InMemoryRepository) StoreAct(act Act) error {
	if strings.TrimSpace(act.ID) == "" || strings.TrimSpace(act.ProfileID) == "" {
		return ErrInvalidProfile
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.profiles[act.ProfileID]; !ok {
		return ErrInvalidProfile
	}
	r.acts[act.ID] = act
	return nil
}

func (r *InMemoryRepository) UpdatePlace(value *Place) error {
	if err := value.Validate(); err != nil {
		return err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	current, ok := r.places[value.ID]
	if !ok {
		return ErrInvalidPlace
	}
	if current.Version != value.Version {
		return ErrVersionConflict
	}
	value.Version++
	r.places[value.ID] = *value
	return nil
}
func (r *InMemoryRepository) UpdateVenue(value *Venue) error {
	value.EnforceLocationConsent()
	if err := value.Validate(); err != nil {
		return err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	current, ok := r.venues[value.ID]
	if !ok {
		return ErrInvalidPlace
	}
	if current.Version != value.Version {
		return ErrVersionConflict
	}
	value.Version++
	r.venues[value.ID] = *value
	return nil
}
func (r *InMemoryRepository) UpdateProfile(value *Profile) error {
	if err := value.Validate(); err != nil {
		return err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	current, ok := r.profiles[value.ID]
	if !ok {
		return ErrInvalidProfile
	}
	if current.Version != value.Version {
		return ErrVersionConflict
	}
	value.Version++
	r.profiles[value.ID] = *value
	return nil
}
func (r *InMemoryRepository) UpdateTour(value *Tour) error {
	if err := value.Validate(); err != nil {
		return err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	current, ok := r.tours[value.ID]
	if !ok {
		return ErrTourNotFound
	}
	if current.Version != value.Version {
		return ErrVersionConflict
	}
	value.Version++
	r.tours[value.ID] = *value
	return nil
}
func (r *InMemoryRepository) UpdateAppearance(value *Appearance) error {
	if err := value.Validate(); err != nil {
		return err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	current, ok := r.appearances[value.ID]
	if !ok {
		return ErrAppearanceNotFound
	}
	if current.Version != value.Version {
		return ErrVersionConflict
	}
	value.Version++
	r.appearances[value.ID] = copyAppearance(*value)
	return nil
}

func (r *InMemoryRepository) AddHomeTerritory(territory HomeTerritory) error {
	if err := territory.Validate(); err != nil {
		return err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.acts[territory.ActID]; !ok {
		return ErrInvalidHomeTerritory
	}
	if _, ok := r.places[territory.PlaceID]; !ok {
		return ErrInvalidHomeTerritory
	}
	r.homeTerritories[territory.ActID] = append(r.homeTerritories[territory.ActID], territory)
	return nil
}

func (r *InMemoryRepository) GetPlace(id string) (Place, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	place, ok := r.places[id]
	if !ok {
		return Place{}, ErrInvalidPlace
	}
	return place, nil
}

func (r *InMemoryRepository) GetVenue(id string) (Venue, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	venue, ok := r.venues[id]
	if !ok {
		return Venue{}, ErrInvalidPlace
	}
	return venue, nil
}

func (r *InMemoryRepository) GetProfile(id string) (Profile, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	profile, ok := r.profiles[id]
	if !ok {
		return Profile{}, ErrInvalidProfile
	}
	return profile, nil
}

func (r *InMemoryRepository) GetAct(id string) (Act, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	act, ok := r.acts[id]
	if !ok {
		return Act{}, ErrInvalidProfile
	}
	return act, nil
}

func (r *InMemoryRepository) FindActByProfile(profileID string) (Act, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	for _, act := range r.acts {
		if act.ProfileID == profileID {
			return act, nil
		}
	}
	return Act{}, ErrInvalidProfile
}

func (r *InMemoryRepository) ListHomeTerritories(actID string) ([]HomeTerritory, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	territories := append([]HomeTerritory(nil), r.homeTerritories[actID]...)
	return territories, nil
}

func (r *InMemoryRepository) ListAppearances() ([]Appearance, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	appearances := make([]Appearance, 0, len(r.appearances))
	for _, appearance := range r.appearances {
		appearances = append(appearances, copyAppearance(appearance))
	}
	sort.Slice(appearances, func(i, j int) bool { return appearanceBefore(appearances[i], appearances[j]) })
	return appearances, nil
}

func (r *InMemoryRepository) ListAppearancesForAct(actID string) ([]Appearance, error) {
	appearances, err := r.ListAppearances()
	if err != nil {
		return nil, err
	}
	filtered := appearances[:0]
	for _, appearance := range appearances {
		if appearance.ActID == actID {
			filtered = append(filtered, appearance)
		}
	}
	return filtered, nil
}

func (r *InMemoryRepository) VerificationForEntity(entityType, entityID string) string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	state := "unverified"
	var latest time.Time
	for _, assertion := range r.assertions {
		if assertion.EntityType == entityType && assertion.EntityID == entityID && assertion.AssertedAt.After(latest) {
			state, latest = assertion.State, assertion.AssertedAt
		}
	}
	return state
}

func (r *InMemoryRepository) CreateTour(tour Tour, primaryAddedByDID string) error {
	if err := tour.Validate(); err != nil {
		return err
	}
	primary := TourAct{TourID: tour.ID, ActID: tour.PrimaryActID, Role: "primary", AddedByDID: primaryAddedByDID}
	if err := primary.Validate(); err != nil {
		return err
	}

	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.tours[tour.ID]; ok {
		return ErrDuplicateTour
	}
	r.tours[tour.ID] = tour
	r.tourActs[tour.ID] = append(r.tourActs[tour.ID], primary)
	return nil
}

func (r *InMemoryRepository) GetTour(id string) (Tour, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	tour, ok := r.tours[id]
	if !ok {
		return Tour{}, ErrTourNotFound
	}
	return copyTour(tour), nil
}

func (r *InMemoryRepository) AddTourAct(tourAct TourAct) error {
	if err := tourAct.Validate(); err != nil {
		return err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.tours[tourAct.TourID]; !ok {
		return ErrTourNotFound
	}
	for _, existing := range r.tourActs[tourAct.TourID] {
		if existing.ActID == tourAct.ActID && existing.Role == tourAct.Role {
			return ErrDuplicateTourAct
		}
	}
	r.tourActs[tourAct.TourID] = append(r.tourActs[tourAct.TourID], tourAct)
	return nil
}

func (r *InMemoryRepository) ListTourActs(tourID string) ([]TourAct, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	if _, ok := r.tours[tourID]; !ok {
		return nil, ErrTourNotFound
	}
	acts := append([]TourAct(nil), r.tourActs[tourID]...)
	sort.Slice(acts, func(i, j int) bool {
		if acts[i].Role == acts[j].Role {
			return acts[i].ActID < acts[j].ActID
		}
		return acts[i].Role < acts[j].Role
	})
	return acts, nil
}

func (r *InMemoryRepository) CreateAppearance(appearance Appearance) error {
	if err := appearance.Validate(); err != nil {
		return err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.appearances[appearance.ID]; ok {
		return ErrDuplicateAppearance
	}
	if appearance.TourID != nil {
		if _, ok := r.tours[*appearance.TourID]; !ok {
			return ErrTourNotFound
		}
	}
	for _, existing := range r.appearances {
		if existing.EventID == appearance.EventID && existing.ActID == appearance.ActID && existing.Role == appearance.Role && sameTime(existing.StartsAt, appearance.StartsAt) {
			return ErrDuplicateAppearance
		}
	}
	r.appearances[appearance.ID] = copyAppearance(appearance)
	return nil
}

func (r *InMemoryRepository) GetAppearance(id string) (Appearance, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	appearance, ok := r.appearances[id]
	if !ok {
		return Appearance{}, ErrAppearanceNotFound
	}
	return copyAppearance(appearance), nil
}

func (r *InMemoryRepository) ListAppearancesForTour(tourID string) ([]Appearance, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	if _, ok := r.tours[tourID]; !ok {
		return nil, ErrTourNotFound
	}
	appearances := make([]Appearance, 0)
	for _, appearance := range r.appearances {
		if appearance.TourID != nil && *appearance.TourID == tourID {
			appearances = append(appearances, copyAppearance(appearance))
		}
	}
	sort.Slice(appearances, func(i, j int) bool {
		if appearances[i].StartsAt == nil {
			return false
		}
		if appearances[j].StartsAt == nil {
			return true
		}
		return appearances[i].StartsAt.Before(*appearances[j].StartsAt)
	})
	return appearances, nil
}

func (r *InMemoryRepository) AddEventHost(host EventHost) error {
	if err := host.Validate(); err != nil {
		return err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, existing := range r.eventHosts[host.EventID] {
		if existing.Role == host.Role && sameHost(existing, host) {
			return ErrDuplicateHost
		}
	}
	r.eventHosts[host.EventID] = append(r.eventHosts[host.EventID], copyEventHost(host))
	return nil
}

func (r *InMemoryRepository) ListEventHosts(eventID string) ([]EventHost, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	hosts := make([]EventHost, len(r.eventHosts[eventID]))
	for i, host := range r.eventHosts[eventID] {
		hosts[i] = copyEventHost(host)
	}
	return hosts, nil
}

func (r *InMemoryRepository) UpsertSource(source Source) (Source, error) {
	if err := source.Validate(); err != nil {
		return Source{}, err
	}
	key := sourceKey(source)
	r.mu.Lock()
	defer r.mu.Unlock()
	if existing, ok := r.sources[source.ID]; ok && sourceKey(existing) != key {
		return Source{}, ErrDuplicateSource
	}
	if existingID, ok := r.sourceKeys[key]; ok {
		existing := r.sources[existingID]
		if source.LastSeenAt.After(existing.LastSeenAt) {
			existing.LastSeenAt = source.LastSeenAt
			existing.PayloadSHA256 = source.PayloadSHA256
			r.sources[existingID] = existing
		}
		return copySource(existing), nil
	}
	r.sources[source.ID] = copySource(source)
	r.sourceKeys[key] = source.ID
	return copySource(source), nil
}

func (r *InMemoryRepository) CreateAssertion(assertion EntityAssertion) error {
	if err := assertion.Validate(); err != nil {
		return err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.assertions[assertion.ID]; ok {
		return ErrDuplicateAssertion
	}
	if _, ok := r.sources[assertion.SourceID]; !ok {
		return ErrSourceNotFound
	}
	if assertion.SupersedesID != nil {
		previous, ok := r.assertions[*assertion.SupersedesID]
		if !ok || previous.EntityType != assertion.EntityType || previous.EntityID != assertion.EntityID || previous.ID == assertion.ID {
			return ErrInvalidSupersession
		}
	}
	r.assertions[assertion.ID] = copyAssertion(assertion)
	return nil
}

func (r *InMemoryRepository) GetAssertion(id string) (EntityAssertion, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	assertion, ok := r.assertions[id]
	if !ok {
		return EntityAssertion{}, ErrAssertionNotFound
	}
	return copyAssertion(assertion), nil
}

func sameTime(left, right *time.Time) bool {
	if left == nil || right == nil {
		return left == nil && right == nil
	}
	return left.Equal(*right)
}

func appearanceBefore(left, right Appearance) bool {
	if left.StartsAt == nil {
		return false
	}
	if right.StartsAt == nil {
		return true
	}
	if left.StartsAt.Equal(*right.StartsAt) {
		return left.ID < right.ID
	}
	return left.StartsAt.Before(*right.StartsAt)
}

func sameHost(left, right EventHost) bool {
	if left.SceneID != nil || right.SceneID != nil {
		return left.SceneID != nil && right.SceneID != nil && *left.SceneID == *right.SceneID
	}
	return left.ProfileID != nil && right.ProfileID != nil && *left.ProfileID == *right.ProfileID
}

func sourceKey(source Source) string {
	externalID := ""
	if source.ExternalID != nil {
		externalID = strings.TrimSpace(*source.ExternalID)
	}
	canonicalURL := ""
	if source.CanonicalURL != nil {
		canonicalURL = strings.TrimSpace(*source.CanonicalURL)
	}
	return strings.ToLower(strings.TrimSpace(source.Provider)) + "\x00" + externalID + "\x00" + canonicalURL
}

func copyTour(tour Tour) Tour {
	copy := tour
	if tour.StartsOn != nil {
		value := *tour.StartsOn
		copy.StartsOn = &value
	}
	if tour.EndsOn != nil {
		value := *tour.EndsOn
		copy.EndsOn = &value
	}
	return copy
}

func copyAppearance(appearance Appearance) Appearance {
	copy := appearance
	if appearance.TourID != nil {
		value := *appearance.TourID
		copy.TourID = &value
	}
	if appearance.StartsAt != nil {
		value := *appearance.StartsAt
		copy.StartsAt = &value
	}
	if appearance.EndsAt != nil {
		value := *appearance.EndsAt
		copy.EndsAt = &value
	}
	return copy
}

func copyEventHost(host EventHost) EventHost {
	copy := host
	if host.SceneID != nil {
		value := *host.SceneID
		copy.SceneID = &value
	}
	if host.ProfileID != nil {
		value := *host.ProfileID
		copy.ProfileID = &value
	}
	return copy
}

func copySource(source Source) Source {
	copy := source
	if source.ExternalID != nil {
		value := *source.ExternalID
		copy.ExternalID = &value
	}
	if source.CanonicalURL != nil {
		value := *source.CanonicalURL
		copy.CanonicalURL = &value
	}
	return copy
}

func copyAssertion(assertion EntityAssertion) EntityAssertion {
	copy := assertion
	copy.AssertedFields = make(map[string]any, len(assertion.AssertedFields))
	for key, value := range assertion.AssertedFields {
		copy.AssertedFields[key] = value
	}
	if assertion.SubmittedByDID != nil {
		value := *assertion.SubmittedByDID
		copy.SubmittedByDID = &value
	}
	if assertion.IntegrationID != nil {
		value := *assertion.IntegrationID
		copy.IntegrationID = &value
	}
	if assertion.ReviewedByDID != nil {
		value := *assertion.ReviewedByDID
		copy.ReviewedByDID = &value
	}
	if assertion.ReviewedAt != nil {
		value := *assertion.ReviewedAt
		copy.ReviewedAt = &value
	}
	if assertion.SupersedesID != nil {
		value := *assertion.SupersedesID
		copy.SupersedesID = &value
	}
	return copy
}
