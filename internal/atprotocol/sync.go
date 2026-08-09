package atprotocol

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"time"
)

// TapEnvelope is Tap's durable webhook event shape.
type TapEnvelope struct {
	ID       int64           `json:"id"`
	Type     string          `json:"type"`
	Record   TapRecordEvent  `json:"record"`
	Identity json.RawMessage `json:"identity,omitempty"`
}

// TapRecordEvent contains only the record data Subcults needs to project.
type TapRecordEvent struct {
	Live       bool            `json:"live"`
	Rev        string          `json:"rev"`
	DID        string          `json:"did"`
	Collection string          `json:"collection"`
	RKey       string          `json:"rkey"`
	Action     string          `json:"action"`
	CID        string          `json:"cid"`
	Record     json.RawMessage `json:"record"`
}

// IngestResult is safe to return to Tap; it contains no record payload.
type IngestResult struct {
	ATURI     string `json:"at_uri,omitempty"`
	Outcome   string `json:"outcome"`
	Duplicate bool   `json:"duplicate,omitempty"`
}

// IngestTap validates and durably observes a Tap event before acknowledging it.
func (s *OAuthService) IngestTap(ctx context.Context, envelope TapEnvelope) (IngestResult, error) {
	if envelope.Type == "identity" {
		return IngestResult{Outcome: "identity_ignored"}, nil
	}
	if envelope.Type != "record" {
		return IngestResult{}, ErrInvalidRecord
	}
	return s.store.IngestObservation(ctx, "tap", envelope.ID, envelope.Record, time.Now().UTC())
}

// IngestObservation shares idempotency and projection state across Tap and
// direct reconciliation. Raw public records are validated in memory; only a
// digest and provenance history enter the synchronization log.
func (s *SQLStore) IngestObservation(ctx context.Context, source string, sourceEventID int64, event TapRecordEvent, observedAt time.Time) (IngestResult, error) {
	if event.DID == "" || event.Collection == "" || event.RKey == "" || event.Rev == "" ||
		(event.Action != "create" && event.Action != "update" && event.Action != "delete") {
		return IngestResult{}, ErrInvalidRecord
	}
	atURI := fmt.Sprintf("at://%s/%s/%s", event.DID, event.Collection, event.RKey)
	digest := ""
	if event.Action != "delete" {
		if IsCanonicalCollection(event.Collection) {
			if err := ValidatePublicRecord(event.Collection, event.Record); err != nil {
				return s.quarantineObservation(ctx, source, sourceEventID, event, atURI, "record_validation", err.Error(), observedAt)
			}
		} else if !IsLegacyCollection(event.Collection) {
			return IngestResult{}, ErrUnsupportedCollection
		}
		sum := sha256.Sum256(event.Record)
		digest = hex.EncodeToString(sum[:])
	}

	tx, err := s.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		return IngestResult{}, err
	}
	defer tx.Rollback()
	if IsLegacyCollection(event.Collection) {
		inserted, err := insertObservation(ctx, tx, source, sourceEventID, event, atURI, digest, "legacy_observed", observedAt)
		if err != nil {
			return IngestResult{}, err
		}
		if err = tx.Commit(); err != nil {
			return IngestResult{}, err
		}
		return IngestResult{ATURI: atURI, Outcome: "legacy_observed", Duplicate: !inserted}, nil
	}

	var entityType, entityID, expectedCID, projectionStatus string
	err = tx.QueryRowContext(ctx, `SELECT entity_type,entity_id::text,COALESCE(cid,''),projection_status
		FROM atproto_record_mappings WHERE at_uri=$1 FOR UPDATE`, atURI).
		Scan(&entityType, &entityID, &expectedCID, &projectionStatus)
	if errors.Is(err, sql.ErrNoRows) {
		_ = tx.Rollback()
		return s.quarantineObservation(ctx, source, sourceEventID, event, atURI, "unmapped_record", "canonical record has no local entity mapping", observedAt)
	}
	if err != nil {
		return IngestResult{}, err
	}
	if event.Action != "delete" && expectedCID != "" && expectedCID != event.CID {
		_ = tx.Rollback()
		return s.quarantineObservation(ctx, source, sourceEventID, event, atURI, "cid_conflict", "observed CID differs from the pending local publication", observedAt)
	}

	outcome := "projected"
	if event.Action == "delete" {
		outcome = "deleted"
	}
	inserted, err := insertObservation(ctx, tx, source, sourceEventID, event, atURI, digest, outcome, observedAt)
	if err != nil {
		return IngestResult{}, err
	}
	if !inserted {
		if err = tx.Commit(); err != nil {
			return IngestResult{}, err
		}
		return IngestResult{ATURI: atURI, Outcome: outcome, Duplicate: true}, nil
	}
	if event.Action == "delete" {
		_, err = tx.ExecContext(ctx, `UPDATE atproto_record_mappings SET projection_status='deleted',last_seen_at=$2,updated_at=$2 WHERE at_uri=$1`, atURI, observedAt)
	} else {
		_, err = tx.ExecContext(ctx, `UPDATE atproto_record_mappings SET cid=$2,projection_status='projected',last_seen_at=$3,updated_at=$3 WHERE at_uri=$1`, atURI, event.CID, observedAt)
	}
	if err != nil {
		return IngestResult{}, err
	}
	if err = setEntityProjectionState(ctx, tx, entityType, entityID, event.Action == "delete"); err != nil {
		return IngestResult{}, err
	}
	if err = tx.Commit(); err != nil {
		return IngestResult{}, err
	}
	_ = projectionStatus
	return IngestResult{ATURI: atURI, Outcome: outcome}, nil
}

