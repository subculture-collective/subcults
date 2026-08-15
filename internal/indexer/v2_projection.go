// Package indexer folds Jetstream v2 batches into Subcults projections.
package indexer

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"math"
	"strconv"
	"strings"
	"time"

	jetstream "github.com/bluesky-social/jetstream"
	"github.com/onnwee/subcults/internal/atprotocol"
)

// ProjectionTarget identifies the database projection a v2 batch updates.
type ProjectionTarget string

const (
	// ProjectionTargetActive updates the product-facing projection tables.
	ProjectionTargetActive ProjectionTarget = "active"
	// ProjectionTargetShadow folds records into an isolated replay projection.
	ProjectionTargetShadow ProjectionTarget = "shadow"
)

// ProjectionRequest names the durable cursor and destination for a batch.
type ProjectionRequest struct {
	Consumer  string
	Target    ProjectionTarget
	RebuildID string
	DryRun    bool
}

// BatchResult summarizes the outcome of a committed Jetstream batch.
type BatchResult struct {
	Processed   int64
	Skipped     int64
	Quarantined int64
	Deleted     int64
}

// V2Projector atomically folds event batches and their v2 cursor.
type V2Projector interface {
	Cursor(ctx context.Context, request ProjectionRequest) (uint64, error)
	ApplyBatch(ctx context.Context, request ProjectionRequest, events []jetstream.Event, cursor uint64) (BatchResult, error)
}

// PostgresV2Projector persists Jetstream v2 state in PostgreSQL.
type PostgresV2Projector struct {
	db     *sql.DB
	repo   *PostgresRecordRepository
	filter *RecordFilter
	logger *slog.Logger
}

// NewPostgresV2Projector creates an atomic Jetstream v2 projector.
func NewPostgresV2Projector(db *sql.DB, filter *RecordFilter, logger *slog.Logger) *PostgresV2Projector {
	if logger == nil {
		logger = slog.Default()
	}
	return &PostgresV2Projector{
		db:     db,
		repo:   NewPostgresRecordRepository(db, logger),
		filter: filter,
		logger: logger,
	}
}

// Cursor returns the last v2 sequence committed for a projection namespace.
func (p *PostgresV2Projector) Cursor(ctx context.Context, request ProjectionRequest) (uint64, error) {
	if err := validateProjectionRequest(request); err != nil {
		return 0, err
	}
	var raw string
	err := p.db.QueryRowContext(ctx, `SELECT cursor::text FROM jetstream_v2_cursors
		WHERE consumer=$1 AND target=$2 AND rebuild_id=$3`, request.Consumer, request.Target, request.RebuildID).Scan(&raw)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, nil
	}
	if err != nil {
		return 0, fmt.Errorf("read Jetstream v2 cursor: %w", err)
	}
	cursor, err := strconv.ParseUint(raw, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("parse Jetstream v2 cursor %q: %w", raw, err)
	}
	return cursor, nil
}

// ApplyBatch folds all events and advances the cursor in one transaction.
func (p *PostgresV2Projector) ApplyBatch(ctx context.Context, request ProjectionRequest, events []jetstream.Event, cursor uint64) (BatchResult, error) {
	if err := validateProjectionRequest(request); err != nil {
		return BatchResult{}, err
	}
	if err := validateBatch(events, cursor); err != nil {
		return BatchResult{}, err
	}
	if request.DryRun {
		return p.validateOnly(events)
	}

	tx, err := p.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		return BatchResult{}, fmt.Errorf("begin Jetstream v2 batch: %w", err)
	}
	defer func() {
		if rollbackErr := tx.Rollback(); rollbackErr != nil && !errors.Is(rollbackErr, sql.ErrTxDone) {
			p.logger.Warn("failed to roll back Jetstream v2 batch", slog.String("error", rollbackErr.Error()))
		}
	}()

	if _, err = tx.ExecContext(ctx, `INSERT INTO jetstream_v2_cursors(consumer,target,rebuild_id,cursor)
		VALUES($1,$2,$3,0) ON CONFLICT DO NOTHING`, request.Consumer, request.Target, request.RebuildID); err != nil {
		return BatchResult{}, fmt.Errorf("initialize Jetstream v2 cursor: %w", err)
	}
	var currentRaw string
	if err = tx.QueryRowContext(ctx, `SELECT cursor::text FROM jetstream_v2_cursors
		WHERE consumer=$1 AND target=$2 AND rebuild_id=$3 FOR UPDATE`, request.Consumer, request.Target, request.RebuildID).Scan(&currentRaw); err != nil {
		return BatchResult{}, fmt.Errorf("lock Jetstream v2 cursor: %w", err)
	}
	current, err := strconv.ParseUint(currentRaw, 10, 64)
	if err != nil {
		return BatchResult{}, fmt.Errorf("parse locked Jetstream v2 cursor %q: %w", currentRaw, err)
	}
	if cursor <= current {
		return BatchResult{Skipped: int64(len(events))}, nil
	}

	firstUncommitted := 0
	for firstUncommitted < len(events) && events[firstUncommitted].Seq <= current {
		firstUncommitted++
	}
	result := BatchResult{Skipped: int64(firstUncommitted)}
	for i := firstUncommitted; i < len(events); i++ {
		var eventResult BatchResult
		if request.Target == ProjectionTargetShadow {
			eventResult, err = p.applyShadowEvent(ctx, tx, request.RebuildID, &events[i])
		} else {
			eventResult, err = p.applyActiveEvent(ctx, tx, &events[i])
		}
		if err != nil {
			return BatchResult{}, fmt.Errorf("apply Jetstream v2 event seq=%d kind=%s: %w", events[i].Seq, events[i].Kind, err)
		}
		result.add(eventResult)
	}

	if _, err = tx.ExecContext(ctx, `UPDATE jetstream_v2_cursors
		SET cursor=$4,updated_at=NOW()
		WHERE consumer=$1 AND target=$2 AND rebuild_id=$3`, request.Consumer, request.Target, request.RebuildID, strconv.FormatUint(cursor, 10)); err != nil {
		return BatchResult{}, fmt.Errorf("advance Jetstream v2 cursor: %w", err)
	}
	if err = tx.Commit(); err != nil {
		return BatchResult{}, fmt.Errorf("commit Jetstream v2 batch and cursor: %w", err)
	}
	return result, nil
}

