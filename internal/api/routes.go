// Package api provides HTTP handlers for the Subcults API.
package api

import (
	"net/http"
	"strings"

	"github.com/onnwee/subcults/internal/identity"
	"github.com/onnwee/subcults/internal/middleware"
)

// RouteDeps holds infrastructure dependencies shared across route registrars.
type RouteDeps struct {
	RateLimitStore   middleware.RateLimitStore
	RateLimitMetrics *middleware.Metrics
}

// RateLimit wraps a handler with rate limiting using the given config and key function.
func (d *RouteDeps) RateLimit(handler http.HandlerFunc, config middleware.RateLimitConfig, keyFunc middleware.KeyFunc) http.Handler {
	return middleware.RateLimiter(d.RateLimitStore, config, keyFunc, d.RateLimitMetrics)(http.HandlerFunc(handler))
}

// RequireCreator returns middleware that ensures the authenticated user has
// creator or admin role.
func RequireCreator(identityService *identity.Service) func(http.HandlerFunc) http.HandlerFunc {
	return func(next http.HandlerFunc) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			userID := middleware.GetUserID(r.Context())
			if userID == "" {
				WriteError(w, r.Context(), http.StatusUnauthorized, ErrCodeUnauthorized, "Authentication required")
				return
			}
			user, err := identityService.GetUser(r.Context(), userID)
			if err != nil || (user.Role != "creator" && user.Role != "admin") {
				WriteError(w, r.Context(), http.StatusForbidden, ErrCodeForbidden, "Approved creator access required")
				return
			}
			next(w, r)
		}
	}
}

// v1EventSubrouter returns a handler that routes /api/v1/events/{id}/location
// and /api/v1/events/{id}/rsvp sub-resources.
func v1EventSubrouter(protectedLocations http.Handler, createRSVP http.HandlerFunc) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		pathParts := strings.Split(strings.TrimPrefix(r.URL.Path, "/api/v1/events/"), "/")
		if len(pathParts) == 2 && pathParts[0] != "" && pathParts[1] == "location" {
			protectedLocations.ServeHTTP(w, r)
			return
		}
		if len(pathParts) == 2 && pathParts[0] != "" && pathParts[1] == "rsvp" && r.Method == http.MethodPost {
			originalPath := r.URL.Path
			r.URL.Path = "/events/" + pathParts[0] + "/rsvp"
			createRSVP(w, r)
			r.URL.Path = originalPath
			return
		}
		ctx := middleware.SetErrorCode(r.Context(), ErrCodeNotFound)
		WriteError(w, ctx, http.StatusNotFound, ErrCodeNotFound, "Not found")
	})
}