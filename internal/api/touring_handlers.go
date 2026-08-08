package api

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/onnwee/subcults/internal/middleware"
	"github.com/onnwee/subcults/internal/scene"
	"github.com/onnwee/subcults/internal/touring"
)

// TouringHandlers exposes additive, read-only touring resources. Its repository
// is currently in-memory in the development runtime; no handler treats it as a
// durable production query source.
type TouringHandlers struct {
	touringRepo touring.Repository
	eventRepo   scene.EventRepository
	sceneRepo   scene.SceneRepository
}

func NewTouringHandlers(touringRepo touring.Repository, eventRepo scene.EventRepository, sceneRepo scene.SceneRepository) *TouringHandlers {
	return &TouringHandlers{touringRepo: touringRepo, eventRepo: eventRepo, sceneRepo: sceneRepo}
}

type touringProfileResponse struct {
	ID            string `json:"id"`
	Name          string `json:"name"`
	HomeTerritory string `json:"home_territory,omitempty"`
}

type touringTourResponse struct {
	ID    string `json:"id"`
	Title string `json:"title"`
}

type appearanceEventResponse struct {
	ID         string            `json:"id"`
	Title      string            `json:"title"`
	StartsAt   time.Time         `json:"starts_at"`
	Kind       string            `json:"kind"`
	Occurrence *PublicOccurrence `json:"occurrence"`
}

type appearanceActResponse struct {
	ID            string `json:"id"`
	Name          string `json:"name"`
	HomeTerritory string `json:"home_territory,omitempty"`
}

type appearanceSummaryResponse struct {
	ID           string                  `json:"id"`
	Event        appearanceEventResponse `json:"event"`
	Act          appearanceActResponse   `json:"act"`
	Tour         *touringTourResponse    `json:"tour,omitempty"`
	HostNames    []string                `json:"host_names"`
	Context      string                  `json:"context"`
	Locality     string                  `json:"locality"`
	Status       string                  `json:"status"`
	Verification string                  `json:"verification"`
}

type touringDetailResponse struct {
	Profile     *touringProfileResponse     `json:"profile,omitempty"`
	Tour        *touringTourResponse        `json:"tour,omitempty"`
	Appearances []appearanceSummaryResponse `json:"appearances"`
}

type appearanceSearchOptions struct {
	PlaceID  string
	Bbox     *touringBBox
	From     *time.Time
	To       *time.Time
	ActID    string
	TourID   string
	Festival *bool
	SceneID  string
	Kind     string
	Locality string
	Access   string
}

type touringBBox struct{ minLng, minLat, maxLng, maxLat float64 }

// Profile handles GET /profiles/{id}.
func (h *TouringHandlers) Profile(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		touringMethodNotAllowed(w, r)
		return
	}
	id := strings.Trim(strings.TrimPrefix(r.URL.Path, "/profiles/"), "/")
	if id == "" || strings.Contains(id, "/") {
		touringNotFound(w, r)
		return
	}
	profile, err := h.touringRepo.GetProfile(id)
	if err != nil {
		touringNotFound(w, r)
		return
	}
	act, err := h.touringRepo.FindActByProfile(profile.ID)
	if err != nil {
		touringNotFound(w, r)
		return
	}
	summaries, err := h.summaries(h.appearancesForAct(act.ID), appearanceSearchOptions{Access: "public"})
	if err != nil {
		touringInternalError(w, r)
		return
	}
	response := touringDetailResponse{Profile: h.profileResponse(profile, act.ID), Appearances: summaries}
	touringWriteJSON(w, http.StatusOK, response)
}

