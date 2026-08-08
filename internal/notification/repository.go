package notification

import (
	"context"
	"database/sql"
	"fmt"
	"sync"
	"time"
)

type Subscription struct {
	ID         string
	UserID     string
	Endpoint   []byte
	EndpointID string
	P256DH     []byte
	Auth       []byte
	UserAgent  string
	RevokedAt  *time.Time
}

type Repository interface {
	Upsert(context.Context, Subscription, time.Time) error
	Revoke(context.Context, string, string, time.Time) error
	ListActive(context.Context, string) ([]Subscription, error)
}

type SQLRepository struct{ db *sql.DB }

func NewSQLRepository(db *sql.DB) *SQLRepository { return &SQLRepository{db: db} }

func (r *SQLRepository) Upsert(ctx context.Context, item Subscription, now time.Time) error {
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO web_push_subscriptions
			(user_id, endpoint_ciphertext, endpoint_hmac, p256dh_ciphertext, auth_ciphertext, user_agent_hash, last_seen_at)
		VALUES ($1::uuid, $2, $3, $4, $5, $6, $7)
		ON CONFLICT (endpoint_hmac) WHERE revoked_at IS NULL DO UPDATE SET
			user_id = EXCLUDED.user_id, endpoint_ciphertext = EXCLUDED.endpoint_ciphertext,
			p256dh_ciphertext = EXCLUDED.p256dh_ciphertext, auth_ciphertext = EXCLUDED.auth_ciphertext,
			user_agent_hash = EXCLUDED.user_agent_hash, last_seen_at = EXCLUDED.last_seen_at`,
		item.UserID, item.Endpoint, item.EndpointID, item.P256DH, item.Auth, item.UserAgent, now)
	if err != nil {
		return fmt.Errorf("upsert web push subscription: %w", err)
	}
	return nil
}

func (r *SQLRepository) Revoke(ctx context.Context, userID, endpointID string, now time.Time) error {
	_, err := r.db.ExecContext(ctx, `UPDATE web_push_subscriptions SET revoked_at = $3 WHERE user_id = $1::uuid AND endpoint_hmac = $2 AND revoked_at IS NULL`, userID, endpointID, now)
	return err
}

func (r *SQLRepository) ListActive(ctx context.Context, userID string) ([]Subscription, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT id::text, user_id::text, endpoint_ciphertext, endpoint_hmac, p256dh_ciphertext, auth_ciphertext, COALESCE(user_agent_hash, '') FROM web_push_subscriptions WHERE user_id = $1::uuid AND revoked_at IS NULL`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var result []Subscription
	for rows.Next() {
		var item Subscription
		if err := rows.Scan(&item.ID, &item.UserID, &item.Endpoint, &item.EndpointID, &item.P256DH, &item.Auth, &item.UserAgent); err != nil {
			return nil, err
		}
		result = append(result, item)
	}
	return result, rows.Err()
}

type InMemoryRepository struct {
	mu    sync.RWMutex
	items map[string]Subscription
}

func NewInMemoryRepository() *InMemoryRepository {
	return &InMemoryRepository{items: make(map[string]Subscription)}
}
func (r *InMemoryRepository) Upsert(ctx context.Context, item Subscription, _ time.Time) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.items[item.EndpointID] = item
	return nil
}
func (r *InMemoryRepository) Revoke(ctx context.Context, userID, endpointID string, now time.Time) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	item, ok := r.items[endpointID]
	if ok && item.UserID == userID {
		item.RevokedAt = &now
		r.items[endpointID] = item
	}
	return nil
}
func (r *InMemoryRepository) ListActive(ctx context.Context, userID string) ([]Subscription, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	var result []Subscription
	for _, item := range r.items {
		if item.UserID == userID && item.RevokedAt == nil {
			result = append(result, item)
		}
	}
	return result, nil
}
