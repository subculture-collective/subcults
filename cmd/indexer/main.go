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
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
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
			slog.Error("failed to write health response", "error", err)
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

	// TODO: Initialize Jetstream indexer with metrics

	// Wait for interrupt signal for graceful shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	logger.Info("shutting down indexer...")

	// Create context with timeout for shutdown
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := metricsServer.Shutdown(ctx); err != nil {
		logger.Error("metrics server forced to shutdown", "error", err)
		os.Exit(1)
	}

	logger.Info("indexer stopped")
}
