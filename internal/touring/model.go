// Package touring models public identities and traveling event appearances.
package touring

import (
	"errors"
	"fmt"
	"strings"
	"time"
)

var (
	ErrInvalidPlace         = errors.New("invalid place")
	ErrInvalidProfile       = errors.New("invalid profile")
	ErrInvalidHomeTerritory = errors.New("invalid home territory")
	ErrPreciseHomeTerritory = errors.New("home territory cannot contain precise coordinates")
	ErrInvalidTour          = errors.New("invalid tour")
	ErrInvalidTourAct       = errors.New("invalid tour act")
	ErrInvalidAppearance    = errors.New("invalid appearance")
	ErrInvalidEventHost     = errors.New("invalid event host")
	ErrInvalidSource        = errors.New("invalid source")
	ErrInvalidAssertion     = errors.New("invalid entity assertion")
	ErrInvalidSupersession  = errors.New("invalid assertion supersession")
)

const (
	VisibilityPublic   = "public"
	VisibilityPrivate  = "private"
	VisibilityUnlisted = "unlisted"

	EventKindShow      = "show"
	EventKindFestival  = "festival"
	EventKindParty     = "party"
	EventKindMeetup    = "meetup"
	EventKindBroadcast = "broadcast"
	EventKindOther     = "other"

	TourStatusDraft     = "draft"
	TourStatusAnnounced = "announced"
	TourStatusChanged   = "changed"
	TourStatusCancelled = "cancelled"
	TourStatusCompleted = "completed"

	AppearanceStatusAnnounced = "announced"
	AppearanceStatusConfirmed = "confirmed"
	AppearanceStatusCancelled = "cancelled"
	AppearanceStatusCompleted = "completed"

	AppearanceProjectionTourStop           = "tour_stop"
	AppearanceProjectionFestivalAppearance = "festival_appearance"
	AppearanceProjectionOneOff             = "one_off"
)

var profileKinds = map[string]struct{}{
	"artist": {}, "venue": {}, "festival": {}, "promoter": {},
	"collective": {}, "label": {}, "curator": {},
}

var visibilityValues = map[string]struct{}{
	VisibilityPublic: {}, VisibilityPrivate: {}, VisibilityUnlisted: {},
}

var eventKinds = map[string]struct{}{
	EventKindShow: {}, EventKindFestival: {}, EventKindParty: {},
	EventKindMeetup: {}, EventKindBroadcast: {}, EventKindOther: {},
}

var tourStatuses = map[string]struct{}{
	TourStatusDraft: {}, TourStatusAnnounced: {}, TourStatusChanged: {},
	TourStatusCancelled: {}, TourStatusCompleted: {},
}

var tourActRoles = map[string]struct{}{
	"primary": {}, "co_headliner": {}, "support": {}, "guest": {},
}

var appearanceRoles = map[string]struct{}{
	"headliner": {}, "support": {}, "performer": {}, "dj": {},
	"speaker": {}, "host": {}, "other": {},
}

var appearanceStatuses = map[string]struct{}{
	AppearanceStatusAnnounced: {}, AppearanceStatusConfirmed: {},
	AppearanceStatusCancelled: {}, AppearanceStatusCompleted: {},
}

var eventHostRoles = map[string]struct{}{
	"host": {}, "promoter": {}, "venue": {}, "publisher": {},
}

var entityTypes = map[string]struct{}{
	"event": {}, "appearance": {}, "tour": {}, "profile": {}, "venue": {},
}

var assertionStates = map[string]struct{}{
	"unverified": {}, "claimed": {}, "verified": {}, "disputed": {}, "rejected": {},
}

var submitterTypes = map[string]struct{}{
	"did": {}, "integration": {},
}

var authorityTypes = map[string]struct{}{
	"artist": {}, "host": {}, "venue": {}, "promoter": {}, "ticketing_provider": {}, "community_proposal": {},
}

// Point is accepted on input models only where the domain explicitly permits
// precise coordinates. HomeTerritory rejects it unconditionally.
type Point struct {
	Lat float64 `json:"lat"`
	Lng float64 `json:"lng"`
}

// Place is a canonical city, market, or regional discovery context.
type Place struct {
	ID                string `json:"id"`
	CanonicalName     string `json:"canonical_name"`
	AdminRegion       string `json:"admin_region,omitempty"`
	CountryCode       string `json:"country_code"`
	Timezone          string `json:"timezone"`
	CoarseGeohash     string `json:"coarse_geohash"`
	Version           int64  `json:"version"`
	CreatedByUserID   string `json:"-"`
	PublicationStatus string `json:"publication_status,omitempty"`
	ATURI             string `json:"at_uri,omitempty"`
	CID               string `json:"cid,omitempty"`
	PublisherDID      string `json:"publisher_did,omitempty"`
	PublisherHandle   string `json:"publisher_handle,omitempty"`
	ProjectionStatus  string `json:"projection_status,omitempty"`
}