func validateProjectionRequest(request ProjectionRequest) error {
	if strings.TrimSpace(request.Consumer) == "" {
		return errors.New("jetstream v2 consumer name is required")
	}
	if request.Target != ProjectionTargetActive && request.Target != ProjectionTargetShadow {
		return fmt.Errorf("unsupported Jetstream v2 projection target %q", request.Target)
	}
	if request.Target == ProjectionTargetShadow && strings.TrimSpace(request.RebuildID) == "" {
		return errors.New("rebuild ID is required for a shadow projection")
	}
	if request.Target == ProjectionTargetActive && request.RebuildID != "" {
		return errors.New("rebuild ID must be empty for the active projection")
	}
	return nil
}

func validateBatch(events []jetstream.Event, cursor uint64) error {
	if len(events) == 0 {
		if cursor != 0 {
			return errors.New("non-zero cursor supplied for an empty Jetstream batch")
		}
		return nil
	}
	var previous uint64
	for i := range events {
		if events[i].Seq == 0 {
			return fmt.Errorf("event %d has zero v2 sequence", i)
		}
		if i > 0 && events[i].Seq <= previous {
			return fmt.Errorf("event sequences are not strictly ordered: %d before %d", previous, events[i].Seq)
		}
		if events[i].Seq > cursor {
			return fmt.Errorf("event sequence %d exceeds batch cursor %d", events[i].Seq, cursor)
		}
		previous = events[i].Seq
	}
	if previous != cursor {
		return fmt.Errorf("batch cursor %d does not equal final event sequence %d", cursor, previous)
	}
	return nil
}

func (r *BatchResult) add(other BatchResult) {
	r.Processed += other.Processed
	r.Skipped += other.Skipped
	r.Quarantined += other.Quarantined
	r.Deleted += other.Deleted
}

func (p *PostgresV2Projector) validateOnly(events []jetstream.Event) (BatchResult, error) {
	var result BatchResult
	for i := range events {
		event := &events[i]
		switch event.Kind {
		case jetstream.KindIdentity:
			if event.Identity == nil {
				return BatchResult{}, fmt.Errorf("identity event seq=%d has no identity payload", event.Seq)
			}
			result.Processed++
			continue
		case jetstream.KindAccount:
			if event.Account == nil {
				return BatchResult{}, fmt.Errorf("account event seq=%d has no account payload", event.Seq)
			}
			result.Processed++
			continue
		case jetstream.KindSync:
			if event.Sync == nil {
				return BatchResult{}, fmt.Errorf("sync event seq=%d has no sync payload", event.Seq)
			}
			result.Processed++
			continue
		case jetstream.KindCommit:
			if event.Commit == nil {
				return BatchResult{}, fmt.Errorf("commit event seq=%d has no commit payload", event.Seq)
			}
		default:
			return BatchResult{}, fmt.Errorf("unknown Jetstream event kind %q at seq=%d", event.Kind, event.Seq)
		}
		if event.Commit.Operation != jetstream.OpCreate && event.Commit.Operation != jetstream.OpUpdate && event.Commit.Operation != jetstream.OpDelete {
			return BatchResult{}, fmt.Errorf("unsupported commit operation %q at seq=%d", event.Commit.Operation, event.Seq)
		}
		if !atprotocol.IsCanonicalCollection(event.Commit.Collection) && !atprotocol.IsLegacyCollection(event.Commit.Collection) {
			result.Skipped++
			continue
		}
		if event.Commit.Operation == jetstream.OpDelete {
			result.Deleted++
			result.Processed++
			continue
		}
		payload, err := json.Marshal(event.Commit.Record)
		if err != nil {
			return BatchResult{}, fmt.Errorf("marshal record seq=%d: %w", event.Seq, err)
		}
		filtered := p.filter.Filter(event.Commit.Collection, payload)
		if !filtered.Matched {
			result.Skipped++
			continue
		}
		if !filtered.Valid {
			result.Quarantined++
			continue
		}
		result.Processed++
	}
	return result, nil
}

