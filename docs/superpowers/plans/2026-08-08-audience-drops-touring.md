# Audience, Drops, and Touring Implementation Plan

> **For agentic workers:** Execute this plan task-by-task. Recommended path:
> dispatch a fresh subagent per task, review each result with `review-quality`,
> then continue. For complex multi-agent splits, use
> `parallel-feature-development`, `team-composition-patterns`, and
> `team-communication-protocols`. Steps use checkbox (`- [ ]`) syntax for
> tracking.

If an auxiliary workflow named above is unavailable in the execution
environment, use the repository's available `multi-agent-workflows` and
`review-quality` checks without blocking the implementation.

**Goal:** Add durable, privacy-preserving touring discovery and consented Signal activation while preserving Scene sovereignty and the distinction between participation and messaging permission.

**Architecture:** Keep Event as the atomic place-and-time occurrence. Add Profile/Act, Place/Venue, Appearance, and Tour relations for traveling activity, then add Audience/Consent and Signal bounded contexts whose canonical state remains in Subcult while provider adapters handle delivery. Complete durable Postgres/auth and public-location projection prerequisites before exposing the new product surfaces.

**Tech Stack:** Go 1.24 `net/http`, PostgreSQL 16/PostGIS, `golang-migrate`, React 19/TypeScript/Vite, Zustand, MapLibre, Vitest, Go `testing`, OpenAPI 3.

---

## File Structure

### Create

- `migrations/000033_places_profiles_acts.up.sql` / `.down.sql`: identity and geographic context.
- `migrations/000034_tours_appearances_provenance.up.sql` / `.down.sql`: occurrence relations and source history.
- `migrations/000035_audience_consent.up.sql` / `.down.sql`: private relationship, contact-link, consent, and suppression ledger.
- `migrations/000036_signals_delivery.up.sql` / `.down.sql`: campaign revisions, delivery, engagement, and conversion ledger.
- `internal/touring/model.go`: touring domain types and lifecycle constants.
- `internal/touring/repository.go`: repository interfaces and Postgres implementation boundary.
- `internal/touring/service.go`: authority, provenance, and conflict-aware commands.
- `internal/touring/*_test.go`: domain/repository/service tests.
- `internal/audience/model.go`, `repository.go`, `service.go`: verified contacts, consent, suppression, and segment resolution.
- `internal/signal/model.go`, `repository.go`, `service.go`, `delivery.go`: versioned Signals and provider-neutral delivery.
- `internal/api/touring_handlers.go`, `audience_handlers.go`, `signal_handlers.go`: HTTP boundary.
- `web/src/types/touring.ts`, `audience.ts`, `signal.ts`: client DTOs.
- `web/src/pages/ProfileDetailPage.tsx`, `TourDetailPage.tsx`, `SignalDetailPage.tsx`: public product surfaces.
- `web/src/components/discovery/AppearanceCard.tsx`, `TourMapLayer.tsx`: location-aware touring UI.

### Modify

- `cmd/api/main.go`: durable repositories and new route registration.
- `internal/scene/model.go`: Event kind and occurrence Place/Venue references.
- `internal/scene/repository.go`: coarse-cell bbox eligibility and touring facets.
- `internal/api/event_handlers.go`: public occurrence projection and appearance expansion.
- `web/src/utils/geojson.ts`: consume public occurrence projection only.
- `web/src/routes/index.tsx`: Profile/Tour/Signal routes.
- `docs/openapi.yaml`: canonical public API.
- `docs/PRIVACY.md`, `docs/DATA_CLASSIFICATION.md`, `docs/GLOSSARY.md`: final implemented contracts.
- `migrations/README.md`, `README.md`, and `docs/ARCHITECTURE.md`: evidence/status updates after each completed phase.

## Task 1: Establish Durable Runtime Prerequisites

**Files:**

- Modify: `cmd/api/main.go:187-205`
- Create: `internal/db/runtime_repositories.go`
- Test: `internal/db/runtime_repositories_test.go`
- Modify: `docker-compose.yml:21-40`

- [ ] **Step 1: Write a failing runtime repository construction test**

```go
func TestNewRuntimeRepositoriesRequiresDatabase(t *testing.T) {
	_, err := NewRuntimeRepositories(context.Background(), "")
	if !errors.Is(err, ErrDatabaseURLRequired) {
		t.Fatalf("error = %v, want ErrDatabaseURLRequired", err)
	}
}
```

- [ ] **Step 2: Verify the test fails before implementation**

Run: `go test ./internal/db -run TestNewRuntimeRepositoriesRequiresDatabase -count=1`
Expected: FAIL because `NewRuntimeRepositories` and `ErrDatabaseURLRequired` do not exist.

- [ ] **Step 3: Add the explicit runtime repository constructor**

```go
var ErrDatabaseURLRequired = errors.New("database URL is required")

type RuntimeRepositories struct {
	Pool *pgxpool.Pool
}

func NewRuntimeRepositories(ctx context.Context, databaseURL string) (*RuntimeRepositories, error) {
	if strings.TrimSpace(databaseURL) == "" {
		return nil, ErrDatabaseURLRequired
	}
	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		return nil, fmt.Errorf("open database pool: %w", err)
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("ping database: %w", err)
	}
	return &RuntimeRepositories{Pool: pool}, nil
}
```

- [ ] **Step 4: Replace production in-memory construction behind an explicit development switch**

Use `SUBCULT_IN_MEMORY_REPOSITORIES=true` only for tests/local fixtures. A non-test environment without `DATABASE_URL` must fail startup rather than silently losing CRM/touring state.

- [ ] **Step 5: Run prerequisite validation**

Run: `go test ./internal/db ./cmd/api -count=1`
Expected: PASS.
Run: `docker compose config`
Expected: API and Postgres services render with a Postgres dependency and no unresolved YAML error.

