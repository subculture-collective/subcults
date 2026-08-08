package locationaccess

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"sync"
	"time"
)

var ErrNotAuthorized = errors.New("precise event location is not authorized")

type Grant struct {
	EventID     string     `json:"event_id"`
	UserID      string     `json:"user_id"`
	Reason      string     `json:"reason"`
	GrantedByID string     `json:"granted_by_user_id,omitempty"`
	GrantedAt   time.Time  `json:"granted_at"`
	ExpiresAt   *time.Time `json:"expires_at,omitempty"`
}

type Repository interface {
	Grant(context.Context, Grant) error
	Revoke(context.Context, string, string, time.Time) error
	CanView(context.Context, string, string, time.Time) (bool, error)
}

type SQLRepository struct{ db *sql.DB }

func NewSQLRepository(db *sql.DB) *SQLRepository { return &SQLRepository{db: db} }

func (r *SQLRepository) Grant(ctx context.Context, grant Grant) error {
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO event_location_grants
			(event_id, user_id, reason, granted_by_user_id, granted_at, expires_at)
		VALUES ($1::uuid, $2::uuid, $3, NULLIF($4, '')::uuid, $5, $6)`,
		grant.EventID, grant.UserID, grant.Reason, grant.GrantedByID, grant.GrantedAt, grant.ExpiresAt)
	if err != nil {
		return fmt.Errorf("grant event location: %w", err)
	}
	return nil
}

func (r *SQLRepository) Revoke(ctx context.Context, eventID, userID string, now time.Time) error {
	_, err := r.db.ExecContext(ctx, `
		UPDATE event_location_grants SET revoked_at = $3
		WHERE event_id = $1::uuid AND user_id = $2::uuid AND revoked_at IS NULL`, eventID, userID, now)
	return err
}

func (r *SQLRepository) CanView(ctx context.Context, eventID, userID string, now time.Time) (bool, error) {
	var allowed bool
	err := r.db.QueryRowContext(ctx, `SELECT EXISTS (
		SELECT 1 FROM event_location_grants
		WHERE event_id = $1::uuid AND user_id = $2::uuid AND revoked_at IS NULL
		  AND (expires_at IS NULL OR expires_at > $3)
	)`, eventID, userID, now).Scan(&allowed)
	return allowed, err
}

type InMemoryRepository struct {
	mu      sync.RWMutex
	grants  []Grant
	revoked map[string]time.Time
}

func NewInMemoryRepository() *InMemoryRepository {
	return &InMemoryRepository{revoked: make(map[string]time.Time)}
}

func grantKey(eventID, userID string) string { return eventID + "\x00" + userID }

func (r *InMemoryRepository) Grant(ctx context.Context, grant Grant) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.grants = append(r.grants, grant)
	delete(r.revoked, grantKey(grant.EventID, grant.UserID))
	return nil
}

func (r *InMemoryRepository) Revoke(ctx context.Context, eventID, userID string, now time.Time) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.revoked[grantKey(eventID, userID)] = now
	return nil
}

func (r *InMemoryRepository) CanView(ctx context.Context, eventID, userID string, now time.Time) (bool, error) {
	if err := ctx.Err(); err != nil {
		return false, err
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	if _, revoked := r.revoked[grantKey(eventID, userID)]; revoked {
		return false, nil
	}
	for _, grant := range r.grants {
		if grant.EventID == eventID && grant.UserID == userID && (grant.ExpiresAt == nil || grant.ExpiresAt.After(now)) {
			return true, nil
		}
	}
	return false, nil
}