func (p *PostgresV2Projector) applyActiveEvent(ctx context.Context, tx *sql.Tx, event *jetstream.Event) (BatchResult, error) {
	switch event.Kind {
	case jetstream.KindCommit:
		return p.applyActiveCommit(ctx, tx, event)
	case jetstream.KindIdentity:
		if event.Identity == nil {
			return BatchResult{}, errors.New("identity event has no identity payload")
		}
		return BatchResult{Processed: 1}, p.upsertIdentity(ctx, tx, event)
	case jetstream.KindAccount:
		if event.Account == nil {
			return BatchResult{}, errors.New("account event has no account payload")
		}
		deleted, err := p.upsertAccount(ctx, tx, event)
		return BatchResult{Processed: 1, Deleted: deleted}, err
	case jetstream.KindSync:
		if event.Sync == nil {
			return BatchResult{}, errors.New("sync event has no sync payload")
		}
		return BatchResult{Processed: 1}, p.enqueueReconciliation(ctx, tx, event.DID, event.Sync.Rev, "sync", event.Sync.Seq, event.Seq)
	default:
		return BatchResult{}, fmt.Errorf("unknown Jetstream event kind %q", event.Kind)
	}
}

func (p *PostgresV2Projector) applyActiveCommit(ctx context.Context, tx *sql.Tx, event *jetstream.Event) (BatchResult, error) {
	if event.Commit == nil {
		return BatchResult{}, errors.New("commit event has no commit payload")
	}
	commit := event.Commit
	if commit.Operation != jetstream.OpCreate && commit.Operation != jetstream.OpUpdate && commit.Operation != jetstream.OpDelete {
		return BatchResult{}, fmt.Errorf("unsupported commit operation %q", commit.Operation)
	}
	if !atprotocol.IsCanonicalCollection(commit.Collection) && !atprotocol.IsLegacyCollection(commit.Collection) {
		return BatchResult{Skipped: 1}, nil
	}
	if atprotocol.IsCanonicalCollection(commit.Collection) {
		return p.applyCanonicalCommit(ctx, tx, event)
	}
	if commit.Operation == jetstream.OpDelete {
		if err := p.deleteLegacyRecordTx(ctx, tx, event.DID, commit.Collection, commit.Rkey); err != nil {
			return BatchResult{}, err
		}
		if _, err := p.insertObservation(ctx, tx, event, "legacy_observed", ""); err != nil {
			return BatchResult{}, err
		}
		return BatchResult{Processed: 1, Deleted: 1}, nil
	}

	payload, err := json.Marshal(commit.Record)
	if err != nil {
		return BatchResult{}, fmt.Errorf("marshal commit record: %w", err)
	}
	filtered := p.filter.Filter(commit.Collection, payload)
	filtered.DID = event.DID
	filtered.RKey = commit.Rkey
	filtered.Rev = commit.Rev
	filtered.Operation = string(commit.Operation)
	if !filtered.Matched {
		return BatchResult{Skipped: 1}, nil
	}
	if !filtered.Valid {
		if err := p.quarantineTx(ctx, tx, event, "record_validation", filtered.Error.Error(), payload); err != nil {
			return BatchResult{}, err
		}
		return BatchResult{Quarantined: 1}, nil
	}
	recordID, _, duplicate, err := p.upsertLegacyRecordTx(ctx, tx, &filtered)
	if err != nil {
		return BatchResult{}, err
	}
	digest := digestPayload(payload)
	if _, err = p.insertObservation(ctx, tx, event, "legacy_observed", digest); err != nil {
		return BatchResult{}, err
	}
	if duplicate || recordID == "" {
		return BatchResult{Skipped: 1}, nil
	}
	return BatchResult{Processed: 1}, nil
}

