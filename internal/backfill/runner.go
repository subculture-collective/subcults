package backfill

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"time"

	jetstream "github.com/bluesky-social/jetstream"
	"github.com/onnwee/subcults/internal/atprotocol"
	"github.com/onnwee/subcults/internal/indexer"
)

// Runner orchestrates a backfill operation with checkpoint-based resume.
type Runner struct {
	config     Config
	repo       indexer.RecordRepository
	filter     *indexer.RecordFilter
	checkpoint CheckpointStore
	logger     *slog.Logger
}

// NewRunner creates a backfill runner.
func NewRunner(cfg Config, repo indexer.RecordRepository, filter *indexer.RecordFilter, checkpoint CheckpointStore) *Runner {
	if cfg.Logger == nil {
		cfg.Logger = slog.Default()
	}
	if cfg.BatchSize <= 0 {
		cfg.BatchSize = 1000
	}
	if cfg.Target == "" {
		cfg.Target = indexer.ProjectionTargetActive
	}
	if cfg.JetstreamFactory == nil {
		cfg.JetstreamFactory = indexer.SubscribeV2
	}
	return &Runner{
		config:     cfg,
		repo:       repo,
		filter:     filter,
		checkpoint: checkpoint,
		logger:     cfg.Logger,
	}
}

// Run executes the backfill operation.
func (r *Runner) Run(ctx context.Context) (*Result, error) {
	start := time.Now()
	switch r.config.Source {
	case "jetstream":
		return r.runJetstream(ctx, start)
	case "car":
		return r.runCAR(ctx, start)
	default:
		return nil, fmt.Errorf("unsupported source: %s", r.config.Source)
	}
}

func (r *Runner) runJetstream(ctx context.Context, start time.Time) (*Result, error) {
	if r.config.JetstreamProjector == nil {
		return nil, errors.New("jetstream v2 projector is required")
	}
	request := indexer.ProjectionRequest{
		Consumer:  "backfill-active",
		Target:    r.config.Target,
		RebuildID: r.config.RebuildID,
		DryRun:    r.config.DryRun,
	}
	if r.config.Target == indexer.ProjectionTargetShadow {
		request.Consumer = "backfill-shadow:" + r.config.RebuildID
	}
	afterSeq := r.config.AfterSeq
	projectedCursor, err := r.config.JetstreamProjector.Cursor(ctx, request)
	if err != nil {
		return nil, fmt.Errorf("read projection cursor: %w", err)
	}
	if projectedCursor > afterSeq {
		afterSeq = projectedCursor
	}
	if r.config.Resume && !r.config.DryRun {
		cp, err := r.checkpoint.GetLatest(ctx, "jetstream", r.config.Target, r.config.RebuildID)
		if err != nil {
			return nil, fmt.Errorf("failed to get checkpoint: %w", err)
		}
		if cp != nil && cp.CursorSeq > afterSeq {
			afterSeq = cp.CursorSeq
			r.logger.Info("resuming from checkpoint",
				"checkpoint_id", cp.ID,
				"cursor_seq", cp.CursorSeq,
			)
		}
	}
	if r.config.BeforeSeq != nil && *r.config.BeforeSeq < afterSeq {
		return nil, fmt.Errorf("before sequence %d is behind durable projection cursor %d", *r.config.BeforeSeq, afterSeq)
	}
	if r.config.BeforeSeq != nil && *r.config.BeforeSeq == afterSeq {
		return &Result{Duration: time.Since(start)}, nil
	}

	var cpID int64
	if !r.config.DryRun {
		cpID, err = r.checkpoint.Create(ctx, "jetstream", r.config.Target, r.config.RebuildID)
		if err != nil {
			return nil, fmt.Errorf("failed to create checkpoint: %w", err)
		}
	}
	result := &Result{}
	fail := func(runErr error) (*Result, error) {
		result.Duration = time.Since(start)
		if cpID != 0 {
			checkpointCtx, checkpointCancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer checkpointCancel()
			if checkpointErr := r.checkpoint.Fail(checkpointCtx, cpID, result.RecordsProcessed, result.RecordsSkipped, result.Errors); checkpointErr != nil {
				r.logger.Error("failed to mark checkpoint failed", "error", checkpointErr)
			}
		}
		return result, runErr
	}

	stream, err := r.config.JetstreamFactory(indexer.V2Subscription{
		Host:         r.config.JetstreamHost,
		APIKey:       r.config.JetstreamAPIKey,
		Collections:  supportedCollections(),
		AfterSeq:     afterSeq,
		Replay:       true,
		BeforeSeq:    r.config.BeforeSeq,
		SnapshotOnly: true,
		BatchSize:    r.config.BatchSize,
		Logger:       r.logger,
	})
	if err != nil {
		return fail(fmt.Errorf("open Jetstream v2 snapshot: %w", err))
	}
	defer func() {
		if closeErr := stream.Close(); closeErr != nil {
			r.logger.Warn("failed to close Jetstream v2 backfill stream", "error", closeErr)
		}
	}()

	r.logger.Info("starting Jetstream v2 snapshot backfill",
		"after_seq", afterSeq,
		"before_seq", r.config.BeforeSeq,
		"target", r.config.Target,
		"rebuild_id", r.config.RebuildID,
		"dry_run", r.config.DryRun,
	)
	for batch, streamErr := range stream.Events(ctx) {
		if streamErr != nil {
			result.Errors++
			if errors.Is(streamErr, jetstream.ErrFatal) {
				return fail(fmt.Errorf("fatal Jetstream v2 snapshot failure: %w", streamErr))
			}
			r.logger.Warn("recoverable Jetstream v2 snapshot error", "error", streamErr)
			continue
		}
		batchResult, applyErr := r.config.JetstreamProjector.ApplyBatch(ctx, request, batch.Events, batch.Cursor)
		if applyErr != nil {
			result.Errors++
			return fail(fmt.Errorf("project Jetstream v2 snapshot batch ending at %d: %w", batch.Cursor, applyErr))
		}
		result.RecordsProcessed += batchResult.Processed
		result.RecordsSkipped += batchResult.Skipped
		result.Errors += batchResult.Quarantined
		if cpID != 0 {
			checkpoint := &Checkpoint{
				ID:               cpID,
				Source:           "jetstream",
				CursorSeq:        batch.Cursor,
				Target:           r.config.Target,
				RebuildID:        r.config.RebuildID,
				RecordsProcessed: result.RecordsProcessed,
				RecordsSkipped:   result.RecordsSkipped,
				ErrorsCount:      result.Errors,
			}
			if updateErr := r.checkpoint.Update(ctx, checkpoint); updateErr != nil {
				return fail(fmt.Errorf("update Jetstream v2 checkpoint: %w", updateErr))
			}
		}
	}
	if ctx.Err() != nil {
		return fail(ctx.Err())
	}

	if cpID != 0 {
		if err := r.checkpoint.Complete(ctx, cpID, result.RecordsProcessed, result.RecordsSkipped, result.Errors); err != nil {
			return fail(fmt.Errorf("mark Jetstream v2 checkpoint complete: %w", err))
		}
	}
	result.Duration = time.Since(start)
	return result, nil
}

