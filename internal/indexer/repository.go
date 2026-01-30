// Package indexer provides filtering and processing of AT Protocol records
// for the Subcults Jetstream indexer.
package indexer

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"log/slog"
	"sync"

	"github.com/google/uuid"
)

var (
	// ErrRecordExists is returned when attempting to insert a duplicate record.
	ErrRecordExists = errors.New("record already exists")
	
	// ErrTransactionFailed is returned when a transaction cannot be completed.
	ErrTransactionFailed = errors.New("transaction failed")
)

// RecordRepository provides transactional database operations for AT Protocol records.
type RecordRepository interface {
	// UpsertRecord atomically inserts or updates a record with idempotency.
	// Returns the record ID and a boolean indicating if it was newly created (true) or updated (false).
	UpsertRecord(ctx context.Context, record *FilterResult) (string, bool, error)
	
	// DeleteRecord atomically removes a record.
	DeleteRecord(ctx context.Context, did, collection, rkey string) error
	
	// CheckIdempotencyKey verifies if an operation has already been processed.
	CheckIdempotencyKey(ctx context.Context, key string) (bool, error)
}

// PostgresRecordRepository implements RecordRepository using PostgreSQL with full transaction support.
type PostgresRecordRepository struct {
	db     *sql.DB
	logger *slog.Logger
}

// NewPostgresRecordRepository creates a new PostgresRecordRepository.
func NewPostgresRecordRepository(db *sql.DB, logger *slog.Logger) *PostgresRecordRepository {
	if logger == nil {
		logger = slog.Default()
	}
	return &PostgresRecordRepository{
		db:     db,
		logger: logger,
	}
}

// UpsertRecord atomically inserts or updates a record with full transaction support.
// This implements the all-or-nothing requirement with idempotency.
func (r *PostgresRecordRepository) UpsertRecord(ctx context.Context, record *FilterResult) (string, bool, error) {
	if !record.Valid || !record.Matched {
		return "", false, fmt.Errorf("invalid or unmatched record")
	}

	// Generate idempotency key from DID + Collection + RKey + Rev
	idempotencyKey := generateIdempotencyKey(record.DID, record.Collection, record.RKey, record.Rev)

	// Begin transaction
	tx, err := r.db.BeginTx(ctx, &sql.TxOptions{
		Isolation: sql.LevelReadCommitted,
	})
	if err != nil {
		r.logger.Error("failed to begin transaction",
			slog.String("error", err.Error()),
			slog.String("did", record.DID),
			slog.String("collection", record.Collection),
			slog.String("rkey", record.RKey))
		return "", false, fmt.Errorf("failed to begin transaction: %w", err)
	}

	// Ensure transaction is rolled back on error
	defer func() {
		if err != nil {
			if rbErr := tx.Rollback(); rbErr != nil {
				r.logger.Error("failed to rollback transaction",
					slog.String("error", rbErr.Error()),
					slog.String("original_error", err.Error()))
			}
		}
	}()

	// Check idempotency - if we've already processed this exact record revision, skip
	var existingKey string
	checkQuery := `
		SELECT idempotency_key FROM ingestion_idempotency 
		WHERE idempotency_key = $1
	`
	err = tx.QueryRowContext(ctx, checkQuery, idempotencyKey).Scan(&existingKey)
	if err == nil {
		// Record already processed, commit and return existing state
		if commitErr := tx.Commit(); commitErr != nil {
			r.logger.Error("failed to commit idempotency check",
				slog.String("error", commitErr.Error()))
			return "", false, fmt.Errorf("failed to commit: %w", commitErr)
		}
		r.logger.Debug("skipping duplicate record (already processed)",
			slog.String("idempotency_key", idempotencyKey),
			slog.String("did", record.DID),
			slog.String("collection", record.Collection),
			slog.String("rkey", record.RKey))
		return "", false, nil // Not an error, just already processed
	} else if err != sql.ErrNoRows {
		// Unexpected error
		r.logger.Error("failed to check idempotency",
			slog.String("error", err.Error()))
		return "", false, fmt.Errorf("failed to check idempotency: %w", err)
	}

	// Route to appropriate table based on collection
	var recordID string
	var isNew bool

	switch record.Collection {
	case CollectionScene:
		recordID, isNew, err = r.upsertScene(ctx, tx, record)
	case CollectionEvent:
		recordID, isNew, err = r.upsertEvent(ctx, tx, record)
	case CollectionPost:
		recordID, isNew, err = r.upsertPost(ctx, tx, record)
	default:
		// For now, only support the three main collections
		// Additional collections (membership, alliance, stream) will be added later
		err = fmt.Errorf("unsupported collection: %s", record.Collection)
	}

	if err != nil {
		r.logger.Error("failed to upsert record",
			slog.String("error", err.Error()),
			slog.String("collection", record.Collection))
		return "", false, fmt.Errorf("failed to upsert %s: %w", record.Collection, err)
	}

	// Store idempotency key to prevent reprocessing
	insertIdempotency := `
		INSERT INTO ingestion_idempotency (idempotency_key, did, collection, rkey, rev, record_id, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, NOW())
	`
	_, err = tx.ExecContext(ctx, insertIdempotency, idempotencyKey, record.DID, record.Collection, record.RKey, record.Rev, recordID)
	if err != nil {
		r.logger.Error("failed to store idempotency key",
			slog.String("error", err.Error()))
		return "", false, fmt.Errorf("failed to store idempotency key: %w", err)
	}

	// Commit transaction
	if err = tx.Commit(); err != nil {
		r.logger.Error("failed to commit transaction",
			slog.String("error", err.Error()))
		return "", false, fmt.Errorf("failed to commit transaction: %w", err)
	}

	// Log successful transaction
	r.logger.Info("record upserted successfully",
		slog.String("record_id", recordID),
		slog.String("collection", record.Collection),
		slog.String("did", record.DID),
		slog.String("rkey", record.RKey),
		slog.Bool("is_new", isNew),
		slog.String("idempotency_key", idempotencyKey))

	return recordID, isNew, nil
}