func (p *PostgresV2Projector) applyCanonicalCommit(ctx context.Context, tx *sql.Tx, event *jetstream.Event) (BatchResult, error) {
	commit := event.Commit
	atURI := recordATURI(event.DID, commit.Collection, commit.Rkey)
	var payload []byte
	var err error
	if commit.Operation != jetstream.OpDelete {
		payload, err = json.Marshal(commit.Record)
		if err != nil {
			return BatchResult{}, fmt.Errorf("marshal canonical record: %w", err)
		}
		if err = atprotocol.ValidatePublicRecord(commit.Collection, payload); err != nil {
			if err = p.quarantineTx(ctx, tx, event, "record_validation", err.Error(), payload); err != nil {
				return BatchResult{}, err
			}
			return BatchResult{Quarantined: 1}, nil
		}
	}

	var entityType, entityID, expectedCID string
	err = tx.QueryRowContext(ctx, `SELECT entity_type,entity_id::text,COALESCE(cid,'')
		FROM atproto_record_mappings WHERE at_uri=$1 FOR UPDATE`, atURI).Scan(&entityType, &entityID, &expectedCID)
	if errors.Is(err, sql.ErrNoRows) {
		if err = p.quarantineTx(ctx, tx, event, "unmapped_record", "canonical record has no local entity mapping", payload); err != nil {
			return BatchResult{}, err
		}
		return BatchResult{Quarantined: 1}, nil
	}
	if err != nil {
		return BatchResult{}, fmt.Errorf("lock canonical record mapping: %w", err)
	}
	if commit.Operation != jetstream.OpDelete && expectedCID != "" && expectedCID != commit.CID {
		if err = p.quarantineTx(ctx, tx, event, "cid_conflict", "observed CID differs from the pending local publication", payload); err != nil {
			return BatchResult{}, err
		}
		return BatchResult{Quarantined: 1}, nil
	}
	outcome := "projected"
	deleted := commit.Operation == jetstream.OpDelete
	if deleted {
		outcome = "deleted"
	}
	inserted, err := p.insertObservation(ctx, tx, event, outcome, digestPayload(payload))
	if err != nil {
		return BatchResult{}, err
	}
	if !inserted {
		return BatchResult{Skipped: 1}, nil
	}
	if deleted {
		_, err = tx.ExecContext(ctx, `UPDATE atproto_record_mappings
			SET projection_status='deleted',last_seen_at=NOW(),updated_at=NOW() WHERE at_uri=$1`, atURI)
	} else {
		_, err = tx.ExecContext(ctx, `UPDATE atproto_record_mappings
			SET cid=$2,projection_status='projected',last_seen_at=NOW(),updated_at=NOW() WHERE at_uri=$1`, atURI, commit.CID)
	}
	if err != nil {
		return BatchResult{}, fmt.Errorf("update canonical record mapping: %w", err)
	}
	if err = setEntityPublicationStateTx(ctx, tx, entityType, entityID, deleted); err != nil {
		return BatchResult{}, err
	}
	result := BatchResult{Processed: 1}
	if deleted {
		result.Deleted = 1
	}
	return result, nil
}

func (p *PostgresV2Projector) applyShadowEvent(ctx context.Context, tx *sql.Tx, rebuildID string, event *jetstream.Event) (BatchResult, error) {
	switch event.Kind {
	case jetstream.KindCommit:
		if event.Commit == nil {
			return BatchResult{}, errors.New("commit event has no commit payload")
		}
		commit := event.Commit
		if commit.Operation != jetstream.OpCreate && commit.Operation != jetstream.OpUpdate && commit.Operation != jetstream.OpDelete {
			return BatchResult{}, fmt.Errorf("unsupported commit operation %q", commit.Operation)
		}
		if !atprotocol.IsCanonicalCollection(commit.Collection) && !atprotocol.IsLegacyCollection(commit.Collection) {
			return BatchResult{Skipped: 1}, nil
		}
		var payload []byte
		var err error
		if commit.Operation != jetstream.OpDelete {
			payload, err = json.Marshal(commit.Record)
			if err != nil {
				return BatchResult{}, fmt.Errorf("marshal shadow record: %w", err)
			}
			filtered := p.filter.Filter(commit.Collection, payload)
			if !filtered.Valid {
				if err = p.quarantineShadowTx(ctx, tx, rebuildID, event, "record_validation", filtered.Error.Error(), payload); err != nil {
					return BatchResult{}, err
				}
				return BatchResult{Quarantined: 1}, nil
			}
		}
		deleted := commit.Operation == jetstream.OpDelete
		_, err = tx.ExecContext(ctx, `INSERT INTO jetstream_v2_shadow_records
			(rebuild_id,at_uri,did,collection,rkey,rev,cid,record,deleted,suppressed,jetstream_seq,time_us)
			VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,
				COALESCE((SELECT NOT active FROM jetstream_v2_shadow_accounts WHERE rebuild_id=$1 AND did=$3),FALSE),$10,$11)
			ON CONFLICT(rebuild_id,at_uri) DO UPDATE SET
			rev=EXCLUDED.rev,cid=EXCLUDED.cid,record=EXCLUDED.record,deleted=EXCLUDED.deleted,
			suppressed=EXCLUDED.suppressed,
			jetstream_seq=EXCLUDED.jetstream_seq,time_us=EXCLUDED.time_us,updated_at=NOW()
			WHERE jetstream_v2_shadow_records.jetstream_seq <= EXCLUDED.jetstream_seq`,
			rebuildID, recordATURI(event.DID, commit.Collection, commit.Rkey), event.DID, commit.Collection,
			commit.Rkey, commit.Rev, commit.CID, nullJSON(payload), deleted, strconv.FormatUint(event.Seq, 10), event.TimeUS)
		if err != nil {
			return BatchResult{}, fmt.Errorf("fold shadow record: %w", err)
		}
		result := BatchResult{Processed: 1}
		if deleted {
			result.Deleted = 1
		}
		return result, nil
	case jetstream.KindIdentity:
		if event.Identity == nil {
			return BatchResult{}, errors.New("identity event has no identity payload")
		}
		return BatchResult{Processed: 1}, p.upsertShadowIdentity(ctx, tx, rebuildID, event)
	case jetstream.KindAccount:
		if event.Account == nil {
			return BatchResult{}, errors.New("account event has no account payload")
		}
		deleted, err := p.upsertShadowAccount(ctx, tx, rebuildID, event)
		return BatchResult{Processed: 1, Deleted: deleted}, err
	case jetstream.KindSync:
		if event.Sync == nil {
			return BatchResult{}, errors.New("sync event has no sync payload")
		}
		return BatchResult{Processed: 1}, p.enqueueShadowReconciliation(ctx, tx, rebuildID, event.DID, event.Sync.Rev, "sync", event.Sync.Seq, event.Seq)
	default:
		return BatchResult{}, fmt.Errorf("unknown Jetstream event kind %q", event.Kind)
	}
}