// ProcessRecord processes a single record through the filter and repository.
func (r *Runner) ProcessRecord(ctx context.Context, collection string, payload []byte, did, rkey, rev string) error {
	filterResult := r.filter.Filter(collection, payload)
	if !filterResult.Matched {
		return nil
	}
	filterResult.DID = did
	filterResult.RKey = rkey
	filterResult.Rev = rev
	filterResult.Operation = "create"
	if !filterResult.Valid {
		return fmt.Errorf("validation failed for %s/%s: %w", collection, rkey, filterResult.Error)
	}
	if r.config.DryRun {
		r.logger.Debug("dry-run: would upsert record",
			"collection", collection, "did", did, "rkey", rkey,
		)
		return nil
	}
	_, _, err := r.repo.UpsertRecord(ctx, &filterResult)
	return err
}

func (r *Runner) runCAR(ctx context.Context, start time.Time) (*Result, error) {
	cpID, err := r.checkpoint.Create(ctx, "car", indexer.ProjectionTargetActive, "")
	if err != nil {
		return nil, fmt.Errorf("failed to create checkpoint: %w", err)
	}
	result := &Result{}
	r.logger.Info("starting CAR file import",
		"path", r.config.CARPath,
		"dry_run", r.config.DryRun,
	)

	f, err := os.Open(r.config.CARPath)
	if err != nil {
		_ = r.checkpoint.Fail(ctx, cpID, 0, 0, 1)
		return nil, fmt.Errorf("failed to open CAR file: %w", err)
	}
	defer func() {
		if closeErr := f.Close(); closeErr != nil {
			r.logger.Warn("failed to close CAR file", "error", closeErr)
		}
	}()

	carReader, err := indexer.NewCARReader(f, r.logger)
	if err != nil {
		_ = r.checkpoint.Fail(ctx, cpID, 0, 0, 1)
		return nil, fmt.Errorf("failed to parse CAR header: %w", err)
	}

	importer := indexer.NewCARImporter(r.repo, r.filter, r.logger)
	importResult, err := importer.Import(ctx, carReader, r.config.DryRun)
	if err != nil {
		result.Errors = importResult.Errors
		_ = r.checkpoint.Fail(ctx, cpID, importResult.RecordsProcessed, importResult.RecordsSkipped, importResult.Errors)
		return nil, fmt.Errorf("CAR import failed: %w", err)
	}

	result.RecordsProcessed = importResult.RecordsProcessed
	result.RecordsSkipped = importResult.RecordsSkipped
	result.Errors = importResult.Errors

	if err := r.checkpoint.Complete(ctx, cpID, result.RecordsProcessed, result.RecordsSkipped, result.Errors); err != nil {
		r.logger.Error("failed to mark checkpoint complete", "error", err)
	}
	result.Duration = time.Since(start)
	return result, nil
}

func supportedCollections() []string {
	collections := append([]string{}, atprotocol.CanonicalCollections...)
	return append(collections,
		indexer.CollectionScene,
		indexer.CollectionEvent,
		indexer.CollectionPost,
		indexer.CollectionAlliance,
	)
}
