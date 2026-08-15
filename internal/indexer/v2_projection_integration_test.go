//go:build integration

package indexer

import (
	"context"
	"math"
	"testing"

	jetstream "github.com/bluesky-social/jetstream"
	"github.com/onnwee/subcults/internal/testutil"
)

func TestPostgresV2ProjectorShadowFoldAndAccountSuppression(t *testing.T) {
	tdb := testutil.NewTestDB(t)
	projector := NewPostgresV2Projector(tdb.DB, NewRecordFilter(NewFilterMetrics()), nil)
	ctx := context.Background()
	request := ProjectionRequest{Consumer: "test-shadow", Target: ProjectionTargetShadow, RebuildID: "release-test"}
	commit := jetstream.Event{
		DID: "did:plc:shadow", Seq: 1, TimeUS: 100,
		Kind: jetstream.KindCommit,
		Commit: &jetstream.Commit{Operation: jetstream.OpCreate, Collection: CollectionScene, Rkey: "scene", Rev: "1", CID: "cid-1",
			Record: map[string]any{"name": "Shadow Scene"}},
	}
	result, err := projector.ApplyBatch(ctx, request, []jetstream.Event{commit}, 1)
	if err != nil {
		t.Fatalf("ApplyBatch(commit): %v", err)
	}
	if result.Processed != 1 {
		t.Fatalf("processed = %d, want 1", result.Processed)
	}
	account := jetstream.Event{
		DID: "did:plc:shadow", Seq: 2, Kind: jetstream.KindAccount,
		Account: &jetstream.Account{DID: "did:plc:shadow", Active: false, Status: "deleted", Seq: 20},
	}
	result, err = projector.ApplyBatch(ctx, request, []jetstream.Event{account}, 2)
	if err != nil {
		t.Fatalf("ApplyBatch(account): %v", err)
	}
	if result.Deleted != 1 {
		t.Fatalf("deleted = %d, want 1", result.Deleted)
	}
	var deleted, suppressed bool
	if err = tdb.DB.QueryRow(`SELECT deleted,suppressed FROM jetstream_v2_shadow_records
		WHERE rebuild_id='release-test' AND at_uri='at://did:plc:shadow/app.subcult.scene/scene'`).Scan(&deleted, &suppressed); err != nil {
		t.Fatalf("read shadow record: %v", err)
	}
	if deleted || !suppressed {
		t.Fatalf("shadow account suppression = (deleted=%t,suppressed=%t), want (false,true)", deleted, suppressed)
	}
	var activeAccountRows int
	if err = tdb.DB.QueryRow(`SELECT COUNT(*) FROM jetstream_v2_accounts WHERE did='did:plc:shadow'`).Scan(&activeAccountRows); err != nil {
		t.Fatalf("count active account state: %v", err)
	}
	if activeAccountRows != 0 {
		t.Fatalf("shadow replay wrote %d live account rows", activeAccountRows)
	}
	account.Seq = 3
	account.Account.Active = true
	account.Account.Status = "active"
	account.Account.Seq = 30
	if _, err = projector.ApplyBatch(ctx, request, []jetstream.Event{account}, 3); err != nil {
		t.Fatalf("ApplyBatch(reactivation): %v", err)
	}
	if err = tdb.DB.QueryRow(`SELECT suppressed FROM jetstream_v2_shadow_records
		WHERE rebuild_id='release-test' AND at_uri='at://did:plc:shadow/app.subcult.scene/scene'`).Scan(&suppressed); err != nil {
		t.Fatalf("read reactivated shadow record: %v", err)
	}
	if suppressed {
		t.Fatal("reactivated shadow account record remains suppressed")
	}
	cursor, err := projector.Cursor(ctx, request)
	if err != nil || cursor != 3 {
		t.Fatalf("cursor = %d, err = %v, want 3", cursor, err)
	}
}

