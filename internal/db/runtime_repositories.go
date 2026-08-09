// Package db provides database utilities and connection handling for Subcults.
package db

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	_ "github.com/lib/pq"
)

var (
	// ErrDatabaseURLRequired prevents a production API process from starting
	// without an explicitly configured database.
	ErrDatabaseURLRequired = errors.New("database URL is required")
)

// RuntimeRepositories owns the SQL connection used by runtime dependency
// checks and durable domain adapters.
type RuntimeRepositories struct {
	DB *sql.DB
}

// NewRuntimeRepositories validates and connects the runtime database. The
// returned, pooled database has passed a PingContext health check and is ready
// to be shared by the durable domain adapters.
func NewRuntimeRepositories(ctx context.Context, databaseURL string) (*RuntimeRepositories, error) {
	databaseURL = strings.TrimSpace(databaseURL)
	if databaseURL == "" {
		return nil, ErrDatabaseURLRequired
	}

	database, err := sql.Open("postgres", withRuntimeTimeouts(databaseURL))
	if err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}

	database.SetMaxOpenConns(envInt("DB_MAX_OPEN_CONNS", 20))
	database.SetMaxIdleConns(envInt("DB_MAX_IDLE_CONNS", 5))
	database.SetConnMaxLifetime(envDuration("DB_CONN_MAX_LIFETIME", 30*time.Minute))
	database.SetConnMaxIdleTime(envDuration("DB_CONN_MAX_IDLE_TIME", 5*time.Minute))

	if err := database.PingContext(ctx); err != nil {
		database.Close()
		return nil, fmt.Errorf("ping database: %w", err)
	}

	return &RuntimeRepositories{DB: database}, nil
}

func withRuntimeTimeouts(databaseURL string) string {
	parsed, err := url.Parse(databaseURL)
	if err != nil || (parsed.Scheme != "postgres" && parsed.Scheme != "postgresql") {
		return databaseURL
	}
	query := parsed.Query()
	if query.Get("statement_timeout") == "" {
		query.Set("statement_timeout", strconv.Itoa(envInt("DB_STATEMENT_TIMEOUT_MS", 5000)))
	}
	if query.Get("lock_timeout") == "" {
		query.Set("lock_timeout", strconv.Itoa(envInt("DB_LOCK_TIMEOUT_MS", 2000)))
	}
	if query.Get("idle_in_transaction_session_timeout") == "" {
		query.Set("idle_in_transaction_session_timeout", strconv.Itoa(envInt("DB_IDLE_TX_TIMEOUT_MS", 10000)))
	}
	if query.Get("application_name") == "" {
		query.Set("application_name", "subcults-api")
	}
	parsed.RawQuery = query.Encode()
	return parsed.String()
}

func envInt(name string, fallback int) int {
	if parsed, err := strconv.Atoi(strings.TrimSpace(os.Getenv(name))); err == nil && parsed > 0 {
		return parsed
	}
	return fallback
}

func envDuration(name string, fallback time.Duration) time.Duration {
	if parsed, err := time.ParseDuration(strings.TrimSpace(os.Getenv(name))); err == nil && parsed > 0 {
		return parsed
	}
	return fallback
}

// Close releases the runtime database connection. It is safe to call with a
// nil receiver so startup cleanup can remain straightforward.
func (r *RuntimeRepositories) Close() error {
	if r == nil || r.DB == nil {
		return nil
	}
	return r.DB.Close()
}
