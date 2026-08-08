package middleware

import (
	"context"
	"net/http"
	"strings"

	"github.com/onnwee/subcults/internal/auth"
)

type userIDContextKey struct{}

func SetUserID(ctx context.Context, userID string) context.Context {
	return context.WithValue(ctx, userIDContextKey{}, userID)
}

func GetUserID(ctx context.Context) string {
	value, _ := ctx.Value(userIDContextKey{}).(string)
	return value
}

// JWTAuth decorates requests with the verified account ID and immutable DID.
// Missing credentials are allowed so public routes can share the same router;
// invalid credentials receive 401 and never degrade to anonymous access.
func JWTAuth(jwt *auth.JWTService) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			header := strings.TrimSpace(r.Header.Get("Authorization"))
			if header == "" {
				next.ServeHTTP(w, r)
				return
			}
			const prefix = "Bearer "
			if !strings.HasPrefix(header, prefix) {
				http.Error(w, `{"error":{"code":"unauthorized","message":"invalid authorization header"}}`, http.StatusUnauthorized)
				return
			}
			claims, err := jwt.ValidateToken(strings.TrimSpace(strings.TrimPrefix(header, prefix)))
			if err != nil || claims.Subject == "" || claims.DID == "" || claims.Type != auth.TokenTypeAccess {
				http.Error(w, `{"error":{"code":"unauthorized","message":"invalid or expired access token"}}`, http.StatusUnauthorized)
				return
			}
			ctx := SetUserID(r.Context(), claims.Subject)
			ctx = SetUserDID(ctx, claims.DID)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}