func (p Place) Validate() error {
	if strings.TrimSpace(p.CanonicalName) == "" ||
		len(strings.TrimSpace(p.CountryCode)) != 2 ||
		strings.TrimSpace(p.Timezone) == "" ||
		strings.TrimSpace(p.CoarseGeohash) == "" {
		return ErrInvalidPlace
	}
	if _, err := time.LoadLocation(p.Timezone); err != nil {
		return fmt.Errorf("%w: timezone: %v", ErrInvalidPlace, err)
	}
	return nil
}

// Venue is a named location inside a Place. Precise coordinates require an
// explicit retention decision and are not inherited by its Place.
type Venue struct {
	ID                string `json:"id"`
	PlaceID           string `json:"place_id"`
	CanonicalName     string `json:"canonical_name"`
	AllowPrecise      bool   `json:"allow_precise"`
	PrecisePoint      *Point `json:"precise_point,omitempty"`
	CoarseGeohash     string `json:"coarse_geohash"`
	Version           int64  `json:"version"`
	PublicationStatus string `json:"publication_status,omitempty"`
	ATURI             string `json:"at_uri,omitempty"`
	CID               string `json:"cid,omitempty"`
	PublisherDID      string `json:"publisher_did,omitempty"`
	PublisherHandle   string `json:"publisher_handle,omitempty"`
	ProjectionStatus  string `json:"projection_status,omitempty"`
}

func (v *Venue) EnforceLocationConsent() {
	if !v.AllowPrecise {
		v.PrecisePoint = nil
	}
}

func (v Venue) Validate() error {
	if strings.TrimSpace(v.ID) == "" || strings.TrimSpace(v.PlaceID) == "" ||
		strings.TrimSpace(v.CanonicalName) == "" || strings.TrimSpace(v.CoarseGeohash) == "" {
		return ErrInvalidPlace
	}
	if v.PrecisePoint != nil && (v.PrecisePoint.Lat < -90 || v.PrecisePoint.Lat > 90 ||
		v.PrecisePoint.Lng < -180 || v.PrecisePoint.Lng > 180) {
		return ErrInvalidPlace
	}
	return nil
}

// Profile is the public presentation and control surface for an artist,
// venue, festival, promoter, or other participating organization.
type Profile struct {
	ID                string `json:"id"`
	Kind              string `json:"kind"`
	CanonicalName     string `json:"canonical_name"`
	Visibility        string `json:"visibility"`
	Version           int64  `json:"version"`
	CreatedByUserID   string `json:"-"`
	PublicationStatus string `json:"publication_status,omitempty"`
	ATURI             string `json:"at_uri,omitempty"`
	CID               string `json:"cid,omitempty"`
	PublisherDID      string `json:"publisher_did,omitempty"`
	PublisherHandle   string `json:"publisher_handle,omitempty"`
	ProjectionStatus  string `json:"projection_status,omitempty"`
}

func (p Profile) Validate() error {
	if strings.TrimSpace(p.CanonicalName) == "" {
		return ErrInvalidProfile
	}
	if _, ok := profileKinds[p.Kind]; !ok {
		return ErrInvalidProfile
	}
	if _, ok := visibilityValues[p.Visibility]; !ok {
		return ErrInvalidProfile
	}
	return nil
}

// Act is a creative project represented by a Profile.
type Act struct {
	ID                string `json:"id"`
	ProfileID         string `json:"profile_id"`
	PublicationStatus string `json:"publication_status,omitempty"`
}

// HomeTerritory is a declared, temporal Act-to-Place affinity. PrecisePoint is
// retained only to reject unsafe input; it must never be persisted.
type HomeTerritory struct {
	ID            string     `json:"id"`
	ActID         string     `json:"act_id"`
	PlaceID       string     `json:"place_id"`
	Visibility    string     `json:"visibility"`
	ValidFrom     time.Time  `json:"valid_from"`
	ValidTo       *time.Time `json:"valid_to,omitempty"`
	AssertedByDID string     `json:"asserted_by_did"`
	PrecisePoint  *Point     `json:"precise_point,omitempty"`
}

func (h HomeTerritory) Validate() error {
	if h.PrecisePoint != nil {
		return ErrPreciseHomeTerritory
	}
	if strings.TrimSpace(h.ActID) == "" || strings.TrimSpace(h.PlaceID) == "" ||
		strings.TrimSpace(h.AssertedByDID) == "" || h.ValidFrom.IsZero() {
		return ErrInvalidHomeTerritory
	}
	if _, ok := visibilityValues[h.Visibility]; !ok {
		return ErrInvalidHomeTerritory
	}
	if h.ValidTo != nil && h.ValidTo.Before(h.ValidFrom) {
		return ErrInvalidHomeTerritory
	}
	return nil
}