func TestPostgresV2ProjectorKeepsShadowIdentityAndSyncStateIsolated(t *testing.T) {
	tdb := testutil.NewTestDB(t)
	projector := NewPostgresV2Projector(tdb.DB, NewRecordFilter(NewFilterMetrics()), nil)
	ctx := context.Background()
	request := ProjectionRequest{Consumer: "shadow-metadata", Target: ProjectionTargetShadow, RebuildID: "metadata-test"}
	events := []jetstream.Event{
		{DID: "did:plc:shadow-meta", Seq: 10, Kind: jetstream.KindIdentity,
			Identity: &jetstream.Identity{DID: "did:plc:shadow-meta", Handle: "shadow.example", Seq: 100}},
		{DID: "did:plc:shadow-meta", Seq: 11, Kind: jetstream.KindSync,
			Sync: &jetstream.Sync{DID: "did:plc:shadow-meta", Rev: "shadow-rev", Seq: 101}},
	}
	if _, err := projector.ApplyBatch(ctx, request, events, 11); err != nil {
		t.Fatalf("ApplyBatch(): %v", err)
	}
	var handle, rev string
	if err := tdb.DB.QueryRow(`SELECT handle FROM jetstream_v2_shadow_identities
		WHERE rebuild_id='metadata-test' AND did='did:plc:shadow-meta'`).Scan(&handle); err != nil {
		t.Fatalf("read shadow identity: %v", err)
	}
	if err := tdb.DB.QueryRow(`SELECT requested_rev FROM jetstream_v2_shadow_reconciliations
		WHERE rebuild_id='metadata-test' AND did='did:plc:shadow-meta'`).Scan(&rev); err != nil {
		t.Fatalf("read shadow reconciliation: %v", err)
	}
	if handle != "shadow.example" || rev != "shadow-rev" {
		t.Fatalf("shadow metadata = (%q,%q), want (shadow.example,shadow-rev)", handle, rev)
	}
	var liveRows int
	if err := tdb.DB.QueryRow(`SELECT
		(SELECT COUNT(*) FROM jetstream_v2_identities WHERE did='did:plc:shadow-meta') +
		(SELECT COUNT(*) FROM jetstream_v2_reconciliations WHERE did='did:plc:shadow-meta')`).Scan(&liveRows); err != nil {
		t.Fatalf("count live metadata: %v", err)
	}
	if liveRows != 0 {
		t.Fatalf("shadow replay wrote %d live metadata rows", liveRows)
	}
}

func TestPostgresV2ProjectorKeepsShadowQuarantineIsolated(t *testing.T) {
	tdb := testutil.NewTestDB(t)
	projector := NewPostgresV2Projector(tdb.DB, NewRecordFilter(NewFilterMetrics()), nil)
	request := ProjectionRequest{Consumer: "shadow-invalid", Target: ProjectionTargetShadow, RebuildID: "invalid-test"}
	event := jetstream.Event{
		DID: "did:plc:shadow-invalid", Seq: 20, Kind: jetstream.KindCommit,
		Commit: &jetstream.Commit{Operation: jetstream.OpCreate, Collection: CollectionScene, Rkey: "invalid", Rev: "20",
			Record: map[string]any{"description": "missing name"}},
	}
	result, err := projector.ApplyBatch(context.Background(), request, []jetstream.Event{event}, 20)
	if err != nil {
		t.Fatalf("ApplyBatch(): %v", err)
	}
	if result.Quarantined != 1 {
		t.Fatalf("quarantined = %d, want 1", result.Quarantined)
	}
	var shadowFailures, liveFailures, liveObservations int
	if err = tdb.DB.QueryRow(`SELECT
		(SELECT COUNT(*) FROM jetstream_v2_shadow_failures WHERE rebuild_id='invalid-test'),
		(SELECT COUNT(*) FROM atproto_projection_failures WHERE publisher_did='did:plc:shadow-invalid'),
		(SELECT COUNT(*) FROM atproto_sync_observations WHERE publisher_did='did:plc:shadow-invalid')`).
		Scan(&shadowFailures, &liveFailures, &liveObservations); err != nil {
		t.Fatalf("count quarantine state: %v", err)
	}
	if shadowFailures != 1 || liveFailures != 0 || liveObservations != 0 {
		t.Fatalf("quarantine counts = shadow:%d live-failures:%d live-observations:%d", shadowFailures, liveFailures, liveObservations)
	}
}

