package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/onnwee/subcults/internal/middleware"
	"github.com/onnwee/subcults/internal/scene"
	"github.com/onnwee/subcults/internal/touring"
)

func TestStudioTouringMutationFlow(t *testing.T) {
	repository := touring.NewInMemoryRepository()
	events := scene.NewInMemoryEventRepository()
	handler := NewTouringHandlers(touring.NewService(repository), events, scene.NewInMemorySceneRepository())

	profileBody := []byte(`{"kind":"artist","canonical_name":"Signal Unit","visibility":"public"}`)
	profileResponse := httptest.NewRecorder()
	handler.CreateProfile(profileResponse, httptest.NewRequest(http.MethodPost, "/api/v1/studio/profiles", bytes.NewReader(profileBody)))
	if profileResponse.Code != http.StatusCreated {
		t.Fatalf("profile: %d %s", profileResponse.Code, profileResponse.Body.String())
	}
	var created struct {
		Profile touring.Profile `json:"profile"`
		Act     touring.Act     `json:"act"`
	}
	if err := json.NewDecoder(profileResponse.Body).Decode(&created); err != nil || created.Act.ID == "" {
		t.Fatalf("decode profile: %#v %v", created, err)
	}

	tourBody, _ := json.Marshal(touring.Tour{PrimaryActID: created.Act.ID, Title: "Two City Signal", Status: touring.TourStatusDraft})
	tourRequest := httptest.NewRequest(http.MethodPost, "/api/v1/studio/tours", bytes.NewReader(tourBody))
	tourRequest = tourRequest.WithContext(middleware.SetUserDID(tourRequest.Context(), "did:web:creator"))
	tourResponse := httptest.NewRecorder()
	handler.CreateTour(tourResponse, tourRequest)
	if tourResponse.Code != http.StatusCreated {
		t.Fatalf("tour: %d %s", tourResponse.Code, tourResponse.Body.String())
	}
}

func TestStudioTouringStaleUpdateReturnsConflict(t *testing.T) {
	repository := touring.NewInMemoryRepository()
	profile := touring.Profile{ID: "profile-versioned", Kind: "artist", CanonicalName: "Versioned", Visibility: "public", Version: 1}
	if err := repository.StoreProfile(profile); err != nil { t.Fatal(err) }
	handler := NewTouringHandlers(touring.NewService(repository), scene.NewInMemoryEventRepository(), scene.NewInMemorySceneRepository())
	body, _ := json.Marshal(touring.Profile{ID: profile.ID, Kind: profile.Kind, CanonicalName: "Stale", Visibility: profile.Visibility, Version: 0})
	response := httptest.NewRecorder()
	handler.CreateProfile(response, httptest.NewRequest(http.MethodPatch, "/api/v1/studio/profiles", bytes.NewReader(body)))
	if response.Code != http.StatusConflict { t.Fatalf("status=%d body=%s", response.Code, response.Body.String()) }
}

func TestTouringHandlersExposeVisitingTourAndProfile(t *testing.T) {
	handler, tourID, profileID := newTouringHandlerFixture(t, touring.EventKindShow)

	search := httptest.NewRecorder()
	handler.SearchAppearances(search, httptest.NewRequest(http.MethodGet,
		"/search/appearances?place=chicago&locality=visiting&bbox=-88,41,-87,42", nil))
	if search.Code != http.StatusOK {
		t.Fatalf("search status=%d body=%s", search.Code, search.Body.String())
	}
	var searchResponse touringDetailResponse
	if err := json.NewDecoder(search.Body).Decode(&searchResponse); err != nil {
		t.Fatalf("decode search: %v", err)
	}
	if len(searchResponse.Appearances) != 1 {
		t.Fatalf("appearances=%#v, want one visiting result", searchResponse.Appearances)
	}
	got := searchResponse.Appearances[0]
	if got.Locality != "visiting" || got.Context != touring.AppearanceProjectionTourStop ||
		got.Event.Occurrence == nil || got.Event.Occurrence.Precision != "coarse" ||
		got.Act.HomeTerritory != "Detroit" || len(got.HostNames) != 1 {
		t.Fatalf("summary=%#v", got)
	}

	profile := httptest.NewRecorder()
	handler.Profile(profile, httptest.NewRequest(http.MethodGet, "/profiles/"+profileID, nil))
	if profile.Code != http.StatusOK {
		t.Fatalf("profile status=%d body=%s", profile.Code, profile.Body.String())
	}

	tour := httptest.NewRecorder()
	handler.Tour(tour, httptest.NewRequest(http.MethodGet, "/tours/"+tourID, nil))
	if tour.Code != http.StatusOK {
		t.Fatalf("tour status=%d body=%s", tour.Code, tour.Body.String())
	}
}

