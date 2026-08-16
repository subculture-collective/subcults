// Package alliance provides a SQL-backed implementation of the AllianceRepository.
package alliance

import (
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
)

type SQLAllianceRepository struct{ db *sql.DB }

func NewSQLAllianceRepository(db *sql.DB) *SQLAllianceRepository {
	return &SQLAllianceRepository{db: db}
}

type rowScanner interface{ Scan(...any) error }

const allianceColumns = `id::text,from_scene_id::text,to_scene_id::text,weight,status,COALESCE(reason,''),COALESCE(record_did,''),COALESCE(record_rkey,''),since,created_at,updated_at,deleted_at`

func scanAlliance(row rowScanner) (*Alliance, error) {
	var a Alliance
	var reason, rdID, rrKey sql.NullString
	var deletedAt sql.NullTime
	err := row.Scan(&a.ID, &a.FromSceneID, &a.ToSceneID, &a.Weight, &a.Status, &reason, &rdID, &rrKey, &a.Since, &a.CreatedAt, &a.UpdatedAt, &deletedAt)
	if err != nil {
		return nil, err
	}
	a.Reason = stringPointer(reason)
	a.RecordDID = stringPointer(rdID)
	a.RecordRKey = stringPointer(rrKey)
	if deletedAt.Valid {
		a.DeletedAt = &deletedAt.Time
	}
	return &a, nil
}

func stringPointer(value sql.NullString) *string {
	if !value.Valid || value.String == "" {
		return nil
	}
	result := value.String
	return &result
}

func nullString(value *string) any {
	if value == nil || strings.TrimSpace(*value) == "" {
		return nil
	}
	return *value
}

func (r *SQLAllianceRepository) Upsert(alliance *Alliance) (*UpsertResult, error) {
	if alliance.RecordDID != nil && *alliance.RecordDID != "" && alliance.RecordRKey != nil && *alliance.RecordRKey != "" {
		existing, err := r.GetByRecordKey(*alliance.RecordDID, *alliance.RecordRKey)
		if err == nil {
			alliance.ID = existing.ID
			alliance.CreatedAt = existing.CreatedAt
			alliance.Since = existing.Since
			if alliance.Since.IsZero() {
				alliance.Since = existing.CreatedAt
			}
			_, err = r.db.Exec(`UPDATE alliances SET from_scene_id=$2::uuid,to_scene_id=$3::uuid,weight=$4,status=$5,reason=$6,updated_at=NOW() WHERE id=$1::uuid`,
				alliance.ID, alliance.FromSceneID, alliance.ToSceneID, alliance.Weight, alliance.Status, nullString(alliance.Reason))
			if err != nil {
				return nil, fmt.Errorf("alliance upsert update: %w", err)
			}
			alliance.UpdatedAt = time.Now()
			return &UpsertResult{ID: alliance.ID}, nil
		}
		if !errors.Is(err, ErrAllianceNotFound) {
			return nil, err
		}
	}
	now := time.Now()
	if alliance.ID == "" {
		alliance.ID = uuid.New().String()
	}
	if alliance.Since.IsZero() {
		alliance.Since = now
	}
	alliance.CreatedAt = now
	alliance.UpdatedAt = now
	_, err := r.db.Exec(`INSERT INTO alliances(id,from_scene_id,to_scene_id,weight,status,reason,record_did,record_rkey,since,created_at,updated_at)
		VALUES($1::uuid,$2::uuid,$3::uuid,$4,$5,$6,$7,$8,$9,$10,$11)`,
		alliance.ID, alliance.FromSceneID, alliance.ToSceneID, alliance.Weight, alliance.Status, nullString(alliance.Reason),
		nullString(alliance.RecordDID), nullString(alliance.RecordRKey), alliance.Since, alliance.CreatedAt, alliance.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("alliance upsert insert: %w", err)
	}
	return &UpsertResult{Inserted: true, ID: alliance.ID}, nil
}

func (r *SQLAllianceRepository) GetByID(id string) (*Alliance, error) {
	a, err := scanAlliance(r.db.QueryRow(`SELECT `+allianceColumns+` FROM alliances WHERE id=$1::uuid`, id))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrAllianceNotFound
	}
	if err != nil {
		return nil, err
	}
	if a.DeletedAt != nil {
		return nil, ErrAllianceDeleted
	}
	return a, nil
}

func (r *SQLAllianceRepository) GetByRecordKey(did, rkey string) (*Alliance, error) {
	a, err := scanAlliance(r.db.QueryRow(`SELECT `+allianceColumns+` FROM alliances WHERE record_did=$1 AND record_rkey=$2`, did, rkey))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrAllianceNotFound
	}
	if err != nil {
		return nil, err
	}
	if a.DeletedAt != nil {
		return nil, ErrAllianceDeleted
	}
	return a, nil
}

func (r *SQLAllianceRepository) Insert(alliance *Alliance) error {
	now := time.Now()
	if alliance.ID == "" {
		alliance.ID = uuid.New().String()
	}
	if alliance.Since.IsZero() {
		alliance.Since = now
	}
	alliance.CreatedAt = now
	alliance.UpdatedAt = now
	_, err := r.db.Exec(`INSERT INTO alliances(id,from_scene_id,to_scene_id,weight,status,reason,record_did,record_rkey,since,created_at,updated_at)
		VALUES($1::uuid,$2::uuid,$3::uuid,$4,$5,$6,$7,$8,$9,$10,$11)`,
		alliance.ID, alliance.FromSceneID, alliance.ToSceneID, alliance.Weight, alliance.Status, nullString(alliance.Reason),
		nullString(alliance.RecordDID), nullString(alliance.RecordRKey), alliance.Since, alliance.CreatedAt, alliance.UpdatedAt)
	if err != nil {
		return fmt.Errorf("insert alliance: %w", err)
	}
	return nil
}

func (r *SQLAllianceRepository) Update(alliance *Alliance) error {
	result, err := r.db.Exec(`UPDATE alliances SET weight=$2,status=$3,reason=$4,updated_at=NOW() WHERE id=$1::uuid AND deleted_at IS NULL`,
		alliance.ID, alliance.Weight, alliance.Status, nullString(alliance.Reason))
	if err != nil {
		return fmt.Errorf("update alliance: %w", err)
	}
	n, _ := result.RowsAffected()
	if n == 0 {
		var deleted bool
		r.db.QueryRow(`SELECT deleted_at IS NOT NULL FROM alliances WHERE id=$1::uuid`, alliance.ID).Scan(&deleted)
		if deleted {
			return ErrAllianceDeleted
		}
		return ErrAllianceNotFound
	}
	return nil
}

func (r *SQLAllianceRepository) Delete(id string) error {
	result, err := r.db.Exec(`UPDATE alliances SET deleted_at=NOW() WHERE id=$1::uuid AND deleted_at IS NULL`, id)
	if err != nil {
		return fmt.Errorf("delete alliance: %w", err)
	}
	n, _ := result.RowsAffected()
	if n == 0 {
		var exists, deleted bool
		sErr := r.db.QueryRow(`SELECT EXISTS(SELECT 1 FROM alliances WHERE id=$1::uuid AND deleted_at IS NOT NULL)`, id).Scan(&deleted)
		if sErr != nil {
			return sErr
		}
		if deleted {
			return ErrAllianceDeleted
		}
		_ = exists
		return ErrAllianceNotFound
	}
	return nil
}

var _ AllianceRepository = (*SQLAllianceRepository)(nil)