- [ ] **Step 6: Commit the durable runtime boundary**

```bash
git add cmd/api/main.go internal/db docker-compose.yml
git commit -m "feat: require durable API repositories"
```

## Task 2: Repair Event Occurrence Location Projection

**Files:**

- Modify: `internal/scene/repository.go`
- Modify: `internal/api/event_handlers.go`
- Modify: `web/src/utils/geojson.ts`
- Test: `internal/scene/search_test.go`
- Test: `internal/api/event_handlers_test.go`
- Test: `web/src/utils/geojson.test.ts`

- [ ] **Step 1: Add failing tests for coarse-only occurrence discovery**

```go
func TestSearchEventsIncludesCoarseOccurrenceInsideBBox(t *testing.T) {
	repo := NewInMemoryEventRepository()
	repo.Insert(&Event{ID: "coarse", SceneID: "host", Title: "Away show", CoarseGeohash: "dp3wj", StartsAt: time.Now().Add(time.Hour)})
	got, _, err := repo.SearchEvents(EventSearchOptions{MinLng: -88, MinLat: 41, MaxLng: -87, MaxLat: 42, From: time.Now(), To: time.Now().Add(24 * time.Hour), Limit: 20})
	if err != nil || len(got) != 1 || got[0].ID != "coarse" {
		t.Fatalf("got=%v err=%v", got, err)
	}
}
```

- [ ] **Step 2: Verify the coarse-only test fails**

Run: `go test ./internal/scene -run TestSearchEventsIncludesCoarseOccurrenceInsideBBox -count=1`
Expected: FAIL because current in-memory bbox filtering excludes an Event without `PrecisePoint`.

- [ ] **Step 3: Add one public occurrence projection**

```go
type PublicOccurrence struct {
	CoarseGeohash string `json:"coarse_geohash"`
	DisplayPoint  *Point `json:"display_point,omitempty"`
	Precision     string `json:"precision"` // coarse or precise
}
```

Decode/intersect the coarse geohash for candidate eligibility and emit only the approved public projection. The frontend must read `display_point`; it must not choose raw `precise_point` itself.

- [ ] **Step 4: Run backend and frontend location tests**

Run: `go test ./internal/scene ./internal/api -run 'SearchEvents|Occurrence|Location' -count=1`
Expected: PASS.
Run: `cd web && npm test -- src/utils/geojson.test.ts --run`
Expected: PASS with coarse and precise public projections covered.

- [ ] **Step 5: Commit the occurrence-location contract**

```bash
git add internal/scene internal/api web/src/utils/geojson.ts web/src/utils/geojson.test.ts
git commit -m "fix: make event occurrence projection privacy safe"
```

## Task 3: Add Place, Venue, Profile, Act, and Home Territory Schema

**Files:**

- Create: `migrations/000033_places_profiles_acts.up.sql`
- Create: `migrations/000033_places_profiles_acts.down.sql`
- Create: `internal/touring/model.go`
- Test: `internal/touring/model_test.go`

- [ ] **Step 1: Write lifecycle and home-territory validation tests**

```go
func TestHomeTerritoryRejectsPreciseResidenceData(t *testing.T) {
	claim := HomeTerritory{ActID: "act", PlaceID: "chicago", Visibility: "public", PrecisePoint: &Point{Lat: 41.88, Lng: -87.63}}
	if err := claim.Validate(); !errors.Is(err, ErrPreciseHomeTerritory) {
		t.Fatalf("error=%v", err)
	}
}
```

- [ ] **Step 2: Create normalized identity/geography tables**

```sql
CREATE TABLE places (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    canonical_name TEXT NOT NULL,
    admin_region TEXT,
    country_code CHAR(2) NOT NULL,
    timezone TEXT NOT NULL,
    coarse_geohash VARCHAR(20) NOT NULL,
    UNIQUE (canonical_name, admin_region, country_code)
);

CREATE TABLE venues (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    place_id UUID NOT NULL REFERENCES places(id),
    canonical_name TEXT NOT NULL,
    allow_precise BOOLEAN NOT NULL DEFAULT FALSE,
    precise_point GEOGRAPHY(Point, 4326),
    coarse_geohash VARCHAR(20) NOT NULL,
    CHECK (allow_precise OR precise_point IS NULL)
);

CREATE TABLE profiles (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    kind TEXT NOT NULL CHECK (kind IN ('artist','venue','festival','promoter','collective','label','curator')),
    canonical_name TEXT NOT NULL,
    visibility TEXT NOT NULL DEFAULT 'public' CHECK (visibility IN ('public','private','unlisted')),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE profile_controllers (
    profile_id UUID NOT NULL REFERENCES profiles(id) ON DELETE CASCADE,
    controller_did TEXT NOT NULL,
    role TEXT NOT NULL CHECK (role IN ('owner','editor')),
    PRIMARY KEY (profile_id, controller_did)
);

CREATE TABLE acts (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    profile_id UUID NOT NULL UNIQUE REFERENCES profiles(id) ON DELETE CASCADE
);

CREATE TABLE act_scene_affiliations (
    act_id UUID NOT NULL REFERENCES acts(id) ON DELETE CASCADE,
    scene_id UUID NOT NULL REFERENCES scenes(id) ON DELETE CASCADE,
    relationship TEXT NOT NULL CHECK (relationship IN ('home','member','associated')),
    PRIMARY KEY (act_id, scene_id, relationship)
);

CREATE TABLE act_home_territories (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    act_id UUID NOT NULL REFERENCES acts(id) ON DELETE CASCADE,
    place_id UUID NOT NULL REFERENCES places(id),
    visibility TEXT NOT NULL CHECK (visibility IN ('public','private','unlisted')),
    valid_from DATE NOT NULL,
    valid_to DATE,
    asserted_by_did TEXT NOT NULL,
    CHECK (valid_to IS NULL OR valid_to >= valid_from)
);
```