// DeleteRecord atomically removes a record with transaction support.
func (r *PostgresRecordRepository) DeleteRecord(ctx context.Context, did, collection, rkey string) error {
	// Begin transaction
	tx, err := r.db.BeginTx(ctx, &sql.TxOptions{
		Isolation: sql.LevelReadCommitted,
	})
	if err != nil {
		r.logger.Error("failed to begin transaction for delete",
			slog.String("error", err.Error()))
		return fmt.Errorf("failed to begin transaction: %w", err)
	}

	defer func() {
		if err != nil {
			if rbErr := tx.Rollback(); rbErr != nil {
				r.logger.Error("failed to rollback delete transaction",
					slog.String("error", rbErr.Error()))
			}
		}
	}()

	// Delete from appropriate table
	var query string
	switch collection {
	case CollectionScene:
		query = `DELETE FROM scenes WHERE record_did = $1 AND record_rkey = $2`
	case CollectionEvent:
		query = `DELETE FROM events WHERE record_did = $1 AND record_rkey = $2`
	case CollectionPost:
		query = `DELETE FROM posts WHERE record_did = $1 AND record_rkey = $2`
	default:
		return fmt.Errorf("unsupported collection for delete: %s", collection)
	}

	result, err := tx.ExecContext(ctx, query, did, rkey)
	if err != nil {
		r.logger.Error("failed to delete record",
			slog.String("error", err.Error()))
		return fmt.Errorf("failed to delete record: %w", err)
	}

	rowsAffected, _ := result.RowsAffected()

	// Commit transaction
	if err = tx.Commit(); err != nil {
		r.logger.Error("failed to commit delete transaction",
			slog.String("error", err.Error()))
		return fmt.Errorf("failed to commit: %w", err)
	}

	r.logger.Info("record deleted successfully",
		slog.String("collection", collection),
		slog.String("did", did),
		slog.String("rkey", rkey),
		slog.Int64("rows_affected", rowsAffected))

	return nil
}

// CheckIdempotencyKey verifies if an operation has already been processed.
func (r *PostgresRecordRepository) CheckIdempotencyKey(ctx context.Context, key string) (bool, error) {
	var exists bool
	query := `SELECT EXISTS(SELECT 1 FROM ingestion_idempotency WHERE idempotency_key = $1)`
	err := r.db.QueryRowContext(ctx, query, key).Scan(&exists)
	if err != nil {
		return false, fmt.Errorf("failed to check idempotency key: %w", err)
	}
	return exists, nil
}

// upsertScene handles scene-specific upsert logic.
func (r *PostgresRecordRepository) upsertScene(ctx context.Context, tx *sql.Tx, record *FilterResult) (string, bool, error) {
	// Check if record exists
	var existingID string
	checkQuery := `SELECT id FROM scenes WHERE record_did = $1 AND record_rkey = $2`
	err := tx.QueryRowContext(ctx, checkQuery, record.DID, record.RKey).Scan(&existingID)

	if err == sql.ErrNoRows {
		// Insert new record
		newID := uuid.New().String()
		insertQuery := `
			INSERT INTO scenes (id, record_did, record_rkey, created_at, updated_at)
			VALUES ($1, $2, $3, NOW(), NOW())
		`
		_, err = tx.ExecContext(ctx, insertQuery, newID, record.DID, record.RKey)
		if err != nil {
			return "", false, fmt.Errorf("failed to insert scene: %w", err)
		}
		return newID, true, nil
	} else if err != nil {
		return "", false, fmt.Errorf("failed to check scene existence: %w", err)
	}

	// Update existing record
	updateQuery := `UPDATE scenes SET updated_at = NOW() WHERE id = $1`
	_, err = tx.ExecContext(ctx, updateQuery, existingID)
	if err != nil {
		return "", false, fmt.Errorf("failed to update scene: %w", err)
	}

	return existingID, false, nil
}