func TestPostgresV2ProjectorRollsBackEventsAndCursorTogether(t *testing.T) {
	tdb := testutil.NewTestDB(t)
	projector := NewPostgresV2Projector(tdb.DB, NewRecordFilter(NewFilterMetrics()), nil)
	ctx := context.Background()
	request := ProjectionRequest{Consumer: "rollback", Target: ProjectionTargetShadow, RebuildID: "rollback"}
	events := []jetstream.Event{
		{DID: "did:plc:rollback", Seq: 10, Kind: jetstream.KindCommit,
			Commit: &jetstream.Commit{Operation: jetstream.OpCreate, Collection: CollectionScene, Rkey: "scene", Rev: "1", Record: map[string]any{"name": "Rollback"}}},
		{DID: "did:plc:rollback", Seq: 11, Kind: jetstream.Kind("unknown")},
	}
	if _, err := projector.ApplyBatch(ctx, request, events, 11); err == nil {
		t.Fatal("ApplyBatch() succeeded, want rollback error")
	}
	cursor, err := projector.Cursor(ctx, request)
	if err != nil {
		t.Fatalf("Cursor(): %v", err)
	}
	if cursor != 0 {
		t.Fatalf("cursor = %d, want 0 after rollback", cursor)
	}
	var count int
	if err = tdb.DB.QueryRow(`SELECT COUNT(*) FROM jetstream_v2_shadow_records WHERE rebuild_id='rollback'`).Scan(&count); err != nil {
		t.Fatalf("count shadow records: %v", err)
	}
	if count != 0 {
		t.Fatalf("shadow records = %d, want 0 after rollback", count)
	}
}

func TestPostgresV2ProjectorProcessesIdentityAndSync(t *testing.T) {
	tdb := testutil.NewTestDB(t)
	projector := NewPostgresV2Projector(tdb.DB, NewRecordFilter(NewFilterMetrics()), nil)
	ctx := context.Background()
	request := ProjectionRequest{Consumer: "metadata", Target: ProjectionTargetActive}
	events := []jetstream.Event{
		{DID: "did:plc:meta", Seq: 30, Kind: jetstream.KindIdentity,
			Identity: &jetstream.Identity{DID: "did:plc:meta", Handle: "meta.example", Seq: 300}},
		{DID: "did:plc:meta", Seq: 31, Kind: jetstream.KindSync,
			Sync: &jetstream.Sync{DID: "did:plc:meta", Rev: "rev-sync", Seq: 301}},
		{DID: "did:plc:max", Seq: math.MaxUint64, Kind: jetstream.KindIdentity,
			Identity: &jetstream.Identity{DID: "did:plc:max", Handle: "max.example", Seq: 302}},
	}
	if _, err := projector.ApplyBatch(ctx, request, events, math.MaxUint64); err != nil {
		t.Fatalf("ApplyBatch(): %v", err)
	}
	var handle string
	if err := tdb.DB.QueryRow(`SELECT handle FROM jetstream_v2_identities WHERE did='did:plc:meta'`).Scan(&handle); err != nil || handle != "meta.example" {
		t.Fatalf("identity handle = %q, err = %v", handle, err)
	}
	var rev, status string
	if err := tdb.DB.QueryRow(`SELECT requested_rev,status FROM jetstream_v2_reconciliations WHERE did='did:plc:meta'`).Scan(&rev, &status); err != nil {
		t.Fatalf("read reconciliation: %v", err)
	}
	if rev != "rev-sync" || status != "pending" {
		t.Fatalf("reconciliation = (%q,%q), want (rev-sync,pending)", rev, status)
	}
	cursor, err := projector.Cursor(ctx, request)
	if err != nil || cursor != math.MaxUint64 {
		t.Fatalf("uint64 cursor = %d, err = %v", cursor, err)
	}
}

