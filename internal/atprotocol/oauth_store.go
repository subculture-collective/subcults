package atprotocol

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"time"

	indigooauth "github.com/bluesky-social/indigo/atproto/auth/oauth"
	"github.com/bluesky-social/indigo/atproto/syntax"
	"github.com/lib/pq"
)

var (
	ErrOAuthSessionNotFound = errors.New("AT Protocol OAuth session not found")
	ErrOAuthRequestNotFound = errors.New("AT Protocol OAuth request not found")
	ErrLinkConflict         = errors.New("AT Protocol identity is already linked")
)

type flowContextKey struct{}

// FlowContext binds an OAuth state to the authenticated local user that began it.
type FlowContext struct {
	UserID         string
	ReturnPath     string
	RequestedScope string
}

// WithFlowContext attaches local authorization context to Indigo's request.
func WithFlowContext(ctx context.Context, flow FlowContext) context.Context {
	return context.WithValue(ctx, flowContextKey{}, flow)
}

// Link is the public status of a user's external AT Protocol identity.
type Link struct {
	UserID        string
	AccountDID    string
	Handle        string
	SessionID     string
	HostURL       string
	GrantedScopes []string
	Status        string
	LinkedAt      time.Time
	UpdatedAt     time.Time
}

// SessionCipher encrypts secret OAuth state before persistence.
type SessionCipher struct {
	aead cipher.AEAD
}

// NewSessionCipherFromBase64 constructs a dedicated AES-256-GCM cipher.
func NewSessionCipherFromBase64(encoded string) (*SessionCipher, error) {
	key, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return nil, fmt.Errorf("decode AT Protocol session encryption key: %w", err)
	}
	if len(key) != 32 {
		return nil, errors.New("AT Protocol session encryption key must decode to 32 bytes")
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("create AT Protocol session cipher: %w", err)
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("create AT Protocol session AEAD: %w", err)
	}
	return &SessionCipher{aead: aead}, nil
}

func (c *SessionCipher) encrypt(value any) ([]byte, error) {
	plaintext, err := json.Marshal(value)
	if err != nil {
		return nil, fmt.Errorf("marshal OAuth state: %w", err)
	}
	nonce := make([]byte, c.aead.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, fmt.Errorf("generate OAuth encryption nonce: %w", err)
	}
	return c.aead.Seal(nonce, nonce, plaintext, nil), nil
}

func (c *SessionCipher) decrypt(data []byte, value any) error {
	nonceSize := c.aead.NonceSize()
	if len(data) < nonceSize {
		return errors.New("encrypted OAuth state is truncated")
	}
	plaintext, err := c.aead.Open(nil, data[:nonceSize], data[nonceSize:], nil)
	if err != nil {
		return fmt.Errorf("decrypt OAuth state: %w", err)
	}
	if err := json.Unmarshal(plaintext, value); err != nil {
		return fmt.Errorf("unmarshal OAuth state: %w", err)
	}
	return nil
}

// SQLStore implements Indigo persistence and the local identity-link ledger.
type SQLStore struct {
	db     *sql.DB
	cipher *SessionCipher
	now    func() time.Time
}

// NewSQLStore creates durable, encrypted Indigo OAuth persistence.
func NewSQLStore(db *sql.DB, sessionCipher *SessionCipher) *SQLStore {
	return &SQLStore{db: db, cipher: sessionCipher, now: time.Now}
}

func stateHash(state string) string {
	sum := sha256.Sum256([]byte(state))
	return hex.EncodeToString(sum[:])
}

// GetSession implements oauth.ClientAuthStore.
func (s *SQLStore) GetSession(ctx context.Context, did syntax.DID, sessionID string) (*indigooauth.ClientSessionData, error) {
	var encrypted []byte
	err := s.db.QueryRowContext(ctx, `
		SELECT encrypted_session FROM atproto_oauth_sessions
		WHERE account_did = $1 AND session_id = $2
	`, did.String(), sessionID).Scan(&encrypted)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrOAuthSessionNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("load AT Protocol OAuth session: %w", err)
	}
	var session indigooauth.ClientSessionData
	if err := s.cipher.decrypt(encrypted, &session); err != nil {
		return nil, err
	}
	return &session, nil
}

