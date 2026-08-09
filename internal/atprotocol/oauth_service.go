package atprotocol

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/bluesky-social/indigo/atproto/atcrypto"
	indigooauth "github.com/bluesky-social/indigo/atproto/auth/oauth"
	"github.com/bluesky-social/indigo/atproto/syntax"
)

// OAuthConfig defines the public confidential-client contract.
type OAuthConfig struct {
	ClientID      string
	CallbackURL   string
	JWKSURL       string
	PrivateKey    string
	KeyID         string
	PublicWebURL  string
	DefaultPDSURL string
	ClientName    string
	PrivacyURL    string
	TermsURL      string
}

// OAuthService coordinates Indigo OAuth with the local identity link ledger.
type OAuthService struct {
	store             *SQLStore
	linkApp           *indigooauth.ClientApp
	publishApp        *indigooauth.ClientApp
	config            OAuthConfig
	publicationLocker PublicationLocker
	tidClock          syntax.TIDClock
	tapURL            string
	tapPassword       string
}

// ConfigureTap enables private repository registration after linking.
func (s *OAuthService) ConfigureTap(baseURL, password string) {
	s.tapURL = strings.TrimRight(strings.TrimSpace(baseURL), "/")
	s.tapPassword = password
}

// NewOAuthService constructs identity-only and publication-scope OAuth clients.
func NewOAuthService(config OAuthConfig, store *SQLStore) (*OAuthService, error) {
	if store == nil {
		return nil, errors.New("AT Protocol OAuth store is required")
	}
	if config.ClientID == "" || config.CallbackURL == "" || config.JWKSURL == "" {
		return nil, errors.New("AT Protocol OAuth public URLs are required")
	}
	if config.PrivateKey == "" || config.KeyID == "" {
		return nil, errors.New("AT Protocol OAuth confidential client key is required")
	}
	privateKey, err := atcrypto.ParsePrivateMultibase(config.PrivateKey)
	if err != nil {
		return nil, fmt.Errorf("parse AT Protocol OAuth client key: %w", err)
	}
	linkConfig := indigooauth.NewPublicConfig(config.ClientID, config.CallbackURL, []string{"atproto"})
	linkConfig.UserAgent = "subcults/atproto-oauth"
	if err := linkConfig.SetClientSecret(privateKey, config.KeyID); err != nil {
		return nil, fmt.Errorf("configure AT Protocol OAuth client key: %w", err)
	}
	publishConfig := indigooauth.NewPublicConfig(config.ClientID, config.CallbackURL, strings.Fields(PublishingScope()))
	publishConfig.UserAgent = linkConfig.UserAgent
	if err := publishConfig.SetClientSecret(privateKey, config.KeyID); err != nil {
		return nil, fmt.Errorf("configure AT Protocol publishing client key: %w", err)
	}
	return &OAuthService{
		store: store, linkApp: indigooauth.NewClientApp(&linkConfig, store),
		publishApp: indigooauth.NewClientApp(&publishConfig, store), config: config,
		tidClock: syntax.NewTIDClock(0),
	}, nil
}

// ClientMetadata returns the discoverable OAuth client document.
func (s *OAuthService) ClientMetadata() indigooauth.ClientMetadata {
	metadata := s.publishApp.Config.ClientMetadata()
	metadata.JWKSURI = &s.config.JWKSURL
	name := s.config.ClientName
	if name == "" {
		name = "Subcults"
	}
	metadata.ClientName = &name
	if s.config.PublicWebURL != "" {
		metadata.ClientURI = &s.config.PublicWebURL
	}
	if s.config.PrivacyURL != "" {
		metadata.PolicyURI = &s.config.PrivacyURL
	}
	if s.config.TermsURL != "" {
		metadata.TosURI = &s.config.TermsURL
	}
	return metadata
}

// PublicJWKS returns the public half of the client attestation key.
func (s *OAuthService) PublicJWKS() indigooauth.JWKS {
	return s.publishApp.Config.PublicJWKS()
}

