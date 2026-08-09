//go:build integration

package db_test

import (
	"context"
	"database/sql"
	"errors"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	_ "github.com/lib/pq"
	"github.com/onnwee/subcults/internal/audience"
	runtimedb "github.com/onnwee/subcults/internal/db"
	"github.com/onnwee/subcults/internal/identity"
	"github.com/onnwee/subcults/internal/locationaccess"
	"github.com/onnwee/subcults/internal/scene"
	domainsignal "github.com/onnwee/subcults/internal/signal"
	"github.com/onnwee/subcults/internal/touring"
)

func integrationDB(t *testing.T) *sql.DB {
	t.Helper()
	url := strings.TrimSpace(os.Getenv("TEST_DATABASE_URL"))
	if url == "" {
		t.Skip("TEST_DATABASE_URL is required")
	}
	database, err := sql.Open("postgres", url)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = database.Close() })
	if err = database.PingContext(context.Background()); err != nil {
		t.Fatal(err)
	}
	return database
}

func TestDurableRepositoriesSurviveRestartAndEnforceGuarantees(t *testing.T) {
	database := integrationDB(t)
	ctx := context.Background()
	suffix := uuid.NewString()
	sceneID := uuid.NewString()
	sceneRepo := scene.NewSQLSceneRepository(database)
	original := &scene.Scene{ID: sceneID, Name: "Durable " + suffix, OwnerDID: "did:web:example:" + suffix, CoarseGeohash: "dp3wj", Visibility: "public", AllowPrecise: true, PrecisePoint: &scene.Point{Lat: 41.88, Lng: -87.63}}
	if err := sceneRepo.Insert(original); err != nil {
		t.Fatalf("insert scene: %v", err)
	}
	// Reconstructing the adapter represents an API restart: no process-local
	// state is shared with the new repository value.
	restartedSceneRepo := scene.NewSQLSceneRepository(database)
	persisted, err := restartedSceneRepo.GetByID(sceneID)
	if err != nil {
		t.Fatalf("scene after restart: %v", err)
	}
	if persisted.Name != original.Name {
		t.Fatalf("persisted name=%q", persisted.Name)
	}
	stale := *persisted
	fresh := *persisted
	fresh.Description = "first edit"
	if err = restartedSceneRepo.Update(&fresh); err != nil {
		t.Fatalf("fresh update: %v", err)
	}
	stale.Description = "stale edit"
	if err = restartedSceneRepo.Update(&stale); !errors.Is(err, scene.ErrVersionConflict) {
		t.Fatalf("stale update=%v", err)
	}

	eventRepo := scene.NewSQLEventRepository(database)
	tourRepo := touring.NewSQLRepository(database)
	placeID := uuid.NewString()
	if err = tourRepo.StorePlace(touring.Place{ID: placeID, CanonicalName: "Chicago " + suffix, CountryCode: "US", Timezone: "America/Chicago", CoarseGeohash: "dp3wj"}); err != nil {
		t.Fatalf("place: %v", err)
	}
	profileID, actID := uuid.NewString(), uuid.NewString()
	if err = tourRepo.StoreProfile(touring.Profile{ID: profileID, Kind: "artist", CanonicalName: "Act " + suffix, Visibility: "public"}); err != nil {
		t.Fatalf("profile: %v", err)
	}
	if err = tourRepo.StoreAct(touring.Act{ID: actID, ProfileID: profileID}); err != nil {
		t.Fatalf("act: %v", err)
	}
	tourID := uuid.NewString()
	startDay := time.Now().UTC().AddDate(0, 0, 1)
	if err = tourRepo.CreateTour(touring.Tour{ID: tourID, PrimaryActID: actID, Title: "Two cities", Status: "announced", StartsOn: &startDay}, original.OwnerDID); err != nil {
		t.Fatalf("tour: %v", err)
	}
	types := []struct {
		kind string
		tour *string
		want string
	}{{"show", &tourID, "tour_stop"}, {"festival", nil, "festival_appearance"}, {"show", nil, "one_off"}}
	for index, item := range types {
		eventID := uuid.NewString()
		starts := time.Now().UTC().Add(time.Duration(index+1) * time.Hour)
		event := &scene.Event{ID: eventID, SceneID: sceneID, Title: item.want, CoarseGeohash: "dp3wj", PlaceID: &placeID, Kind: item.kind, LocationAccess: "public", Status: "scheduled", StartsAt: starts}
		if err = eventRepo.Insert(event); err != nil {
			t.Fatalf("event %s: %v", item.want, err)
		}
		if err = tourRepo.CreateAppearance(touring.Appearance{ID: uuid.NewString(), EventID: eventID, ActID: actID, TourID: item.tour, Role: "performer", Status: "announced", StartsAt: &starts}); err != nil {
			t.Fatalf("appearance %s: %v", item.want, err)
		}
	}
	restartedTourRepo := touring.NewSQLRepository(database)
	appearances, err := restartedTourRepo.ListAppearancesForAct(actID)
	if err != nil {
		t.Fatal(err)
	}
	if len(appearances) != 3 {
		t.Fatalf("appearances after restart=%d", len(appearances))
	}
	seen := map[string]bool{}
	for _, appearance := range appearances {
		event, e := eventRepo.GetByID(appearance.EventID)
		if e != nil {
			t.Fatal(e)
		}
		seen[touring.ProjectAppearanceKind(appearance.TourID, event.Kind)] = true
	}
	for _, item := range types {
		if !seen[item.want] {
			t.Fatalf("missing discovery projection %s", item.want)
		}
	}

	protector, err := identity.NewEphemeralContactProtector()
	if err != nil {
		t.Fatal(err)
	}
	ciphertext, hmacValue, err := protector.Protect("fan@example.test")
	if err != nil {
		t.Fatal(err)
	}
	audienceRepo := audience.NewSQLRepository(database)
	audienceService := audience.NewService(audienceRepo)
	verified := time.Now().UTC()
	contactID := uuid.NewString()
	if err = audienceService.AddContact(ctx, audience.ContactPoint{ID: contactID, Kind: "email", EncryptedValue: ciphertext, ValueHMAC: hmacValue, VerifiedAt: &verified}); err != nil {
		t.Fatalf("contact: %v", err)
	}
	scope := audience.DeliveryScope{SenderType: "profile", SenderID: profileID, Channel: "email", Purpose: "tour_updates", TourID: tourID, DisclosureVersion: "v1"}
	scopeID, err := audienceService.CreateScope(ctx, scope)
	if err != nil {
		t.Fatalf("scope: %v", err)
	}
	if err = audienceService.Grant(ctx, contactID, scopeID, "integration_test", map[string]string{"form": "test"}, verified); err != nil {
		t.Fatalf("grant: %v", err)
	}
	allowed, err := audienceService.CanDeliver(ctx, contactID, scope)
	if err != nil || !allowed {
		t.Fatalf("grant allowed=%v err=%v", allowed, err)
	}
	if err = audienceService.Revoke(ctx, contactID, scopeID, "integration_test", map[string]string{"action": "revoke"}, verified.Add(time.Second)); err != nil {
		t.Fatalf("revoke: %v", err)
	}
	allowed, err = audienceService.CanDeliver(ctx, contactID, scope)
	if err != nil || allowed {
		t.Fatalf("revoked allowed=%v err=%v", allowed, err)
	}
	if err = audienceService.Grant(ctx, contactID, scopeID, "integration_test", map[string]string{"action": "regrant"}, verified.Add(2*time.Second)); err != nil {
		t.Fatalf("regrant: %v", err)
	}
	if err = audienceService.Suppress(ctx, audience.Suppression{ContactPointID: contactID, Level: audience.SuppressionScope, ScopeID: scopeID, OccurredAt: verified.Add(3 * time.Second)}); err != nil {
		t.Fatalf("suppress: %v", err)
	}
	allowed, err = audienceService.CanDeliver(ctx, contactID, scope)
	if err != nil || allowed {
		t.Fatalf("suppressed allowed=%v err=%v", allowed, err)
	}

	userID := uuid.NewString()
	did := "did:web:subcults.test:users:" + suffix
	if _, err = database.Exec(`INSERT INTO users(id,did,handle,internal_did) VALUES($1,$2,$3,$2)`, userID, did, "user-"+suffix[:8]); err != nil {
		t.Fatalf("user: %v", err)
	}
	protectedEventID := uuid.NewString()
	protectedEvent := &scene.Event{ID: protectedEventID, SceneID: sceneID, Title: "Protected", AllowPrecise: true, PrecisePoint: &scene.Point{Lat: 41.9, Lng: -87.7}, CoarseGeohash: "dp3wj", Kind: "show", LocationAccess: "protected", Status: "scheduled", StartsAt: time.Now().Add(time.Hour)}
	if err = eventRepo.Insert(protectedEvent); err != nil {
		t.Fatal(err)
	}
	locationRepo := locationaccess.NewSQLRepository(database)
	granted, err := locationRepo.CanView(ctx, protectedEventID, userID, time.Now())
	if err != nil || granted {
		t.Fatalf("pre-grant=%v err=%v", granted, err)
	}
	if err = locationRepo.Grant(ctx, locationaccess.Grant{EventID: protectedEventID, UserID: userID, Reason: "manual", GrantedAt: time.Now()}); err != nil {
		t.Fatalf("location grant: %v", err)
	}
	granted, err = locationRepo.CanView(ctx, protectedEventID, userID, time.Now())
	if err != nil || !granted {
		t.Fatalf("post-grant=%v err=%v", granted, err)
	}
	publicResults, _, err := eventRepo.SearchEvents(scene.EventSearchOptions{MinLng: -88, MinLat: 41, MaxLng: -87, MaxLat: 42, From: time.Now().Add(-time.Hour), To: time.Now().Add(24 * time.Hour), Limit: 20})
	if err != nil {
		t.Fatalf("public discovery: %v", err)
	}
	for _, event := range publicResults {
		if event.ID == protectedEventID && event.PrecisePoint != nil {
			t.Fatal("protected coordinates entered public discovery projection")
		}
	}

	sourceID := uuid.NewString()
	source, err := restartedTourRepo.UpsertSource(touring.Source{ID: sourceID, Provider: "integration", ExternalID: &suffix, PayloadSHA256: strings.Repeat("a", 64), FirstSeenAt: verified, LastSeenAt: verified})
	if err != nil {
		t.Fatalf("source: %v", err)
	}
	firstAssertionID := uuid.NewString()
	if err = restartedTourRepo.CreateAssertion(touring.EntityAssertion{ID: firstAssertionID, EntityType: "tour", EntityID: tourID, SourceID: source.ID, State: "claimed", SubmitterType: "did", SubmittedByDID: &did, AuthorityType: "artist", AssertedFields: map[string]any{"title": "Two cities"}, AssertedAt: verified}); err != nil {
		t.Fatalf("assertion: %v", err)
	}
	correctionID := uuid.NewString()
	if err = restartedTourRepo.CreateAssertion(touring.EntityAssertion{ID: correctionID, EntityType: "tour", EntityID: tourID, SourceID: source.ID, State: "verified", SubmitterType: "did", SubmittedByDID: &did, AuthorityType: "artist", AssertedFields: map[string]any{"title": "Two cities corrected"}, AssertedAt: verified.Add(time.Second), SupersedesID: &firstAssertionID}); err != nil {
		t.Fatalf("correction: %v", err)
	}
	if _, err = restartedTourRepo.GetAssertion(firstAssertionID); err != nil {
		t.Fatalf("superseded provenance disappeared: %v", err)
	}

	// Signal revisions are append-only and deliveries recover their token from
	// the encrypted contact record rather than persisting plaintext.
	signalRepo := domainsignal.NewSQLRepository(database, protector)
	signalService := domainsignal.NewService(signalRepo)
	signalID := uuid.NewString()
	revision, err := signalService.CreateDraft(ctx, domainsignal.Signal{ID: signalID, OwnerType: "profile", OwnerID: profileID, TargetType: "tour", TargetID: tourID, ConsentScopeIDs: []string{scopeID}}, domainsignal.Content{Subject: "Tour", Body: "Dates announced"}, `{"mode":"consented"}`, did, nil)
	if err != nil {
		t.Fatalf("signal draft: %v", err)
	}
	if _, err = database.Exec(`UPDATE signal_revisions SET content='{"body":"mutated"}'::jsonb WHERE id=$1`, revision.ID); err == nil {
		t.Fatal("database allowed an immutable signal revision update")
	}
	deliveries, err := signalService.SnapshotDeliveries(ctx, revision.ID, "email", "tour_updates", "test", scope, []domainsignal.AudienceMember{{ContactPointID: contactID, ToToken: []byte("must-not-persist")}}, time.Now())
	if err != nil {
		t.Fatalf("delivery snapshot: %v", err)
	}
	loaded, err := signalRepo.GetDelivery(ctx, deliveries[0].ID)
	if err != nil {
		t.Fatalf("delivery restart: %v", err)
	}
	if string(loaded.ToToken) != "fan@example.test" {
		t.Fatalf("delivery token was not recovered from encrypted contact")
	}
}

func TestRuntimePoolAndTimeouts(t *testing.T) {
	url := strings.TrimSpace(os.Getenv("TEST_DATABASE_URL"))
	if url == "" {
		t.Skip("TEST_DATABASE_URL is required")
	}
	t.Setenv("DB_MAX_OPEN_CONNS", "7")
	repositories, err := runtimedb.NewRuntimeRepositories(context.Background(), url)
	if err != nil {
		t.Fatal(err)
	}
	defer repositories.Close()
	if got := repositories.DB.Stats().MaxOpenConnections; got != 7 {
		t.Fatalf("max open connections=%d", got)
	}
	var statementTimeout, lockTimeout string
	if err = repositories.DB.QueryRow(`SELECT current_setting('statement_timeout'),current_setting('lock_timeout')`).Scan(&statementTimeout, &lockTimeout); err != nil {
		t.Fatal(err)
	}
	if statementTimeout != "5s" || lockTimeout != "2s" {
		t.Fatalf("timeouts statement=%s lock=%s", statementTimeout, lockTimeout)
	}
}
