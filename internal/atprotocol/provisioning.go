package atprotocol

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"
)

var (
	ErrProvisioningDisabled      = errors.New("AT Protocol account provisioning is disabled")
	ErrProvisioningLimit         = errors.New("AT Protocol account provisioning limit reached")
	ErrProvisioningConflict      = errors.New("AT Protocol account or handle already provisioned")
	ErrTurnstileFailed           = errors.New("human verification failed")
	ErrEmailVerificationRequired = errors.New("verified Subcults email is required")
)

var handleLabelPattern = regexp.MustCompile("^[a-z0-9](?:[a-z0-9-]{1,30}[a-z0-9])?$")

var reservedHandleLabels = map[string]struct{}{
	"admin": {}, "api": {}, "app": {}, "auth": {}, "help": {}, "mail": {},
	"moderation": {}, "pds": {}, "root": {}, "security": {}, "status": {},
	"subcult": {}, "subcults": {}, "support": {}, "system": {}, "www": {},
}

// ProvisioningConfig configures guarded PDS account invitation issuance.
type ProvisioningConfig struct {
	Enabled          bool
	DefaultPDSURL    string
	ProvisionerURL   string
	ProvisionerToken string
	TurnstileSecret  string
	TermsVersion     string
	HandleDomain     string
	DailyCap         int
}

// ProvisioningRequest is persisted without the invitation plaintext.
type ProvisioningRequest struct {
	ID               string    `json:"id"`
	UserID           string    `json:"-"`
	Handle           string    `json:"handle"`
	Status           string    `json:"status"`
	TermsVersion     string    `json:"terms_version"`
	TurnstileOutcome string    `json:"turnstile_outcome"`
	ExpiresAt        time.Time `json:"expires_at,omitempty"`
	CreatedAt        time.Time `json:"created_at"`
	UpdatedAt        time.Time `json:"updated_at"`
}

// ProvisioningResult gives the browser account creation configuration.
type ProvisioningResult struct {
	RequestID      string    `json:"request_id"`
	Handle         string    `json:"handle"`
	InviteCode     string    `json:"invite_code"`
	PDSURL         string    `json:"pds_url"`
	CreateEndpoint string    `json:"create_endpoint"`
	ExpiresAt      time.Time `json:"expires_at"`
}

// ProvisioningService verifies abuse controls and calls the private provisioner.
type ProvisioningService struct {
	store  *SQLStore
	config ProvisioningConfig
	client *http.Client
	now    func() time.Time
}

// NewProvisioningService creates guarded account provisioning.
func NewProvisioningService(store *SQLStore, config ProvisioningConfig) (*ProvisioningService, error) {
	if store == nil {
		return nil, errors.New("AT Protocol store is required")
	}
	if config.HandleDomain == "" {
		config.HandleDomain = "subcult.tv"
	}
	if config.DailyCap <= 0 {
		config.DailyCap = 25
	}
	if config.TermsVersion == "" {
		return nil, errors.New("AT Protocol provisioning terms version is required")
	}
	if config.Enabled && (config.DefaultPDSURL == "" || config.ProvisionerURL == "" ||
		config.ProvisionerToken == "" || config.TurnstileSecret == "") {
		return nil, errors.New("enabled AT Protocol provisioning requires PDS, provisioner, and Turnstile configuration")
	}
	return &ProvisioningService{
		store: store, config: config,
		client: &http.Client{Timeout: 10 * time.Second}, now: time.Now,
	}, nil
}

