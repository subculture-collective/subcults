// Package membership provides a SQL-backed implementation of the MembershipRepository.
package membership

import (
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/lib/pq"
)

type SQLMembershipRepository struct{ db *sql.DB }

func NewSQLMembershipRepository(db *sql.DB) *SQLMembershipRepository {
	return &SQLMembershipRepository{db: db}
}

type rowScanner interface{ Scan(...any) error }

const membershipColumns = `id::text,scene_id::text,user_did,role,status,trust_weight,COALESCE(record_did,''),COALESCE(record_rkey,''),since,created_at,updated_at`

func scanMembership(row rowScanner) (*Membership, error) {
	var m Membership
	var rdID, rrKey sql.NullString
	err := row.Scan(&m.ID, &m.SceneID, &m.UserDID, &m.Role, &m.Status, &m.TrustWeight, &rdID, &rrKey, &m.Since, &m.CreatedAt, &m.UpdatedAt)
	if err != nil {
		return nil, err
	}
	m.RecordDID = stringPointer(rdID)
	m.RecordRKey = stringPointer(rrKey)
	return &m, nil
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

func (r *SQLMembershipRepository) Upsert(membership *Membership) (*UpsertResult, error) {
	if err := membership.Validate(); err != nil {
		return nil, err
	}
	if membership.RecordDID != nil && *membership.RecordDID != "" && membership.RecordRKey != nil && *membership.RecordRKey != "" {
		existing, err := r.GetByRecordKey(*membership.RecordDID, *membership.RecordRKey)
		if err == nil {
			membership.ID = existing.ID
			membership.CreatedAt = existing.CreatedAt
			membership.Since = existing.Since
			if membership.Since.IsZero() {
				membership.Since = existing.CreatedAt
			}
			_, err = r.db.Exec(`UPDATE memberships SET scene_id=$2::uuid,user_did=$3,role=$4,status=$5,trust_weight=$6,updated_at=NOW() WHERE id=$1::uuid`,
				membership.ID, membership.SceneID, membership.UserDID, membership.Role, membership.Status, membership.TrustWeight)
			if err != nil {
				return nil, fmt.Errorf("membership upsert update: %w", err)
			}
			membership.UpdatedAt = time.Now()
			return &UpsertResult{ID: membership.ID}, nil
		}
		if !errors.Is(err, ErrMembershipNotFound) {
			return nil, err
		}
	}
	now := time.Now()
	if membership.ID == "" {
		membership.ID = uuid.New().String()
	}
	if membership.Since.IsZero() {
		membership.Since = now
	}
	membership.CreatedAt = now
	membership.UpdatedAt = now
	_, err := r.db.Exec(`INSERT INTO memberships(id,scene_id,user_did,role,status,trust_weight,record_did,record_rkey,since,created_at,updated_at)
		VALUES($1::uuid,$2::uuid,$3,$4,$5,$6,$7,$8,$9,$10,$11)`,
		membership.ID, membership.SceneID, membership.UserDID, membership.Role, membership.Status, membership.TrustWeight,
		nullString(membership.RecordDID), nullString(membership.RecordRKey), membership.Since, membership.CreatedAt, membership.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("membership upsert insert: %w", err)
	}
	return &UpsertResult{Inserted: true, ID: membership.ID}, nil
}

func (r *SQLMembershipRepository) GetByID(id string) (*Membership, error) {
	m, err := scanMembership(r.db.QueryRow(`SELECT `+membershipColumns+` FROM memberships WHERE id=$1::uuid`, id))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrMembershipNotFound
	}
	return m, err
}

func (r *SQLMembershipRepository) GetByRecordKey(did, rkey string) (*Membership, error) {
	m, err := scanMembership(r.db.QueryRow(`SELECT `+membershipColumns+` FROM memberships WHERE record_did=$1 AND record_rkey=$2`, did, rkey))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrMembershipNotFound
	}
	return m, err
}

func (r *SQLMembershipRepository) GetBySceneAndUser(sceneID, userDID string) (*Membership, error) {
	m, err := scanMembership(r.db.QueryRow(`SELECT `+membershipColumns+` FROM memberships WHERE scene_id=$1::uuid AND user_did=$2`, sceneID, userDID))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrMembershipNotFound
	}
	return m, err
}

func (r *SQLMembershipRepository) UpdateStatus(id, status string, since *time.Time) error {
	if since != nil {
		result, err := r.db.Exec(`UPDATE memberships SET status=$2,since=$3,updated_at=NOW() WHERE id=$1::uuid`, id, status, *since)
		if err != nil {
			return fmt.Errorf("update membership status: %w", err)
		}
		n, _ := result.RowsAffected()
		if n == 0 {
			return ErrMembershipNotFound
		}
		return nil
	}
	result, err := r.db.Exec(`UPDATE memberships SET status=$2,updated_at=NOW() WHERE id=$1::uuid`, id, status)
	if err != nil {
		return fmt.Errorf("update membership status: %w", err)
	}
	n, _ := result.RowsAffected()
	if n == 0 {
		return ErrMembershipNotFound
	}
	return nil
}

func (r *SQLMembershipRepository) UpdateRole(id, role string) error {
	result, err := r.db.Exec(`UPDATE memberships SET role=$2,updated_at=NOW() WHERE id=$1::uuid`, id, role)
	if err != nil {
		return fmt.Errorf("update membership role: %w", err)
	}
	n, _ := result.RowsAffected()
	if n == 0 {
		return ErrMembershipNotFound
	}
	return nil
}

func (r *SQLMembershipRepository) ListByScene(sceneID, status string) ([]*Membership, error) {
	args := []any{sceneID}
	where := `scene_id=$1::uuid`
	if status != "" {
		where += ` AND status=$2`
		args = append(args, status)
	}
	rows, err := r.db.Query(`SELECT `+membershipColumns+` FROM memberships WHERE `+where+` ORDER BY created_at DESC`, args...)
	if err != nil {
		return nil, fmt.Errorf("list memberships: %w", err)
	}
	defer rows.Close()
	var result []*Membership
	for rows.Next() {
		m, err := scanMembership(rows)
		if err != nil {
			return nil, err
		}
		result = append(result, m)
	}
	return result, rows.Err()
}

func (r *SQLMembershipRepository) CountByScenes(sceneIDs []string, status string) (map[string]int, error) {
	result := make(map[string]int, len(sceneIDs))
	for _, id := range sceneIDs {
		result[id] = 0
	}
	if len(sceneIDs) == 0 {
		return result, nil
	}
	args := []any{pq.Array(sceneIDs)}
	where := `scene_id=ANY($1::uuid[])`
	if status != "" {
		where += ` AND status=$2`
		args = append(args, status)
	}
	rows, err := r.db.Query(`SELECT scene_id::text,COUNT(*) FROM memberships WHERE `+where+` GROUP BY scene_id`, args...)
	if err != nil {
		return nil, fmt.Errorf("count memberships: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var sid string
		var count int
		if err := rows.Scan(&sid, &count); err != nil {
			return nil, err
		}
		result[sid] = count
	}
	return result, rows.Err()
}

var _ MembershipRepository = (*SQLMembershipRepository)(nil)