// Tour handles GET /tours/{id}.
func (h *TouringHandlers) Tour(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		touringMethodNotAllowed(w, r)
		return
	}
	id := strings.Trim(strings.TrimPrefix(r.URL.Path, "/tours/"), "/")
	if id == "" || strings.Contains(id, "/") {
		touringNotFound(w, r)
		return
	}
	tour, err := h.touringRepo.GetTour(id)
	if err != nil {
		touringNotFound(w, r)
		return
	}
	appearances, err := h.touringRepo.ListAppearancesForTour(id)
	if err != nil {
		touringInternalError(w, r)
		return
	}
	summaries, err := h.summaries(appearances, appearanceSearchOptions{Access: "public"})
	if err != nil {
		touringInternalError(w, r)
		return
	}
	touringWriteJSON(w, http.StatusOK, touringDetailResponse{Tour: &touringTourResponse{ID: tour.ID, Title: tour.Title}, Appearances: summaries})
}

// SearchAppearances handles GET /search/appearances. All filters are applied
// before response construction, and the only returned location is the shared
// server-approved occurrence projection.
func (h *TouringHandlers) SearchAppearances(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		touringMethodNotAllowed(w, r)
		return
	}
	opts, err := parseAppearanceSearchOptions(r)
	if err != nil {
		ctx := middleware.SetErrorCode(r.Context(), ErrCodeValidation)
		WriteError(w, ctx, http.StatusBadRequest, ErrCodeValidation, err.Error())
		return
	}
	appearances, err := h.touringRepo.ListAppearances()
	if err != nil {
		touringInternalError(w, r)
		return
	}
	summaries, err := h.summaries(appearances, opts)
	if err != nil {
		touringInternalError(w, r)
		return
	}
	touringWriteJSON(w, http.StatusOK, touringDetailResponse{Appearances: summaries})
}

func (h *TouringHandlers) appearancesForAct(actID string) []touring.Appearance {
	appearances, err := h.touringRepo.ListAppearancesForAct(actID)
	if err != nil {
		return []touring.Appearance{}
	}
	return appearances
}

func (h *TouringHandlers) summaries(appearances []touring.Appearance, opts appearanceSearchOptions) ([]appearanceSummaryResponse, error) {
	results := make([]appearanceSummaryResponse, 0, len(appearances))
	for _, appearance := range appearances {
		event, err := h.eventRepo.GetByID(appearance.EventID)
		if err != nil || !matchesAppearanceEvent(event, appearance, opts) {
			continue
		}
		act, err := h.touringRepo.GetAct(appearance.ActID)
		if err != nil {
			continue
		}
		profile, err := h.touringRepo.GetProfile(act.ProfileID)
		if err != nil || profile.Visibility != touring.VisibilityPublic {
			continue
		}
		homeTerritory, locality := h.homeTerritoryAndLocality(act.ID, event.PlaceID)
		if opts.Locality != "any" && opts.Locality != locality {
			continue
		}
		hostNames := h.hostNames(event.ID)
		if opts.SceneID != "" && event.SceneID != opts.SceneID && !h.eventHasSceneHost(event.ID, opts.SceneID) {
			continue
		}
		if opts.Bbox != nil {
			occurrence := toPublicOccurrence(event)
			if occurrence.Precision == "coarse" {
				if !scene.CoarseGeohashIntersectsBBox(event.CoarseGeohash, opts.Bbox.minLng, opts.Bbox.minLat, opts.Bbox.maxLng, opts.Bbox.maxLat) {
					continue
				}
			} else if occurrence.DisplayPoint == nil || !opts.Bbox.contains(occurrence.DisplayPoint.Lat, occurrence.DisplayPoint.Lng) {
				continue
			}
		}
		var tourResponse *touringTourResponse
		if appearance.TourID != nil {
			if tour, err := h.touringRepo.GetTour(*appearance.TourID); err == nil {
				tourResponse = &touringTourResponse{ID: tour.ID, Title: tour.Title}
			}
		}
		kind := event.Kind
		if kind == "" {
			kind = touring.EventKindShow
		}
		results = append(results, appearanceSummaryResponse{
			ID:    appearance.ID,
			Event: appearanceEventResponse{ID: event.ID, Title: event.Title, StartsAt: event.StartsAt, Kind: kind, Occurrence: toPublicOccurrence(event)},
			Act:   appearanceActResponse{ID: act.ID, Name: profile.CanonicalName, HomeTerritory: homeTerritory},
			Tour:  tourResponse, HostNames: hostNames, Context: touring.ProjectAppearanceKind(appearance.TourID, kind),
			Locality: locality, Status: appearance.Status, Verification: h.touringRepo.VerificationForEntity("appearance", appearance.ID),
		})
	}
	return results, nil
}