func (p *PostgresV2Projector) upsertLegacyRecordTx(ctx context.Context, tx *sql.Tx, record *FilterResult) (string, bool, bool, error) {
	idempotencyKey := generateIdempotencyKey(record.DID, record.Collection, record.RKey, record.Rev)
	var exists bool
	if err := tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM ingestion_idempotency WHERE idempotency_key=$1)`, idempotencyKey).Scan(&exists); err != nil {
		return "", false, false, fmt.Errorf("check record idempotency: %w", err)
	}
	if exists {
		return "", false, true, nil
	}
	var recordID string
	var isNew bool
	var err error
	switch record.Collection {
	case CollectionScene:
		recordID, isNew, err = p.repo.upsertScene(ctx, tx, record)
	case CollectionEvent:
		recordID, isNew, err = p.repo.upsertEvent(ctx, tx, record)
	case CollectionPost:
		recordID, isNew, err = p.repo.upsertPost(ctx, tx, record)
	case CollectionAlliance:
		recordID, isNew, err = p.repo.upsertAlliance(ctx, tx, record)
	default:
		err = fmt.Errorf("unsupported legacy collection %q", record.Collection)
	}
	if err != nil {
		return "", false, false, err
	}
	if _, err = tx.ExecContext(ctx, `INSERT INTO ingestion_idempotency
		(idempotency_key,did,collection,rkey,rev,record_id,created_at)
		VALUES($1,$2,$3,$4,$5,$6,NOW())`, idempotencyKey, record.DID, record.Collection, record.RKey, record.Rev, recordID); err != nil {
		return "", false, false, fmt.Errorf("store record idempotency: %w", err)
	}
	return recordID, isNew, false, nil
}

func (p *PostgresV2Projector) deleteLegacyRecordTx(ctx context.Context, tx *sql.Tx, did, collection, rkey string) error {
	queries := map[string]string{
		CollectionScene:    `UPDATE scenes SET deleted_at=NOW(),updated_at=NOW() WHERE record_did=$1 AND record_rkey=$2 AND deleted_at IS NULL`,
		CollectionEvent:    `UPDATE events SET deleted_at=NOW(),updated_at=NOW() WHERE record_did=$1 AND record_rkey=$2 AND deleted_at IS NULL`,
		CollectionPost:     `UPDATE posts SET deleted_at=NOW(),updated_at=NOW() WHERE record_did=$1 AND record_rkey=$2 AND deleted_at IS NULL`,
		CollectionAlliance: `UPDATE alliances SET deleted_at=NOW(),updated_at=NOW() WHERE record_did=$1 AND record_rkey=$2 AND deleted_at IS NULL`,
	}
	query := queries[collection]
	if query == "" {
		return fmt.Errorf("unsupported legacy collection for delete %q", collection)
	}
	if _, err := tx.ExecContext(ctx, query, did, rkey); err != nil {
		return fmt.Errorf("soft-delete %s record: %w", collection, err)
	}
	return nil
}

func (p *PostgresV2Projector) upsertIdentity(ctx context.Context, tx *sql.Tx, event *jetstream.Event) error {
	identity := event.Identity
	did := event.DID
	if identity.DID != "" {
		did = identity.DID
	}
	_, err := tx.ExecContext(ctx, `INSERT INTO jetstream_v2_identities
		(did,handle,relay_seq,event_time,jetstream_seq) VALUES($1,$2,$3,$4,$5)
		ON CONFLICT(did) DO UPDATE SET handle=EXCLUDED.handle,relay_seq=EXCLUDED.relay_seq,
		event_time=EXCLUDED.event_time,jetstream_seq=EXCLUDED.jetstream_seq,updated_at=NOW()
		WHERE jetstream_v2_identities.jetstream_seq <= EXCLUDED.jetstream_seq`,
		did, identity.Handle, identity.Seq, parseEventTime(identity.Time), strconv.FormatUint(event.Seq, 10))
	if err != nil {
		return fmt.Errorf("upsert Jetstream identity: %w", err)
	}
	return nil
}

func (p *PostgresV2Projector) upsertShadowIdentity(ctx context.Context, tx *sql.Tx, rebuildID string, event *jetstream.Event) error {
	identity := event.Identity
	did := event.DID
	if identity.DID != "" {
		did = identity.DID
	}
	_, err := tx.ExecContext(ctx, `INSERT INTO jetstream_v2_shadow_identities
		(rebuild_id,did,handle,relay_seq,event_time,jetstream_seq) VALUES($1,$2,$3,$4,$5,$6)
		ON CONFLICT(rebuild_id,did) DO UPDATE SET handle=EXCLUDED.handle,relay_seq=EXCLUDED.relay_seq,
		event_time=EXCLUDED.event_time,jetstream_seq=EXCLUDED.jetstream_seq,updated_at=NOW()
		WHERE jetstream_v2_shadow_identities.jetstream_seq <= EXCLUDED.jetstream_seq`,
		rebuildID, did, identity.Handle, identity.Seq, parseEventTime(identity.Time), strconv.FormatUint(event.Seq, 10))
	if err != nil {
		return fmt.Errorf("upsert shadow Jetstream identity: %w", err)
	}
	return nil
}

func (p *PostgresV2Projector) upsertAccount(ctx context.Context, tx *sql.Tx, event *jetstream.Event) (int64, error) {
	account := event.Account
	did := event.DID
	if account.DID != "" {
		did = account.DID
	}
	var previousActive bool
	hadPrevious := true
	if err := tx.QueryRowContext(ctx, `SELECT active FROM jetstream_v2_accounts WHERE did=$1`, did).Scan(&previousActive); errors.Is(err, sql.ErrNoRows) {
		hadPrevious = false
	} else if err != nil {
		return 0, fmt.Errorf("read prior Jetstream account state: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO jetstream_v2_accounts
		(did,active,status,relay_seq,event_time,jetstream_seq) VALUES($1,$2,$3,$4,$5,$6)
		ON CONFLICT(did) DO UPDATE SET active=EXCLUDED.active,status=EXCLUDED.status,
		relay_seq=EXCLUDED.relay_seq,event_time=EXCLUDED.event_time,
		jetstream_seq=EXCLUDED.jetstream_seq,updated_at=NOW()
		WHERE jetstream_v2_accounts.jetstream_seq <= EXCLUDED.jetstream_seq`,
		did, account.Active, account.Status, account.Seq, parseEventTime(account.Time), strconv.FormatUint(event.Seq, 10)); err != nil {
		return 0, fmt.Errorf("upsert Jetstream account: %w", err)
	}
	if account.Active && hadPrevious && !previousActive {
		return 0, p.enqueueReconciliation(ctx, tx, did, "", "account_reactivated", account.Seq, event.Seq)
	}
	if account.Active {
		return 0, nil
	}
	return p.suppressActiveAccount(ctx, tx, did)
}

