// Package auth provides authentication utilities for JWT token management.
package auth

import (
	"errors"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// Token type constants for the typ claim.
const (
	TokenTypeAccess  = "access"
	TokenTypeRefresh = "refresh"
)

// Token expiration durations.
const (
	AccessTokenExpiry  = 15 * time.Minute
	RefreshTokenExpiry = 7 * 24 * time.Hour
)

// Default leeway for token validation.
const DefaultLeeway = 30 * time.Second

// ErrInvalidToken is returned when token validation fails.
var ErrInvalidToken = errors.New("invalid token")

// ErrExpiredToken is returned when the token has expired.
var ErrExpiredToken = errors.New("token has expired")

// ErrEmptyUserID is returned when userID is empty.
var ErrEmptyUserID = errors.New("userID cannot be empty")

// Claims represents custom JWT claims for the application.
type Claims struct {
	jwt.RegisteredClaims
	DID  string `json:"did,omitempty"` // Decentralized Identifier (for access tokens)
	Type string `json:"typ"`           // Token type: "access" or "refresh"
}

// JWTService handles JWT token operations.
type JWTService struct {
	secret []byte
	leeway time.Duration
}

// NewJWTService creates a new JWTService with the given secret.
func NewJWTService(secret string) *JWTService {
	return &JWTService{
		secret: []byte(secret),
		leeway: DefaultLeeway,
	}
}

// NewJWTServiceWithLeeway creates a new JWTService with custom leeway.
func NewJWTServiceWithLeeway(secret string, leeway time.Duration) *JWTService {
	return &JWTService{
		secret: []byte(secret),
		leeway: leeway,
	}
}

// GenerateAccessToken creates a new access token (15m expiry) with userID and DID.
func (s *JWTService) GenerateAccessToken(userID, did string) (string, error) {
	if userID == "" {
		return "", ErrEmptyUserID
	}

	now := time.Now()
	claims := Claims{
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   userID,
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(AccessTokenExpiry)),
		},
		DID:  did,
		Type: TokenTypeAccess,
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(s.secret)
}

// GenerateRefreshToken creates a new refresh token (7d expiry) with userID.
func (s *JWTService) GenerateRefreshToken(userID string) (string, error) {
	if userID == "" {
		return "", ErrEmptyUserID
	}

	now := time.Now()
	claims := Claims{
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   userID,
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(RefreshTokenExpiry)),
		},
		Type: TokenTypeRefresh,
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(s.secret)
}

// ValidateToken parses and validates a JWT token, returning the claims if valid.
func (s *JWTService) ValidateToken(tokenString string) (*Claims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &Claims{}, func(token *jwt.Token) (interface{}, error) {
		// Validate the signing method is HS256
		if token.Method.Alg() != jwt.SigningMethodHS256.Alg() {
			return nil, ErrInvalidToken
		}
		return s.secret, nil
	}, jwt.WithLeeway(s.leeway))

	if err != nil {
		if errors.Is(err, jwt.ErrTokenExpired) {
			return nil, ErrExpiredToken
		}
		return nil, ErrInvalidToken
	}

	claims, ok := token.Claims.(*Claims)
	if !ok || !token.Valid {
		return nil, ErrInvalidToken
	}

	return claims, nil
}