- [ ] **Step 3: Add a complete reverse-order down migration**

```sql
DROP TABLE IF EXISTS act_home_territories;
DROP TABLE IF EXISTS act_scene_affiliations;
DROP TABLE IF EXISTS acts;
DROP TABLE IF EXISTS profile_controllers;
DROP TABLE IF EXISTS profiles;
DROP TABLE IF EXISTS venues;
DROP TABLE IF EXISTS places;
```

- [ ] **Step 4: Validate both migration directions and model tests**

Run: `MIGRATE_ENV_FILE=./configs/dev.env make migrate-up`
Expected: version advances through `000033`.
Run: `MIGRATE_ENV_FILE=./configs/dev.env make migrate-down`
Expected: `000033` rolls back cleanly.
Run: `go test ./internal/touring -run 'Place|Venue|Profile|HomeTerritory' -count=1`
Expected: PASS.

- [ ] **Step 5: Commit the identity/geography model**

```bash
git add migrations/000033_* internal/touring
git commit -m "feat: add touring identity and place model"
```

## Task 4: Add Tours, Appearances, Hosts, and Provenance

**Files:**

- Create: `migrations/000034_tours_appearances_provenance.up.sql`
- Create: `migrations/000034_tours_appearances_provenance.down.sql`
- Modify: `internal/touring/model.go`
- Create: `internal/touring/repository.go`
- Test: `internal/touring/repository_test.go`

- [ ] **Step 1: Write failing relation tests for the three required projections**

```go
func TestAppearanceProjectionKinds(t *testing.T) {
	tourID := "tour"
	cases := []struct{ name string; tourID *string; eventKind string; want string }{
		{"tour stop", &tourID, "show", "tour_stop"},
		{"festival", nil, "festival", "festival_appearance"},
		{"one off", nil, "show", "one_off"},
	}
	for _, tc := range cases {
		if got := ProjectAppearanceKind(tc.tourID, tc.eventKind); got != tc.want { t.Errorf("%s=%s", tc.name, got) }
	}
}
```

- [ ] **Step 2: Create host, Tour, Appearance, and source tables**

```sql
ALTER TABLE events ADD COLUMN place_id UUID REFERENCES places(id);
ALTER TABLE events ADD COLUMN venue_id UUID REFERENCES venues(id);
ALTER TABLE events ADD COLUMN kind TEXT NOT NULL DEFAULT 'show'
    CHECK (kind IN ('show','festival','party','meetup','broadcast','other'));

CREATE TABLE event_hosts (
    event_id UUID NOT NULL REFERENCES events(id) ON DELETE CASCADE,
    scene_id UUID REFERENCES scenes(id) ON DELETE CASCADE,
    profile_id UUID REFERENCES profiles(id) ON DELETE CASCADE,
    role TEXT NOT NULL CHECK (role IN ('host','promoter','venue','publisher')),
    CHECK ((scene_id IS NOT NULL)::int + (profile_id IS NOT NULL)::int = 1)
);

CREATE UNIQUE INDEX event_hosts_scene_unique
    ON event_hosts (event_id, scene_id, role) WHERE scene_id IS NOT NULL;
CREATE UNIQUE INDEX event_hosts_profile_unique
    ON event_hosts (event_id, profile_id, role) WHERE profile_id IS NOT NULL;

CREATE TABLE tours (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    primary_act_id UUID NOT NULL REFERENCES acts(id),
    title TEXT NOT NULL,
    status TEXT NOT NULL CHECK (status IN ('draft','announced','changed','cancelled','completed')),
    starts_on DATE,
    ends_on DATE,
    CHECK (ends_on IS NULL OR starts_on IS NULL OR ends_on >= starts_on)
);

CREATE TABLE tour_acts (
    tour_id UUID NOT NULL REFERENCES tours(id) ON DELETE CASCADE,
    act_id UUID NOT NULL REFERENCES acts(id) ON DELETE CASCADE,
    role TEXT NOT NULL CHECK (role IN ('primary','co_headliner','support','guest')),
    added_by_did TEXT NOT NULL,
    PRIMARY KEY (tour_id, act_id, role)
);

CREATE TABLE appearances (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    event_id UUID NOT NULL REFERENCES events(id) ON DELETE CASCADE,
    act_id UUID NOT NULL REFERENCES acts(id) ON DELETE CASCADE,
    tour_id UUID REFERENCES tours(id) ON DELETE SET NULL,
    role TEXT NOT NULL CHECK (role IN ('headliner','support','performer','dj','speaker','host','other')),
    stage_name TEXT,
    starts_at TIMESTAMPTZ,
    ends_at TIMESTAMPTZ,
    status TEXT NOT NULL CHECK (status IN ('announced','confirmed','cancelled','completed')),
    CHECK (ends_at IS NULL OR starts_at IS NULL OR ends_at > starts_at),
    UNIQUE (event_id, act_id, role, starts_at)
);

CREATE TABLE sources (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    provider TEXT NOT NULL,
    external_id TEXT,
    canonical_url TEXT,
    payload_sha256 CHAR(64) NOT NULL,
    first_seen_at TIMESTAMPTZ NOT NULL,
    last_seen_at TIMESTAMPTZ NOT NULL,
    UNIQUE NULLS NOT DISTINCT (provider, external_id, canonical_url)
);

CREATE TABLE entity_assertions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    entity_type TEXT NOT NULL CHECK (entity_type IN ('event','appearance','tour','profile','venue')),
    entity_id UUID NOT NULL,
    source_id UUID NOT NULL REFERENCES sources(id),
    state TEXT NOT NULL CHECK (state IN ('unverified','claimed','verified','disputed','rejected')),
    submitter_type TEXT NOT NULL CHECK (submitter_type IN ('did','integration')),
    submitted_by_did TEXT,
    integration_id TEXT,
    authority_type TEXT NOT NULL CHECK (authority_type IN ('artist','host','venue','promoter','ticketing_provider','community_proposal')),
    asserted_fields JSONB NOT NULL,
    rationale TEXT,
    reviewed_by_did TEXT,
    reviewed_at TIMESTAMPTZ,
    asserted_at TIMESTAMPTZ NOT NULL,
    supersedes_id UUID REFERENCES entity_assertions(id),
    CHECK (
        (submitter_type = 'did' AND submitted_by_did IS NOT NULL AND integration_id IS NULL)
        OR (submitter_type = 'integration' AND integration_id IS NOT NULL AND submitted_by_did IS NULL)
    ),
    CHECK ((reviewed_by_did IS NULL) = (reviewed_at IS NULL))
);
```