// SaveSession implements oauth.ClientAuthStore.
func (s *SQLStore) SaveSession(ctx context.Context, session indigooauth.ClientSessionData) error {
	encrypted, err := s.cipher.encrypt(session)
	if err != nil {
		return err
	}
	_, err = s.db.ExecContext(ctx, `
		INSERT INTO atproto_oauth_sessions (
			account_did, session_id, encrypted_session, updated_at
		) VALUES ($1, $2, $3, $4)
		ON CONFLICT (account_did, session_id) DO UPDATE
		SET encrypted_session = EXCLUDED.encrypted_session,
		    updated_at = EXCLUDED.updated_at
	`, session.AccountDID.String(), session.SessionID, encrypted, s.now().UTC())
	if err != nil {
		return fmt.Errorf("save AT Protocol OAuth session: %w", err)
	}
	return nil
}

// DeleteSession implements oauth.ClientAuthStore.
func (s *SQLStore) DeleteSession(ctx context.Context, did syntax.DID, sessionID string) error {
	_, err := s.db.ExecContext(ctx, `
		DELETE FROM atproto_oauth_sessions
		WHERE account_did = $1 AND session_id = $2
	`, did.String(), sessionID)
	if err != nil {
		return fmt.Errorf("delete AT Protocol OAuth session: %w", err)
	}
	return nil
}

// GetAuthRequestInfo implements oauth.ClientAuthStore.
func (s *SQLStore) GetAuthRequestInfo(ctx context.Context, state string) (*indigooauth.AuthRequestData, error) {
	var encrypted []byte
	var expiresAt time.Time
	var consumedAt sql.NullTime
	err := s.db.QueryRowContext(ctx, `
		SELECT encrypted_request, expires_at, consumed_at
		FROM atproto_oauth_requests WHERE state_hash = $1
	`, stateHash(state)).Scan(&encrypted, &expiresAt, &consumedAt)
	if errors.Is(err, sql.ErrNoRows) || consumedAt.Valid || !expiresAt.After(s.now()) {
		return nil, ErrOAuthRequestNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("load AT Protocol OAuth request: %w", err)
	}
	var request indigooauth.AuthRequestData
	if err := s.cipher.decrypt(encrypted, &request); err != nil {
		return nil, err
	}
	return &request, nil
}

// SaveAuthRequestInfo implements oauth.ClientAuthStore.
func (s *SQLStore) SaveAuthRequestInfo(ctx context.Context, request indigooauth.AuthRequestData) error {
	flow, ok := ctx.Value(flowContextKey{}).(FlowContext)
	if !ok || flow.UserID == "" {
		return errors.New("local OAuth flow context is required")
	}
	encrypted, err := s.cipher.encrypt(request)
	if err != nil {
		return err
	}
	if flow.ReturnPath == "" {
		flow.ReturnPath = "/settings"
	}
	_, err = s.db.ExecContext(ctx, `
		INSERT INTO atproto_oauth_requests (
			state_hash, user_id, encrypted_request, return_path,
			requested_scope, expires_at
		) VALUES ($1, $2::uuid, $3, $4, $5, $6)
	`, stateHash(request.State), flow.UserID, encrypted, flow.ReturnPath,
		flow.RequestedScope, s.now().UTC().Add(10*time.Minute))
	if err != nil {
		return fmt.Errorf("save AT Protocol OAuth request: %w", err)
	}
	return nil
}

// DeleteAuthRequestInfo implements oauth.ClientAuthStore.
func (s *SQLStore) DeleteAuthRequestInfo(ctx context.Context, state string) error {
	_, err := s.db.ExecContext(ctx, `
		UPDATE atproto_oauth_requests SET consumed_at = $2
		WHERE state_hash = $1 AND consumed_at IS NULL
	`, stateHash(state), s.now().UTC())
	if err != nil {
		return fmt.Errorf("consume AT Protocol OAuth request: %w", err)
	}
	return nil
}

// RequestContext resolves the local user and safe return path for a callback.
func (s *SQLStore) RequestContext(ctx context.Context, state string) (FlowContext, error) {
	var flow FlowContext
	err := s.db.QueryRowContext(ctx, `
		SELECT user_id::text, return_path, requested_scope
		FROM atproto_oauth_requests
		WHERE state_hash = $1 AND consumed_at IS NULL AND expires_at > $2
	`, stateHash(state), s.now().UTC()).Scan(&flow.UserID, &flow.ReturnPath, &flow.RequestedScope)
	if errors.Is(err, sql.ErrNoRows) {
		return FlowContext{}, ErrOAuthRequestNotFound
	}
	if err != nil {
		return FlowContext{}, fmt.Errorf("load local OAuth flow context: %w", err)
	}
	return flow, nil
}

