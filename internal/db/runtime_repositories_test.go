package db

import (
	"context"
	"errors"
	"testing"
)

func TestNewRuntimeRepositoriesRequiresDatabase(t *testing.T) {
	_, err := NewRuntimeRepositories(context.Background(), "")
	if !errors.Is(err, ErrDatabaseURLRequired) {
		t.Fatalf("error = %v, want ErrDatabaseURLRequired", err)
	}
}

func TestRuntimeRepositoriesCloseNilIsSafe(t *testing.T) {
	var repositories *RuntimeRepositories
	if err := repositories.Close(); err != nil {
		t.Fatalf("Close() error = %v, want nil", err)
	}
}
