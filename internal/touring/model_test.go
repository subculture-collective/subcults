package touring

import (
	"errors"
	"testing"
	"time"
)

func TestHomeTerritoryRejectsPreciseResidenceData(t *testing.T) {
	claim := validHomeTerritory()
	claim.PrecisePoint = &Point{Lat: 41.88, Lng: -87.63}
	if err := claim.Validate(); !errors.Is(err, ErrPreciseHomeTerritory) {
		t.Fatalf("error=%v, want ErrPreciseHomeTerritory", err)
	}
}

func TestHomeTerritoryRejectsInvalidDateRange(t *testing.T) {
	claim := validHomeTerritory()
	before := claim.ValidFrom.AddDate(0, 0, -1)
	claim.ValidTo = &before
	if err := claim.Validate(); !errors.Is(err, ErrInvalidHomeTerritory) {
		t.Fatalf("error=%v, want ErrInvalidHomeTerritory", err)
	}
}

func TestPlaceRequiresValidDiscoveryContext(t *testing.T) {
	valid := Place{CanonicalName: "Chicago", CountryCode: "US", Timezone: "America/Chicago", CoarseGeohash: "dp3"}
	if err := valid.Validate(); err != nil {
		t.Fatalf("valid place: %v", err)
	}
	valid.Timezone = "not/a-zone"
	if err := valid.Validate(); !errors.Is(err, ErrInvalidPlace) {
		t.Fatalf("error=%v, want ErrInvalidPlace", err)
	}
}

func TestProfileValidatesKindAndVisibility(t *testing.T) {
	valid := Profile{Kind: "artist", CanonicalName: "Touring Act", Visibility: VisibilityPublic}
	if err := valid.Validate(); err != nil {
		t.Fatalf("valid profile: %v", err)
	}
	valid.Kind = "user"
	if err := valid.Validate(); !errors.Is(err, ErrInvalidProfile) {
		t.Fatalf("error=%v, want ErrInvalidProfile", err)
	}
}

func TestVenueClearsUnconsentedPrecisePoint(t *testing.T) {
	venue := Venue{PrecisePoint: &Point{Lat: 41.88, Lng: -87.63}}
	venue.EnforceLocationConsent()
	if venue.PrecisePoint != nil {
		t.Fatal("precise point retained without venue consent")
	}
}

func validHomeTerritory() HomeTerritory {
	return HomeTerritory{
		ActID: "act", PlaceID: "chicago", Visibility: VisibilityPublic,
		ValidFrom:     time.Date(2026, time.January, 1, 0, 0, 0, 0, time.UTC),
		AssertedByDID: "did:plc:artist",
	}
}
