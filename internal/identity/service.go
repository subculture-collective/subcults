package identity

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"net/mail"
	"net/url"
	"path"
	"strings"
	"time"

	"github.com/onnwee/subcults/internal/auth"
)

const (
	magicLinkLifetime     = 15 * time.Minute
	sessionIdleLifetime   = 7 * 24 * time.Hour
	sessionAbsoluteExpiry = 30 * 24 * time.Hour
)

type Service struct {
	repository Repository
	mailer     Mailer
	protector  *ContactProtector
	jwt        *auth.JWTService
	publicURL  string
	now        func() time.Time
	random     func(int) (string, error)
}

func NewService(repository Repository, mailer Mailer, protector *ContactProtector, jwt *auth.JWTService, publicURL string) (*Service, error) {
	parsed, err := url.Parse(strings.TrimRight(publicURL, "/"))
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return nil, errors.New("public web URL must be absolute")
	}
	return &Service{
		repository: repository,
		mailer:     mailer,
		protector:  protector,
		jwt:        jwt,
		publicURL:  strings.TrimRight(publicURL, "/"),
		now:        time.Now,
		random:     randomToken,
	}, nil
}

func (s *Service) RequestMagicLink(ctx context.Context, rawEmail, returnPath string) error {
	address, err := mail.ParseAddress(strings.TrimSpace(rawEmail))
	if err != nil || !strings.Contains(address.Address, "@") {
		return ErrInvalidEmail
	}
	email := strings.ToLower(address.Address)
	returnPath, err = safeReturnPath(returnPath)
	if err != nil {
		return err
	}
	encrypted, emailHMAC, err := s.protector.Protect(email)
	if err != nil {
		return err
	}
	identity, err := s.repository.UpsertEmailIdentity(ctx, encrypted, emailHMAC)
	if err != nil {
		return err
	}
	token, err := s.random(32)
	if err != nil {
		return err
	}
	if err := s.repository.CreateMagicLink(ctx, MagicLink{
		IdentityID: identity.ID,
		TokenHash:  HashToken(token),
		ReturnPath: returnPath,
		ExpiresAt:  s.now().Add(magicLinkLifetime),
	}); err != nil {
		return err
	}
	verificationURL := s.publicURL + "/auth/verify?token=" + url.QueryEscape(token)
	return s.mailer.SendMagicLink(ctx, email, verificationURL)
}

func (s *Service) VerifyMagicLink(ctx context.Context, token, userAgent string) (AuthResult, error) {
	if strings.TrimSpace(token) == "" {
		return AuthResult{}, ErrInvalidToken
	}
	refreshToken, err := s.random(32)
	if err != nil {
		return AuthResult{}, err
	}
	now := s.now()
	result, err := s.repository.CompleteMagicLink(
		ctx, HashToken(token), HashToken(refreshToken), HashToken(userAgent), now,
		now.Add(sessionIdleLifetime), now.Add(sessionAbsoluteExpiry),
	)
	if err != nil {
		return AuthResult{}, err
	}
	accessToken, err := s.jwt.GenerateAccessToken(result.User.ID, result.User.InternalDID)
	if err != nil {
		return AuthResult{}, err
	}
	return AuthResult{User: result.User, AccessToken: accessToken, RefreshToken: refreshToken, ReturnPath: result.ReturnPath}, nil
}

func (s *Service) Refresh(ctx context.Context, refreshToken, userAgent string) (AuthResult, error) {
	if strings.TrimSpace(refreshToken) == "" {
		return AuthResult{}, ErrInvalidSession
	}
	newToken, err := s.random(32)
	if err != nil {
		return AuthResult{}, err
	}
	now := s.now()
	result, err := s.repository.RotateSession(ctx, HashToken(refreshToken), HashToken(newToken), HashToken(userAgent), now, now.Add(sessionIdleLifetime))
	if err != nil {
		return AuthResult{}, err
	}
	accessToken, err := s.jwt.GenerateAccessToken(result.User.ID, result.User.InternalDID)
	if err != nil {
		return AuthResult{}, err
	}
	return AuthResult{User: result.User, AccessToken: accessToken, RefreshToken: newToken}, nil
}

