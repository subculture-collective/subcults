package touring

import (
	"errors"
	"testing"
	"time"
)

func TestInMemoryRepositoryTourAddsPrimaryActAndOrdersStops(t *testing.T) {
	repo := NewInMemoryRepository()
	tour := validTour()
	if err := repo.CreateTour(tour, "did:plc:artist"); err != nil {
		t.Fatalf("CreateTour() error = %v", err)
	}
	acts, err := repo.ListTourActs(tour.ID)
	if err != nil {
		t.Fatalf("ListTourActs() error = %v", err)
	}
	if len(acts) != 1 || acts[0].ActID != tour.PrimaryActID || acts[0].Role != "primary" {
		t.Fatalf("primary tour act = %#v, want explicit primary relationship", acts)
	}

	later := time.Date(2026, time.October, 3, 2, 0, 0, 0, time.UTC)
	earlier := time.Date(2026, time.September, 30, 2, 0, 0, 0, time.UTC)
	if err := repo.CreateAppearance(Appearance{ID: "later", EventID: "event-later", ActID: tour.PrimaryActID, TourID: &tour.ID, Role: "headliner", StartsAt: &later, Status: AppearanceStatusAnnounced}); err != nil {
		t.Fatalf("CreateAppearance(later) error = %v", err)
	}
	if err := repo.CreateAppearance(Appearance{ID: "earlier", EventID: "event-earlier", ActID: tour.PrimaryActID, TourID: &tour.ID, Role: "headliner", StartsAt: &earlier, Status: AppearanceStatusConfirmed}); err != nil {
		t.Fatalf("CreateAppearance(earlier) error = %v", err)
	}
	stops, err := repo.ListAppearancesForTour(tour.ID)
	if err != nil {
		t.Fatalf("ListAppearancesForTour() error = %v", err)
	}
	if len(stops) != 2 || stops[0].ID != "earlier" || stops[1].ID != "later" {
		t.Fatalf("tour stops = %#v, want chronological order", stops)
	}
}

func TestInMemoryRepositorySupportsFestivalAndOneOffAppearances(t *testing.T) {
	repo := NewInMemoryRepository()
	if err := repo.CreateAppearance(Appearance{ID: "festival", EventID: "festival-event", ActID: "act", Role: "performer", Status: AppearanceStatusConfirmed}); err != nil {
		t.Fatalf("CreateAppearance(festival) error = %v", err)
	}
	if err := repo.CreateAppearance(Appearance{ID: "one-off", EventID: "show-event", ActID: "act", Role: "headliner", Status: AppearanceStatusAnnounced}); err != nil {
		t.Fatalf("CreateAppearance(one-off) error = %v", err)
	}
	if got := ProjectAppearanceKind(nil, EventKindFestival); got != AppearanceProjectionFestivalAppearance {
		t.Fatalf("festival projection = %q", got)
	}
	if got := ProjectAppearanceKind(nil, EventKindShow); got != AppearanceProjectionOneOff {
		t.Fatalf("one-off projection = %q", got)
	}
}

func TestInMemoryRepositoryAllowsMultiHostsButRejectsExactDuplicate(t *testing.T) {
	repo := NewInMemoryRepository()
	sceneID, profileID := "scene", "profile"
	if err := repo.AddEventHost(EventHost{EventID: "event", SceneID: &sceneID, Role: "host"}); err != nil {
		t.Fatalf("AddEventHost(scene) error = %v", err)
	}
	if err := repo.AddEventHost(EventHost{EventID: "event", ProfileID: &profileID, Role: "promoter"}); err != nil {
		t.Fatalf("AddEventHost(profile) error = %v", err)
	}
	if err := repo.AddEventHost(EventHost{EventID: "event", SceneID: &sceneID, Role: "host"}); !errors.Is(err, ErrDuplicateHost) {
		t.Fatalf("duplicate host error = %v, want ErrDuplicateHost", err)
	}
	hosts, err := repo.ListEventHosts("event")
	if err != nil || len(hosts) != 2 {
		t.Fatalf("ListEventHosts() = %#v, %v; want two hosts", hosts, err)
	}
}

func TestInMemoryRepositoryDeduplicatesSourcesAndPreservesCorrections(t *testing.T) {
	repo := NewInMemoryRepository()
	externalID := "event-123"
	firstSeen := time.Date(2026, time.August, 1, 0, 0, 0, 0, time.UTC)
	lastSeen := firstSeen.Add(time.Hour)
	source := Source{ID: "source", Provider: "Ticketing", ExternalID: &externalID, PayloadSHA256: validSHA256, FirstSeenAt: firstSeen, LastSeenAt: firstSeen}
	stored, err := repo.UpsertSource(source)
	if err != nil {
		t.Fatalf("UpsertSource(first) error = %v", err)
	}
	source.ID = "duplicate-id"
	source.LastSeenAt = lastSeen
	source.PayloadSHA256 = alternateSHA256
	storedAgain, err := repo.UpsertSource(source)
	if err != nil {
		t.Fatalf("UpsertSource(second) error = %v", err)
	}
	if storedAgain.ID != stored.ID || !storedAgain.LastSeenAt.Equal(lastSeen) || storedAgain.PayloadSHA256 != alternateSHA256 {
		t.Fatalf("deduplicated source = %#v", storedAgain)
	}

	did := "did:plc:artist"
	original := EntityAssertion{
		ID: "original", EntityType: "appearance", EntityID: "appearance", SourceID: stored.ID,
		State: "claimed", SubmitterType: "did", SubmittedByDID: &did, AuthorityType: "artist",
		AssertedFields: map[string]any{"starts_at": "2026-09-01T20:00:00Z"}, AssertedAt: firstSeen,
	}
	if err := repo.CreateAssertion(original); err != nil {
		t.Fatalf("CreateAssertion(original) error = %v", err)
	}
	correction := original
	correction.ID = "correction"
	correction.AssertedFields = map[string]any{"starts_at": "2026-09-01T21:00:00Z"}
	correction.SupersedesID = &original.ID
	correction.AssertedAt = lastSeen
	if err := repo.CreateAssertion(correction); err != nil {
		t.Fatalf("CreateAssertion(correction) error = %v", err)
	}
	wrongEntity := correction
	wrongEntity.ID = "wrong-entity"
	wrongEntity.EntityID = "other-appearance"
	if err := repo.CreateAssertion(wrongEntity); !errors.Is(err, ErrInvalidSupersession) {
		t.Fatalf("cross-entity correction error = %v, want ErrInvalidSupersession", err)
	}
}

const validSHA256 = "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
const alternateSHA256 = "abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789"

func validTour() Tour {
	startsOn := time.Date(2026, time.September, 1, 0, 0, 0, 0, time.UTC)
	endsOn := time.Date(2026, time.October, 10, 0, 0, 0, 0, time.UTC)
	return Tour{ID: "tour", PrimaryActID: "act", Title: "Two City Run", Status: TourStatusAnnounced, StartsOn: &startsOn, EndsOn: &endsOn}
}