// Start begins either an identity-only link or a publication permission upgrade.
func (s *OAuthService) Start(ctx context.Context, userID, identifier, returnPath string, publishing bool) (string, error) {
	if userID == "" {
		return "", errors.New("authenticated user is required")
	}
	identifier = strings.TrimSpace(identifier)
	if strings.HasPrefix(identifier, "http://") || strings.HasPrefix(identifier, "https://") {
		return "", errors.New("use an AT Protocol handle or DID, not a server URL")
	}
	if _, err := syntax.ParseAtIdentifier(identifier); err != nil {
		return "", errors.New("valid AT Protocol handle or DID is required")
	}
	if !safeReturnPath(returnPath) {
		return "", errors.New("return path must be a local absolute path")
	}
	app := s.linkApp
	scope := "atproto"
	if publishing {
		app = s.publishApp
		scope = PublishingScope()
	}
	flowCtx := WithFlowContext(ctx, FlowContext{
		UserID: userID, ReturnPath: returnPath, RequestedScope: scope,
	})
	redirect, err := app.StartAuthFlow(flowCtx, identifier)
	if err != nil {
		return "", fmt.Errorf("start AT Protocol OAuth flow: %w", err)
	}
	return redirect, nil
}

// Callback completes OAuth and links the independently verified DID.
func (s *OAuthService) Callback(ctx context.Context, values url.Values) (Link, string, error) {
	state := values.Get("state")
	flow, err := s.store.RequestContext(ctx, state)
	if err != nil {
		return Link{}, "", err
	}
	session, err := s.publishApp.ProcessCallback(ctx, values)
	if err != nil {
		return Link{}, flow.ReturnPath, fmt.Errorf("complete AT Protocol OAuth flow: %w", err)
	}
	handle := ""
	identity, err := s.publishApp.Dir.LookupDID(ctx, session.AccountDID)
	if err != nil {
		return Link{}, flow.ReturnPath, fmt.Errorf("verify linked AT Protocol identity: %w", err)
	}
	if !identity.Handle.IsInvalidHandle() {
		handle = identity.Handle.String()
	}
	link, err := s.store.SaveLink(ctx, flow.UserID, handle, *session)
	if err != nil {
		return Link{}, flow.ReturnPath, err
	}
	if err := s.registerTapRepo(ctx, link.AccountDID); err != nil {
		_ = s.store.RecordOperationalFailure(ctx, "tap", link.AccountDID, "tap_registration", "linked repository could not be registered with Tap")
	}
	return link, flow.ReturnPath, nil
}

func (s *OAuthService) registerTapRepo(ctx context.Context, did string) error {
	if s.tapURL == "" || s.tapPassword == "" {
		return nil
	}
	body, _ := json.Marshal(map[string][]string{"dids": {did}})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, s.tapURL+"/repos/add", bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.SetBasicAuth("admin", s.tapPassword)
	resp, err := (&http.Client{Timeout: 10 * time.Second}).Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("Tap registration returned HTTP %d", resp.StatusCode)
	}
	return nil
}

// Status returns a user's current link.
func (s *OAuthService) Status(ctx context.Context, userID string) (Link, error) {
	return s.store.LinkForUser(ctx, userID)
}

// Upgrade starts the exact publication-scope flow for an existing link.
func (s *OAuthService) Upgrade(ctx context.Context, userID, returnPath string) (string, error) {
	link, err := s.store.LinkForUser(ctx, userID)
	if err != nil {
		return "", err
	}
	return s.Start(ctx, userID, link.AccountDID, returnPath, true)
}

// Unlink revokes the OAuth session and retains local drafts and public records.
func (s *OAuthService) Unlink(ctx context.Context, userID string) error {
	link, err := s.store.LinkForUser(ctx, userID)
	if err != nil {
		return err
	}
	did, err := syntax.ParseDID(link.AccountDID)
	if err != nil {
		return fmt.Errorf("parse linked DID: %w", err)
	}
	if err := s.publishApp.Logout(ctx, did, link.SessionID); err != nil &&
		!errors.Is(err, ErrOAuthSessionNotFound) {
		return fmt.Errorf("revoke AT Protocol OAuth session: %w", err)
	}
	_, err = s.store.RevokeLink(ctx, userID)
	return err
}

func safeReturnPath(path string) bool {
	return strings.HasPrefix(path, "/") && !strings.HasPrefix(path, "//") &&
		!strings.ContainsAny(path, "\r\n")
}
