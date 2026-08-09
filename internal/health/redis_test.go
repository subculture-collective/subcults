package health

import (
	"context"
	"testing"

	"github.com/redis/go-redis/v9"
)

func TestRedisChecker_Creation(t *testing.T) {
	client := redis.NewClient(&redis.Options{Addr: "localhost:6379"})
	checker := NewRedisChecker(client)
	if checker == nil {
		t.Fatal("expected checker to be non-nil")
	}
	if checker.client != client {
		t.Error("expected checker client to match")
	}
}

func TestRedisChecker_NilClient(t *testing.T) {
	checker := NewRedisChecker(nil)
	if checker == nil {
		t.Fatal("expected checker to be non-nil")
	}
	if checker.client != nil {
		t.Error("expected checker client to be nil")
	}
}

func TestRedisCheckerNilClientFailsWithoutPanic(t *testing.T) {
	if err := NewRedisChecker(nil).HealthCheck(context.Background()); err == nil {
		t.Fatal("expected unconfigured redis error")
	}
}
