package identity

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/lib/pq"
)

type SQLRepository struct{ db *sql.DB }

func NewSQLRepository(db *sql.DB) *SQLRepository { return &SQLRepository{db: db} }

func (r *SQLRepository) UpsertEmailIdentity(ctx context.Context, encryptedEmail []byte, emailHMAC string) (EmailIdentity, error) {
	var identity EmailIdentity
	var userID sql.NullString
	var verifiedAt sql.NullTime
	err := r.db.QueryRowContext(ctx, `
		INSERT INTO auth_email_identities (encrypted_email, email_hmac)
		VALUES ($1, $2)
		ON CONFLICT (email_hmac) DO UPDATE SET updated_at = NOW()
		RETURNING id::text, user_id::text, encrypted_email, email_hmac, verified_at`, encryptedEmail, emailHMAC,
	).Scan(&identity.ID, &userID, &identity.EncryptedEmail, &identity.EmailHMAC, &verifiedAt)
	if err != nil {
		return EmailIdentity{}, fmt.Errorf("upsert email identity: %w", err)
	}
	identity.UserID = userID.String
	if verifiedAt.Valid {
		identity.VerifiedAt = &verifiedAt.Time
	}
	return identity, nil
}

func (r *SQLRepository) CreateMagicLink(ctx context.Context, link MagicLink) error {
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO auth_magic_links (email_identity_id, token_hash, return_path, expires_at)
		VALUES ($1::uuid, $2, $3, $4)`, link.IdentityID, link.TokenHash, link.ReturnPath, link.ExpiresAt)
	if err != nil {
		return fmt.Errorf("create magic link: %w", err)
	}
	return nil
}

func (r *SQLRepository) CompleteMagicLink(ctx context.Context, tokenHash, refreshHash, userAgentHash string, now, idleExpiresAt, absoluteExpiresAt time.Time) (SessionResult, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return SessionResult{}, fmt.Errorf("begin magic-link transaction: %w", err)
	}
	defer tx.Rollback()

	var linkID, identityID, returnPath string
	var userID sql.NullString
	var expiresAt time.Time
	var consumedAt sql.NullTime
	err = tx.QueryRowContext(ctx, `
		SELECT ml.id::text, ml.email_identity_id::text, ei.user_id::text,
		       ml.return_path, ml.expires_at, ml.consumed_at
		FROM auth_magic_links ml
		JOIN auth_email_identities ei ON ei.id = ml.email_identity_id
		WHERE ml.token_hash = $1
		FOR UPDATE OF ml, ei`, tokenHash,
	).Scan(&linkID, &identityID, &userID, &returnPath, &expiresAt, &consumedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return SessionResult{}, ErrInvalidToken
	}
	if err != nil {
		return SessionResult{}, fmt.Errorf("load magic link: %w", err)
	}
	if consumedAt.Valid || !expiresAt.After(now) {
		return SessionResult{}, ErrInvalidToken
	}

	if !userID.Valid {
		err = tx.QueryRowContext(ctx, `
			WITH generated AS (SELECT gen_random_uuid() AS id)
			INSERT INTO users (id, handle, internal_did)
			SELECT id,
			       'member-' || left(replace(id::text, '-', ''), 8),
			       'did:web:subcults.subcult.tv:users:' || id::text
			FROM generated
			RETURNING id::text`,
		).Scan(&userID.String)
		if err != nil {
			return SessionResult{}, fmt.Errorf("create passwordless user: %w", err)
		}
		userID.Valid = true
		if _, err = tx.ExecContext(ctx, `UPDATE auth_email_identities SET user_id = $1::uuid, verified_at = $2, updated_at = $2 WHERE id = $3::uuid`, userID.String, now, identityID); err != nil {
			return SessionResult{}, fmt.Errorf("link email identity: %w", err)
		}
		if _, err = tx.ExecContext(ctx, `INSERT INTO user_roles (user_id, role) VALUES ($1::uuid, 'participant')`, userID.String); err != nil {
			return SessionResult{}, fmt.Errorf("grant participant role: %w", err)
		}
	} else {
		if _, err = tx.ExecContext(ctx, `UPDATE auth_email_identities SET verified_at = COALESCE(verified_at, $1), updated_at = $1 WHERE id = $2::uuid`, now, identityID); err != nil {
			return SessionResult{}, fmt.Errorf("verify email identity: %w", err)
		}
	}

	if _, err = tx.ExecContext(ctx, `UPDATE auth_magic_links SET consumed_at = $1 WHERE id = $2::uuid`, now, linkID); err != nil {
		return SessionResult{}, fmt.Errorf("consume magic link: %w", err)
	}

	var sessionID string
	err = tx.QueryRowContext(ctx, `
		INSERT INTO auth_sessions (
			user_id, refresh_token_hash, user_agent_hash,
			idle_expires_at, absolute_expires_at
		) VALUES ($1::uuid, $2, $3, $4, $5)
		RETURNING id::text`, userID.String, refreshHash, userAgentHash, idleExpiresAt, absoluteExpiresAt,
	).Scan(&sessionID)
	if err != nil {
		return SessionResult{}, fmt.Errorf("create auth session: %w", err)
	}

	user, err := getUserTx(ctx, tx, userID.String)
	if err != nil {
		return SessionResult{}, err
	}
	if err := tx.Commit(); err != nil {
		return SessionResult{}, fmt.Errorf("commit magic-link transaction: %w", err)
	}
	return SessionResult{User: user, SessionID: sessionID, ReturnPath: returnPath}, nil
}

func (r *SQLRepository) RotateSession(ctx context.Context, oldRefreshHash, newRefreshHash, userAgentHash string, now, idleExpiresAt time.Time) (SessionResult, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return SessionResult{}, fmt.Errorf("begin refresh transaction: %w", err)
	}
	defer tx.Rollback()

	var oldID, userID string
	var idleExpiry, absoluteExpiry time.Time
	var revokedAt sql.NullTime
	err = tx.QueryRowContext(ctx, `
		SELECT id::text, user_id::text, idle_expires_at, absolute_expires_at, revoked_at
		FROM auth_sessions WHERE refresh_token_hash = $1 FOR UPDATE`, oldRefreshHash,
	).Scan(&oldID, &userID, &idleExpiry, &absoluteExpiry, &revokedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return SessionResult{}, ErrInvalidSession
	}
	if err != nil {
		return SessionResult{}, fmt.Errorf("load auth session: %w", err)
	}
	if revokedAt.Valid || !idleExpiry.After(now) || !absoluteExpiry.After(now) {
		return SessionResult{}, ErrInvalidSession
	}
	if idleExpiresAt.After(absoluteExpiry) {
		idleExpiresAt = absoluteExpiry
	}
	if _, err = tx.ExecContext(ctx, `UPDATE auth_sessions SET revoked_at = $1, last_seen_at = $1 WHERE id = $2::uuid`, now, oldID); err != nil {
		return SessionResult{}, fmt.Errorf("revoke rotated session: %w", err)
	}
	var sessionID string
	err = tx.QueryRowContext(ctx, `
		INSERT INTO auth_sessions (
			user_id, refresh_token_hash, user_agent_hash, idle_expires_at, absolute_expires_at
		) VALUES ($1::uuid, $2, $3, $4, $5)
		RETURNING id::text`, userID, newRefreshHash, userAgentHash, idleExpiresAt, absoluteExpiry,
	).Scan(&sessionID)
	if err != nil {
		return SessionResult{}, fmt.Errorf("insert rotated session: %w", err)
	}
	user, err := getUserTx(ctx, tx, userID)
	if err != nil {
		return SessionResult{}, err
	}
	if err := tx.Commit(); err != nil {
		return SessionResult{}, fmt.Errorf("commit refresh transaction: %w", err)
	}
	return SessionResult{User: user, SessionID: sessionID}, nil
}

func (r *SQLRepository) GetUser(ctx context.Context, userID string) (User, error) {
	return scanUser(r.db.QueryRowContext(ctx, userQuery, userID))
}

func (r *SQLRepository) CompleteProfile(ctx context.Context, userID, handle, displayName string) (User, error) {
	_, err := r.db.ExecContext(ctx, `UPDATE users SET handle = $1, display_name = $2, onboarding_complete = TRUE, updated_at = NOW() WHERE id = $3::uuid`, handle, displayName, userID)
	if err != nil {
		var pqErr *pq.Error
		if errors.As(err, &pqErr) && pqErr.Code == "23505" {
			return User{}, ErrHandleUnavailable
		}
		return User{}, fmt.Errorf("complete user profile: %w", err)
	}
	return r.GetUser(ctx, userID)
}

func (r *SQLRepository) RevokeSession(ctx context.Context, refreshHash string, now time.Time) error {
	_, err := r.db.ExecContext(ctx, `UPDATE auth_sessions SET revoked_at = COALESCE(revoked_at, $1) WHERE refresh_token_hash = $2`, now, refreshHash)
	return err
}

func (r *SQLRepository) RevokeAllSessions(ctx context.Context, userID string, now time.Time) error {
	_, err := r.db.ExecContext(ctx, `UPDATE auth_sessions SET revoked_at = $1 WHERE user_id = $2::uuid AND revoked_at IS NULL`, now, userID)
	return err
}

func (r *SQLRepository) CreateCreatorAccessRequest(ctx context.Context, userID, statement string, now time.Time) (CreatorAccessRequest, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return CreatorAccessRequest{}, err
	}
	defer tx.Rollback()
	if existing, err := scanCreatorRequest(tx.QueryRowContext(ctx, creatorRequestSelect+` WHERE user_id = $1::uuid AND status = 'pending' ORDER BY created_at DESC LIMIT 1`, userID)); err == nil {
		return existing, nil
	} else if !errors.Is(err, sql.ErrNoRows) {
		return CreatorAccessRequest{}, err
	}
	var request CreatorAccessRequest
	err = tx.QueryRowContext(ctx, `
		INSERT INTO creator_access_requests (user_id, statement, created_at, updated_at)
		VALUES ($1::uuid, $2, $3, $3)
		RETURNING id::text, user_id::text, statement, status, COALESCE(review_note, ''), created_at, reviewed_at, COALESCE(reviewed_by_user_id::text, '')`,
		userID, statement, now,
	).Scan(&request.ID, &request.UserID, &request.Statement, &request.Status, &request.ReviewNote, &request.CreatedAt, &request.ReviewedAt, &request.ReviewedByID)
	if err != nil {
		return CreatorAccessRequest{}, fmt.Errorf("create creator access request: %w", err)
	}
	if _, err = tx.ExecContext(ctx, `UPDATE user_roles SET revoked_at = $2 WHERE user_id = $1::uuid AND revoked_at IS NULL`, userID, now); err != nil {
		return CreatorAccessRequest{}, err
	}
	if _, err = tx.ExecContext(ctx, `INSERT INTO user_roles (user_id, role, granted_at) VALUES ($1::uuid, 'creator_pending', $2)`, userID, now); err != nil {
		return CreatorAccessRequest{}, err
	}
	if err = tx.Commit(); err != nil {
		return CreatorAccessRequest{}, err
	}
	return request, nil
}

func (r *SQLRepository) GetCreatorAccessRequest(ctx context.Context, userID string) (CreatorAccessRequest, error) {
	return scanCreatorRequest(r.db.QueryRowContext(ctx, creatorRequestSelect+` WHERE user_id = $1::uuid ORDER BY created_at DESC LIMIT 1`, userID))
}

func (r *SQLRepository) ReviewCreatorAccessRequest(ctx context.Context, requestID, reviewerID, status, note string, now time.Time) (CreatorAccessRequest, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return CreatorAccessRequest{}, err
	}
	defer tx.Rollback()
	var request CreatorAccessRequest
	err = tx.QueryRowContext(ctx, `
		UPDATE creator_access_requests
		SET status = $2, review_note = NULLIF($3, ''), reviewed_by_user_id = $4::uuid,
		    reviewed_at = $5, updated_at = $5
		WHERE id = $1::uuid AND status = 'pending'
		RETURNING id::text, user_id::text, statement, status, COALESCE(review_note, ''), created_at, reviewed_at, COALESCE(reviewed_by_user_id::text, '')`,
		requestID, status, note, reviewerID, now,
	).Scan(&request.ID, &request.UserID, &request.Statement, &request.Status, &request.ReviewNote, &request.CreatedAt, &request.ReviewedAt, &request.ReviewedByID)
	if errors.Is(err, sql.ErrNoRows) {
		return CreatorAccessRequest{}, sql.ErrNoRows
	}
	if err != nil {
		return CreatorAccessRequest{}, err
	}
	if _, err = tx.ExecContext(ctx, `UPDATE user_roles SET revoked_at = $2 WHERE user_id = $1::uuid AND revoked_at IS NULL`, request.UserID, now); err != nil {
		return CreatorAccessRequest{}, err
	}
	role := "participant"
	if status == "approved" {
		role = "creator"
	}
	if _, err = tx.ExecContext(ctx, `INSERT INTO user_roles (user_id, role, granted_by_user_id, granted_at) VALUES ($1::uuid, $2, $3::uuid, $4)`, request.UserID, role, reviewerID, now); err != nil {
		return CreatorAccessRequest{}, err
	}
	if err = tx.Commit(); err != nil {
		return CreatorAccessRequest{}, err
	}
	return request, nil
}

const creatorRequestSelect = `SELECT id::text, user_id::text, statement, status, COALESCE(review_note, ''), created_at, reviewed_at, COALESCE(reviewed_by_user_id::text, '') FROM creator_access_requests`

func scanCreatorRequest(row rowScanner) (CreatorAccessRequest, error) {
	var request CreatorAccessRequest
	if err := row.Scan(&request.ID, &request.UserID, &request.Statement, &request.Status, &request.ReviewNote, &request.CreatedAt, &request.ReviewedAt, &request.ReviewedByID); err != nil {
		return CreatorAccessRequest{}, err
	}
	return request, nil
}

const userQuery = `
	SELECT u.id::text, u.internal_did, u.handle, COALESCE(u.display_name, ''),
	       COALESCE(ur.role, 'participant'), u.onboarding_complete
	FROM users u
	LEFT JOIN user_roles ur ON ur.user_id = u.id AND ur.revoked_at IS NULL
	WHERE u.id = $1::uuid`

type rowScanner interface{ Scan(...any) error }

func scanUser(row rowScanner) (User, error) {
	var user User
	if err := row.Scan(&user.ID, &user.InternalDID, &user.Handle, &user.DisplayName, &user.Role, &user.OnboardingComplete); err != nil {
		return User{}, fmt.Errorf("get user: %w", err)
	}
	return user, nil
}

func getUserTx(ctx context.Context, tx *sql.Tx, userID string) (User, error) {
	return scanUser(tx.QueryRowContext(ctx, userQuery, userID))
}