- [ ] **Step 3: Backfill every existing Event's compatibility host relation**

```sql
INSERT INTO event_hosts (event_id, scene_id, role)
SELECT id, scene_id, 'host' FROM events
ON CONFLICT DO NOTHING;
```

- [ ] **Step 4: Add the reverse migration in dependency order**

```sql
DROP TABLE IF EXISTS entity_assertions;
DROP TABLE IF EXISTS sources;
DROP TABLE IF EXISTS appearances;
DROP TABLE IF EXISTS tour_acts;
DROP TABLE IF EXISTS tours;
DROP TABLE IF EXISTS event_hosts;
ALTER TABLE events DROP COLUMN IF EXISTS kind;
ALTER TABLE events DROP COLUMN IF EXISTS venue_id;
ALTER TABLE events DROP COLUMN IF EXISTS place_id;
```

- [ ] **Step 5: Test timezone, festival, cancellation, and dedup boundaries**

Run: `go test ./internal/touring -run 'Appearance|Tour|Source|Assertion' -count=1`
Expected: PASS, including two-city tour, festival lineup, one-off, changed venue, and conflicting-source fixtures.

- [ ] **Step 6: Commit touring relations**

```bash
git add migrations/000034_* internal/touring
git commit -m "feat: model tours appearances and provenance"
```

## Task 5: Expose Touring Search and Public Read Models

**Files:**

- Create: `internal/api/touring_handlers.go`
- Modify: `internal/scene/repository.go`
- Modify: `cmd/api/main.go`
- Modify: `docs/openapi.yaml`
- Test: `internal/api/touring_handlers_test.go`

- [ ] **Step 1: Write failing HTTP tests**

```go
func TestSearchAppearancesFiltersVisitingActsByOccurrencePlace(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/search/appearances?place=chicago&locality=visiting&from=2026-09-01T00:00:00Z&to=2026-10-01T00:00:00Z", nil)
	rr := httptest.NewRecorder()
	handler.SearchAppearances(rr, req)
	if rr.Code != http.StatusOK { t.Fatalf("status=%d body=%s", rr.Code, rr.Body.String()) }
}
```

- [ ] **Step 2: Implement explicit query facets**

Support `place`, `bbox`, `from`, `to`, `act`, `tour`, `festival`, `scene`,
`kind`, `locality=any|local|visiting`, and `access`. Apply disclosure, status,
time, and occurrence geography before ranking.

- [ ] **Step 3: Register additive routes**

```go
mux.HandleFunc("/profiles/", touringHandlers.Profile)
mux.HandleFunc("/tours/", touringHandlers.Tour)
mux.HandleFunc("/events/", existingEventDispatcherWithAppearances)
mux.HandleFunc("/search/appearances", touringHandlers.SearchAppearances)
```

Preserve the existing `/events` and `/search/events` contracts; add response
expansion or versioned fields rather than breaking clients.

- [ ] **Step 4: Validate OpenAPI and handler behavior**

Run: `go test ./internal/api ./internal/scene -run 'Tour|Appearance|Search' -count=1`
Expected: PASS.
Run: `npx @redocly/cli lint docs/openapi.yaml`
Expected: zero OpenAPI errors.

- [ ] **Step 5: Commit the touring read API**

```bash
git add internal/api internal/scene cmd/api/main.go docs/openapi.yaml
git commit -m "feat: expose location-aware touring discovery"
```

## Task 6: Add Web Touring Discovery Surfaces

**Files:**

- Create: `web/src/types/touring.ts`
- Create: `web/src/components/discovery/AppearanceCard.tsx`
- Create: `web/src/components/discovery/TourMapLayer.tsx`
- Create: `web/src/pages/ProfileDetailPage.tsx`
- Create: `web/src/pages/TourDetailPage.tsx`
- Modify: `web/src/routes/index.tsx`
- Test: corresponding `*.test.tsx` files

- [ ] **Step 1: Write a failing card semantics test**

```tsx
it('shows host, occurrence, and visiting context separately', () => {
  render(<AppearanceCard appearance={visitingFixture} />)
  expect(screen.getByText('Metro, Chicago')).toBeInTheDocument()
  expect(screen.getByText('Hosted by Smartbar')).toBeInTheDocument()
  expect(screen.getByText('Visiting from Detroit')).toBeInTheDocument()
})
```

- [ ] **Step 2: Define stable client DTOs**

```ts
export interface AppearanceSummary {
  id: string
  event: { id: string; title: string; starts_at: string; kind: string; occurrence: PublicOccurrence }
  act: { id: string; name: string; home_territory?: string }
  tour?: { id: string; title: string }
  host_names: string[]
  context: 'tour_stop' | 'festival_appearance' | 'one_off'
  locality: 'local' | 'visiting' | 'unknown'
  status: 'announced' | 'confirmed' | 'cancelled' | 'completed'
  verification: 'unverified' | 'claimed' | 'verified' | 'disputed' | 'rejected'
}
```

- [ ] **Step 3: Implement occurrence-based map/list behavior**