func (s *Service) CompleteProfile(ctx context.Context, userID, handle, displayName string) (User, error) {
	handle = strings.ToLower(strings.TrimSpace(handle))
	displayName = strings.TrimSpace(displayName)
	if len(handle) < 3 || len(handle) > 32 || len(displayName) < 1 || len(displayName) > 80 {
		return User{}, ErrHandleUnavailable
	}
	for _, char := range handle {
		if (char < 'a' || char > 'z') && (char < '0' || char > '9') && char != '-' && char != '_' {
			return User{}, ErrHandleUnavailable
		}
	}
	return s.repository.CompleteProfile(ctx, userID, handle, displayName)
}

func (s *Service) GetUser(ctx context.Context, userID string) (User, error) {
	return s.repository.GetUser(ctx, userID)
}

func (s *Service) Logout(ctx context.Context, refreshToken string) error {
	if strings.TrimSpace(refreshToken) == "" {
		return nil
	}
	return s.repository.RevokeSession(ctx, HashToken(refreshToken), s.now())
}

func (s *Service) LogoutAll(ctx context.Context, userID string) error {
	return s.repository.RevokeAllSessions(ctx, userID, s.now())
}

func (s *Service) RequestCreatorAccess(ctx context.Context, userID, statement string) (CreatorAccessRequest, error) {
	statement = strings.TrimSpace(statement)
	if len(statement) < 20 || len(statement) > 2000 {
		return CreatorAccessRequest{}, errors.New("creator statement must contain 20 to 2000 characters")
	}
	return s.repository.CreateCreatorAccessRequest(ctx, userID, statement, s.now())
}

func (s *Service) GetCreatorAccess(ctx context.Context, userID string) (CreatorAccessRequest, error) {
	return s.repository.GetCreatorAccessRequest(ctx, userID)
}

func (s *Service) ReviewCreatorAccess(ctx context.Context, requestID, reviewerID, status, note string) (CreatorAccessRequest, error) {
	reviewer, err := s.repository.GetUser(ctx, reviewerID)
	if err != nil || reviewer.Role != "admin" {
		return CreatorAccessRequest{}, errors.New("administrator role required")
	}
	if status != "approved" && status != "rejected" {
		return CreatorAccessRequest{}, errors.New("review status must be approved or rejected")
	}
	return s.repository.ReviewCreatorAccessRequest(ctx, requestID, reviewerID, status, strings.TrimSpace(note), s.now())
}

func (s *Service) ListCreatorAccess(ctx context.Context, reviewerID, status string) ([]CreatorAccessRequest, error) {
	reviewer, err := s.repository.GetUser(ctx, reviewerID)
	if err != nil || reviewer.Role != "admin" {
		return nil, errors.New("administrator role required")
	}
	if status != "" && status != "pending" && status != "approved" && status != "rejected" && status != "withdrawn" {
		return nil, errors.New("invalid creator request status")
	}
	return s.repository.ListCreatorAccessRequests(ctx, status)
}

func safeReturnPath(value string) (string, error) {
	if value == "" {
		return "/", nil
	}
	parsed, err := url.Parse(value)
	if err != nil || parsed.IsAbs() || parsed.Host != "" || !strings.HasPrefix(parsed.Path, "/") || strings.HasPrefix(parsed.Path, "//") {
		return "", ErrInvalidReturnPath
	}
	parsed.Path = path.Clean(parsed.Path)
	return parsed.String(), nil
}

func randomToken(size int) (string, error) {
	value := make([]byte, size)
	if _, err := rand.Read(value); err != nil {
		return "", fmt.Errorf("generate token: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(value), nil
}
