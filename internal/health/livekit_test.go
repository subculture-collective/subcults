package health

import (
"context"
"testing"
)

// TestLiveKitChecker_Creation tests that the LiveKit checker is created correctly.
func TestLiveKitChecker_Creation(t *testing.T) {
url := "https://livekit.example.com"

checker := NewLiveKitChecker(url)
if checker == nil {
t.Fatal("expected checker to be non-nil")
}

if checker.url != url {
t.Errorf("expected checker url to be %s, got %s", url, checker.url)
}
}

// TestLiveKitChecker_EmptyURL tests that an empty URL returns an error.
func TestLiveKitChecker_EmptyURL(t *testing.T) {
checker := NewLiveKitChecker("")

ctx := context.Background()
err := checker.HealthCheck(ctx)

if err == nil {
t.Error("expected error with empty URL")
}

expectedMsg := "livekit url not configured"
if err.Error() != expectedMsg {
t.Errorf("expected error message %q, got %q", expectedMsg, err.Error())
}
}

// TestLiveKitChecker_InvalidURL tests that an invalid URL returns an error.
func TestLiveKitChecker_InvalidURL(t *testing.T) {
checker := NewLiveKitChecker("invalid-url")

ctx := context.Background()
err := checker.HealthCheck(ctx)

// Should fail with invalid URL
if err == nil {
t.Error("expected error with invalid URL")
}
}