func (p *PostgresV2Projector) upsertShadowAccount(ctx context.Context, tx *sql.Tx, rebuildID string, event *jetstream.Event) (int64, error) {
	account := event.Account
	did := event.DID
	if account.DID != "" {
		did = account.DID
	}
	var previousActive bool
	hadPrevious := true
	if err := tx.QueryRowContext(ctx, `SELECT active FROM jetstream_v2_shadow_accounts
		WHERE rebuild_id=$1 AND did=$2`, rebuildID, did).Scan(&previousActive); errors.Is(err, sql.ErrNoRows) {
		hadPrevious = false
	} else if err != nil {
		return 0, fmt.Errorf("read prior shadow Jetstream account state: %w", err)
	}
	result, err := tx.ExecContext(ctx, `INSERT INTO jetstream_v2_shadow_accounts
		(rebuild_id,did,active,status,relay_seq,event_time,jetstream_seq) VALUES($1,$2,$3,$4,$5,$6,$7)
		ON CONFLICT(rebuild_id,did) DO UPDATE SET active=EXCLUDED.active,status=EXCLUDED.status,
		relay_seq=EXCLUDED.relay_seq,event_time=EXCLUDED.event_time,
		jetstream_seq=EXCLUDED.jetstream_seq,updated_at=NOW()
		WHERE jetstream_v2_shadow_accounts.jetstream_seq <= EXCLUDED.jetstream_seq`,
		rebuildID, did, account.Active, account.Status, account.Seq, parseEventTime(account.Time), strconv.FormatUint(event.Seq, 10))
	if err != nil {
		return 0, fmt.Errorf("upsert shadow Jetstream account: %w", err)
	}
	changed, _ := result.RowsAffected()
	if changed == 0 {
		return 0, nil
	}
	if account.Active && hadPrevious && !previousActive {
		if err = p.enqueueShadowReconciliation(ctx, tx, rebuildID, did, "", "account_reactivated", account.Seq, event.Seq); err != nil {
			return 0, err
		}
	}
	result, err = tx.ExecContext(ctx, `UPDATE jetstream_v2_shadow_records SET suppressed=$3,updated_at=NOW()
		WHERE rebuild_id=$1 AND did=$2 AND suppressed<>$3`, rebuildID, did, !account.Active)
	if err != nil {
		return 0, fmt.Errorf("update shadow account suppression: %w", err)
	}
	if account.Active {
		return 0, nil
	}
	rows, _ := result.RowsAffected()
	return rows, nil
}