func TestPostgresV2ProjectorSuppressesActiveRecordsForInactiveAccount(t *testing.T) {
	tdb := testutil.NewTestDB(t)
	projector := NewPostgresV2Projector(tdb.DB, NewRecordFilter(NewFilterMetrics()), nil)
	ctx := context.Background()
	if _, err := tdb.DB.Exec(`INSERT INTO scenes
		(id,name,owner_did,coarse_geohash,visibility,palette,record_did,record_rkey)
		VALUES(gen_random_uuid(),'Suppressed Scene','did:plc:deleted','u4pruyd','public','{}','did:plc:deleted','scene')`); err != nil {
		t.Fatalf("insert active scene: %v", err)
	}
	event := jetstream.Event{
		DID: "did:plc:deleted", Seq: 90, Kind: jetstream.KindAccount,
		Account: &jetstream.Account{DID: "did:plc:deleted", Active: false, Status: "deleted", Seq: 900},
	}
	request := ProjectionRequest{Consumer: "active-delete", Target: ProjectionTargetActive}
	result, err := projector.ApplyBatch(ctx, request, []jetstream.Event{event}, 90)
	if err != nil {
		t.Fatalf("ApplyBatch(): %v", err)
	}
	if result.Deleted != 1 {
		t.Fatalf("deleted = %d, want 1", result.Deleted)
	}
	var suppressed bool
	if err = tdb.DB.QueryRow(`SELECT deleted_at IS NOT NULL FROM scenes
		WHERE record_did='did:plc:deleted' AND record_rkey='scene'`).Scan(&suppressed); err != nil {
		t.Fatalf("read scene suppression: %v", err)
	}
	if !suppressed {
		t.Fatal("inactive account scene remains visible")
	}
}

func TestPostgresV2ProjectorCommitsLegacyRecordAndCursorAtomically(t *testing.T) {
	tdb := testutil.NewTestDB(t)
	projector := NewPostgresV2Projector(tdb.DB, NewRecordFilter(NewFilterMetrics()), nil)
	ctx := context.Background()
	request := ProjectionRequest{Consumer: "active-record", Target: ProjectionTargetActive}
	event := jetstream.Event{
		DID: "did:plc:active", Seq: 120, Kind: jetstream.KindCommit,
		Commit: &jetstream.Commit{
			Operation:  jetstream.OpCreate,
			Collection: CollectionScene,
			Rkey:       "warehouse",
			Rev:        "rev-120",
			CID:        "cid-120",
			Record:     map[string]any{"name": "Warehouse Scene", "tags": []any{"industrial"}},
		},
	}
	if _, err := projector.ApplyBatch(ctx, request, []jetstream.Event{event}, 120); err != nil {
		t.Fatalf("ApplyBatch(): %v", err)
	}
	var name, firstTag string
	if err := tdb.DB.QueryRow(`SELECT name,tags[1] FROM scenes WHERE record_did='did:plc:active' AND record_rkey='warehouse'`).Scan(&name, &firstTag); err != nil {
		t.Fatalf("read projected scene: %v", err)
	}
	if name != "Warehouse Scene" || firstTag != "industrial" {
		t.Fatalf("scene = (%q,%q), want (Warehouse Scene,industrial)", name, firstTag)
	}
	cursor, err := projector.Cursor(ctx, request)
	if err != nil || cursor != 120 {
		t.Fatalf("cursor = %d, err = %v, want 120", cursor, err)
	}
	if _, err := projector.ApplyBatch(ctx, request, []jetstream.Event{event}, 120); err != nil {
		t.Fatalf("replay ApplyBatch(): %v", err)
	}
	overlap := []jetstream.Event{event, {
		DID: "did:plc:active", Seq: 121, Kind: jetstream.KindIdentity,
		Identity: &jetstream.Identity{DID: "did:plc:active", Handle: "active.example", Seq: 1210},
	}}
	result, err := projector.ApplyBatch(ctx, request, overlap, 121)
	if err != nil {
		t.Fatalf("overlapping ApplyBatch(): %v", err)
	}
	if result.Skipped != 1 || result.Processed != 1 {
		t.Fatalf("overlap result = %+v, want one skipped and one processed", result)
	}
	var count int
	if err := tdb.DB.QueryRow(`SELECT COUNT(*) FROM scenes WHERE record_did='did:plc:active' AND record_rkey='warehouse'`).Scan(&count); err != nil {
		t.Fatalf("count projected scenes: %v", err)
	}
	if count != 1 {
		t.Fatalf("scene count = %d, want 1 after replay", count)
	}
	if cursor, err = projector.Cursor(ctx, request); err != nil || cursor != 121 {
		t.Fatalf("overlap cursor = %d, err = %v, want 121", cursor, err)
	}
}