Use `event.occurrence.display_point` for markers. Render multiple Appearances at
one Event inside one marker/detail card instead of duplicating map points.

- [ ] **Step 4: Add routes and run focused UI tests**

Run: `cd web && npm test -- src/components/discovery src/pages/ProfileDetailPage.test.tsx src/pages/TourDetailPage.test.tsx --run`
Expected: PASS.
Run: `cd web && npm run build`
Expected: successful production build.

- [ ] **Step 5: Commit the touring UI**

```bash
git add web/src/types/touring.ts web/src/components/discovery web/src/pages web/src/routes/index.tsx
git commit -m "feat: add touring and festival discovery UI"
```

## Task 7: Add Audience, Contact, Consent, and Suppression Ledger

**Files:**

- Create: `migrations/000035_audience_consent.up.sql`
- Create: `migrations/000035_audience_consent.down.sql`
- Create: `internal/audience/model.go`, `repository.go`, `service.go`
- Test: `internal/audience/service_test.go`

- [ ] **Step 1: Write failing consent-invariant tests**

```go
func TestRSVPDoesNotAuthorizePush(t *testing.T) {
	service := newAudienceFixtureWithVerifiedLink()
	request := DeliveryScope{SenderType: "profile", SenderID: "profile-a", Channel: "push", Purpose: "marketing"}
	service.CreateScope(request)
	service.RecordRelationship(Relationship{Kind: "rsvp", SubjectID: "did:plc:a", ProgramID: "profile-a"})
	allowed, err := service.CanDeliver(context.Background(), "contact-a", request)
	if err != nil || allowed {
		t.Fatal("RSVP must not authorize push marketing")
	}
}

func TestRevocationOverridesScheduledDelivery(t *testing.T) {
	service := newAudienceFixtureWithGrant()
	request := DeliveryScope{SenderType: "profile", SenderID: "profile-a", Channel: "email", Purpose: "tour_updates"}
	scopeID := service.ScopeIDFor(request)
	service.Revoke("contact-a", scopeID)
	allowed, err := service.CanDeliver(context.Background(), "contact-a", request)
	if err != nil || allowed { t.Fatal("revocation must win") }
}

func TestVerifiedContactWithoutGrantCannotReceiveMarketing(t *testing.T) {
	service := newAudienceFixtureWithVerifiedContact()
	request := DeliveryScope{SenderType: "profile", SenderID: "profile-a", Channel: "email", Purpose: "tour_updates"}
	service.CreateScope(request)
	allowed, err := service.CanDeliver(context.Background(), "contact-a", request)
	if err != nil || allowed {
		t.Fatal("verification without a matching grant must not authorize delivery")
	}
}

func TestGlobalSuppressionOverridesExactGrant(t *testing.T) {
	service := newAudienceFixtureWithGrant()
	request := DeliveryScope{SenderType: "profile", SenderID: "profile-a", Channel: "email", Purpose: "tour_updates"}
	service.SuppressGlobally("contact-a")
	allowed, err := service.CanDeliver(context.Background(), "contact-a", request)
	if err != nil || allowed {
		t.Fatal("global suppression must override exact grant")
	}
}

func TestNarrowTourRevocationOverridesProfileWideGrant(t *testing.T) {
	service := newAudienceFixtureWithProfileWideGrant("profile-a", "email", "tour_updates")
	request := DeliveryScope{SenderType: "profile", SenderID: "profile-a", Channel: "email", Purpose: "tour_updates", TourID: "tour-a"}
	service.Revoke("contact-a", service.ScopeIDFor(request))
	allowed, err := service.CanDeliver(context.Background(), "contact-a", request)
	if err != nil || allowed {
		t.Fatal("a tour revocation must override the containing profile-wide grant")
	}
}

func TestTourPlaceGrantMatchesOnlyThatTourAndPlace(t *testing.T) {
	service := newAudienceFixtureWithGrantFor(DeliveryScope{
		SenderType: "profile", SenderID: "profile-a", Channel: "email", Purpose: "tour_updates",
		TourID: "tour-a", PlaceID: "chicago",
	})
	allowed, _ := service.CanDeliver(context.Background(), "contact-a", DeliveryScope{
		SenderType: "profile", SenderID: "profile-a", Channel: "email", Purpose: "tour_updates",
		TourID: "tour-a", PlaceID: "chicago",
	})
	wrongPlace, _ := service.CanDeliver(context.Background(), "contact-a", DeliveryScope{
		SenderType: "profile", SenderID: "profile-a", Channel: "email", Purpose: "tour_updates",
		TourID: "tour-a", PlaceID: "detroit",
	})
	if !allowed || wrongPlace { t.Fatalf("allowed=%v wrongPlace=%v", allowed, wrongPlace) }
}
```

- [ ] **Step 2: Create private audience and append-only consent tables**