// SaveLink transactionally enforces one DID per user and one user per DID.
func (s *SQLStore) SaveLink(ctx context.Context, userID, handle string, session indigooauth.ClientSessionData) (Link, error) {
	now := s.now().UTC()
	var link Link
	err := s.db.QueryRowContext(ctx, `
		INSERT INTO atproto_oauth_links (
			user_id, account_did, handle, session_id, host_url,
			granted_scopes, status, linked_at, updated_at
		) VALUES ($1::uuid, $2, NULLIF($3, ''), $4, $5, $6, 'active', $7, $7)
		ON CONFLICT (user_id) DO UPDATE
		SET account_did = EXCLUDED.account_did,
		    handle = EXCLUDED.handle,
		    session_id = EXCLUDED.session_id,
		    host_url = EXCLUDED.host_url,
		    granted_scopes = EXCLUDED.granted_scopes,
		    status = 'active',
		    updated_at = EXCLUDED.updated_at,
		    revoked_at = NULL
		WHERE atproto_oauth_links.account_did = EXCLUDED.account_did
		RETURNING user_id::text, account_did, COALESCE(handle, ''), session_id,
		          host_url, granted_scopes, status, linked_at, updated_at
	`, userID, session.AccountDID.String(), handle, session.SessionID,
		session.HostURL, pq.Array(session.Scopes), now).Scan(
		&link.UserID, &link.AccountDID, &link.Handle, &link.SessionID,
		&link.HostURL, pq.Array(&link.GrantedScopes), &link.Status,
		&link.LinkedAt, &link.UpdatedAt,
	)
	if err != nil {
		if pqErr, ok := err.(*pq.Error); ok && pqErr.Code == "23505" {
			return Link{}, ErrLinkConflict
		}
		if errors.Is(err, sql.ErrNoRows) {
			return Link{}, ErrLinkConflict
		}
		return Link{}, fmt.Errorf("save AT Protocol link: %w", err)
	}
	_, _ = s.db.ExecContext(ctx, `UPDATE atproto_provisioning_requests
		SET status='consumed',updated_at=$3
		WHERE user_id=$1::uuid AND lower(handle)=lower($2) AND status='issued'`, userID, handle, now)
	return link, nil
}

// LinkForUser returns the user's current external identity.
func (s *SQLStore) LinkForUser(ctx context.Context, userID string) (Link, error) {
	var link Link
	err := s.db.QueryRowContext(ctx, `
		SELECT user_id::text, account_did, COALESCE(handle, ''), session_id,
		       host_url, granted_scopes, status, linked_at, updated_at
		FROM atproto_oauth_links WHERE user_id = $1::uuid
	`, userID).Scan(
		&link.UserID, &link.AccountDID, &link.Handle, &link.SessionID,
		&link.HostURL, pq.Array(&link.GrantedScopes), &link.Status,
		&link.LinkedAt, &link.UpdatedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return Link{}, ErrOAuthSessionNotFound
	}
	if err != nil {
		return Link{}, fmt.Errorf("load AT Protocol link: %w", err)
	}
	return link, nil
}

// RevokeLink marks the local link revoked without deleting public PDS records.
func (s *SQLStore) RevokeLink(ctx context.Context, userID string) (Link, error) {
	var link Link
	now := s.now().UTC()
	err := s.db.QueryRowContext(ctx, `
		UPDATE atproto_oauth_links
		SET status = 'revoked', revoked_at = $2, updated_at = $2
		WHERE user_id = $1::uuid AND status = 'active'
		RETURNING user_id::text, account_did, COALESCE(handle, ''), session_id,
		          host_url, granted_scopes, status, linked_at, updated_at
	`, userID, now).Scan(
		&link.UserID, &link.AccountDID, &link.Handle, &link.SessionID,
		&link.HostURL, pq.Array(&link.GrantedScopes), &link.Status,
		&link.LinkedAt, &link.UpdatedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return Link{}, ErrOAuthSessionNotFound
	}
	if err != nil {
		return Link{}, fmt.Errorf("revoke AT Protocol link: %w", err)
	}
	return link, nil
}

var _ indigooauth.ClientAuthStore = (*SQLStore)(nil)