func TestSearchAppearancesFestivalAndValidationFacets(t *testing.T) {
	handler, _, _ := newTouringHandlerFixture(t, touring.EventKindFestival)
	festival := httptest.NewRecorder()
	handler.SearchAppearances(festival, httptest.NewRequest(http.MethodGet, "/search/appearances?festival=true", nil))
	if festival.Code != http.StatusOK {
		t.Fatalf("festival status=%d body=%s", festival.Code, festival.Body.String())
	}
	var response touringDetailResponse
	if err := json.NewDecoder(festival.Body).Decode(&response); err != nil || len(response.Appearances) != 1 ||
		response.Appearances[0].Context != touring.AppearanceProjectionFestivalAppearance {
		t.Fatalf("festival response=%#v err=%v", response, err)
	}

	invalid := httptest.NewRecorder()
	handler.SearchAppearances(invalid, httptest.NewRequest(http.MethodGet, "/search/appearances?locality=resident", nil))
	if invalid.Code != http.StatusBadRequest {
		t.Fatalf("invalid locality status=%d body=%s", invalid.Code, invalid.Body.String())
	}
}

func newTouringHandlerFixture(t *testing.T, eventKind string) (*TouringHandlers, string, string) {
	t.Helper()
	touringRepo := touring.NewInMemoryRepository()
	eventRepo := scene.NewInMemoryEventRepository()
	sceneRepo := scene.NewInMemorySceneRepository()

	chicago := touring.Place{ID: "chicago", CanonicalName: "Chicago", CountryCode: "US", Timezone: "America/Chicago", CoarseGeohash: "dp3"}
	detroit := touring.Place{ID: "detroit", CanonicalName: "Detroit", CountryCode: "US", Timezone: "America/Detroit", CoarseGeohash: "dps"}
	for _, place := range []touring.Place{chicago, detroit} {
		if err := touringRepo.StorePlace(place); err != nil {
			t.Fatalf("store place: %v", err)
		}
	}
	profile := touring.Profile{ID: "profile-a", Kind: "artist", CanonicalName: "Away Act", Visibility: touring.VisibilityPublic}
	if err := touringRepo.StoreProfile(profile); err != nil {
		t.Fatalf("store profile: %v", err)
	}
	act := touring.Act{ID: "act-a", ProfileID: profile.ID}
	if err := touringRepo.StoreAct(act); err != nil {
		t.Fatalf("store act: %v", err)
	}
	validFrom := time.Date(2020, time.January, 1, 0, 0, 0, 0, time.UTC)
	validTo := time.Date(2030, time.January, 1, 0, 0, 0, 0, time.UTC)
	if err := touringRepo.AddHomeTerritory(touring.HomeTerritory{
		ID: "home-a", ActID: act.ID, PlaceID: detroit.ID, Visibility: touring.VisibilityPublic,
		ValidFrom: validFrom, ValidTo: &validTo, AssertedByDID: "did:plc:artist",
	}); err != nil {
		t.Fatalf("add home territory: %v", err)
	}
	hostScene := &scene.Scene{ID: "scene-host", Name: "Smartbar", OwnerDID: "did:plc:host", CoarseGeohash: "dp3wj", Visibility: scene.VisibilityPublic}
	if err := sceneRepo.Insert(hostScene); err != nil {
		t.Fatalf("insert scene: %v", err)
	}
	placeID := chicago.ID
	event := &scene.Event{
		ID: "event-a", SceneID: hostScene.ID, Title: "Chicago Date", Status: "scheduled",
		StartsAt:      time.Date(2026, time.September, 1, 20, 0, 0, 0, time.UTC),
		CoarseGeohash: "dp3wj", PlaceID: &placeID, Kind: eventKind,
	}
	if err := eventRepo.Insert(event); err != nil {
		t.Fatalf("insert event: %v", err)
	}
	tour := touring.Tour{ID: "tour-a", PrimaryActID: act.ID, Title: "Away Run", Status: touring.TourStatusAnnounced}
	if err := touringRepo.CreateTour(tour, "did:plc:artist"); err != nil {
		t.Fatalf("create tour: %v", err)
	}
	startsAt := event.StartsAt
	var appearanceTourID *string
	if eventKind != touring.EventKindFestival {
		appearanceTourID = &tour.ID
	}
	if err := touringRepo.CreateAppearance(touring.Appearance{
		ID: "appearance-a", EventID: event.ID, ActID: act.ID, TourID: appearanceTourID,
		Role: "headliner", StartsAt: &startsAt, Status: touring.AppearanceStatusConfirmed,
	}); err != nil {
		t.Fatalf("create appearance: %v", err)
	}
	hostID := hostScene.ID
	if err := touringRepo.AddEventHost(touring.EventHost{EventID: event.ID, SceneID: &hostID, Role: "host"}); err != nil {
		t.Fatalf("add host: %v", err)
	}
	return NewTouringHandlers(touring.NewService(touringRepo), eventRepo, sceneRepo), tour.ID, profile.ID
}