func matchesAppearanceEvent(event *scene.Event, appearance touring.Appearance, opts appearanceSearchOptions) bool {
	if opts.PlaceID != "" && (event.PlaceID == nil || *event.PlaceID != opts.PlaceID) {
		return false
	}
	if opts.ActID != "" && appearance.ActID != opts.ActID {
		return false
	}
	if opts.TourID != "" && (appearance.TourID == nil || *appearance.TourID != opts.TourID) {
		return false
	}
	kind := event.Kind
	if kind == "" {
		kind = touring.EventKindShow
	}
	if opts.Kind != "" && kind != opts.Kind {
		return false
	}
	if opts.Festival != nil && (kind == touring.EventKindFestival) != *opts.Festival {
		return false
	}
	if opts.From != nil && event.StartsAt.Before(*opts.From) {
		return false
	}
	if opts.To != nil && event.StartsAt.After(*opts.To) {
		return false
	}
	return true
}

func (h *TouringHandlers) homeTerritoryAndLocality(actID string, eventPlaceID *string) (string, string) {
	territories, err := h.touringRepo.ListHomeTerritories(actID)
	if err != nil {
		return "", "unknown"
	}
	now := time.Now().UTC()
	for _, territory := range territories {
		if territory.Visibility != touring.VisibilityPublic || territory.ValidFrom.After(now) || (territory.ValidTo != nil && territory.ValidTo.Before(now)) {
			continue
		}
		place, err := h.touringRepo.GetPlace(territory.PlaceID)
		if err != nil {
			continue
		}
		locality := "unknown"
		if eventPlaceID != nil {
			locality = "visiting"
			if territory.PlaceID == *eventPlaceID {
				locality = "local"
			}
		}
		return place.CanonicalName, locality
	}
	return "", "unknown"
}

func (h *TouringHandlers) profileResponse(profile touring.Profile, actID string) *touringProfileResponse {
	home, _ := h.homeTerritoryAndLocality(actID, nil)
	return &touringProfileResponse{ID: profile.ID, Name: profile.CanonicalName, HomeTerritory: home}
}

func (h *TouringHandlers) hostNames(eventID string) []string {
	hosts, err := h.touringRepo.ListEventHosts(eventID)
	if err != nil {
		return []string{}
	}
	names := make([]string, 0, len(hosts))
	for _, host := range hosts {
		if host.SceneID != nil {
			if hostScene, err := h.sceneRepo.GetByID(*host.SceneID); err == nil {
				names = append(names, hostScene.Name)
			}
		}
		if host.ProfileID != nil {
			if profile, err := h.touringRepo.GetProfile(*host.ProfileID); err == nil {
				names = append(names, profile.CanonicalName)
			}
		}
	}
	return names
}

func (h *TouringHandlers) eventHasSceneHost(eventID, sceneID string) bool {
	hosts, err := h.touringRepo.ListEventHosts(eventID)
	if err != nil {
		return false
	}
	for _, host := range hosts {
		if host.SceneID != nil && *host.SceneID == sceneID {
			return true
		}
	}
	return false
}