func (p *PostgresV2Projector) suppressActiveAccount(ctx context.Context, tx *sql.Tx, did string) (int64, error) {
	queries := []string{
		`UPDATE posts SET deleted_at=NOW(),updated_at=NOW() WHERE record_did=$1 AND deleted_at IS NULL`,
		`UPDATE events SET deleted_at=NOW(),updated_at=NOW() WHERE record_did=$1 AND deleted_at IS NULL`,
		`UPDATE alliances SET deleted_at=NOW(),updated_at=NOW() WHERE record_did=$1 AND deleted_at IS NULL`,
		`UPDATE scenes SET deleted_at=NOW(),updated_at=NOW() WHERE record_did=$1 AND deleted_at IS NULL`,
	}
	var deleted int64
	for _, query := range queries {
		result, err := tx.ExecContext(ctx, query, did)
		if err != nil {
			return 0, fmt.Errorf("suppress account records: %w", err)
		}
		rows, _ := result.RowsAffected()
		deleted += rows
	}
	rows, err := tx.QueryContext(ctx, `UPDATE atproto_record_mappings SET projection_status='deleted',
		last_seen_at=NOW(),updated_at=NOW() WHERE publisher_did=$1 AND projection_status <> 'deleted'
		RETURNING entity_type,entity_id::text`, did)
	if err != nil {
		return 0, fmt.Errorf("suppress canonical account mappings: %w", err)
	}
	type mappedEntity struct{ entityType, entityID string }
	var entities []mappedEntity
	for rows.Next() {
		var entity mappedEntity
		if err = rows.Scan(&entity.entityType, &entity.entityID); err != nil {
			if closeErr := rows.Close(); closeErr != nil {
				p.logger.Warn("failed to close canonical mapping rows after scan failure", slog.String("error", closeErr.Error()))
			}
			return 0, fmt.Errorf("scan suppressed canonical mapping: %w", err)
		}
		entities = append(entities, entity)
	}
	if err = rows.Err(); err != nil {
		if closeErr := rows.Close(); closeErr != nil {
			p.logger.Warn("failed to close canonical mapping rows after iteration failure", slog.String("error", closeErr.Error()))
		}
		return 0, fmt.Errorf("iterate suppressed canonical mappings: %w", err)
	}
	if err = rows.Close(); err != nil {
		return 0, fmt.Errorf("close suppressed canonical mappings: %w", err)
	}
	for _, entity := range entities {
		if err = setEntityPublicationStateTx(ctx, tx, entity.entityType, entity.entityID, true); err != nil {
			return 0, err
		}
		deleted++
	}
	return deleted, nil
}

func (p *PostgresV2Projector) enqueueReconciliation(ctx context.Context, tx *sql.Tx, did, rev, reason string, relaySeq int64, jetstreamSeq uint64) error {
	_, err := tx.ExecContext(ctx, `INSERT INTO jetstream_v2_reconciliations
		(did,requested_rev,reason,relay_seq,jetstream_seq,status,requested_at,updated_at)
		VALUES($1,$2,$3,$4,$5,'pending',NOW(),NOW())
		ON CONFLICT(did) DO UPDATE SET requested_rev=EXCLUDED.requested_rev,reason=EXCLUDED.reason,
		relay_seq=EXCLUDED.relay_seq,jetstream_seq=EXCLUDED.jetstream_seq,status='pending',updated_at=NOW()
		WHERE jetstream_v2_reconciliations.jetstream_seq <= EXCLUDED.jetstream_seq`,
		did, rev, reason, relaySeq, strconv.FormatUint(jetstreamSeq, 10))
	if err != nil {
		return fmt.Errorf("enqueue targeted reconciliation: %w", err)
	}
	if _, err = tx.ExecContext(ctx, `SELECT pg_notify('jetstream_reconciliation',$1)`, did); err != nil {
		return fmt.Errorf("notify targeted reconciliation: %w", err)
	}
	return nil
}

func (p *PostgresV2Projector) enqueueShadowReconciliation(ctx context.Context, tx *sql.Tx, rebuildID, did, rev, reason string, relaySeq int64, jetstreamSeq uint64) error {
	_, err := tx.ExecContext(ctx, `INSERT INTO jetstream_v2_shadow_reconciliations
		(rebuild_id,did,requested_rev,reason,relay_seq,jetstream_seq,updated_at)
		VALUES($1,$2,$3,$4,$5,$6,NOW())
		ON CONFLICT(rebuild_id,did) DO UPDATE SET requested_rev=EXCLUDED.requested_rev,reason=EXCLUDED.reason,
		relay_seq=EXCLUDED.relay_seq,jetstream_seq=EXCLUDED.jetstream_seq,updated_at=NOW()
		WHERE jetstream_v2_shadow_reconciliations.jetstream_seq <= EXCLUDED.jetstream_seq`,
		rebuildID, did, rev, reason, relaySeq, strconv.FormatUint(jetstreamSeq, 10))
	if err != nil {
		return fmt.Errorf("enqueue shadow targeted reconciliation: %w", err)
	}
	return nil
}

