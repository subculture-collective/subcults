package api

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/onnwee/subcults/internal/identity"
	"github.com/onnwee/subcults/internal/middleware"
	"github.com/onnwee/subcults/internal/scene"
	"github.com/onnwee/subcults/internal/touring"
)

// TouringHandlers exposes additive, read-only touring resources. Business logic
// lives in the touring.Service layer; the handler handles HTTP decoding/encoding.
type TouringHandlers struct {
	touringService *touring.Service
	eventRepo      scene.EventRepository
	sceneRepo      scene.SceneRepository
}

// CreateProfile handles creator-authorized Studio profile creation. Authorization
// is applied by the route wrapper so the domain handler can stay testable.
func (h *TouringHandlers) CreateProfile(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodPatch {
		h.UpdateProfile(w, r)
		return
	}
	if r.Method != http.MethodPost {
		touringMethodNotAllowed(w, r)
		return
	}
	var request struct {
		Kind          string `json:"kind"`
		CanonicalName string `json:"canonical_name"`
		Visibility    string `json:"visibility"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 16*1024)).Decode(&request); err != nil {
		touringValidationError(w, r, "invalid profile")
		return
	}
	profile, act, err := h.touringService.CreateProfile(r.Context(), request.Kind, request.CanonicalName, request.Visibility, middleware.GetUserID(r.Context()))
	if err != nil {
		touringValidationError(w, r, "invalid profile")
		return
	}
	response := map[string]any{"profile": profile}
	if act != nil {
		response["act"] = *act
	}
	touringWriteJSON(w, http.StatusCreated, response)
}

func (h *TouringHandlers) CreatePlace(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodPatch {
		h.UpdatePlace(w, r)
		return
	}
	if r.Method != http.MethodPost {
		touringMethodNotAllowed(w, r)
		return
	}
	var place touring.Place
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 16*1024)).Decode(&place); err != nil {
		touringValidationError(w, r, "invalid place")
		return
	}
	created, err := h.touringService.CreatePlace(r.Context(), place, middleware.GetUserID(r.Context()))
	if err != nil {
		touringValidationError(w, r, "invalid place")
		return
	}
	touringWriteJSON(w, http.StatusCreated, created)
}

func (h *TouringHandlers) CreateVenue(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodPatch {
		var venue touring.Venue
		if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 16*1024)).Decode(&venue); err != nil {
			touringValidationError(w, r, "invalid venue")
			return
		}
		h.updateTouring(w, r, func() error { return h.touringService.UpdateVenue(venue) }, func() any { return venue })
		return
	}
	if r.Method != http.MethodPost {
		touringMethodNotAllowed(w, r)
		return
	}
	var venue touring.Venue
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 16*1024)).Decode(&venue); err != nil {
		touringValidationError(w, r, "invalid venue")
		return
	}
	created, err := h.touringService.CreateVenue(r.Context(), venue)
	if err != nil {
		touringValidationError(w, r, "invalid venue")
		return
	}
	touringWriteJSON(w, http.StatusCreated, created)
}

func (h *TouringHandlers) CreateTour(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodPatch {
		h.UpdateTour(w, r)
		return
	}
	if r.Method != http.MethodPost {
		touringMethodNotAllowed(w, r)
		return
	}
	var tour touring.Tour
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 16*1024)).Decode(&tour); err != nil {
		touringValidationError(w, r, "invalid tour")
		return
	}
	created, err := h.touringService.CreateTour(r.Context(), tour, middleware.GetUserID(r.Context()), middleware.GetUserDID(r.Context()))
	if err != nil {
		touringValidationError(w, r, "invalid tour")
		return
	}
	touringWriteJSON(w, http.StatusCreated, created)
}

func (h *TouringHandlers) CreateAppearance(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodPatch {
		h.UpdateAppearance(w, r)
		return
	}
	if r.Method != http.MethodPost {
		touringMethodNotAllowed(w, r)
		return
	}
	var appearance touring.Appearance
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 16*1024)).Decode(&appearance); err != nil {
		touringValidationError(w, r, "invalid appearance")
		return
	}
	if _, err := h.eventRepo.GetByID(appearance.EventID); err != nil {
		touringValidationError(w, r, "event not found")
		return
	}
	created, err := h.touringService.CreateAppearance(r.Context(), appearance, middleware.GetUserID(r.Context()))
	if err != nil {
		touringValidationError(w, r, "invalid appearance")
		return
	}
	touringWriteJSON(w, http.StatusCreated, created)
}

func (h *TouringHandlers) UpdateProfile(w http.ResponseWriter, r *http.Request) {
	var value touring.Profile
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 16*1024)).Decode(&value); err != nil {
		touringValidationError(w, r, "invalid profile")
		return
	}
	h.updateTouring(w, r, func() error { return h.touringService.UpdateProfile(value) }, func() any { return map[string]any{"profile": value} })
}
func (h *TouringHandlers) UpdatePlace(w http.ResponseWriter, r *http.Request) {
	var value touring.Place
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 16*1024)).Decode(&value); err != nil {
		touringValidationError(w, r, "invalid place")
		return
	}
	h.updateTouring(w, r, func() error { return h.touringService.UpdatePlace(value) }, func() any { return value })
}
func (h *TouringHandlers) UpdateTour(w http.ResponseWriter, r *http.Request) {
	var value touring.Tour
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 16*1024)).Decode(&value); err != nil {
		touringValidationError(w, r, "invalid tour")
		return
	}
	h.updateTouring(w, r, func() error { return h.touringService.UpdateTour(value) }, func() any { return value })
}
func (h *TouringHandlers) UpdateAppearance(w http.ResponseWriter, r *http.Request) {
	var value touring.Appearance
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 16*1024)).Decode(&value); err != nil {
		touringValidationError(w, r, "invalid appearance")
		return
	}
	h.updateTouring(w, r, func() error { return h.touringService.UpdateAppearance(value) }, func() any { return value })
}
func (h *TouringHandlers) updateTouring(w http.ResponseWriter, r *http.Request, update func() error, response func() any) {
	if err := update(); err != nil {
		if _, _, _, ok := MapDomainError(err); ok {
			WriteError(w, middleware.SetErrorCode(r.Context(), ErrCodeConflict), http.StatusConflict, ErrCodeConflict, "The record changed; refresh before saving again")
			return
		}
		touringValidationError(w, r, "invalid or missing record")
		return
	}
	touringWriteJSON(w, http.StatusOK, response())
}

func touringValidationError(w http.ResponseWriter, r *http.Request, message string) {
	ctx := middleware.SetErrorCode(r.Context(), ErrCodeValidation)
	WriteError(w, ctx, http.StatusBadRequest, ErrCodeValidation, message)
}

func NewTouringHandlers(touringService *touring.Service, eventRepo scene.EventRepository, sceneRepo scene.SceneRepository) *TouringHandlers {
	return &TouringHandlers{touringService: touringService, eventRepo: eventRepo, sceneRepo: sceneRepo}
}

type touringProfileResponse struct {
	ID               string `json:"id"`
	Name             string `json:"name"`
	HomeTerritory    string `json:"home_territory,omitempty"`
	ATURI            string `json:"at_uri,omitempty"`
	CID              string `json:"cid,omitempty"`
	PublisherDID     string `json:"publisher_did,omitempty"`
	PublisherHandle  string `json:"publisher_handle,omitempty"`
	ProjectionStatus string `json:"projection_status,omitempty"`
}

type touringTourResponse struct {
	ID               string `json:"id"`
	Title            string `json:"title"`
	ATURI            string `json:"at_uri,omitempty"`
	CID              string `json:"cid,omitempty"`
	PublisherDID     string `json:"publisher_did,omitempty"`
	PublisherHandle  string `json:"publisher_handle,omitempty"`
	ProjectionStatus string `json:"projection_status,omitempty"`
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
	ProfileID     string `json:"profile_id"`
	Name          string `json:"name"`
	HomeTerritory string `json:"home_territory,omitempty"`
}

type appearanceSummaryResponse struct {
	ID               string                  `json:"id"`
	Event            appearanceEventResponse `json:"event"`
	Act              appearanceActResponse   `json:"act"`
	Tour             *touringTourResponse    `json:"tour,omitempty"`
	HostNames        []string                `json:"host_names"`
	Context          string                  `json:"context"`
	Locality         string                  `json:"locality"`
	Status           string                  `json:"status"`
	Verification     string                  `json:"verification"`
	ATURI            string                  `json:"at_uri,omitempty"`
	CID              string                  `json:"cid,omitempty"`
	PublisherDID     string                  `json:"publisher_did,omitempty"`
	PublisherHandle  string                  `json:"publisher_handle,omitempty"`
	ProjectionStatus string                  `json:"projection_status,omitempty"`
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
	id := touringPathID(r.URL.Path, "/profiles/")
	if id == "" || strings.Contains(id, "/") {
		touringNotFound(w, r)
		return
	}
	profile, err := h.touringService.GetProfile(id)
	if err != nil || (profile.PublicationStatus != "" && profile.PublicationStatus != "published") {
		touringNotFound(w, r)
		return
	}
	act, err := h.touringService.FindActByProfile(profile.ID)
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
	id := touringPathID(r.URL.Path, "/tours/")
	if id == "" || strings.Contains(id, "/") {
		touringNotFound(w, r)
		return
	}
	tour, err := h.touringService.GetTour(id)
	if err != nil || (tour.PublicationStatus != "" && tour.PublicationStatus != "published") {
		touringNotFound(w, r)
		return
	}
	appearances, err := h.touringService.ListAppearancesForTour(id)
	if err != nil {
		touringInternalError(w, r)
		return
	}
	summaries, err := h.summaries(appearances, appearanceSearchOptions{Access: "public"})
	if err != nil {
		touringInternalError(w, r)
		return
	}
	touringWriteJSON(w, http.StatusOK, touringDetailResponse{Tour: tourResponseFromModel(tour), Appearances: summaries})
}

func touringPathID(path, prefix string) string {
	path = strings.TrimPrefix(path, "/api")
	return strings.Trim(strings.TrimPrefix(path, prefix), "/")
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
	appearances, err := h.touringService.ListAppearances()
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
	appearances, err := h.touringService.ListAppearancesForAct(actID)
	if err != nil {
		return []touring.Appearance{}
	}
	return appearances
}

func (h *TouringHandlers) summaries(appearances []touring.Appearance, opts appearanceSearchOptions) ([]appearanceSummaryResponse, error) {
	results := make([]appearanceSummaryResponse, 0, len(appearances))
	for _, appearance := range appearances {
		if appearance.PublicationStatus != "" && appearance.PublicationStatus != "published" {
			continue
		}
		event, err := h.eventRepo.GetByID(appearance.EventID)
		if err != nil || !matchesAppearanceEvent(event, appearance, opts) {
			continue
		}
		act, err := h.touringService.GetAct(appearance.ActID)
		if err != nil {
			continue
		}
		profile, err := h.touringService.GetProfile(act.ProfileID)
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
			if tour, err := h.touringService.GetTour(*appearance.TourID); err == nil {
				tourResponse = tourResponseFromModel(tour)
			}
		}
		kind := event.Kind
		if kind == "" {
			kind = touring.EventKindShow
		}
		results = append(results, appearanceSummaryResponse{
			ID:    appearance.ID,
			Event: appearanceEventResponse{ID: event.ID, Title: event.Title, StartsAt: event.StartsAt, Kind: kind, Occurrence: toPublicOccurrence(event)},
			Act:   appearanceActResponse{ID: act.ID, ProfileID: profile.ID, Name: profile.CanonicalName, HomeTerritory: homeTerritory},
			Tour:  tourResponse, HostNames: hostNames, Context: touring.ProjectAppearanceKind(appearance.TourID, kind),
			Locality: locality, Status: appearance.Status, Verification: h.touringService.VerificationForEntity("appearance", appearance.ID),
			ATURI: appearance.ATURI, CID: appearance.CID, PublisherDID: appearance.PublisherDID, PublisherHandle: appearance.PublisherHandle, ProjectionStatus: appearance.ProjectionStatus,
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
	territories, err := h.touringService.ListHomeTerritories(actID)
	if err != nil {
		return "", "unknown"
	}
	now := time.Now().UTC()
	for _, territory := range territories {
		if territory.Visibility != touring.VisibilityPublic || territory.ValidFrom.After(now) || (territory.ValidTo != nil && territory.ValidTo.Before(now)) {
			continue
		}
		place, err := h.touringService.GetPlace(territory.PlaceID)
		if err != nil {
			continue
		}
		if place.PublicationStatus != "" && place.PublicationStatus != "published" {
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
	return &touringProfileResponse{ID: profile.ID, Name: profile.CanonicalName, HomeTerritory: home, ATURI: profile.ATURI, CID: profile.CID, PublisherDID: profile.PublisherDID, PublisherHandle: profile.PublisherHandle, ProjectionStatus: profile.ProjectionStatus}
}

func tourResponseFromModel(tour touring.Tour) *touringTourResponse {
	return &touringTourResponse{ID: tour.ID, Title: tour.Title, ATURI: tour.ATURI, CID: tour.CID, PublisherDID: tour.PublisherDID, PublisherHandle: tour.PublisherHandle, ProjectionStatus: tour.ProjectionStatus}
}

func (h *TouringHandlers) hostNames(eventID string) []string {
	hosts, err := h.touringService.ListEventHosts(eventID)
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
			if profile, err := h.touringService.GetProfile(*host.ProfileID); err == nil {
				names = append(names, profile.CanonicalName)
			}
		}
	}
	return names
}

func (h *TouringHandlers) eventHasSceneHost(eventID, sceneID string) bool {
	hosts, err := h.touringService.ListEventHosts(eventID)
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

// RegisterTouringRoutes registers all touring-related routes on the given mux.
// identityService is required for RequireCreator middleware on studio routes.
func RegisterTouringRoutes(mux *http.ServeMux, deps *RouteDeps, h *TouringHandlers, identityService *identity.Service) {
	requireCreator := RequireCreator(identityService)

	mux.HandleFunc("/api/v1/studio/profiles", requireCreator(h.CreateProfile))
	mux.HandleFunc("/api/v1/studio/places", requireCreator(h.CreatePlace))
	mux.HandleFunc("/api/v1/studio/venues", requireCreator(h.CreateVenue))
	mux.HandleFunc("/api/v1/studio/tours", requireCreator(h.CreateTour))
	mux.HandleFunc("/api/v1/studio/appearances", requireCreator(h.CreateAppearance))

	mux.HandleFunc("/profiles/", h.Profile)
	mux.HandleFunc("/tours/", h.Tour)
	mux.HandleFunc("/search/appearances", h.SearchAppearances)
	mux.HandleFunc("/api/profiles/", h.Profile)
	mux.HandleFunc("/api/tours/", h.Tour)
	mux.HandleFunc("/api/search/appearances", h.SearchAppearances)
}
