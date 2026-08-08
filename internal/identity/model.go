package identity

import (
	"errors"
	"time"
)

var (
	ErrInvalidEmail      = errors.New("invalid email address")
	ErrInvalidReturnPath = errors.New("invalid return path")
	ErrInvalidToken      = errors.New("invalid or expired magic link")
	ErrInvalidSession    = errors.New("invalid or expired session")
	ErrHandleUnavailable = errors.New("handle is unavailable")
)

type User struct {
	ID                 string
	InternalDID        string
	Handle             string
	DisplayName        string
	Role               string
	OnboardingComplete bool
}

type EmailIdentity struct {
	ID             string
	UserID         string
	EncryptedEmail []byte
	EmailHMAC      string
	VerifiedAt     *time.Time
}

type MagicLink struct {
	IdentityID string
	TokenHash  string
	ReturnPath string
	ExpiresAt  time.Time
}

type SessionResult struct {
	User       User
	SessionID  string
	ReturnPath string
}

type AuthResult struct {
	User         User
	AccessToken  string
	RefreshToken string
	ReturnPath   string
}

type CreatorAccessRequest struct {
	ID           string     `json:"id"`
	UserID       string     `json:"user_id"`
	Statement    string     `json:"statement"`
	Status       string     `json:"status"`
	ReviewNote   string     `json:"review_note,omitempty"`
	CreatedAt    time.Time  `json:"created_at"`
	ReviewedAt   *time.Time `json:"reviewed_at,omitempty"`
	ReviewedByID string     `json:"reviewed_by_user_id,omitempty"`
}