func (p *PostgresV2Projector) insertObservation(ctx context.Context, tx *sql.Tx, event *jetstream.Event, outcome, digest string) (bool, error) {
	commit := event.Commit
	var sourceEventID any
	if event.Seq <= math.MaxInt64 {
		sourceEventID = int64(event.Seq)
	}
	result, err := tx.ExecContext(ctx, `INSERT INTO atproto_sync_observations
		(source,source_event_id,publisher_did,collection,rkey,at_uri,cid,revision,action,payload_digest,projection_outcome,observed_at)
		VALUES('jetstream',$1,$2,$3,$4,$5,NULLIF($6,''),$7,$8,NULLIF($9,''),$10,NOW())
		ON CONFLICT DO NOTHING`, sourceEventID, event.DID, commit.Collection, commit.Rkey,
		recordATURI(event.DID, commit.Collection, commit.Rkey), commit.CID, commit.Rev, commit.Operation, digest, outcome)
	if err != nil {
		return false, fmt.Errorf("insert Jetstream sync observation: %w", err)
	}
	rows, _ := result.RowsAffected()
	return rows == 1, nil
}

func (p *PostgresV2Projector) quarantineTx(ctx context.Context, tx *sql.Tx, event *jetstream.Event, reason, detail string, payload []byte) error {
	inserted, err := p.insertObservation(ctx, tx, event, "quarantined", digestPayload(payload))
	if err != nil || !inserted {
		return err
	}
	commit := event.Commit
	_, err = tx.ExecContext(ctx, `INSERT INTO atproto_projection_failures
		(source,publisher_did,collection,rkey,cid,reason_code,safe_detail,payload_digest,quarantined)
		VALUES('jetstream-v2',$1,$2,$3,NULLIF($4,''),$5,$6,NULLIF($7,''),TRUE)`,
		event.DID, commit.Collection, commit.Rkey, commit.CID, reason, truncateDetail(detail, 500), digestPayload(payload))
	if err != nil {
		return fmt.Errorf("quarantine Jetstream event: %w", err)
	}
	return nil
}

func (p *PostgresV2Projector) quarantineShadowTx(ctx context.Context, tx *sql.Tx, rebuildID string, event *jetstream.Event, reason, detail string, payload []byte) error {
	commit := event.Commit
	_, err := tx.ExecContext(ctx, `INSERT INTO jetstream_v2_shadow_failures
		(rebuild_id,jetstream_seq,did,collection,rkey,reason_code,safe_detail,payload_digest)
		VALUES($1,$2,$3,$4,$5,$6,$7,NULLIF($8,''))
		ON CONFLICT(rebuild_id,jetstream_seq) DO NOTHING`,
		rebuildID, strconv.FormatUint(event.Seq, 10), event.DID, commit.Collection, commit.Rkey,
		reason, truncateDetail(detail, 500), digestPayload(payload))
	if err != nil {
		return fmt.Errorf("quarantine shadow Jetstream event: %w", err)
	}
	return nil
}

func setEntityPublicationStateTx(ctx context.Context, tx *sql.Tx, entityType, entityID string, deleted bool) error {
	tables := map[string]string{
		"scene": "scenes", "event": "events", "profile": "profiles", "place": "places",
		"venue": "venues", "act": "acts", "tour": "tours", "appearance": "appearances",
	}
	table := tables[entityType]
	if table == "" {
		return nil
	}
	status := "published"
	if deleted {
		status = "archived"
	}
	if _, err := tx.ExecContext(ctx, "UPDATE "+table+" SET publication_status=$2 WHERE id=$1::uuid", entityID, status); err != nil {
		return fmt.Errorf("set %s publication state: %w", entityType, err)
	}
	return nil
}

func recordATURI(did, collection, rkey string) string {
	return "at://" + did + "/" + collection + "/" + rkey
}

func digestPayload(payload []byte) string {
	if len(payload) == 0 {
		return ""
	}
	sum := sha256.Sum256(payload)
	return hex.EncodeToString(sum[:])
}

func nullJSON(payload []byte) any {
	if len(payload) == 0 {
		return nil
	}
	return payload
}

func parseEventTime(value string) any {
	if value == "" {
		return nil
	}
	parsed, err := time.Parse(time.RFC3339Nano, value)
	if err != nil {
		return nil
	}
	return parsed
}

func truncateDetail(value string, limit int) string {
	runes := []rune(value)
	if len(runes) > limit {
		return string(runes[:limit])
	}
	return value
}