// Tour is an Act-led itinerary. It groups appearances without changing their
// independent Event, venue, scene-host, or provenance relationships.
type Tour struct {
	ID                string     `json:"id"`
	PrimaryActID      string     `json:"primary_act_id"`
	Title             string     `json:"title"`
	Status            string     `json:"status"`
	StartsOn          *time.Time `json:"starts_on,omitempty"`
	EndsOn            *time.Time `json:"ends_on,omitempty"`
	Version           int64      `json:"version"`
	CreatedByUserID   string     `json:"-"`
	PublicationStatus string     `json:"publication_status,omitempty"`
	ATURI             string     `json:"at_uri,omitempty"`
	CID               string     `json:"cid,omitempty"`
	PublisherDID      string     `json:"publisher_did,omitempty"`
	PublisherHandle   string     `json:"publisher_handle,omitempty"`
	ProjectionStatus  string     `json:"projection_status,omitempty"`
}

func (t Tour) Validate() error {
	if strings.TrimSpace(t.ID) == "" || strings.TrimSpace(t.PrimaryActID) == "" || strings.TrimSpace(t.Title) == "" {
		return ErrInvalidTour
	}
	if _, ok := tourStatuses[t.Status]; !ok {
		return ErrInvalidTour
	}
	if t.StartsOn != nil && t.EndsOn != nil && t.EndsOn.Before(*t.StartsOn) {
		return ErrInvalidTour
	}
	return nil
}

// TourAct records a billed Act relationship for a Tour. The primary act is
// persisted explicitly so all billing relationships share one representation.
type TourAct struct {
	TourID     string `json:"tour_id"`
	ActID      string `json:"act_id"`
	Role       string `json:"role"`
	AddedByDID string `json:"added_by_did"`
}

func (t TourAct) Validate() error {
	if strings.TrimSpace(t.TourID) == "" || strings.TrimSpace(t.ActID) == "" || strings.TrimSpace(t.AddedByDID) == "" {
		return ErrInvalidTourAct
	}
	if _, ok := tourActRoles[t.Role]; !ok {
		return ErrInvalidTourAct
	}
	return nil
}

// Appearance is an Act's participation in one Event. A tour stop, festival
// appearance, and one-off are display projections of this one relationship.
type Appearance struct {
	ID                string     `json:"id"`
	EventID           string     `json:"event_id"`
	ActID             string     `json:"act_id"`
	TourID            *string    `json:"tour_id,omitempty"`
	Role              string     `json:"role"`
	StageName         string     `json:"stage_name,omitempty"`
	StartsAt          *time.Time `json:"starts_at,omitempty"`
	EndsAt            *time.Time `json:"ends_at,omitempty"`
	Status            string     `json:"status"`
	Version           int64      `json:"version"`
	CreatedByUserID   string     `json:"-"`
	PublicationStatus string     `json:"publication_status,omitempty"`
	ATURI             string     `json:"at_uri,omitempty"`
	CID               string     `json:"cid,omitempty"`
	PublisherDID      string     `json:"publisher_did,omitempty"`
	PublisherHandle   string     `json:"publisher_handle,omitempty"`
	ProjectionStatus  string     `json:"projection_status,omitempty"`
}

func (a Appearance) Validate() error {
	if strings.TrimSpace(a.ID) == "" || strings.TrimSpace(a.EventID) == "" || strings.TrimSpace(a.ActID) == "" {
		return ErrInvalidAppearance
	}
	if _, ok := appearanceRoles[a.Role]; !ok {
		return ErrInvalidAppearance
	}
	if _, ok := appearanceStatuses[a.Status]; !ok {
		return ErrInvalidAppearance
	}
	if a.EndsAt != nil && a.StartsAt != nil && !a.EndsAt.After(*a.StartsAt) {
		return ErrInvalidAppearance
	}
	return nil
}

// ProjectAppearanceKind returns presentation language without introducing
// duplicate domain records for tour stops, festival appearances, or one-offs.
func ProjectAppearanceKind(tourID *string, eventKind string) string {
	if tourID != nil && strings.TrimSpace(*tourID) != "" {
		return AppearanceProjectionTourStop
	}
	if eventKind == EventKindFestival {
		return AppearanceProjectionFestivalAppearance
	}
	return AppearanceProjectionOneOff
}

