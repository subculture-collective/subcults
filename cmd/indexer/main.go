// Package main runs the Subcults Jetstream v2 replay-to-live indexer.
package main

import (
	"context"
	"database/sql"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	_ "github.com/lib/pq"
	"github.com/onnwee/subcults/internal/atprotocol"
	appdb "github.com/onnwee/subcults/internal/db"
	"github.com/onnwee/subcults/internal/indexer"
	"github.com/onnwee/subcults/internal/middleware"
	"github.com/prometheus/client_golang/prometheus"
)

const (
	defaultJetstreamHost = "jetstream.us-west.bsky.network"
	defaultBatchSize     = 256
	activeConsumer       = "indexer-live"
)

func main() {
	help := flag.Bool("help", false, "display help message")
	flag.Parse()
	if *help {
		fmt.Println("Subcults Jetstream v2 Indexer")
		fmt.Println()
		fmt.Println("Usage: indexer [options]")
		fmt.Println()
		fmt.Println("The indexer resumes from its durable v2 sequence, replays any missing archive range, and cuts over to live delivery.")
		flag.PrintDefaults()
		return
	}

	env := os.Getenv("SUBCULT_ENV")
	if env == "" {
		env = "production"
	}
	logger := middleware.NewLogger(env)
	slog.SetDefault(logger)

	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		logger.Error("DATABASE_URL is required for durable Jetstream v2 cursor commits")
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
		logger.Error("failed to ping database", "error", err)
		os.Exit(1)
	}
	connectCancel()
	logger.Info("using Postgres Jetstream v2 projection")

	registry := prometheus.NewRegistry()
	metrics := indexer.NewMetrics()
	if err = metrics.Register(registry); err != nil {
		logger.Error("failed to register indexer metrics", "error", err)
		os.Exit(1)
	}
	dbMetrics := appdb.NewSlowQueryMetrics()
	if err = dbMetrics.Register(registry); err != nil {
		logger.Error("failed to register database metrics", "error", err)
		os.Exit(1)
	}
	metricsServer := newMetricsServer(registry, logger)
	serverDone := make(chan error, 1)
	go func() {
		logger.Info("starting metrics server", "address", metricsServer.Addr)
		serverDone <- metricsServer.ListenAndServe()
	}()

	filter := indexer.NewRecordFilter(indexer.NewFilterMetrics())
	projector := indexer.NewPostgresV2Projector(database, filter, logger)
	request := indexer.ProjectionRequest{Consumer: activeConsumer, Target: indexer.ProjectionTargetActive}
	appCtx, appCancel := context.WithCancel(context.Background())
	defer appCancel()
	cursor, err := projector.Cursor(appCtx, request)
	if err != nil {
		logger.Error("failed to read Jetstream v2 cursor", "error", err)
		os.Exit(1)
	}

	batchSize, err := envInt("JETSTREAM_BATCH_SIZE", defaultBatchSize)
	if err != nil {
		logger.Error("invalid Jetstream batch size", "error", err)
		os.Exit(1)
	}
	host := os.Getenv("JETSTREAM_HOST")
	if host == "" {
		host = defaultJetstreamHost
		logger.Warn("JETSTREAM_HOST not set, using official v2 default", "host", host)
	}
	collections := append([]string{}, atprotocol.CanonicalCollections...)
	collections = append(collections,
		indexer.CollectionScene,
		indexer.CollectionEvent,
		indexer.CollectionPost,
		indexer.CollectionAlliance,
	)
	stream, err := indexer.SubscribeV2(indexer.V2Subscription{
		Host:        host,
		APIKey:      os.Getenv("JETSTREAM_API_KEY"),
		Collections: collections,
		AfterSeq:    cursor,
		Replay:      true,
		BatchSize:   batchSize,
		Logger:      logger,
	})
	if err != nil {
		logger.Error("failed to initialize Jetstream v2 stream", "error", err)
		os.Exit(1)
	}
	defer func() {
		if closeErr := stream.Close(); closeErr != nil {
			logger.Warn("failed to close Jetstream v2 stream", "error", closeErr)
		}
	}()
	logger.Info("starting Jetstream v2 replay-to-live consumer",
		"host", host,
		"after_seq", cursor,
		"batch_size", batchSize)

	cleanupService := indexer.NewCleanupService(database, logger, indexer.DefaultCleanupConfig())
	cleanupService.Start(appCtx)
	defer cleanupService.Stop()

	consumerDone := make(chan error, 1)
	go func() {
		consumerDone <- indexer.RunV2Consumer(appCtx, stream, projector, request, logger, metrics)
	}()
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	defer signal.Stop(quit)

	var runErr error
	consumerExited := false
	select {
	case sig := <-quit:
		logger.Info("received shutdown signal", "signal", sig)
	case runErr = <-consumerDone:
		consumerExited = true
		if runErr != nil && !errors.Is(runErr, context.Canceled) {
			logger.Error("Jetstream v2 consumer stopped", "error", runErr)
		}
	case serverErr := <-serverDone:
		if serverErr != nil && !errors.Is(serverErr, http.ErrServerClosed) {
			runErr = fmt.Errorf("metrics server: %w", serverErr)
			logger.Error("metrics server stopped", "error", serverErr)
		}
	}

	appCancel()
	if closeErr := stream.Close(); closeErr != nil {
		logger.Warn("failed to close Jetstream v2 stream during shutdown", "error", closeErr)
	}
	if !consumerExited {
		select {
		case consumerErr := <-consumerDone:
			if runErr == nil && consumerErr != nil && !errors.Is(consumerErr, context.Canceled) {
				runErr = consumerErr
			}
		case <-time.After(15 * time.Second):
			logger.Warn("Jetstream v2 consumer shutdown timeout exceeded")
		}
	}
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()
	if shutdownErr := metricsServer.Shutdown(shutdownCtx); shutdownErr != nil {
		logger.Error("metrics server shutdown failed", "error", shutdownErr)
		os.Exit(1)
	}
	if runErr != nil {
		os.Exit(1)
	}
	logger.Info("indexer stopped")
}

func newMetricsServer(registry *prometheus.Registry, logger *slog.Logger) *http.Server {
	metricsPort := os.Getenv("METRICS_PORT")
	if metricsPort == "" {
		metricsPort = "9090"
	}
	metricsHandler := indexer.MetricsHandler(registry)
	if token := os.Getenv("INTERNAL_AUTH_TOKEN"); token != "" {
		metricsHandler = indexer.InternalAuthMiddleware(token)(metricsHandler)
	}
	mux := http.NewServeMux()
	mux.Handle("/internal/indexer/metrics", metricsHandler)
	mux.HandleFunc("/health", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		if _, err := w.Write([]byte(`{"status":"healthy"}`)); err != nil {
			logger.Error("failed to write health response", "error", err)
		}
	})
	return &http.Server{
		Addr:         ":" + metricsPort,
		Handler:      mux,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}
}

func envInt(name string, fallback int) (int, error) {
	raw := os.Getenv(name)
	if raw == "" {
		return fallback, nil
	}
	value, err := strconv.Atoi(raw)
	if err != nil || value <= 0 {
		return 0, fmt.Errorf("%s must be a positive integer", name)
	}
	return value, nil
}
