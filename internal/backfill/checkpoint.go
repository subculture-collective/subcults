package backfill

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"strconv"
	"time"

	"github.com/onnwee/subcults/internal/indexer"
)

// Config holds backfill operation configuration.
type Config struct {
	Source             string
	AfterSeq           uint64
	BeforeSeq          *uint64
	Target             indexer.ProjectionTarget
	RebuildID          string
	JetstreamHost      string
	JetstreamAPIKey    string
	JetstreamFactory   func(indexer.V2Subscription) (indexer.V2Stream, error)
	JetstreamProjector indexer.V2Projector
	CARPath            string
	BatchSize          int
	DryRun             bool
	Resume             bool
	Logger             *slog.Logger
}

// Result contains the outcome of a backfill run.
type Result struct {
	RecordsProcessed int64
	RecordsSkipped   int64
	Errors           int64
	Duration         time.Duration
}

// Checkpoint tracks backfill progress for resumability.
type Checkpoint struct {
	ID               int64
	Source           string
	CursorTS         int64
	CursorSeq        uint64
	CAROffset        int64
	Target           indexer.ProjectionTarget
	RebuildID        string
	Status           string
	RecordsProcessed int64
	RecordsSkipped   int64
	ErrorsCount      int64
	StartedAt        *time.Time
	UpdatedAt        time.Time
	CompletedAt      *time.Time
}

// CheckpointStore persists backfill progress.
type CheckpointStore interface {
	GetLatest(ctx context.Context, source string, target indexer.ProjectionTarget, rebuildID string) (*Checkpoint, error)
	Create(ctx context.Context, source string, target indexer.ProjectionTarget, rebuildID string) (int64, error)
	Update(ctx context.Context, cp *Checkpoint) error
	Complete(ctx context.Context, id int64, processed, skipped, errors int64) error
	Fail(ctx context.Context, id int64, processed, skipped, errors int64) error
}

// PostgresCheckpointStore implements CheckpointStore with the backfill_checkpoints table.
type PostgresCheckpointStore struct {
	db     *sql.DB
	logger *slog.Logger
}

// NewPostgresCheckpointStore creates a new checkpoint store.
func NewPostgresCheckpointStore(db *sql.DB, logger *slog.Logger) *PostgresCheckpointStore {
	if logger == nil {
		logger = slog.Default()
	}
	return &PostgresCheckpointStore{db: db, logger: logger}
}

func (s *PostgresCheckpointStore) GetLatest(ctx context.Context, source string, target indexer.ProjectionTarget, rebuildID string) (*Checkpoint, error) {
	query := `SELECT id, source, cursor_ts, cursor_seq::text, car_offset, target, rebuild_id, status,
		records_processed, records_skipped, errors_count,
		started_at, updated_at, completed_at
		FROM backfill_checkpoints
		WHERE source = $1 AND target = $2 AND rebuild_id = $3
		ORDER BY id DESC LIMIT 1`
	var cp Checkpoint
	var cursorSeq string
	err := s.db.QueryRowContext(ctx, query, source, target, rebuildID).Scan(
		&cp.ID, &cp.Source, &cp.CursorTS, &cursorSeq, &cp.CAROffset, &cp.Target, &cp.RebuildID, &cp.Status,
		&cp.RecordsProcessed, &cp.RecordsSkipped, &cp.ErrorsCount,
		&cp.StartedAt, &cp.UpdatedAt, &cp.CompletedAt,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get latest checkpoint: %w", err)
	}
	cp.CursorSeq, err = strconv.ParseUint(cursorSeq, 10, 64)
	if err != nil {
		return nil, fmt.Errorf("parse checkpoint cursor sequence %q: %w", cursorSeq, err)
	}
	return &cp, nil
}

func (s *PostgresCheckpointStore) Create(ctx context.Context, source string, target indexer.ProjectionTarget, rebuildID string) (int64, error) {
	now := time.Now()
	var id int64
	err := s.db.QueryRowContext(ctx,
		`INSERT INTO backfill_checkpoints (source, target, rebuild_id, status, started_at, updated_at)
		VALUES ($1, $2, $3, 'running', $4, $4) RETURNING id`,
		source, target, rebuildID, now,
	).Scan(&id)
	if err != nil {
		return 0, fmt.Errorf("create checkpoint: %w", err)
	}
	return id, nil
}

func (s *PostgresCheckpointStore) Update(ctx context.Context, cp *Checkpoint) error {
	_, err := s.db.ExecContext(ctx,
		`UPDATE backfill_checkpoints
		SET cursor_ts = $2, cursor_seq = $3, car_offset = $4,
			records_processed = $5, records_skipped = $6, errors_count = $7,
			updated_at = NOW()
		WHERE id = $1`,
		cp.ID, cp.CursorTS, strconv.FormatUint(cp.CursorSeq, 10), cp.CAROffset,
		cp.RecordsProcessed, cp.RecordsSkipped, cp.ErrorsCount,
	)
	if err != nil {
		return fmt.Errorf("update checkpoint: %w", err)
	}
	return nil
}

func (s *PostgresCheckpointStore) Complete(ctx context.Context, id int64, processed, skipped, errors int64) error {
	_, err := s.db.ExecContext(ctx,
		`UPDATE backfill_checkpoints
		SET status = 'completed', records_processed = $2, records_skipped = $3,
			errors_count = $4, completed_at = NOW(), updated_at = NOW()
		WHERE id = $1`,
		id, processed, skipped, errors,
	)
	if err != nil {
		return fmt.Errorf("complete checkpoint: %w", err)
	}
	return nil
}

func (s *PostgresCheckpointStore) Fail(ctx context.Context, id int64, processed, skipped, errors int64) error {
	_, err := s.db.ExecContext(ctx,
		`UPDATE backfill_checkpoints
		SET status = 'failed', records_processed = $2, records_skipped = $3,
			errors_count = $4, updated_at = NOW()
		WHERE id = $1`,
		id, processed, skipped, errors,
	)
	if err != nil {
		return fmt.Errorf("fail checkpoint: %w", err)
	}
	return nil
}