// EventHost supplements the legacy events.scene_id relation with all parties
// that host, promote, provide a venue, or publish an Event.
type EventHost struct {
	EventID   string  `json:"event_id"`
	SceneID   *string `json:"scene_id,omitempty"`
	ProfileID *string `json:"profile_id,omitempty"`
	Role      string  `json:"role"`
}

func (h EventHost) Validate() error {
	if strings.TrimSpace(h.EventID) == "" {
		return ErrInvalidEventHost
	}
	if (h.SceneID == nil) == (h.ProfileID == nil) ||
		(h.SceneID != nil && strings.TrimSpace(*h.SceneID) == "") ||
		(h.ProfileID != nil && strings.TrimSpace(*h.ProfileID) == "") {
		return ErrInvalidEventHost
	}
	if _, ok := eventHostRoles[h.Role]; !ok {
		return ErrInvalidEventHost
	}
	return nil
}

// Source identifies an independently observed external or first-party record.
// The payload hash identifies what was observed without retaining the payload.
type Source struct {
	ID            string    `json:"id"`
	Provider      string    `json:"provider"`
	ExternalID    *string   `json:"external_id,omitempty"`
	CanonicalURL  *string   `json:"canonical_url,omitempty"`
	PayloadSHA256 string    `json:"payload_sha256"`
	FirstSeenAt   time.Time `json:"first_seen_at"`
	LastSeenAt    time.Time `json:"last_seen_at"`
}

func (s Source) Validate() error {
	if strings.TrimSpace(s.ID) == "" || strings.TrimSpace(s.Provider) == "" ||
		(s.ExternalID == nil && s.CanonicalURL == nil) ||
		(s.ExternalID != nil && strings.TrimSpace(*s.ExternalID) == "") ||
		(s.CanonicalURL != nil && strings.TrimSpace(*s.CanonicalURL) == "") ||
		len(s.PayloadSHA256) != 64 ||
		s.FirstSeenAt.IsZero() || s.LastSeenAt.IsZero() || s.LastSeenAt.Before(s.FirstSeenAt) {
		return ErrInvalidSource
	}
	for _, char := range s.PayloadSHA256 {
		if !(char >= '0' && char <= '9') && !(char >= 'a' && char <= 'f') && !(char >= 'A' && char <= 'F') {
			return ErrInvalidSource
		}
	}
	return nil
}

// EntityAssertion preserves an attributed claim about a public entity. It is
// deliberately not the entity itself: multiple claims may conflict, and a
// correction points to the claim it supersedes.
type EntityAssertion struct {
	ID             string         `json:"id"`
	EntityType     string         `json:"entity_type"`
	EntityID       string         `json:"entity_id"`
	SourceID       string         `json:"source_id"`
	State          string         `json:"state"`
	SubmitterType  string         `json:"submitter_type"`
	SubmittedByDID *string        `json:"submitted_by_did,omitempty"`
	IntegrationID  *string        `json:"integration_id,omitempty"`
	AuthorityType  string         `json:"authority_type"`
	AssertedFields map[string]any `json:"asserted_fields"`
	Rationale      string         `json:"rationale,omitempty"`
	ReviewedByDID  *string        `json:"reviewed_by_did,omitempty"`
	ReviewedAt     *time.Time     `json:"reviewed_at,omitempty"`
	AssertedAt     time.Time      `json:"asserted_at"`
	SupersedesID   *string        `json:"supersedes_id,omitempty"`
}

func (a EntityAssertion) Validate() error {
	if strings.TrimSpace(a.ID) == "" || strings.TrimSpace(a.EntityID) == "" || strings.TrimSpace(a.SourceID) == "" ||
		len(a.AssertedFields) == 0 || a.AssertedAt.IsZero() {
		return ErrInvalidAssertion
	}
	if _, ok := entityTypes[a.EntityType]; !ok {
		return ErrInvalidAssertion
	}
	if _, ok := assertionStates[a.State]; !ok {
		return ErrInvalidAssertion
	}
	if _, ok := submitterTypes[a.SubmitterType]; !ok {
		return ErrInvalidAssertion
	}
	if _, ok := authorityTypes[a.AuthorityType]; !ok {
		return ErrInvalidAssertion
	}
	didPresent := a.SubmittedByDID != nil && strings.TrimSpace(*a.SubmittedByDID) != ""
	integrationPresent := a.IntegrationID != nil && strings.TrimSpace(*a.IntegrationID) != ""
	if (a.SubmitterType == "did" && (!didPresent || integrationPresent)) ||
		(a.SubmitterType == "integration" && (!integrationPresent || didPresent)) {
		return ErrInvalidAssertion
	}
	if (a.ReviewedByDID == nil) != (a.ReviewedAt == nil) {
		return ErrInvalidAssertion
	}
	return nil
}