```sql
CREATE TABLE contact_points (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    kind TEXT NOT NULL CHECK (kind IN ('email','phone','web_push','apns','provider_social')),
    encrypted_value BYTEA NOT NULL,
    value_hmac CHAR(64) NOT NULL,
    verified_at TIMESTAMPTZ,
    UNIQUE (kind, value_hmac)
);

CREATE TABLE contact_point_links (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    contact_point_id UUID NOT NULL REFERENCES contact_points(id) ON DELETE CASCADE,
    user_did TEXT NOT NULL,
    verification_method TEXT NOT NULL,
    evidence JSONB NOT NULL,
    verified_at TIMESTAMPTZ NOT NULL,
    revoked_at TIMESTAMPTZ,
    CHECK (revoked_at IS NULL OR revoked_at >= verified_at)
);

CREATE TABLE audience_relationships (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    subject_did TEXT,
    contact_point_id UUID REFERENCES contact_points(id),
    program_type TEXT NOT NULL CHECK (program_type IN ('scene','profile')),
    program_id UUID NOT NULL,
    kind TEXT NOT NULL CHECK (kind IN ('membership','rsvp','attendance','purchase','stream','interest','delivery_consent')),
    source_type TEXT NOT NULL,
    source_id TEXT NOT NULL,
    occurred_at TIMESTAMPTZ NOT NULL,
    CHECK ((subject_did IS NOT NULL)::int + (contact_point_id IS NOT NULL)::int = 1)
);

CREATE TABLE consent_scopes (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    sender_type TEXT NOT NULL CHECK (sender_type IN ('scene','profile')),
    sender_id UUID NOT NULL,
    channel TEXT NOT NULL,
    purpose TEXT NOT NULL,
    tour_id UUID REFERENCES tours(id) ON DELETE CASCADE,
    event_id UUID REFERENCES events(id) ON DELETE CASCADE,
    appearance_id UUID REFERENCES appearances(id) ON DELETE CASCADE,
    place_id UUID REFERENCES places(id),
    disclosure_version TEXT NOT NULL,
    region TEXT,
    UNIQUE NULLS NOT DISTINCT (
        sender_type, sender_id, channel, purpose,
        tour_id, event_id, appearance_id, place_id, disclosure_version, region
    )
);

CREATE TABLE consent_events (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    contact_point_id UUID NOT NULL REFERENCES contact_points(id),
    consent_scope_id UUID NOT NULL REFERENCES consent_scopes(id),
    action TEXT NOT NULL CHECK (action IN ('grant','revoke')),
    capture_source TEXT NOT NULL,
    occurred_at TIMESTAMPTZ NOT NULL,
    evidence JSONB NOT NULL DEFAULT '{}'::jsonb
);

CREATE TABLE suppressions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    contact_point_id UUID NOT NULL REFERENCES contact_points(id),
    level TEXT NOT NULL CHECK (level IN ('global','channel','sender','scope')),
    channel TEXT,
    sender_type TEXT CHECK (sender_type IN ('scene','profile')),
    sender_id UUID,
    consent_scope_id UUID REFERENCES consent_scopes(id),
    reason TEXT NOT NULL,
    occurred_at TIMESTAMPTZ NOT NULL,
    lifted_at TIMESTAMPTZ,
    CHECK (lifted_at IS NULL OR lifted_at >= occurred_at),
    CHECK (
        (level = 'global' AND channel IS NULL AND sender_type IS NULL AND sender_id IS NULL AND consent_scope_id IS NULL)
        OR (level = 'channel' AND channel IS NOT NULL AND sender_type IS NULL AND sender_id IS NULL AND consent_scope_id IS NULL)
        OR (level = 'sender' AND sender_type IS NOT NULL AND sender_id IS NOT NULL AND consent_scope_id IS NULL)
        OR (level = 'scope' AND channel IS NULL AND sender_type IS NULL AND sender_id IS NULL AND consent_scope_id IS NOT NULL)
    )
);
```

- [ ] **Step 3: Implement containment-aware scope resolution and authorization**

```go
type DeliveryScope struct {
	SenderType, SenderID string
	Channel, Purpose string
	TourID, EventID, AppearanceID, PlaceID string
}

func (s *Service) CanDeliver(ctx context.Context, contactID string, request DeliveryScope) (bool, error) {
	verified, err := s.repo.IsContactVerified(ctx, contactID)
	if err != nil || !verified { return false, err }
	if suppressed, err := s.repo.HasApplicableSuppression(ctx, contactID, request); err != nil || suppressed {
		return false, err
	}
	states, err := s.repo.LatestApplicableConsentStates(ctx, contactID, request)
	if err != nil { return false, err }
	hasGrant := false
	for _, state := range states {
		if state.Action == ConsentRevoke { return false, nil }
		if state.Action == ConsentGrant { hasGrant = true }
	}
	return hasGrant, nil
}
```

`LatestApplicableConsentStates` must first require exact sender, channel, and
purpose. A stored scope is applicable only when every non-null optional
restriction on it equals the corresponding request field. It returns only the
latest event for each applicable scope. This makes a Profile-wide grant contain
a Tour + Place request while preserving a narrower revocation as a denial.

- [ ] **Step 4: Add the reverse migration in dependency order**

```sql
DROP TABLE IF EXISTS suppressions;
DROP TABLE IF EXISTS consent_events;
DROP TABLE IF EXISTS consent_scopes;
DROP TABLE IF EXISTS audience_relationships;
DROP TABLE IF EXISTS contact_point_links;
DROP TABLE IF EXISTS contact_points;
```

- [ ] **Step 5: Run consent and migration tests**

Run: `go test ./internal/audience -count=1`
Expected: PASS for independent identities, grant, verification, revocation, and suppression.
Run: `MIGRATE_ENV_FILE=./configs/dev.env make migrate-up` then `MIGRATE_ENV_FILE=./configs/dev.env make migrate-down`
Expected: `000035` applies and reverses without losing earlier tables.

- [ ] **Step 6: Commit the consent boundary**

```bash
git add migrations/000035_audience_consent.* internal/audience
git commit -m "feat: add scoped audience consent ledger"
```

## Task 8: Add Versioned Signals and Provider-Neutral Delivery

**Files:**

- Create: `migrations/000036_signals_delivery.up.sql`
- Create: `migrations/000036_signals_delivery.down.sql`
- Create: `internal/signal/model.go`, `repository.go`, `service.go`, `delivery.go`
- Create: `internal/api/signal_handlers.go`
- Test: `internal/signal/service_test.go`

- [ ] **Step 1: Write failing publish and pre-send suppression tests**

