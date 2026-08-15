// Package main runs sequence-bounded Jetstream v2 snapshots and CAR imports.
package main

import (
	"context"
	"database/sql"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	_ "github.com/lib/pq"
	"github.com/onnwee/subcults/internal/backfill"
	"github.com/onnwee/subcults/internal/indexer"
	"github.com/onnwee/subcults/internal/middleware"
)

const defaultJetstreamHost = "jetstream.us-west.bsky.network"

func main() {
	source := flag.String("source", "jetstream", "backfill source: jetstream or car")
	afterSeq := flag.Uint64("after-seq", 0, "exclusive Jetstream v2 lower sequence bound (0 = archive start)")
	beforeSeq := flag.Uint64("before-seq", 0, "inclusive Jetstream v2 upper sequence bound (0 = sealed archive tip)")
	target := flag.String("target", "shadow", "Jetstream projection target: shadow or active")
	rebuildID := flag.String("rebuild-id", "", "stable shadow rebuild name; required with --target=shadow")
	carPath := flag.String("car-file", "", "path to a CAR file in CAR mode")
	batchSize := flag.Int("batch", 1000, "maximum Jetstream events per SDK batch")
	dryRun := flag.Bool("dry-run", false, "validate without changing projection tables or projection cursors")
	resume := flag.Bool("resume", true, "resume from the durable sequence checkpoint for this target")
	help := flag.Bool("help", false, "display help message")
	flag.Parse()

	if *help {
		fmt.Println("Subcults Backfill Tool")
		fmt.Println()
		fmt.Println("Creates a sequence-bounded Jetstream v2 archive snapshot or imports a CAR file.")
		fmt.Println("Jetstream defaults to an isolated shadow projection; active-table writes require --target=active.")
		fmt.Println()
		fmt.Println("Usage: backfill [options]")
		flag.PrintDefaults()
		fmt.Println()
		fmt.Println("Examples:")
		fmt.Println("  backfill --source=jetstream --target=shadow --rebuild-id=release-2026-08 --after-seq=0")
		fmt.Println("  backfill --source=jetstream --target=shadow --rebuild-id=analysis --after-seq=1000 --before-seq=5000")
		fmt.Println("  backfill --source=car --car-file=export.car")
		return
	}

	env := os.Getenv("SUBCULT_ENV")
	if env == "" {
		env = "development"
	}
	logger := middleware.NewLogger(env)
	slog.SetDefault(logger)
	if err := validateFlags(*source, *target, *rebuildID, *carPath, *afterSeq, *beforeSeq, *batchSize); err != nil {
		logger.Error("invalid backfill configuration", "error", err)
		os.Exit(1)
	}

	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		logger.Error("DATABASE_URL is required")
		os.Exit(1)
	}
	database, err := sql.Open("postgres", databaseURL)
	if err != nil {
		logger.Error("failed to open database", "error", err)
		os.Exit(1)
	}
	defer func() {
		if closeErr := database.Close(); closeErr != nil {
			logger.Warn("failed to close database", "error", closeErr)
		}
	}()
	connectCtx, connectCancel := context.WithTimeout(context.Background(), 5*time.Second)
	if err = database.PingContext(connectCtx); err != nil {
		connectCancel()
		logger.Error("failed to connect to database", "error", err)
		os.Exit(1)
	}
	connectCancel()

	filter := indexer.NewRecordFilter(indexer.NewFilterMetrics())
	repo := indexer.NewPostgresRecordRepository(database, logger)
	checkpointStore := backfill.NewPostgresCheckpointStore(database, logger)
	cfg := backfill.Config{
		Source:    *source,
		AfterSeq:  *afterSeq,
		CARPath:   *carPath,
		BatchSize: *batchSize,
		DryRun:    *dryRun,
		Resume:    *resume,
		Logger:    logger,
		Target:    indexer.ProjectionTargetActive,
		RebuildID: "",
	}
	if *source == "jetstream" {
		cfg.Target = indexer.ProjectionTarget(*target)
		cfg.RebuildID = *rebuildID
		cfg.JetstreamHost = os.Getenv("JETSTREAM_HOST")
		if cfg.JetstreamHost == "" {
			cfg.JetstreamHost = defaultJetstreamHost
		}
		cfg.JetstreamAPIKey = os.Getenv("JETSTREAM_API_KEY")
		cfg.JetstreamProjector = indexer.NewPostgresV2Projector(database, filter, logger)
		if *beforeSeq != 0 {
			cfg.BeforeSeq = beforeSeq
		}
	}
	runner := backfill.NewRunner(cfg, repo, filter, checkpointStore)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	signals := make(chan os.Signal, 1)
	signal.Notify(signals, syscall.SIGINT, syscall.SIGTERM)
	defer signal.Stop(signals)
	go func() {
		sig := <-signals
		logger.Info("received signal, stopping backfill", "signal", sig)
		cancel()
	}()

	logger.Info("starting backfill",
		"source", cfg.Source,
		"after_seq", cfg.AfterSeq,
		"before_seq", cfg.BeforeSeq,
		"target", cfg.Target,
		"rebuild_id", cfg.RebuildID,
		"batch_size", cfg.BatchSize,
		"dry_run", cfg.DryRun,
		"resume", cfg.Resume)
	result, err := runner.Run(ctx)
	if err != nil {
		logger.Error("backfill failed", "error", err)
		os.Exit(1)
	}
	logger.Info("backfill completed",
		"records_processed", result.RecordsProcessed,
		"records_skipped", result.RecordsSkipped,
		"errors", result.Errors,
		"duration", result.Duration)
}

func validateFlags(source, target, rebuildID, carPath string, afterSeq, beforeSeq uint64, batchSize int) error {
	if source != "jetstream" && source != "car" {
		return fmt.Errorf("source must be jetstream or car, got %q", source)
	}
	if batchSize <= 0 {
		return errors.New("batch must be a positive integer")
	}
	if source == "car" {
		if carPath == "" {
			return errors.New("car-file is required in CAR mode")
		}
		return nil
	}
	if target != string(indexer.ProjectionTargetActive) && target != string(indexer.ProjectionTargetShadow) {
		return fmt.Errorf("target must be active or shadow, got %q", target)
	}
	if target == string(indexer.ProjectionTargetShadow) && rebuildID == "" {
		return errors.New("rebuild-id is required for a shadow projection")
	}
	if target == string(indexer.ProjectionTargetActive) && rebuildID != "" {
		return errors.New("rebuild-id is only valid for a shadow projection")
	}
	if beforeSeq != 0 && beforeSeq <= afterSeq {
		return fmt.Errorf("before-seq %d must be greater than after-seq %d", beforeSeq, afterSeq)
	}
	return nil
}