// Provision verifies Turnstile, enforces transactional limits, and issues one invite.
func (s *ProvisioningService) Provision(ctx context.Context, userID, label, turnstileToken, remoteIP string) (ProvisioningResult, error) {
	if !s.config.Enabled {
		return ProvisioningResult{}, ErrProvisioningDisabled
	}
	handle, err := normalizeProvisionedHandle(label, s.config.HandleDomain)
	if err != nil {
		return ProvisioningResult{}, err
	}
	passed, err := s.verifyTurnstile(ctx, turnstileToken, remoteIP)
	if err != nil || !passed {
		return ProvisioningResult{}, ErrTurnstileFailed
	}
	ipHash := keyedDigest(s.config.ProvisionerToken, canonicalIP(remoteIP))
	request, err := s.store.CreateProvisioningRequest(ctx, userID, handle, ipHash, s.config.TermsVersion, s.config.DailyCap)
	if err != nil {
		return ProvisioningResult{}, err
	}
	invite, err := s.issueInvite(ctx, userID, handle)
	if err != nil {
		_ = s.store.RejectProvisioningRequest(ctx, request.ID)
		return ProvisioningResult{}, fmt.Errorf("issue PDS invitation: %w", err)
	}
	expiresAt := s.now().UTC().Add(10 * time.Minute)
	if err := s.store.MarkProvisioningIssued(ctx, request.ID, keyedDigest(s.config.ProvisionerToken, invite), expiresAt); err != nil {
		return ProvisioningResult{}, err
	}
	pdsURL := strings.TrimRight(s.config.DefaultPDSURL, "/")
	return ProvisioningResult{
		RequestID: request.ID, Handle: handle, InviteCode: invite,
		PDSURL: pdsURL, CreateEndpoint: pdsURL + "/xrpc/com.atproto.server.createAccount",
		ExpiresAt: expiresAt,
	}, nil
}

// Status returns the latest provisioning request for the local user.
func (s *ProvisioningService) Status(ctx context.Context, userID string) (ProvisioningRequest, error) {
	return s.store.ProvisioningStatus(ctx, userID)
}

func (s *ProvisioningService) verifyTurnstile(ctx context.Context, token, remoteIP string) (bool, error) {
	if strings.TrimSpace(token) == "" {
		return false, nil
	}
	form := url.Values{}
	form.Set("secret", s.config.TurnstileSecret)
	form.Set("response", token)
	if ip := canonicalIP(remoteIP); ip != "" {
		form.Set("remoteip", ip)
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost,
		"https://challenges.cloudflare.com/turnstile/v0/siteverify",
		strings.NewReader(form.Encode()))
	if err != nil {
		return false, err
	}
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	response, err := s.client.Do(request)
	if err != nil {
		return false, err
	}
	defer response.Body.Close()
	var result struct {
		Success bool `json:"success"`
	}
	if err := json.NewDecoder(response.Body).Decode(&result); err != nil {
		return false, err
	}
	return response.StatusCode == http.StatusOK && result.Success, nil
}

func (s *ProvisioningService) issueInvite(ctx context.Context, userID, handle string) (string, error) {
	body, err := json.Marshal(map[string]any{
		"user_id": userID, "handle": handle, "expires_in_seconds": 600,
	})
	if err != nil {
		return "", err
	}
	endpoint := strings.TrimRight(s.config.ProvisionerURL, "/") + "/v1/invites"
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	request.Header.Set("Authorization", "Bearer "+s.config.ProvisionerToken)
	request.Header.Set("Content-Type", "application/json")
	response, err := s.client.Do(request)
	if err != nil {
		return "", err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusCreated {
		return "", fmt.Errorf("private provisioner returned HTTP %d", response.StatusCode)
	}
	var result struct {
		Code string `json:"code"`
	}
	if err := json.NewDecoder(response.Body).Decode(&result); err != nil {
		return "", err
	}
	if strings.TrimSpace(result.Code) == "" {
		return "", errors.New("private provisioner returned an empty invitation")
	}
	return result.Code, nil
}

func normalizeProvisionedHandle(label, domain string) (string, error) {
	label = strings.ToLower(strings.TrimSpace(label))
	label = strings.TrimSuffix(label, "."+domain)
	if len(label) < 3 || len(label) > 32 || !handleLabelPattern.MatchString(label) {
		return "", errors.New("handle must be 3-32 lowercase letters, numbers, or interior hyphens")
	}
	if _, reserved := reservedHandleLabels[label]; reserved {
		return "", errors.New("handle is reserved")
	}
	return label + "." + domain, nil
}

func keyedDigest(key, value string) string {
	mac := hmac.New(sha256.New, []byte(key))
	_, _ = mac.Write([]byte(value))
	return hex.EncodeToString(mac.Sum(nil))
}

func canonicalIP(value string) string {
	value = strings.TrimSpace(strings.Split(value, ",")[0])
	host, _, err := net.SplitHostPort(value)
	if err == nil {
		value = host
	}
	parsed := net.ParseIP(strings.Trim(value, "[]"))
	if parsed == nil {
		return ""
	}
	return parsed.String()
}