```go
func TestPublishedSignalMaterialChangeCreatesRevision(t *testing.T) {
	svc := newSignalFixture()
	id := svc.Publish(draftSignal())
	rev := svc.ChangeAudience(id, "tour-city:chicago")
	if rev.Number != 2 || rev.Supersedes == nil { t.Fatalf("revision=%+v", rev) }
}

func TestDispatcherRechecksConsentImmediatelyBeforeSend(t *testing.T) {
	fixture := scheduledDeliveryFixture()
	fixture.audience.Revoke(fixture.contactID, fixture.consentScopeID)
	if err := fixture.dispatcher.Dispatch(context.Background(), fixture.deliveryID); !errors.Is(err, ErrSuppressed) { t.Fatalf("err=%v", err) }
}
```

- [ ] **Step 2: Add Signal, revision, delivery, and event tables**

```sql
CREATE TABLE signals (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    owner_type TEXT NOT NULL CHECK (owner_type IN ('scene','profile')),
    owner_id UUID NOT NULL,
    target_type TEXT NOT NULL CHECK (target_type IN ('scene','profile','event','appearance','tour','post','stream','offer')),
    target_id UUID NOT NULL,
    state TEXT NOT NULL CHECK (state IN ('draft','scheduled','published','completed','cancelled')),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE signal_revisions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    signal_id UUID NOT NULL REFERENCES signals(id) ON DELETE CASCADE,
    revision INT NOT NULL,
    content JSONB NOT NULL,
    audience_definition JSONB NOT NULL,
    publish_at TIMESTAMPTZ,
    created_by_did TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (signal_id, revision)
);

CREATE TABLE deliveries (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    signal_revision_id UUID NOT NULL REFERENCES signal_revisions(id),
    contact_point_id UUID NOT NULL REFERENCES contact_points(id),
    channel TEXT NOT NULL,
    purpose TEXT NOT NULL,
    provider TEXT NOT NULL,
    state TEXT NOT NULL CHECK (state IN ('queued','suppressed','sent','delivered','failed','cancelled')),
    provider_message_id TEXT,
    scheduled_at TIMESTAMPTZ NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (signal_revision_id, contact_point_id, channel)
);

CREATE TABLE engagement_events (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    delivery_id UUID REFERENCES deliveries(id),
    signal_id UUID NOT NULL REFERENCES signals(id),
    kind TEXT NOT NULL CHECK (kind IN ('view','click','rsvp','purchase','unsubscribe','complaint')),
    event_id UUID REFERENCES events(id),
    appearance_id UUID REFERENCES appearances(id),
    tour_id UUID REFERENCES tours(id),
    occurred_at TIMESTAMPTZ NOT NULL,
    provenance JSONB NOT NULL
);
```

- [ ] **Step 3: Define a narrow provider interface**

```go
type Provider interface {
	Send(ctx context.Context, message Message) (providerMessageID string, err error)
}

type Message struct {
	Channel string
	ToToken []byte
	Subject string
	Body string
	DeepLink string
}
```

Start with web push and one email provider. Do not implement SMS or social
delivery in this task.

- [ ] **Step 4: Add the reverse migration in dependency order**

```sql
DROP TABLE IF EXISTS engagement_events;
DROP TABLE IF EXISTS deliveries;
DROP TABLE IF EXISTS signal_revisions;
DROP TABLE IF EXISTS signals;
```

- [ ] **Step 5: Run Signal lifecycle and dispatch tests**

Run: `go test ./internal/signal ./internal/audience ./internal/api -run 'Signal|Delivery|Consent|Suppress' -count=1`
Expected: PASS with deterministic audience snapshot, versioning, idempotent delivery, and last-moment suppression.

- [ ] **Step 6: Commit Signals and initial delivery adapters**

```bash
git add migrations/000036_signals_delivery.* internal/signal internal/api/signal_handlers.go
git commit -m "feat: add versioned signals and consented delivery"
```

## Task 9: Add Signal and Consent Web Experiences

**Files:**

- Create: `web/src/types/audience.ts`, `web/src/types/signal.ts`
- Create: `web/src/pages/SignalDetailPage.tsx`
- Create: `web/src/components/ConsentControl.tsx`
- Modify: `web/src/components/NotificationSettings.tsx`
- Modify: `web/src/routes/index.tsx`
- Test: corresponding `*.test.tsx` files

- [ ] **Step 1: Write a failing consent UX test**

```tsx
it('does not treat RSVP as consent and permits city-scoped tour interest', async () => {
  render(<ConsentControl fixture={rsvpWithoutConsent} />)
  expect(screen.getByText('RSVP saved')).toBeInTheDocument()
  expect(screen.getByRole('checkbox', { name: 'Email me about Chicago dates' })).not.toBeChecked()
})
```

- [ ] **Step 2: Implement explicit scope and disclosure rendering**

Display sender/Profile, channel, purpose, city/Tour scope, frequency, policy
version, verification state, and revoke action before enabling delivery.

- [ ] **Step 3: Add Signal detail and deep-link routes**

```tsx
{ path: 'signals/:id', element: <SignalDetailPage /> },
{ path: 'profiles/:id', element: <ProfileDetailPage /> },
{ path: 'tours/:id', element: <TourDetailPage /> },
```

- [ ] **Step 4: Run accessibility and build checks**

Run: `cd web && npm test -- src/components/ConsentControl.test.tsx src/pages/SignalDetailPage.test.tsx --run`
Expected: PASS.
Run: `cd web && npm run lint && npm run build`
Expected: zero lint errors and successful build.

- [ ] **Step 5: Commit the participant consent experience**

```bash
git add web/src/types web/src/components/ConsentControl* web/src/pages/SignalDetailPage* web/src/components/NotificationSettings.tsx web/src/routes/index.tsx
git commit -m "feat: add signal and consent experiences"
```

## Task 10: Add Commerce Attribution and a Provenance-Preserving CSV Import

**Files:**

- Modify: `internal/payment/model.go`, `repository.go`
- Modify: `internal/payment/webhook_repository.go`
- Create: `internal/touring/importer.go`, `reconciliation.go`
- Test: `internal/payment/*_test.go`, `internal/touring/importer_test.go`

