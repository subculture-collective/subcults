package audience

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"strings"
)

// SQLRepository stores private audience evidence. Authorization reads use one
// repeatable-read transaction so consent and suppression cannot be assembled
// from inconsistent snapshots.
type SQLRepository struct{ db *sql.DB }

func NewSQLRepository(database *sql.DB) *SQLRepository { return &SQLRepository{db: database} }

func (r *SQLRepository) PutContact(ctx context.Context, v ContactPoint) error {
	if len(v.EncryptedValue) == 0 || len(v.ValueHMAC) != 64 {
		return errors.New("encrypted contact value and 64-character HMAC are required")
	}
	_, err := r.db.ExecContext(ctx, `INSERT INTO contact_points(id,kind,encrypted_value,value_hmac,verified_at) VALUES($1,$2,$3,$4,$5) ON CONFLICT(id) DO UPDATE SET verified_at=COALESCE(EXCLUDED.verified_at,contact_points.verified_at)`, v.ID, v.Kind, v.EncryptedValue, v.ValueHMAC, v.VerifiedAt)
	return err
}
func (r *SQLRepository) GetContact(ctx context.Context, id string) (ContactPoint, error) {
	var v ContactPoint
	err := r.db.QueryRowContext(ctx, `SELECT id::text,kind,encrypted_value,value_hmac,verified_at FROM contact_points WHERE id=$1`, id).Scan(&v.ID, &v.Kind, &v.EncryptedValue, &v.ValueHMAC, &v.VerifiedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return ContactPoint{}, ErrContactNotFound
	}
	return v, err
}
func (r *SQLRepository) PutLink(ctx context.Context, v ContactPointLink) error {
	_, err := r.db.ExecContext(ctx, `INSERT INTO contact_point_links(contact_point_id,user_did,verification_method,evidence,verified_at,revoked_at) VALUES($1,$2,'verified_contact','{}'::jsonb,$3,$4)`, v.ContactPointID, v.UserDID, v.VerifiedAt, v.RevokedAt)
	return err
}
func (r *SQLRepository) ActiveContactsForDID(ctx context.Context, did string) ([]ContactPoint, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT DISTINCT cp.id::text,cp.kind,cp.encrypted_value,cp.value_hmac,cp.verified_at FROM contact_points cp JOIN contact_point_links l ON l.contact_point_id=cp.id WHERE l.user_did=$1 AND l.revoked_at IS NULL ORDER BY cp.id`, did)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []ContactPoint
	for rows.Next() {
		var v ContactPoint
		if err = rows.Scan(&v.ID, &v.Kind, &v.EncryptedValue, &v.ValueHMAC, &v.VerifiedAt); err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	return out, rows.Err()
}
func (r *SQLRepository) RecordRelationship(ctx context.Context, v Relationship) error {
	var did any = v.SubjectDID
	var contact any = nil
	if strings.TrimSpace(v.ContactPointID) != "" {
		did = nil
		contact = v.ContactPointID
	}
	_, err := r.db.ExecContext(ctx, `INSERT INTO audience_relationships(subject_did,contact_point_id,program_type,program_id,kind,source_type,source_id,occurred_at) VALUES($1,$2,$3,$4,$5,'application',$6,$7)`, did, contact, v.ProgramType, v.ProgramID, v.Kind, v.ProgramID+":"+v.Kind, v.OccurredAt)
	return err
}
func optionalUUID(value string) any {
	if strings.TrimSpace(value) == "" {
		return nil
	}
	return value
}
func (r *SQLRepository) PutScope(ctx context.Context, v DeliveryScope) (string, error) {
	if err := v.Validate(); err != nil {
		return "", err
	}
	var id string
	err := r.db.QueryRowContext(ctx, `INSERT INTO consent_scopes(sender_type,sender_id,channel,purpose,tour_id,event_id,appearance_id,place_id,disclosure_version,region) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,NULLIF($10,'')) ON CONFLICT(sender_type,sender_id,channel,purpose,tour_id,event_id,appearance_id,place_id,disclosure_version,region) DO UPDATE SET disclosure_version=EXCLUDED.disclosure_version RETURNING id::text`, v.SenderType, v.SenderID, v.Channel, v.Purpose, optionalUUID(v.TourID), optionalUUID(v.EventID), optionalUUID(v.AppearanceID), optionalUUID(v.PlaceID), v.DisclosureVersion, v.Region).Scan(&id)
	return id, err
}
func (r *SQLRepository) ScopeIDFor(ctx context.Context, v DeliveryScope) (string, error) {
	var id string
	err := r.db.QueryRowContext(ctx, `SELECT id::text FROM consent_scopes WHERE sender_type=$1 AND sender_id=$2 AND channel=$3 AND purpose=$4 AND tour_id IS NOT DISTINCT FROM $5::uuid AND event_id IS NOT DISTINCT FROM $6::uuid AND appearance_id IS NOT DISTINCT FROM $7::uuid AND place_id IS NOT DISTINCT FROM $8::uuid AND disclosure_version=$9 AND region IS NOT DISTINCT FROM NULLIF($10,'')`, v.SenderType, v.SenderID, v.Channel, v.Purpose, optionalUUID(v.TourID), optionalUUID(v.EventID), optionalUUID(v.AppearanceID), optionalUUID(v.PlaceID), v.DisclosureVersion, v.Region).Scan(&id)
	if errors.Is(err, sql.ErrNoRows) {
		return "", ErrScopeNotFound
	}
	return id, err
}
func scanScope(row interface{ Scan(...any) error }) (DeliveryScope, error) {
	var v DeliveryScope
	var tour, event, appearance, place, region sql.NullString
	err := row.Scan(&v.SenderType, &v.SenderID, &v.Channel, &v.Purpose, &tour, &event, &appearance, &place, &v.DisclosureVersion, &region)
	v.TourID = tour.String
	v.EventID = event.String
	v.AppearanceID = appearance.String
	v.PlaceID = place.String
	v.Region = region.String
	return v, err
}
func (r *SQLRepository) GetScope(ctx context.Context, id string) (DeliveryScope, error) {
	v, err := scanScope(r.db.QueryRowContext(ctx, `SELECT sender_type,sender_id::text,channel,purpose,tour_id::text,event_id::text,appearance_id::text,place_id::text,disclosure_version,region FROM consent_scopes WHERE id=$1`, id))
	if errors.Is(err, sql.ErrNoRows) {
		return DeliveryScope{}, ErrScopeNotFound
	}
	return v, err
}
func (r *SQLRepository) RecordConsent(ctx context.Context, v ConsentEvent) error {
	evidence, err := json.Marshal(v.Evidence)
	if err != nil {
		return err
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err = tx.ExecContext(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1,0))`, v.ContactPointID); err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO consent_events(contact_point_id,consent_scope_id,action,capture_source,occurred_at,evidence) VALUES($1,$2,$3,$4,$5,$6)`, v.ContactPointID, v.ScopeID, v.Action, v.CaptureSource, v.OccurredAt, evidence)
	if err != nil {
		return err
	}
	return tx.Commit()
}
func (r *SQLRepository) ApplicableConsent(ctx context.Context, contactID string, request DeliveryScope) ([]ConsentEvent, error) {
	return applicableConsent(ctx, r.db, contactID, request)
}

type queryer interface {
	QueryContext(context.Context, string, ...any) (*sql.Rows, error)
	QueryRowContext(context.Context, string, ...any) *sql.Row
}

func applicableConsent(ctx context.Context, q queryer, contactID string, request DeliveryScope) ([]ConsentEvent, error) {
	rows, err := q.QueryContext(ctx, `SELECT DISTINCT ON(ce.consent_scope_id) ce.contact_point_id::text,ce.consent_scope_id::text,ce.action,ce.capture_source,ce.evidence,ce.occurred_at,cs.sender_type,cs.sender_id::text,cs.channel,cs.purpose,cs.tour_id::text,cs.event_id::text,cs.appearance_id::text,cs.place_id::text,cs.disclosure_version,cs.region FROM consent_events ce JOIN consent_scopes cs ON cs.id=ce.consent_scope_id WHERE ce.contact_point_id=$1 AND cs.sender_type=$2 AND cs.sender_id=$3 AND cs.channel=$4 AND cs.purpose=$5 AND cs.disclosure_version=$6 ORDER BY ce.consent_scope_id,ce.occurred_at DESC,ce.id DESC`, contactID, request.SenderType, request.SenderID, request.Channel, request.Purpose, request.DisclosureVersion)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []ConsentEvent
	for rows.Next() {
		var v ConsentEvent
		var evidence []byte
		var stored DeliveryScope
		var tour, event, appearance, place, region sql.NullString
		if err = rows.Scan(&v.ContactPointID, &v.ScopeID, &v.Action, &v.CaptureSource, &evidence, &v.OccurredAt, &stored.SenderType, &stored.SenderID, &stored.Channel, &stored.Purpose, &tour, &event, &appearance, &place, &stored.DisclosureVersion, &region); err != nil {
			return nil, err
		}
		stored.TourID = tour.String
		stored.EventID = event.String
		stored.AppearanceID = appearance.String
		stored.PlaceID = place.String
		stored.Region = region.String
		if !stored.Contains(request) {
			continue
		}
		_ = json.Unmarshal(evidence, &v.Evidence)
		out = append(out, v)
	}
	return out, rows.Err()
}
func (r *SQLRepository) RecordSuppression(ctx context.Context, v Suppression) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err = tx.ExecContext(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1,0))`, v.ContactPointID); err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO suppressions(contact_point_id,level,channel,sender_type,sender_id,consent_scope_id,reason,occurred_at,lifted_at) VALUES($1,$2,NULLIF($3,''),NULLIF($4,''),$5,$6,'application',$7,$8)`, v.ContactPointID, v.Level, v.Channel, v.SenderType, optionalUUID(v.SenderID), optionalUUID(v.ScopeID), v.OccurredAt, v.LiftedAt)
	if err != nil {
		return err
	}
	return tx.Commit()
}
func (r *SQLRepository) HasApplicableSuppression(ctx context.Context, contactID string, request DeliveryScope) (bool, error) {
	return hasSuppression(ctx, r.db, contactID, request)
}
func hasSuppression(ctx context.Context, q queryer, contactID string, request DeliveryScope) (bool, error) {
	var suppressed bool
	err := q.QueryRowContext(ctx, `SELECT EXISTS(
		SELECT 1 FROM suppressions s
		LEFT JOIN consent_scopes cs ON cs.id=s.consent_scope_id
		WHERE s.contact_point_id=$1 AND s.lifted_at IS NULL AND (
			s.level='global' OR
			(s.level='channel' AND s.channel=$2) OR
			(s.level='sender' AND s.sender_type=$3 AND s.sender_id=$4::uuid) OR
			(s.level='scope' AND cs.sender_type=$3 AND cs.sender_id=$4::uuid AND cs.channel=$2 AND cs.purpose=$5
			 AND cs.disclosure_version=$6
			 AND (cs.tour_id IS NULL OR cs.tour_id=NULLIF($7,'')::uuid)
			 AND (cs.event_id IS NULL OR cs.event_id=NULLIF($8,'')::uuid)
			 AND (cs.appearance_id IS NULL OR cs.appearance_id=NULLIF($9,'')::uuid)
			 AND (cs.place_id IS NULL OR cs.place_id=NULLIF($10,'')::uuid)
			 AND (cs.region IS NULL OR cs.region=$11))
		))`, contactID, request.Channel, request.SenderType, request.SenderID, request.Purpose,
		request.DisclosureVersion, request.TourID, request.EventID, request.AppearanceID, request.PlaceID, request.Region).Scan(&suppressed)
	return suppressed, err
}

// CanDeliverTransactional resolves verified state, suppressions, and effective
// consent from one repeatable-read snapshot under the contact advisory lock.
func (r *SQLRepository) CanDeliverTransactional(ctx context.Context, contactID string, request DeliveryScope) (bool, error) {
	tx, err := r.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelRepeatableRead, ReadOnly: false})
	if err != nil {
		return false, err
	}
	defer tx.Rollback()
	if _, err = tx.ExecContext(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1,0))`, contactID); err != nil {
		return false, err
	}
	var verified bool
	if err = tx.QueryRowContext(ctx, `SELECT verified_at IS NOT NULL FROM contact_points WHERE id=$1`, contactID).Scan(&verified); errors.Is(err, sql.ErrNoRows) {
		return false, ErrContactNotFound
	} else if err != nil {
		return false, err
	}
	if !verified {
		return false, nil
	}
	suppressed, err := hasSuppression(ctx, tx, contactID, request)
	if err != nil || suppressed {
		return false, err
	}
	events, err := applicableConsent(ctx, tx, contactID, request)
	if err != nil {
		return false, err
	}
	granted := false
	for _, event := range events {
		if event.Action == ConsentRevoke {
			return false, nil
		}
		if event.Action == ConsentGrant {
			granted = true
		}
	}
	if err = tx.Commit(); err != nil {
		return false, err
	}
	return granted, nil
}

var _ Repository = (*SQLRepository)(nil)
