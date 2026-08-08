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
)

const (
	VisibilityPublic   = "public"
	VisibilityPrivate  = "private"
	VisibilityUnlisted = "unlisted"
)

var profileKinds = map[string]struct{}{
	"artist": {}, "venue": {}, "festival": {}, "promoter": {},
	"collective": {}, "label": {}, "curator": {},
}

var visibilityValues = map[string]struct{}{
	VisibilityPublic: {}, VisibilityPrivate: {}, VisibilityUnlisted: {},
}

// Point is accepted on input models only where the domain explicitly permits
// precise coordinates. HomeTerritory rejects it unconditionally.
type Point struct {
	Lat float64 `json:"lat"`
	Lng float64 `json:"lng"`
}

// Place is a canonical city, market, or regional discovery context.
type Place struct {
	ID            string `json:"id"`
	CanonicalName string `json:"canonical_name"`
	AdminRegion   string `json:"admin_region,omitempty"`
	CountryCode   string `json:"country_code"`
	Timezone      string `json:"timezone"`
	CoarseGeohash string `json:"coarse_geohash"`
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
	ID            string `json:"id"`
	PlaceID       string `json:"place_id"`
	CanonicalName string `json:"canonical_name"`
	AllowPrecise  bool   `json:"allow_precise"`
	PrecisePoint  *Point `json:"precise_point,omitempty"`
	CoarseGeohash string `json:"coarse_geohash"`
}

func (v *Venue) EnforceLocationConsent() {
	if !v.AllowPrecise {
		v.PrecisePoint = nil
	}
}

// Profile is the public presentation and control surface for an artist,
// venue, festival, promoter, or other participating organization.
type Profile struct {
	ID            string `json:"id"`
	Kind          string `json:"kind"`
	CanonicalName string `json:"canonical_name"`
	Visibility    string `json:"visibility"`
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
	ID        string `json:"id"`
	ProfileID string `json:"profile_id"`
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