- [ ] **Step 1: Write failing attribution-window and duplicate-import tests**

```go
func TestConversionStoresModelAndWindow(t *testing.T) {
	got := AttributePurchase(clickAt, purchaseAt, 7*24*time.Hour)
	if !got.Attributed || got.Model != "last_signal_click" || got.Window != 7*24*time.Hour { t.Fatalf("got=%+v", got) }
}

func TestImporterDoesNotMergeAmbiguousFestivalAfterparty(t *testing.T) {
	result := Import(conflictingFestivalAndAfterpartyFixtures())
	if result.AutoMerged != 0 || result.ReviewCandidates != 1 { t.Fatalf("result=%+v", result) }
}
```

- [ ] **Step 2: Add explicit Signal/Event/Appearance/Tour attribution references**

Persist `signal_id`, optional `delivery_id`, `event_id`, `appearance_id`,
`tour_id`, attribution model, window, provider event ID, raw payload digest, and
received time. Never label modeled attribution as causal ground truth.

- [ ] **Step 3: Implement the first bounded CSV adapter**

Accept the exact header below and reject files with missing or additional
columns. Each row becomes a source assertion; it never overwrites a canonical
Event directly.

```text
external_id,title,act_name,venue_name,place_name,country_code,timezone,starts_at,ends_at,status,source_url
```

```go
type CSVRow struct {
	ExternalID string
	Title string
	ActName string
	VenueName string
	PlaceName string
	CountryCode string
	Timezone string
	StartsAt time.Time
	EndsAt *time.Time
	Status string
	SourceURL string
}

func (i *Importer) ImportCSV(ctx context.Context, provider string, r io.Reader) (ImportResult, error) {
	rows, err := decodeStrictCSV(r)
	if err != nil { return ImportResult{}, err }
	result := ImportResult{}
	for _, row := range rows {
		assertion, err := i.normalizeAndAssert(ctx, provider, row)
		if err != nil { return result, err }
		if assertion.NeedsReview { result.ReviewCandidates++ } else { result.Accepted++ }
	}
	return result, nil
}
```

Ticketing-specific webhook adapters follow only after a beachhead provider is
chosen; they reuse the same source/assertion/reconciliation boundary.

- [ ] **Step 4: Run signed-webhook, replay, and reconciliation tests**

Run: `go test ./internal/payment ./internal/touring -run 'Webhook|Attribution|Import|Reconciliation' -count=1`
Expected: PASS for signature failure, replay idempotency, exact external-ID
match, ambiguous candidate, cancellation, and correction.

- [ ] **Step 5: Commit commerce/import integration**

```bash
git add internal/payment internal/touring
git commit -m "feat: attribute signals and import verified event data"
```

## Task 11: Reconcile Documentation, Mobile Contract, and Status Evidence

**Files:**

- Modify: `README.md`
- Modify: `docs/product/AUDIENCE_DROPS_AND_TOURING.md`
- Modify: `docs/ARCHITECTURE.md`
- Modify: `docs/GLOSSARY.md`
- Modify: `docs/PRIVACY.md`
- Modify: `docs/DATA_CLASSIFICATION.md`
- Modify: `docs/IOS_NATIVE_APP_HANDOFF.md`
- Modify: `docs/openapi.yaml`
- Modify: `migrations/README.md`

- [ ] **Step 1: Update status labels from planned to implemented only where tests passed**

Record separate evidence for schema, unit/API tests, browser UI, configured
runtime, and any external provider smoke. Do not treat documentation or fixtures
as deployed behavior.

- [ ] **Step 2: Run contradiction and placeholder scans**

Run:

```bash
rg -n "event within a scene|scene_id.*home|RSVP.*consent|purchase.*consent|TBD|TODO|implement later|similar to" README.md docs migrations/README.md
```

Expected: no incorrect event/home-base or implicit-consent claims; no unresolved
placeholders in the product or implementation plan.

- [ ] **Step 3: Validate all local Markdown links**

Run: `npx markdown-link-check README.md docs/**/*.md`
Expected: no broken local links; external network failures are recorded
separately from local-link failures.

- [ ] **Step 4: Run the repository quality gate**

Run: `make test`
Expected: Go and frontend tests pass. If system `libvips` metadata is absent,
install the documented development dependency or run in the repository's
supported build image; do not report an unexecuted suite as passing.
Run: `make lint && make build`
Expected: zero lint/build errors.

- [ ] **Step 5: Perform the product validation matrix**

Verify fixtures and UI/API results for:

1. two-city tour;
2. three-act festival with set times;
3. guest one-off without Tour;
4. changed, postponed, and cancelled date;
5. same-name venues in different Places;
6. protected location revealed only after authorization;
7. coarse-only public occurrence;
8. private home Scene with a public away Appearance;
9. conflicting provider assertions and non-destructive correction;
10. consent revoked after scheduling but before delivery.

- [ ] **Step 6: Commit truthful final documentation**

```bash
git add README.md docs migrations/README.md
git commit -m "docs: qualify audience drops and touring delivery"
```

## Self-Review Checklist

- [ ] Every requirement in `docs/product/AUDIENCE_DROPS_AND_TOURING.md` maps to a task above.
- [ ] Event occurrence Place, Act Home Territory, and user location remain distinct in schema, API, and UI.
- [ ] Tour stop, festival appearance, and one-off derive from Event + Appearance (+ optional Tour).
- [ ] RSVP, membership, purchase, and participation do not create messaging consent.
- [ ] Source assertions and corrections are retained without ambiguous automatic merging.
- [ ] Provider delivery and attribution are adapters over Subcult-owned state.
- [ ] SMS/social automation and autonomous agents remain outside the first delivery slice.
- [ ] Documentation status matches test/runtime/provider evidence.
