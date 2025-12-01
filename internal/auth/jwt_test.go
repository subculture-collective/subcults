package auth

import (
	"strings"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

const testSecret = "test-secret-key-for-jwt-testing"

func TestGenerateAccessToken(t *testing.T) {
	svc := NewJWTService(testSecret)

	tests := []struct {
		name    string
		userID  string
		did     string
		wantErr bool
	}{
		{
			name:    "valid access token",
			userID:  "user-123",
			did:     "did:web:example.com",
			wantErr: false,
		},
		{
			name:    "empty userID",
			userID:  "",
			did:     "did:web:example.com",
			wantErr: true,
		},
		{
			name:    "empty did",
			userID:  "user-123",
			did:     "",
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			token, err := svc.GenerateAccessToken(tt.userID, tt.did)
			if (err != nil) != tt.wantErr {
				t.Errorf("GenerateAccessToken() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if err == nil && token == "" {
				t.Error("GenerateAccessToken() returned empty token")
			}
		})
	}
}

func TestGenerateRefreshToken(t *testing.T) {
	svc := NewJWTService(testSecret)

	tests := []struct {
		name    string
		userID  string
		wantErr bool
	}{
		{
			name:    "valid refresh token",
			userID:  "user-123",
			wantErr: false,
		},
		{
			name:    "empty userID",
			userID:  "",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			token, err := svc.GenerateRefreshToken(tt.userID)
			if (err != nil) != tt.wantErr {
				t.Errorf("GenerateRefreshToken() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if err == nil && token == "" {
				t.Error("GenerateRefreshToken() returned empty token")
			}
		})
	}
}

func TestValidateAccessToken(t *testing.T) {
	svc := NewJWTService(testSecret)

	// Generate a valid access token
	validToken, err := svc.GenerateAccessToken("user-123", "did:web:example.com")
	if err != nil {
		t.Fatalf("Failed to generate access token: %v", err)
	}

	tests := []struct {
		name       string
		token      string
		wantUserID string
		wantDID    string
		wantType   string
		wantErr    error
	}{
		{
			name:       "valid access token",
			token:      validToken,
			wantUserID: "user-123",
			wantDID:    "did:web:example.com",
			wantType:   TokenTypeAccess,
			wantErr:    nil,
		},
		{
			name:    "invalid token format",
			token:   "not-a-valid-token",
			wantErr: ErrInvalidToken,
		},
		{
			name:    "empty token",
			token:   "",
			wantErr: ErrInvalidToken,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			claims, err := svc.ValidateToken(tt.token)
			if tt.wantErr != nil {
				if err != tt.wantErr {
					t.Errorf("ValidateToken() error = %v, wantErr %v", err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Errorf("ValidateToken() unexpected error = %v", err)
				return
			}
			if claims.Subject != tt.wantUserID {
				t.Errorf("ValidateToken() Subject = %v, want %v", claims.Subject, tt.wantUserID)
			}
			if claims.DID != tt.wantDID {
				t.Errorf("ValidateToken() DID = %v, want %v", claims.DID, tt.wantDID)
			}
			if claims.Type != tt.wantType {
				t.Errorf("ValidateToken() Type = %v, want %v", claims.Type, tt.wantType)
			}
		})
	}
}

func TestValidateRefreshToken(t *testing.T) {
	svc := NewJWTService(testSecret)

	// Generate a valid refresh token
	validToken, err := svc.GenerateRefreshToken("user-456")
	if err != nil {
		t.Fatalf("Failed to generate refresh token: %v", err)
	}

	claims, err := svc.ValidateToken(validToken)
	if err != nil {
		t.Fatalf("ValidateToken() unexpected error = %v", err)
	}
	if claims.Subject != "user-456" {
		t.Errorf("ValidateToken() Subject = %v, want user-456", claims.Subject)
	}
	if claims.DID != "" {
		t.Errorf("ValidateToken() DID = %v, want empty", claims.DID)
	}
	if claims.Type != TokenTypeRefresh {
		t.Errorf("ValidateToken() Type = %v, want %v", claims.Type, TokenTypeRefresh)
	}
}

func TestExpiredToken(t *testing.T) {
	svc := NewJWTServiceWithLeeway(testSecret, 0) // No leeway for this test

	// Create an expired token manually
	now := time.Now()
	claims := Claims{
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   "user-expired",
			IssuedAt:  jwt.NewNumericDate(now.Add(-2 * time.Hour)),
			ExpiresAt: jwt.NewNumericDate(now.Add(-1 * time.Hour)), // Expired 1 hour ago
		},
		Type: TokenTypeAccess,
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenString, err := token.SignedString([]byte(testSecret))
	if err != nil {
		t.Fatalf("Failed to create expired token: %v", err)
	}

	_, err = svc.ValidateToken(tokenString)
	if err != ErrExpiredToken {
		t.Errorf("ValidateToken() error = %v, want %v", err, ErrExpiredToken)
	}
}

func TestTamperedToken(t *testing.T) {
	svc := NewJWTService(testSecret)

	// Generate a valid token
	validToken, err := svc.GenerateAccessToken("user-123", "did:web:example.com")
	if err != nil {
		t.Fatalf("Failed to generate access token: %v", err)
	}

	// Tamper with the token by modifying the signature
	parts := strings.Split(validToken, ".")
	if len(parts) != 3 {
		t.Fatalf("Invalid token format")
	}

	// Corrupt the signature
	tamperedToken := parts[0] + "." + parts[1] + ".tamperedsignature"

	_, err = svc.ValidateToken(tamperedToken)
	if err != ErrInvalidToken {
		t.Errorf("ValidateToken() error = %v, want %v", err, ErrInvalidToken)
	}
}

func TestWrongSecretToken(t *testing.T) {
	svc1 := NewJWTService("secret-one")
	svc2 := NewJWTService("secret-two")

	// Generate a token with one secret
	token, err := svc1.GenerateAccessToken("user-123", "did:web:example.com")
	if err != nil {
		t.Fatalf("Failed to generate access token: %v", err)
	}

	// Try to validate with a different secret
	_, err = svc2.ValidateToken(token)
	if err != ErrInvalidToken {
		t.Errorf("ValidateToken() error = %v, want %v", err, ErrInvalidToken)
	}
}

func TestTokenClaims(t *testing.T) {
	svc := NewJWTService(testSecret)

	t.Run("access token claims", func(t *testing.T) {
		beforeGen := time.Now().Add(-1 * time.Second)
		token, err := svc.GenerateAccessToken("user-123", "did:web:example.com")
		if err != nil {
			t.Fatalf("Failed to generate access token: %v", err)
		}
		afterGen := time.Now().Add(1 * time.Second)

		claims, err := svc.ValidateToken(token)
		if err != nil {
			t.Fatalf("ValidateToken() error = %v", err)
		}

		// Check sub claim
		if claims.Subject != "user-123" {
			t.Errorf("Subject = %v, want user-123", claims.Subject)
		}

		// Check did claim
		if claims.DID != "did:web:example.com" {
			t.Errorf("DID = %v, want did:web:example.com", claims.DID)
		}

		// Check typ claim
		if claims.Type != TokenTypeAccess {
			t.Errorf("Type = %v, want %v", claims.Type, TokenTypeAccess)
		}

		// Check iat claim (issued at)
		if claims.IssuedAt == nil {
			t.Error("IssuedAt is nil")
		} else {
			iat := claims.IssuedAt.Time
			if iat.Before(beforeGen) || iat.After(afterGen) {
				t.Errorf("IssuedAt = %v, want between %v and %v", iat, beforeGen, afterGen)
			}
		}

		// Check exp claim
		if claims.ExpiresAt == nil {
			t.Error("ExpiresAt is nil")
		} else {
			expectedExp := claims.IssuedAt.Time.Add(AccessTokenExpiry)
			if !claims.ExpiresAt.Time.Equal(expectedExp) {
				t.Errorf("ExpiresAt = %v, want %v", claims.ExpiresAt.Time, expectedExp)
			}
		}
	})

	t.Run("refresh token claims", func(t *testing.T) {
		beforeGen := time.Now().Add(-1 * time.Second)
		token, err := svc.GenerateRefreshToken("user-456")
		if err != nil {
			t.Fatalf("Failed to generate refresh token: %v", err)
		}
		afterGen := time.Now().Add(1 * time.Second)

		claims, err := svc.ValidateToken(token)
		if err != nil {
			t.Fatalf("ValidateToken() error = %v", err)
		}

		// Check sub claim
		if claims.Subject != "user-456" {
			t.Errorf("Subject = %v, want user-456", claims.Subject)
		}

		// Check did claim (should be empty for refresh tokens)
		if claims.DID != "" {
			t.Errorf("DID = %v, want empty", claims.DID)
		}

		// Check typ claim
		if claims.Type != TokenTypeRefresh {
			t.Errorf("Type = %v, want %v", claims.Type, TokenTypeRefresh)
		}

		// Check iat claim
		if claims.IssuedAt == nil {
			t.Error("IssuedAt is nil")
		} else {
			iat := claims.IssuedAt.Time
			if iat.Before(beforeGen) || iat.After(afterGen) {
				t.Errorf("IssuedAt = %v, want between %v and %v", iat, beforeGen, afterGen)
			}
		}

		// Check exp claim
		if claims.ExpiresAt == nil {
			t.Error("ExpiresAt is nil")
		} else {
			expectedExp := claims.IssuedAt.Time.Add(RefreshTokenExpiry)
			if !claims.ExpiresAt.Time.Equal(expectedExp) {
				t.Errorf("ExpiresAt = %v, want %v", claims.ExpiresAt.Time, expectedExp)
			}
		}
	})
}

func TestLeewayValidation(t *testing.T) {
	// Create a token that expired just now (within leeway)
	now := time.Now()
	claims := Claims{
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   "user-leeway",
			IssuedAt:  jwt.NewNumericDate(now.Add(-1 * time.Hour)),
			ExpiresAt: jwt.NewNumericDate(now.Add(-10 * time.Second)), // Expired 10 seconds ago
		},
		Type: TokenTypeAccess,
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenString, err := token.SignedString([]byte(testSecret))
	if err != nil {
		t.Fatalf("Failed to create token: %v", err)
	}

	t.Run("with default leeway (30s) - should pass", func(t *testing.T) {
		svc := NewJWTService(testSecret) // Default 30s leeway
		_, err := svc.ValidateToken(tokenString)
		if err != nil {
			t.Errorf("ValidateToken() error = %v, expected no error (within leeway)", err)
		}
	})

	t.Run("with no leeway - should fail", func(t *testing.T) {
		svc := NewJWTServiceWithLeeway(testSecret, 0)
		_, err := svc.ValidateToken(tokenString)
		if err != ErrExpiredToken {
			t.Errorf("ValidateToken() error = %v, want %v", err, ErrExpiredToken)
		}
	})
}

func TestEmptyUserIDError(t *testing.T) {
	svc := NewJWTService(testSecret)

	t.Run("access token with empty userID", func(t *testing.T) {
		_, err := svc.GenerateAccessToken("", "did:web:example.com")
		if err != ErrEmptyUserID {
			t.Errorf("GenerateAccessToken() error = %v, want %v", err, ErrEmptyUserID)
		}
	})

	t.Run("refresh token with empty userID", func(t *testing.T) {
		_, err := svc.GenerateRefreshToken("")
		if err != ErrEmptyUserID {
			t.Errorf("GenerateRefreshToken() error = %v, want %v", err, ErrEmptyUserID)
		}
	})
}
