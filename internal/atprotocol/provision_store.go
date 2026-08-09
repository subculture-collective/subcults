package atprotocol

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/lib/pq"
)

// CreateProvisioningRequest enforces all account and capacity limits under one transaction.
func (s *SQLStore) CreateProvisioningRequest(ctx context.Context, userID, handle, ipHash, termsVersion string, dailyCap int) (ProvisioningRequest, error) {
	tx, err := s.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelSerializable})
	if err != nil {
		return ProvisioningRequest{}, fmt.Errorf("begin provisioning transaction: %w", err)
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, "SELECT pg_advisory_xact_lock(hashtext('atproto-provisioning'))"); err != nil {
		return ProvisioningRequest{}, fmt.Errorf("lock provisioning capacity: %w", err)
	}
	now := s.now().UTC()
	since := now.Add(-24 * time.Hour)
	var emailVerified bool
	if err := tx.QueryRowContext(ctx, `SELECT EXISTS (
		SELECT 1 FROM auth_email_identities WHERE user_id=$1::uuid AND verified_at IS NOT NULL
	)`, userID).Scan(&emailVerified); err != nil {
		return ProvisioningRequest{}, fmt.Errorf("check verified email: %w", err)
	}
	if !emailVerified {
		return ProvisioningRequest{}, ErrEmailVerificationRequired
	}
	var hasAccount bool
	if err := tx.QueryRowContext(ctx, `
		SELECT EXISTS (
			SELECT 1 FROM atproto_oauth_links WHERE user_id = $1::uuid
			UNION ALL
			SELECT 1 FROM atproto_provisioning_requests
			WHERE user_id = $1::uuid AND status = 'consumed'
		)
	`, userID).Scan(&hasAccount); err != nil {
		return ProvisioningRequest{}, fmt.Errorf("check existing PDS account: %w", err)
	}
	if hasAccount {
		return ProvisioningRequest{}, ErrProvisioningConflict
	}
	var userCount, ipCount, globalCount int
	if err := tx.QueryRowContext(ctx, `
		SELECT
			(SELECT COUNT(*) FROM atproto_provisioning_requests WHERE user_id = $1::uuid AND created_at >= $3),
			(SELECT COUNT(*) FROM atproto_provisioning_requests WHERE ip_hash = $2 AND created_at >= $3),
			(SELECT COUNT(*) FROM atproto_provisioning_requests WHERE created_at >= date_trunc('day', $4::timestamptz))
	`, userID, ipHash, since, now).Scan(&userCount, &ipCount, &globalCount); err != nil {
		return ProvisioningRequest{}, fmt.Errorf("check provisioning limits: %w", err)
	}
	if userCount >= 3 || ipCount >= 3 || globalCount >= dailyCap {
		return ProvisioningRequest{}, ErrProvisioningLimit
	}
	var request ProvisioningRequest
	err = tx.QueryRowContext(ctx, `
		INSERT INTO atproto_provisioning_requests (
			user_id, handle, status, terms_version, turnstile_outcome,
			ip_hash, created_at, updated_at
		) VALUES ($1::uuid, $2, 'requested', $3, 'passed', $4, $5, $5)
		RETURNING id::text, user_id::text, handle, status, terms_version,
		          turnstile_outcome, created_at, updated_at
	`, userID, handle, termsVersion, ipHash, now).Scan(
		&request.ID, &request.UserID, &request.Handle, &request.Status,
		&request.TermsVersion, &request.TurnstileOutcome,
		&request.CreatedAt, &request.UpdatedAt,
	)
	if err != nil {
		if pqErr, ok := err.(*pq.Error); ok && pqErr.Code == "23505" {
			return ProvisioningRequest{}, ErrProvisioningConflict
		}
		return ProvisioningRequest{}, fmt.Errorf("create provisioning request: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return ProvisioningRequest{}, fmt.Errorf("commit provisioning request: %w", err)
	}
	return request, nil
}

// MarkProvisioningIssued records only a keyed invitation digest.
func (s *SQLStore) MarkProvisioningIssued(ctx context.Context, requestID, invitationHash string, expiresAt time.Time) error {
	result, err := s.db.ExecContext(ctx, `
		UPDATE atproto_provisioning_requests
		SET status = 'issued', invitation_hash = $2, expires_at = $3, updated_at = $4
		WHERE id = $1::uuid AND status = 'requested'
	`, requestID, invitationHash, expiresAt, s.now().UTC())
	if err != nil {
		return fmt.Errorf("mark invitation issued: %w", err)
	}
	if rows, _ := result.RowsAffected(); rows != 1 {
		return ErrProvisioningConflict
	}
	return nil
}

// RejectProvisioningRequest closes a failed external issuance attempt.
func (s *SQLStore) RejectProvisioningRequest(ctx context.Context, requestID string) error {
	_, err := s.db.ExecContext(ctx, `
		UPDATE atproto_provisioning_requests
		SET status = 'rejected', updated_at = $2
		WHERE id = $1::uuid AND status = 'requested'
	`, requestID, s.now().UTC())
	return err
}

// ProvisioningStatus returns the user's latest safe provisioning state.
func (s *SQLStore) ProvisioningStatus(ctx context.Context, userID string) (ProvisioningRequest, error) {
	var request ProvisioningRequest
	var expiresAt sql.NullTime
	err := s.db.QueryRowContext(ctx, `
		SELECT id::text, user_id::text, handle, status, terms_version,
		       turnstile_outcome, expires_at, created_at, updated_at
		FROM atproto_provisioning_requests
		WHERE user_id = $1::uuid ORDER BY created_at DESC LIMIT 1
	`, userID).Scan(
		&request.ID, &request.UserID, &request.Handle, &request.Status,
		&request.TermsVersion, &request.TurnstileOutcome, &expiresAt,
		&request.CreatedAt, &request.UpdatedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return ProvisioningRequest{}, ErrOAuthRequestNotFound
	}
	if err != nil {
		return ProvisioningRequest{}, fmt.Errorf("load provisioning status: %w", err)
	}
	if expiresAt.Valid {
		request.ExpiresAt = expiresAt.Time
		if request.Status == "issued" && !expiresAt.Time.After(s.now()) {
			request.Status = "expired"
		}
	}
	return request, nil
}
