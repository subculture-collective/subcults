// Package db provides database utilities and connection handling for Subcults.
package db

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	_ "github.com/lib/pq"
)

var (
	// ErrDatabaseURLRequired prevents a production API process from starting
	// without an explicitly configured database.
	ErrDatabaseURLRequired = errors.New("database URL is required")

	// ErrDurableRepositoriesUnavailable makes the current implementation
	// boundary explicit. A successful SQL connection alone does not make the
	// API's domain repositories durable.
	ErrDurableRepositoriesUnavailable = errors.New("durable API repositories are not implemented")
)

// RuntimeRepositories owns the SQL connection used by runtime dependency
// checks. It intentionally does not expose the API's in-memory repositories as
// durable implementations; domain-specific Postgres adapters must be added
// before this type can be used to serve production traffic.
type RuntimeRepositories struct {
	DB *sql.DB
}

// NewRuntimeRepositories validates and connects the runtime database. The
// returned database has passed a PingContext health check, but callers must
// still provide actual durable domain repositories before serving production
// API traffic.
func NewRuntimeRepositories(ctx context.Context, databaseURL string) (*RuntimeRepositories, error) {
	databaseURL = strings.TrimSpace(databaseURL)
	if databaseURL == "" {
		return nil, ErrDatabaseURLRequired
	}

	database, err := sql.Open("postgres", databaseURL)
	if err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}

	if err := database.PingContext(ctx); err != nil {
		database.Close()
		return nil, fmt.Errorf("ping database: %w", err)
	}

	return &RuntimeRepositories{DB: database}, nil
}

// Close releases the runtime database connection. It is safe to call with a
// nil receiver so startup cleanup can remain straightforward.
func (r *RuntimeRepositories) Close() error {
	if r == nil || r.DB == nil {
		return nil
	}
	return r.DB.Close()
}