func insertObservation(ctx context.Context, tx *sql.Tx, source string, sourceEventID int64, event TapRecordEvent, atURI, digest, outcome string, observedAt time.Time) (bool, error) {
	result, err := tx.ExecContext(ctx, `INSERT INTO atproto_sync_observations
		(source,source_event_id,publisher_did,collection,rkey,at_uri,cid,revision,action,payload_digest,projection_outcome,observed_at)
		VALUES($1,NULLIF($2::bigint,0),$3,$4,$5,$6,NULLIF($7,''),$8,$9,NULLIF($10,''),$11,$12)
		ON CONFLICT DO NOTHING`, source, sourceEventID, event.DID, event.Collection, event.RKey, atURI, event.CID, event.Rev, event.Action, digest, outcome, observedAt)
	if err != nil {
		return false, err
	}
	rows, _ := result.RowsAffected()
	return rows == 1, nil
}

func (s *SQLStore) quarantineObservation(ctx context.Context, source string, sourceEventID int64, event TapRecordEvent, atURI, reason, detail string, observedAt time.Time) (IngestResult, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return IngestResult{}, err
	}
	defer tx.Rollback()
	digest := ""
	if len(event.Record) > 0 {
		sum := sha256.Sum256(event.Record)
		digest = hex.EncodeToString(sum[:])
	}
	inserted, err := insertObservation(ctx, tx, source, sourceEventID, event, atURI, digest, "quarantined", observedAt)
	if err != nil {
		return IngestResult{}, err
	}
	if inserted {
		_, err = tx.ExecContext(ctx, `INSERT INTO atproto_projection_failures
			(source,publisher_did,collection,rkey,cid,reason_code,safe_detail,payload_digest,first_seen_at,last_seen_at)
			VALUES($1,$2,$3,$4,NULLIF($5,''),$6,$7,NULLIF($8,''),$9,$9)`, source, event.DID, event.Collection, event.RKey, event.CID, reason, truncateSafe(detail, 500), digest, observedAt)
		if err != nil {
			return IngestResult{}, err
		}
	}
	if err = tx.Commit(); err != nil {
		return IngestResult{}, err
	}
	return IngestResult{ATURI: atURI, Outcome: "quarantined", Duplicate: !inserted}, nil
}

func setEntityProjectionState(ctx context.Context, tx *sql.Tx, entityType, entityID string, deleted bool) error {
	tableByType := map[string]string{"scene": "scenes", "event": "events", "profile": "profiles", "place": "places", "venue": "venues", "act": "acts", "tour": "tours", "appearance": "appearances"}
	table := tableByType[entityType]
	if table == "" {
		return nil
	}
	status := "published"
	if deleted {
		status = "archived"
	}
	_, err := tx.ExecContext(ctx, "UPDATE "+table+" SET publication_status=$2 WHERE id=$1::uuid", entityID, status)
	return err
}

func truncateSafe(value string, limit int) string {
	runes := []rune(value)
	if len(runes) > limit {
		return string(runes[:limit])
	}
	return value
}

// RecordOperationalFailure stores a secret-free operational failure for alerting.
func (s *SQLStore) RecordOperationalFailure(ctx context.Context, source, did, reason, detail string) error {
	_, err := s.db.ExecContext(ctx, `INSERT INTO atproto_projection_failures
		(source,publisher_did,reason_code,safe_detail,quarantined) VALUES($1,$2,$3,$4,false)`, source, did, reason, truncateSafe(detail, 500))
	return err
}
