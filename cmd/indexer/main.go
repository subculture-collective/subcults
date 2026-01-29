// Package main is the entry point for the Jetstream indexer.
package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/onnwee/subcults/internal/indexer"
	"github.com/onnwee/subcults/internal/middleware"
	"github.com/prometheus/client_golang/prometheus"
)

func main() {
	help := flag.Bool("help", false, "display help message")
	flag.Parse()

	if *help {
		fmt.Println("Subcults Jetstream Indexer")
		fmt.Println()
		fmt.Println("Usage: indexer [options]")
		fmt.Println()
		fmt.Println("Options:")
		flag.PrintDefaults()
		os.Exit(0)
	}

	// Initialize logger
	// Defaults to production mode (JSON format) if SUBCULT_ENV not set
	env := os.Getenv("SUBCULT_ENV")
	if env == "" {
		env = "production"
	}
	logger := middleware.NewLogger(env)
	slog.SetDefault(logger)

	// Get metrics port from environment or default to 9090
	metricsPort := os.Getenv("METRICS_PORT")
	if metricsPort == "" {
		metricsPort = "9090"
	}

	// Get internal auth token from environment (optional)
	internalToken := os.Getenv("INTERNAL_AUTH_TOKEN")

	// Initialize Prometheus registry and metrics
	reg := prometheus.NewRegistry()
	metrics := indexer.NewMetrics()
	if err := metrics.Register(reg); err != nil {
		logger.Error("failed to register metrics", "error", err)
		os.Exit(1)
	}

	// Create HTTP server for metrics
	mux := http.NewServeMux()
	metricsHandler := indexer.MetricsHandler(reg)
	if internalToken != "" {
		metricsHandler = indexer.InternalAuthMiddleware(internalToken)(metricsHandler)
	}
	mux.Handle("/internal/indexer/metrics", metricsHandler)

	// Health check endpoint
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		if _, err := w.Write([]byte(`{"status":"healthy"}`)); err != nil {
			logger.Error("failed to write health response", "error", err)
		}
	})

	metricsServer := &http.Server{
		Addr:         ":" + metricsPort,
		Handler:      mux,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// Start metrics server in goroutine
	go func() {
		logger.Info("starting metrics server", "port", metricsPort)
		if err := metricsServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Error("metrics server error", "error", err)
			os.Exit(1)
		}
	}()

	// Initialize Jetstream client with backpressure handling
	jetstreamURL := os.Getenv("JETSTREAM_URL")
	if jetstreamURL == "" {
		jetstreamURL = "wss://jetstream1.us-east.bsky.network/subscribe"
		logger.Warn("JETSTREAM_URL not set, using default", "url", jetstreamURL)
	}

	config := indexer.DefaultConfig(jetstreamURL)

	// Message handler (placeholder - will be replaced with actual record processing)
	handler := func(messageType int, payload []byte) error {
		start := time.Now()

		// Decode CBOR message to extract timestamp for lag calculation
		msg, err := indexer.DecodeCBORMessage(payload)
		if err != nil {
			// Log error with context but don't fail - continue processing
			logger.Debug("failed to decode message for lag calculation",
				slog.String("error", err.Error()))
		} else if msg != nil && msg.TimeUS > 0 {
			// Calculate processing lag: current time - message timestamp
			messageTime := time.Unix(0, msg.TimeUS*1000) // Convert microseconds to nanoseconds
			lag := time.Since(messageTime)
			metrics.SetProcessingLag(lag.Seconds())

			logger.Debug("processing message",
				slog.String("kind", msg.Kind),
				slog.Duration("lag", lag))
		}

		// TODO: Implement actual record filtering and database persistence
		// For now, just increment the processed counter
		metrics.IncMessagesProcessed()

		// Record ingestion latency
		metrics.ObserveIngestLatency(time.Since(start).Seconds())

		// Example of how to track database failures:
		// if err := database.Write(record); err != nil {
		//     metrics.IncDatabaseWritesFailed()
		//     logger.Error("database write failed",
		//         slog.String("error", err.Error()))
		//     return err
		// }
		// metrics.IncUpserts()

		return nil
	}

	client, err := indexer.NewClientWithMetrics(config, handler, logger, metrics)
	if err != nil {
		logger.Error("failed to create jetstream client", "error", err)
		os.Exit(1)
	}

	// Start Jetstream client in background
	clientCtx, clientCancel := context.WithCancel(context.Background())
	defer clientCancel()

	clientDone := make(chan error, 1)
	go func() {
		logger.Info("starting jetstream client", "url", jetstreamURL)
		clientDone <- client.Run(clientCtx)
	}()

	// Wait for interrupt signal for graceful shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	select {
	case <-quit:
		logger.Info("received shutdown signal")
	case err := <-clientDone:
		logger.Error("jetstream client exited unexpectedly", "error", err)
	}

	logger.Info("shutting down indexer...")

	// Cancel client context
	clientCancel()

	// Wait for client to finish with longer timeout to account for drain
	select {
	case <-clientDone:
		logger.Info("jetstream client stopped")
	case <-time.After(15 * time.Second):
		logger.Warn("jetstream client shutdown timeout exceeded")
	}

	// Create context with timeout for metrics server shutdown
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := metricsServer.Shutdown(ctx); err != nil {
		logger.Error("metrics server forced to shutdown", "error", err)
		os.Exit(1)
	}

	logger.Info("indexer stopped")
}
