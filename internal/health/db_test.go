package health

import (
	"context"
	"testing"
)

// TestDBChecker_Creation verifies the constructor without requiring a live DB.
func TestDBChecker_Creation(t *testing.T) {
	checker := NewDBChecker(nil)
	if checker == nil {
		t.Fatal("expected checker to be non-nil")
	}
	if checker.db != nil {
		t.Error("expected checker db to be nil when nil is passed")
	}
}

func TestDBCheckerNilDatabaseFailsWithoutPanic(t *testing.T) {
	if err := NewDBChecker(nil).HealthCheck(context.Background()); err == nil {
		t.Fatal("expected unconfigured database error")
	}
}
