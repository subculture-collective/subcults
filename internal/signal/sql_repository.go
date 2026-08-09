package signal

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/lib/pq"
	"github.com/onnwee/subcults/internal/identity"
)

// SQLRepository stores immutable revisions and idempotent delivery snapshots.
// Contact delivery tokens are decrypted only when a queued delivery is loaded;
// plaintext tokens are never persisted in deliveries.
type SQLRepository struct {
	db        *sql.DB
	protector *identity.ContactProtector
}

func NewSQLRepository(database *sql.DB, protector *identity.ContactProtector) *SQLRepository {
	return &SQLRepository{db: database, protector: protector}
}

func (r *SQLRepository) CreateSignal(ctx context.Context, s Signal, revision Revision) error {
	if err := s.Validate(); err != nil {
		return err
	}
	if err := revision.Validate(); err != nil {
		return err
	}
	content, _ := json.Marshal(revision.Content)
	audienceDefinition := json.RawMessage(revision.AudienceDefinition)
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	_, err = tx.ExecContext(ctx, `INSERT INTO signals(id,owner_type,owner_id,target_type,target_id,state,created_at) VALUES($1,$2,$3,$4,$5,$6,$7)`, s.ID, s.OwnerType, s.OwnerID, s.TargetType, s.TargetID, s.State, s.CreatedAt)
	if err != nil {
		return mapSignalError(err, ErrInvalidSignal)
	}
	for _, scopeID := range s.ConsentScopeIDs {
		if _, err = tx.ExecContext(ctx, `INSERT INTO signal_consent_scopes(signal_id,consent_scope_id) VALUES($1,$2)`, s.ID, scopeID); err != nil {
			return err
		}
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO signal_revisions(id,signal_id,revision,content,audience_definition,publish_at,created_by_did,supersedes_signal_revision_id,created_at) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9)`, revision.ID, revision.SignalID, revision.Number, content, audienceDefinition, revision.PublishAt, revision.CreatedByDID, revision.Supersedes, revision.CreatedAt)
	if err != nil {
		return mapSignalError(err, ErrInvalidSignal)
	}
	return tx.Commit()
}
func (r *SQLRepository) GetSignal(ctx context.Context, id string) (Signal, error) {
	var s Signal
	err := r.db.QueryRowContext(ctx, `SELECT id::text,owner_type,owner_id::text,target_type,target_id::text,state,created_at FROM signals WHERE id=$1`, id).Scan(&s.ID, &s.OwnerType, &s.OwnerID, &s.TargetType, &s.TargetID, &s.State, &s.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return Signal{}, ErrSignalNotFound
	}
	if err != nil {
		return Signal{}, err
	}
	rows, err := r.db.QueryContext(ctx, `SELECT consent_scope_id::text FROM signal_consent_scopes WHERE signal_id=$1 ORDER BY consent_scope_id`, id)
	if err != nil {
		return Signal{}, err
	}
	defer rows.Close()
	for rows.Next() {
		var scope string
		if err = rows.Scan(&scope); err != nil {
			return Signal{}, err
		}
		s.ConsentScopeIDs = append(s.ConsentScopeIDs, scope)
	}
	return s, rows.Err()
}
func scanRevision(row interface{ Scan(...any) error }) (Revision, error) {
	var v Revision
	var content, audienceDefinition []byte
	err := row.Scan(&v.ID, &v.SignalID, &v.Number, &content, &audienceDefinition, &v.PublishAt, &v.CreatedByDID, &v.Supersedes, &v.CreatedAt)
	if err != nil {
		return Revision{}, err
	}
	if err = json.Unmarshal(content, &v.Content); err != nil {
		return Revision{}, err
	}
	v.AudienceDefinition = string(audienceDefinition)
	return v, nil
}

const revisionSelect = `SELECT id,signal_id::text,revision,content,audience_definition,publish_at,created_by_did,supersedes_signal_revision_id,created_at FROM signal_revisions`

func (r *SQLRepository) GetLatestRevision(ctx context.Context, id string) (Revision, error) {
	v, err := scanRevision(r.db.QueryRowContext(ctx, revisionSelect+` WHERE signal_id=$1 ORDER BY revision DESC LIMIT 1`, id))
	if errors.Is(err, sql.ErrNoRows) {
		return Revision{}, ErrRevisionNotFound
	}
	return v, err
}
func (r *SQLRepository) GetRevision(ctx context.Context, id string) (Revision, error) {
	v, err := scanRevision(r.db.QueryRowContext(ctx, revisionSelect+` WHERE id=$1`, id))
	if errors.Is(err, sql.ErrNoRows) {
		return Revision{}, ErrRevisionNotFound
	}
	return v, err
}
func (r *SQLRepository) CreateRevision(ctx context.Context, v Revision) error {
	if err := v.Validate(); err != nil {
		return err
	}
	content, _ := json.Marshal(v.Content)
	audienceDefinition := json.RawMessage(v.AudienceDefinition)
	_, err := r.db.ExecContext(ctx, `INSERT INTO signal_revisions(id,signal_id,revision,content,audience_definition,publish_at,created_by_did,supersedes_signal_revision_id,created_at) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9)`, v.ID, v.SignalID, v.Number, content, audienceDefinition, v.PublishAt, v.CreatedByDID, v.Supersedes, v.CreatedAt)
	return mapSignalError(err, ErrInvalidSignal)
}
func (r *SQLRepository) UpdateSignalState(ctx context.Context, id string, state State) error {
	result, err := r.db.ExecContext(ctx, `UPDATE signals SET state=$2 WHERE id=$1`, id, state)
	if err != nil {
		return err
	}
	n, _ := result.RowsAffected()
	if n == 0 {
		return ErrSignalNotFound
	}
	return nil
}
func (r *SQLRepository) CreateDelivery(ctx context.Context, v Delivery) (Delivery, error) {
	if err := v.Validate(); err != nil {
		return Delivery{}, err
	}
	authorization, _ := json.Marshal(v.Scope)
	var id string
	err := r.db.QueryRowContext(ctx, `INSERT INTO deliveries(id,signal_revision_id,contact_point_id,channel,purpose,provider,authorization_scope,state,provider_message_id,scheduled_at,updated_at) VALUES($1,$2,$3,$4,$5,$6,$7,$8,NULLIF($9,''),$10,$11) ON CONFLICT(signal_revision_id,contact_point_id,channel) DO UPDATE SET signal_revision_id=EXCLUDED.signal_revision_id RETURNING id`, v.ID, v.SignalRevisionID, v.ContactPointID, v.Channel, v.Purpose, v.Provider, authorization, v.State, v.ProviderMessageID, v.ScheduledAt, v.UpdatedAt).Scan(&id)
	if err != nil {
		return Delivery{}, mapSignalError(err, ErrInvalidSignal)
	}
	return r.GetDelivery(ctx, id)
}
func (r *SQLRepository) scanDelivery(row interface{ Scan(...any) error }) (Delivery, error) {
	var v Delivery
	var authorization, ciphertext []byte
	err := row.Scan(&v.ID, &v.SignalRevisionID, &v.ContactPointID, &v.Channel, &v.Purpose, &v.Provider, &authorization, &v.State, &v.ProviderMessageID, &v.ScheduledAt, &v.UpdatedAt, &ciphertext)
	if err != nil {
		return Delivery{}, err
	}
	if err = json.Unmarshal(authorization, &v.Scope); err != nil {
		return Delivery{}, err
	}
	if len(ciphertext) > 0 && r.protector != nil {
		token, revealErr := r.protector.Reveal(ciphertext)
		if revealErr != nil {
			return Delivery{}, fmt.Errorf("reveal delivery token: %w", revealErr)
		}
		v.ToToken = []byte(token)
	}
	return v, nil
}
func (r *SQLRepository) GetDelivery(ctx context.Context, id string) (Delivery, error) {
	row := r.db.QueryRowContext(ctx, `SELECT d.id,d.signal_revision_id,d.contact_point_id::text,d.channel,d.purpose,d.provider,d.authorization_scope,d.state,COALESCE(d.provider_message_id,''),d.scheduled_at,d.updated_at,cp.encrypted_value FROM deliveries d JOIN contact_points cp ON cp.id=d.contact_point_id WHERE d.id=$1`, id)
	v, err := r.scanDelivery(row)
	if errors.Is(err, sql.ErrNoRows) {
		return Delivery{}, ErrDeliveryNotFound
	}
	return v, err
}
func (r *SQLRepository) UpdateDelivery(ctx context.Context, v Delivery) error {
	result, err := r.db.ExecContext(ctx, `UPDATE deliveries SET state=$2,provider_message_id=NULLIF($3,''),updated_at=$4 WHERE id=$1`, v.ID, v.State, v.ProviderMessageID, v.UpdatedAt)
	if err != nil {
		return err
	}
	n, _ := result.RowsAffected()
	if n == 0 {
		return ErrDeliveryNotFound
	}
	return nil
}
func mapSignalError(err, kind error) error {
	if err == nil {
		return nil
	}
	var pqerr *pq.Error
	if errors.As(err, &pqerr) {
		if pqerr.Code == "23505" || pqerr.Code == "23503" || pqerr.Code == "23514" {
			return kind
		}
		if pqerr.Code == "55000" {
			return fmt.Errorf("immutable revision: %w", err)
		}
	}
	if strings.Contains(err.Error(), "immutable") {
		return fmt.Errorf("immutable revision: %w", err)
	}
	return err
}

var _ Repository = (*SQLRepository)(nil)
