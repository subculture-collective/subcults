// Package main is the entry point for the API server.
package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/onnwee/subcults/internal/api"
	"github.com/onnwee/subcults/internal/audit"
	"github.com/onnwee/subcults/internal/livekit"
	"github.com/onnwee/subcults/internal/middleware"
	"github.com/onnwee/subcults/internal/scene"
)

func main() {
	help := flag.Bool("help", false, "display help message")
	flag.Parse()

	if *help {
		fmt.Println("Subcults API Server")
		fmt.Println()
		fmt.Println("Usage: api [options]")
		fmt.Println()
		fmt.Println("Options:")
		flag.PrintDefaults()
		os.Exit(0)
	}

	// Get port from environment or default to 8080
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	// Initialize logger
	env := os.Getenv("SUBCULT_ENV")
	if env == "" {
		env = "development"
	}
	logger := middleware.NewLogger(env)
	slog.SetDefault(logger)

	// Initialize repositories
	eventRepo := scene.NewInMemoryEventRepository()
	sceneRepo := scene.NewInMemorySceneRepository()
	auditRepo := audit.NewInMemoryRepository()
	rsvpRepo := scene.NewInMemoryRSVPRepository()

	// Initialize LiveKit token service
	// Get credentials from environment variables
	livekitAPIKey := os.Getenv("LIVEKIT_API_KEY")
	livekitAPISecret := os.Getenv("LIVEKIT_API_SECRET")
	
	var livekitHandlers *api.LiveKitHandlers
	if livekitAPIKey != "" && livekitAPISecret != "" {
		tokenService, err := livekit.NewTokenService(livekitAPIKey, livekitAPISecret)
		if err != nil {
			logger.Error("failed to initialize LiveKit token service", "error", err)
			os.Exit(1)
		}
		livekitHandlers = api.NewLiveKitHandlers(tokenService, auditRepo)
		logger.Info("LiveKit token service initialized")
	} else {
		logger.Warn("LiveKit credentials not configured, token endpoint will not be available")
	}

	// Initialize handlers
	eventHandlers := api.NewEventHandlers(eventRepo, sceneRepo, auditRepo, rsvpRepo)
	rsvpHandlers := api.NewRSVPHandlers(rsvpRepo, eventRepo)

	// Create HTTP server with routes
	mux := http.NewServeMux()

	// Event routes
	mux.HandleFunc("/events", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodPost:
			eventHandlers.CreateEvent(w, r)
		default:
			ctx := middleware.SetErrorCode(r.Context(), api.ErrCodeBadRequest)
			api.WriteError(w, ctx, http.StatusMethodNotAllowed, api.ErrCodeBadRequest, "Method not allowed")
		}
	})

	mux.HandleFunc("/events/", func(w http.ResponseWriter, r *http.Request) {
		// Parse path to check for special endpoints
		// Expected patterns: /events/{id}, /events/{id}/cancel, /events/{id}/rsvp
		pathParts := strings.Split(strings.TrimPrefix(r.URL.Path, "/events/"), "/")
		
		// Check if this is a cancel request: /events/{id}/cancel
		if len(pathParts) == 2 && pathParts[0] != "" && pathParts[1] == "cancel" && r.Method == http.MethodPost {
			eventHandlers.CancelEvent(w, r)
			return
		}
		
		// Check if this is an RSVP request: /events/{id}/rsvp
		if len(pathParts) == 2 && pathParts[0] != "" && pathParts[1] == "rsvp" {
			switch r.Method {
			case http.MethodPost:
				rsvpHandlers.CreateOrUpdateRSVP(w, r)
			case http.MethodDelete:
				rsvpHandlers.DeleteRSVP(w, r)
			default:
				ctx := middleware.SetErrorCode(r.Context(), api.ErrCodeBadRequest)
				api.WriteError(w, ctx, http.StatusMethodNotAllowed, api.ErrCodeBadRequest, "Method not allowed")
			}
			return
		}
		
		switch r.Method {
		case http.MethodGet:
			eventHandlers.GetEvent(w, r)
		case http.MethodPatch:
			eventHandlers.UpdateEvent(w, r)
		default:
			ctx := middleware.SetErrorCode(r.Context(), api.ErrCodeBadRequest)
			api.WriteError(w, ctx, http.StatusMethodNotAllowed, api.ErrCodeBadRequest, "Method not allowed")
		}
	})

	// LiveKit token endpoint (if configured)
	if livekitHandlers != nil {
		mux.HandleFunc("/livekit/token", func(w http.ResponseWriter, r *http.Request) {
			if r.Method != http.MethodPost {
				ctx := middleware.SetErrorCode(r.Context(), api.ErrCodeBadRequest)
				api.WriteError(w, ctx, http.StatusMethodNotAllowed, api.ErrCodeBadRequest, "Method not allowed")
				return
			}
			livekitHandlers.IssueToken(w, r)
		})
	}

	// Health check endpoint
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		if _, err := w.Write([]byte(`{"status":"healthy"}`)); err != nil {
			slog.Error("failed to write health response", "error", err)
		}
	})

	// Placeholder root endpoint
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		// Only handle exact root path, everything else returns 404
		if r.URL.Path != "/" {
			// Return structured 404 error
			ctx := middleware.SetErrorCode(r.Context(), api.ErrCodeNotFound)
			api.WriteError(w, ctx, http.StatusNotFound, api.ErrCodeNotFound, "The requested resource was not found")
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		if _, err := w.Write([]byte(`{"service":"subcults-api","version":"0.0.1"}`)); err != nil {
			slog.Error("failed to write response", "error", err)
		}
	})

	// Apply middleware: RequestID -> Logging
	handler := middleware.RequestID(middleware.Logging(logger)(mux))

	server := &http.Server{
		Addr:         ":" + port,
		Handler:      handler,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// Start server in goroutine
	go func() {
		logger.Info("starting server", "port", port)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Error("server error", "error", err)
			os.Exit(1)
		}
	}()

	// Wait for interrupt signal for graceful shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	logger.Info("shutting down server...")

	// Create context with timeout for shutdown
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
		logger.Error("server forced to shutdown", "error", err)
		os.Exit(1)
	}

	logger.Info("server stopped")
}
