package middleware

import (
	"encoding/json"
	"net/http"
	"strings"
)

// DurableBetaSurface explicitly disables routes backed only by legacy
// in-memory repositories. This is applied only to the production Postgres
// runtime; local fixture mode retains the full development surface.
func DurableBetaSurface(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Path
		disabled := strings.HasPrefix(path, "/streams") || strings.Contains(path, "/stream/") ||
			strings.HasPrefix(path, "/livekit/") || strings.HasPrefix(path, "/posts") ||
			strings.HasPrefix(path, "/search/posts") || strings.HasPrefix(path, "/search/global") ||
			strings.Contains(path, "/feed") || strings.Contains(path, "/memberships") ||
			strings.HasPrefix(path, "/alliances") || strings.HasPrefix(path, "/trust/") ||
			strings.HasPrefix(path, "/payments/") || strings.HasPrefix(path, "/internal/stripe") ||
			strings.HasPrefix(path, "/uploads/") || strings.HasPrefix(path, "/api/telemetry") ||
			strings.HasPrefix(path, "/api/log/client-error") || strings.HasPrefix(path, "/api/account/")
		if disabled {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusNotImplemented)
			_ = json.NewEncoder(w).Encode(map[string]any{"error": map[string]string{
				"code":    "feature_disabled_beta",
				"message": "This volatile subsystem is disabled in the durable public beta.",
			}})
			return
		}
		next.ServeHTTP(w, r)
	})
}