func parseAppearanceSearchOptions(r *http.Request) (appearanceSearchOptions, error) {
	query := r.URL.Query()
	opts := appearanceSearchOptions{PlaceID: strings.TrimSpace(query.Get("place")), ActID: strings.TrimSpace(query.Get("act")), TourID: strings.TrimSpace(query.Get("tour")), SceneID: strings.TrimSpace(query.Get("scene")), Kind: strings.TrimSpace(query.Get("kind")), Locality: strings.TrimSpace(query.Get("locality")), Access: strings.TrimSpace(query.Get("access"))}
	if opts.Locality == "" {
		opts.Locality = "any"
	}
	if opts.Access == "" {
		opts.Access = "public"
	}
	if opts.Locality != "any" && opts.Locality != "local" && opts.Locality != "visiting" {
		return opts, invalidTouringQuery("locality must be any, local, or visiting")
	}
	if opts.Access != "public" && opts.Access != "all" {
		return opts, invalidTouringQuery("access must be public or all")
	}
	if opts.Kind != "" && !isTouringEventKind(opts.Kind) {
		return opts, invalidTouringQuery("kind is invalid")
	}
	if raw := strings.TrimSpace(query.Get("festival")); raw != "" {
		value, err := strconv.ParseBool(raw)
		if err != nil {
			return opts, invalidTouringQuery("festival must be true or false")
		}
		opts.Festival = &value
	}
	for _, field := range []struct {
		raw  string
		dest **time.Time
		name string
	}{{query.Get("from"), &opts.From, "from"}, {query.Get("to"), &opts.To, "to"}} {
		if strings.TrimSpace(field.raw) == "" {
			continue
		}
		parsed, err := time.Parse(time.RFC3339, field.raw)
		if err != nil {
			return opts, invalidTouringQuery(field.name + " must be RFC3339")
		}
		*field.dest = &parsed
	}
	if opts.From != nil && opts.To != nil && opts.To.Before(*opts.From) {
		return opts, invalidTouringQuery("to must be after from")
	}
	if raw := strings.TrimSpace(query.Get("bbox")); raw != "" {
		parts := strings.Split(raw, ",")
		if len(parts) != 4 {
			return opts, invalidTouringQuery("bbox must be west,south,east,north")
		}
		values := [4]float64{}
		for index, part := range parts {
			value, err := strconv.ParseFloat(strings.TrimSpace(part), 64)
			if err != nil {
				return opts, invalidTouringQuery("bbox contains an invalid coordinate")
			}
			values[index] = value
		}
		if values[0] < -180 || values[2] > 180 || values[1] < -90 || values[3] > 90 || values[0] >= values[2] || values[1] >= values[3] {
			return opts, invalidTouringQuery("bbox is invalid")
		}
		opts.Bbox = &touringBBox{values[0], values[1], values[2], values[3]}
	}
	return opts, nil
}

func (b touringBBox) contains(lat, lng float64) bool {
	return lng >= b.minLng && lng <= b.maxLng && lat >= b.minLat && lat <= b.maxLat
}
func isTouringEventKind(kind string) bool {
	switch kind {
	case touring.EventKindShow, touring.EventKindFestival, touring.EventKindParty, touring.EventKindMeetup, touring.EventKindBroadcast, touring.EventKindOther:
		return true
	}
	return false
}
func invalidTouringQuery(message string) error { return &touringQueryError{message: message} }

type touringQueryError struct{ message string }

func (e *touringQueryError) Error() string { return e.message }
func touringWriteJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}
func touringNotFound(w http.ResponseWriter, r *http.Request) {
	ctx := middleware.SetErrorCode(r.Context(), ErrCodeNotFound)
	WriteError(w, ctx, http.StatusNotFound, ErrCodeNotFound, "Touring resource not found")
}
func touringMethodNotAllowed(w http.ResponseWriter, r *http.Request) {
	ctx := middleware.SetErrorCode(r.Context(), ErrCodeBadRequest)
	WriteError(w, ctx, http.StatusMethodNotAllowed, ErrCodeBadRequest, "Method not allowed")
}
func touringInternalError(w http.ResponseWriter, r *http.Request) {
	ctx := middleware.SetErrorCode(r.Context(), ErrCodeInternal)
	WriteError(w, ctx, http.StatusInternalServerError, ErrCodeInternal, "Failed to load touring data")
}