// upsertEvent handles event-specific upsert logic.
func (r *PostgresRecordRepository) upsertEvent(ctx context.Context, tx *sql.Tx, record *FilterResult) (string, bool, error) {
	var existingID string
	checkQuery := `SELECT id FROM events WHERE record_did = $1 AND record_rkey = $2`
	err := tx.QueryRowContext(ctx, checkQuery, record.DID, record.RKey).Scan(&existingID)

	if err == sql.ErrNoRows {
		newID := uuid.New().String()
		insertQuery := `
			INSERT INTO events (id, record_did, record_rkey, created_at, updated_at)
			VALUES ($1, $2, $3, NOW(), NOW())
		`
		_, err = tx.ExecContext(ctx, insertQuery, newID, record.DID, record.RKey)
		if err != nil {
			return "", false, fmt.Errorf("failed to insert event: %w", err)
		}
		return newID, true, nil
	} else if err != nil {
		return "", false, fmt.Errorf("failed to check event existence: %w", err)
	}

	updateQuery := `UPDATE events SET updated_at = NOW() WHERE id = $1`
	_, err = tx.ExecContext(ctx, updateQuery, existingID)
	if err != nil {
		return "", false, fmt.Errorf("failed to update event: %w", err)
	}

	return existingID, false, nil
}

// upsertPost handles post-specific upsert logic.
func (r *PostgresRecordRepository) upsertPost(ctx context.Context, tx *sql.Tx, record *FilterResult) (string, bool, error) {
	var existingID string
	checkQuery := `SELECT id FROM posts WHERE record_did = $1 AND record_rkey = $2`
	err := tx.QueryRowContext(ctx, checkQuery, record.DID, record.RKey).Scan(&existingID)

	if err == sql.ErrNoRows {
		newID := uuid.New().String()
		insertQuery := `
			INSERT INTO posts (id, record_did, record_rkey, created_at)
			VALUES ($1, $2, $3, NOW())
		`
		_, err = tx.ExecContext(ctx, insertQuery, newID, record.DID, record.RKey)
		if err != nil {
			return "", false, fmt.Errorf("failed to insert post: %w", err)
		}
		return newID, true, nil
	} else if err != nil {
		return "", false, fmt.Errorf("failed to check post existence: %w", err)
	}

	// Posts don't typically have updates in the same way, but we'll touch the record
	return existingID, false, nil
}

// generateIdempotencyKey creates a deterministic key from record metadata.
// Format: SHA256(did + collection + rkey + rev)
func generateIdempotencyKey(did, collection, rkey, rev string) string {
	data := fmt.Sprintf("%s:%s:%s:%s", did, collection, rkey, rev)
	hash := sha256.Sum256([]byte(data))
	return hex.EncodeToString(hash[:])
}

// InMemoryRecordRepository provides an in-memory implementation for testing.
type InMemoryRecordRepository struct {
	mu              sync.RWMutex
	records         map[string]*FilterResult
	idempotencyKeys map[string]bool
	logger          *slog.Logger
}

// NewInMemoryRecordRepository creates a new in-memory repository.
func NewInMemoryRecordRepository(logger *slog.Logger) *InMemoryRecordRepository {
	if logger == nil {
		logger = slog.Default()
	}
	return &InMemoryRecordRepository{
		records:         make(map[string]*FilterResult),
		idempotencyKeys: make(map[string]bool),
		logger:          logger,
	}
}

// UpsertRecord implements the interface for in-memory storage.
func (r *InMemoryRecordRepository) UpsertRecord(ctx context.Context, record *FilterResult) (string, bool, error) {
	if !record.Valid || !record.Matched {
		return "", false, fmt.Errorf("invalid or unmatched record")
	}

	idempotencyKey := generateIdempotencyKey(record.DID, record.Collection, record.RKey, record.Rev)

	r.mu.Lock()
	defer r.mu.Unlock()

	// Check idempotency
	if r.idempotencyKeys[idempotencyKey] {
		r.logger.Debug("skipping duplicate record",
			slog.String("idempotency_key", idempotencyKey))
		return "", false, nil
	}

	// Generate composite key
	key := fmt.Sprintf("%s:%s:%s", record.DID, record.Collection, record.RKey)

	// Check if record exists
	_, exists := r.records[key]

	// Store record
	r.records[key] = record
	r.idempotencyKeys[idempotencyKey] = true

	recordID := uuid.New().String()
	r.logger.Info("record upserted in memory",
		slog.String("record_id", recordID),
		slog.Bool("is_new", !exists))

	return recordID, !exists, nil
}

// DeleteRecord implements the interface for in-memory storage.
func (r *InMemoryRecordRepository) DeleteRecord(ctx context.Context, did, collection, rkey string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	key := fmt.Sprintf("%s:%s:%s", did, collection, rkey)
	delete(r.records, key)
	r.logger.Info("record deleted from memory",
		slog.String("did", did),
		slog.String("collection", collection),
		slog.String("rkey", rkey))
	return nil
}

// CheckIdempotencyKey implements the interface for in-memory storage.
func (r *InMemoryRecordRepository) CheckIdempotencyKey(ctx context.Context, key string) (bool, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	return r.idempotencyKeys[key], nil
}
