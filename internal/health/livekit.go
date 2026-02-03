package health

import (
"context"
"fmt"
"net/http"
"time"
)

// LiveKitChecker implements health checking for LiveKit.
type LiveKitChecker struct {
url string
}

// NewLiveKitChecker creates a new LiveKit health checker.
// The url should be the base URL of the LiveKit server (e.g., "https://livekit.example.com").
func NewLiveKitChecker(url string) *LiveKitChecker {
return &LiveKitChecker{
url: url,
}
}

// HealthCheck performs a health check on LiveKit by making an HTTP request.
// LiveKit doesn't have a standard health endpoint, so we check if the server is reachable.
func (l *LiveKitChecker) HealthCheck(ctx context.Context) error {
if l.url == "" {
return fmt.Errorf("livekit url not configured")
}

client := &http.Client{
Timeout: 3 * time.Second,
}

req, err := http.NewRequestWithContext(ctx, http.MethodGet, l.url, nil)
if err != nil {
return fmt.Errorf("failed to create request: %w", err)
}

resp, err := client.Do(req)
if err != nil {
return fmt.Errorf("failed to reach livekit server: %w", err)
}
defer resp.Body.Close()

// Accept any response that indicates the server is reachable
// LiveKit may return 404 or other status codes, but as long as we get a response, it's healthy
return nil
}
