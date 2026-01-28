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
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	"github.com/onnwee/subcults/internal/api"
	"github.com/onnwee/subcults/internal/attachment"
	"github.com/onnwee/subcults/internal/audit"
	"github.com/onnwee/subcults/internal/idempotency"
	"github.com/onnwee/subcults/internal/livekit"
	"github.com/onnwee/subcults/internal/membership"
	"github.com/onnwee/subcults/internal/middleware"
	"github.com/onnwee/subcults/internal/payment"
	"github.com/onnwee/subcults/internal/post"
	"github.com/onnwee/subcults/internal/scene"
	"github.com/onnwee/subcults/internal/stream"
	"github.com/onnwee/subcults/internal/trust"
	"github.com/onnwee/subcults/internal/upload"
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

	// Parse trust ranking feature flag from environment
	// Accepts: true/false, 1/0, yes/no, on/off (case-insensitive)
	// Default: false (safe rollout)
	rankTrustEnabled := false
	if val := os.Getenv("RANK_TRUST_ENABLED"); val != "" {
		valLower := strings.ToLower(val)
		switch valLower {
		case "true", "1", "yes", "on":
			rankTrustEnabled = true
		case "false", "0", "no", "off":
			rankTrustEnabled = false
		}
	}

	// Initialize trust ranking feature flag
	trust.SetRankingEnabled(rankTrustEnabled)
	logger.Info("trust ranking enabled", "component", "trust", "state", rankTrustEnabled)

	// Initialize repositories
	eventRepo := scene.NewInMemoryEventRepository()
	sceneRepo := scene.NewInMemorySceneRepository()
	auditRepo := audit.NewInMemoryRepository()
	rsvpRepo := scene.NewInMemoryRSVPRepository()
	streamRepo := stream.NewInMemorySessionRepository()
	analyticsRepo := stream.NewInMemoryAnalyticsRepository(streamRepo)
	postRepo := post.NewInMemoryPostRepository()
	membershipRepo := membership.NewInMemoryMembershipRepository()

	// Initialize trust score components
	trustDataSource := trust.NewInMemoryDataSource()
	trustScoreStore := trust.NewInMemoryScoreStore()
	trustDirtyTracker := trust.NewDirtyTracker()

	// Initialize Prometheus metrics
	promRegistry := prometheus.NewRegistry()
	streamMetrics := stream.NewMetrics()
	if err := streamMetrics.Register(promRegistry); err != nil {
		logger.Error("failed to register stream metrics", "error", err)
		os.Exit(1)
	}
	logger.Info("stream metrics registered")

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

	// Initialize Upload service for R2 signed URLs
	// Get R2 credentials from environment variables
	r2BucketName := os.Getenv("R2_BUCKET_NAME")
	r2AccessKeyID := os.Getenv("R2_ACCESS_KEY_ID")
	r2SecretAccessKey := os.Getenv("R2_SECRET_ACCESS_KEY")
	r2Endpoint := os.Getenv("R2_ENDPOINT")
	r2MaxSizeMB := 15 // Default 15MB
	if maxSizeStr := os.Getenv("R2_MAX_UPLOAD_SIZE_MB"); maxSizeStr != "" {
		if parsed, err := strconv.Atoi(maxSizeStr); err == nil && parsed > 0 {
			r2MaxSizeMB = parsed
		}
	}

	var uploadHandlers *api.UploadHandlers
	var uploadService *upload.Service
	if r2BucketName != "" && r2AccessKeyID != "" && r2SecretAccessKey != "" && r2Endpoint != "" {
		var err error
		uploadService, err = upload.NewService(upload.ServiceConfig{
			BucketName:       r2BucketName,
			AccessKeyID:      r2AccessKeyID,
			SecretAccessKey:  r2SecretAccessKey,
			Endpoint:         r2Endpoint,
			MaxSizeMB:        r2MaxSizeMB,
			URLExpiryMinutes: 5, // 5 minutes expiry
		})
		if err != nil {
			logger.Error("failed to initialize upload service", "error", err)
			os.Exit(1)
		}
		uploadHandlers = api.NewUploadHandlers(uploadService)
		logger.Info("upload service initialized", "bucket", r2BucketName, "max_size_mb", r2MaxSizeMB)
	} else {
		logger.Warn("R2 credentials not configured, upload endpoint will not be available")
	}

	// Initialize attachment metadata service (if upload service is configured)
	var metadataService *attachment.MetadataService
	if uploadService != nil {
		var err error
		metadataService, err = attachment.NewMetadataService(attachment.MetadataServiceConfig{
			S3Client:   uploadService.GetS3Client(),
			BucketName: uploadService.GetBucketName(),
		})
		if err != nil {
			logger.Error("failed to initialize metadata service", "error", err)
			os.Exit(1)
		}
		logger.Info("attachment metadata service initialized")
	}

	// Initialize Stripe payment handlers
	// Get Stripe credentials from environment variables
	stripeAPIKey := os.Getenv("STRIPE_API_KEY")
	stripeOnboardingReturnURL := os.Getenv("STRIPE_ONBOARDING_RETURN_URL")
	stripeOnboardingRefreshURL := os.Getenv("STRIPE_ONBOARDING_REFRESH_URL")
	
	// Parse application fee percentage (default: 5.0%)
	stripeApplicationFeePercent := 5.0
	if feePercentStr := os.Getenv("STRIPE_APPLICATION_FEE_PERCENT"); feePercentStr != "" {
		if parsed, err := strconv.ParseFloat(feePercentStr, 64); err == nil {
			stripeApplicationFeePercent = parsed
		} else {
			logger.Warn("invalid STRIPE_APPLICATION_FEE_PERCENT, using default 5.0%", "error", err)
		}
	}
	
	// Validate fee percentage
	if stripeApplicationFeePercent < 0 || stripeApplicationFeePercent >= 100 {
		logger.Error("invalid STRIPE_APPLICATION_FEE_PERCENT: must be between 0 and 100", "value", stripeApplicationFeePercent)
		os.Exit(1)
	}

	// Parse application fee percentage (default: 5.0%)
	stripeApplicationFeePercent := 5.0
	if feePercentStr := os.Getenv("STRIPE_APPLICATION_FEE_PERCENT"); feePercentStr != "" {
		if parsed, err := strconv.ParseFloat(feePercentStr, 64); err == nil {
			stripeApplicationFeePercent = parsed
		} else {
			logger.Warn("invalid STRIPE_APPLICATION_FEE_PERCENT, using default 5.0%", "error", err)
		}
	}

	// Validate fee percentage
	if stripeApplicationFeePercent < 0 || stripeApplicationFeePercent >= 100 {
		logger.Error("invalid STRIPE_APPLICATION_FEE_PERCENT: must be between 0 and 100", "value", stripeApplicationFeePercent)
		os.Exit(1)
	}

	var paymentHandlers *api.PaymentHandlers
	var webhookHandlers *api.WebhookHandlers
	var idempotencyMiddleware func(http.Handler) http.Handler
	stripeWebhookSecret := os.Getenv("STRIPE_WEBHOOK_SECRET")
	
	if stripeAPIKey != "" && stripeOnboardingReturnURL != "" && stripeOnboardingRefreshURL != "" {
		stripeClient := payment.NewStripeClient(stripeAPIKey)
		paymentRepo := payment.NewInMemoryPaymentRepository()
		webhookRepo := payment.NewInMemoryWebhookRepository()
		
		// Initialize idempotency repository for payment operations
		idempotencyRepo := idempotency.NewInMemoryRepository()
		idempotencyRoutes := map[string]bool{
			"/payments/checkout": true,
		}
		idempotencyMiddleware = middleware.IdempotencyMiddleware(idempotencyRepo, idempotencyRoutes)
		logger.Info("idempotency middleware initialized", "routes", idempotencyRoutes)
		
		paymentHandlers = api.NewPaymentHandlers(
			sceneRepo,
			paymentRepo,
			stripeClient,
			stripeOnboardingReturnURL,
			stripeOnboardingRefreshURL,
			stripeApplicationFeePercent,
		)
		logger.Info("Stripe payment handlers initialized", "application_fee_percent", stripeApplicationFeePercent)
		
		// Initialize webhook handler if secret is configured
		if stripeWebhookSecret != "" {
			webhookHandlers = api.NewWebhookHandlers(
				stripeWebhookSecret,
				paymentRepo,
				webhookRepo,
				sceneRepo,
			)
			logger.Info("Stripe webhook handler initialized")
		} else {
			logger.Warn("STRIPE_WEBHOOK_SECRET not configured, webhook endpoint will not be available")
		}
	} else {
		logger.Warn("Stripe credentials not fully configured, payment endpoints will not be available")
	}

	// Initialize handlers
	// Pass trustScoreStore to eventHandlers to enable trust-weighted ranking
	trustStoreAdapter := api.NewTrustScoreStoreAdapter(trustScoreStore)
	eventHandlers := api.NewEventHandlers(eventRepo, sceneRepo, auditRepo, rsvpRepo, streamRepo, trustStoreAdapter)
	rsvpHandlers := api.NewRSVPHandlers(rsvpRepo, eventRepo)
	streamHandlers := api.NewStreamHandlers(streamRepo, analyticsRepo, sceneRepo, eventRepo, auditRepo, streamMetrics)
	postHandlers := api.NewPostHandlers(postRepo, sceneRepo, membershipRepo, metadataService)
	trustHandlers := api.NewTrustHandlers(sceneRepo, trustDataSource, trustScoreStore, trustDirtyTracker)
	searchHandlers := api.NewSearchHandlers(sceneRepo, postRepo, trustStoreAdapter)

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
		// Expected patterns: /events/{id}, /events/{id}/cancel, /events/{id}/rsvp, /events/{id}/feed
		pathParts := strings.Split(strings.TrimPrefix(r.URL.Path, "/events/"), "/")

		// Check if this is a feed request: /events/{id}/feed
		if len(pathParts) == 2 && pathParts[0] != "" && pathParts[1] == "feed" && r.Method == http.MethodGet {
			postHandlers.GetEventFeed(w, r)
			return
		}

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

	// Scene feed route
	mux.HandleFunc("/scenes/", func(w http.ResponseWriter, r *http.Request) {
		// Parse path to check for feed endpoint
		// Expected pattern: /scenes/{id}/feed
		pathParts := strings.Split(strings.TrimPrefix(r.URL.Path, "/scenes/"), "/")

		// Check if this is a feed request: /scenes/{id}/feed
		if len(pathParts) == 2 && pathParts[0] != "" && pathParts[1] == "feed" && r.Method == http.MethodGet {
			postHandlers.GetSceneFeed(w, r)
			return
		}

		// No other scene endpoints yet, return 404
		ctx := middleware.SetErrorCode(r.Context(), api.ErrCodeNotFound)
		api.WriteError(w, ctx, http.StatusNotFound, api.ErrCodeNotFound, "The requested resource was not found")
	})

	// Search endpoints
	mux.HandleFunc("/search/events", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			eventHandlers.SearchEvents(w, r)
		default:
			ctx := middleware.SetErrorCode(r.Context(), api.ErrCodeBadRequest)
			api.WriteError(w, ctx, http.StatusMethodNotAllowed, api.ErrCodeBadRequest, "Method not allowed")
		}
	})

	mux.HandleFunc("/search/scenes", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			searchHandlers.SearchScenes(w, r)
		default:
			ctx := middleware.SetErrorCode(r.Context(), api.ErrCodeBadRequest)
			api.WriteError(w, ctx, http.StatusMethodNotAllowed, api.ErrCodeBadRequest, "Method not allowed")
		}
	})

	mux.HandleFunc("/search/posts", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			searchHandlers.SearchPosts(w, r)
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

	// Stream session routes
	mux.HandleFunc("/streams", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			ctx := middleware.SetErrorCode(r.Context(), api.ErrCodeBadRequest)
			api.WriteError(w, ctx, http.StatusMethodNotAllowed, api.ErrCodeBadRequest, "Method not allowed")
			return
		}
		streamHandlers.CreateStream(w, r)
	})

	mux.HandleFunc("/streams/", func(w http.ResponseWriter, r *http.Request) {
		// Expected patterns: /streams/{id}/end, /streams/{id}/join, /streams/{id}/leave, /streams/{id}/analytics
		pathParts := strings.Split(strings.TrimPrefix(r.URL.Path, "/streams/"), "/")

		// Check if this is an analytics request: /streams/{id}/analytics
		if len(pathParts) == 2 && pathParts[0] != "" && pathParts[1] == "analytics" && r.Method == http.MethodGet {
			streamHandlers.GetStreamAnalytics(w, r)
			return
		}

		// Check if this is an end request: /streams/{id}/end
		if len(pathParts) == 2 && pathParts[0] != "" && pathParts[1] == "end" && r.Method == http.MethodPost {
			streamHandlers.EndStream(w, r)
			return
		}

		// Check if this is a join request: /streams/{id}/join
		if len(pathParts) == 2 && pathParts[0] != "" && pathParts[1] == "join" && r.Method == http.MethodPost {
			streamHandlers.JoinStream(w, r)
			return
		}

		// Check if this is a leave request: /streams/{id}/leave
		if len(pathParts) == 2 && pathParts[0] != "" && pathParts[1] == "leave" && r.Method == http.MethodPost {
			streamHandlers.LeaveStream(w, r)
			return
		}

		// No other stream endpoints yet, return 404
		ctx := middleware.SetErrorCode(r.Context(), api.ErrCodeNotFound)
		api.WriteError(w, ctx, http.StatusNotFound, api.ErrCodeNotFound, "The requested resource was not found")
	})

	// Metrics endpoint (Prometheus) - protected with bearer token auth if configured
	metricsToken := os.Getenv("METRICS_AUTH_TOKEN")
	metricsHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// If token is configured, enforce authentication
		if metricsToken != "" {
			const bearerPrefix = "Bearer "
			authHeader := r.Header.Get("Authorization")
			if !strings.HasPrefix(authHeader, bearerPrefix) || authHeader[len(bearerPrefix):] != metricsToken {
				http.Error(w, http.StatusText(http.StatusUnauthorized), http.StatusUnauthorized)
				return
			}
		}
		// If no token is configured, allow unauthenticated access (for development)
		promhttp.HandlerFor(promRegistry, promhttp.HandlerOpts{}).ServeHTTP(w, r)
	})
	mux.Handle("/metrics", metricsHandler)

	// Post routes
	mux.HandleFunc("/posts", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodPost:
			postHandlers.CreatePost(w, r)
		default:
			ctx := middleware.SetErrorCode(r.Context(), api.ErrCodeBadRequest)
			api.WriteError(w, ctx, http.StatusMethodNotAllowed, api.ErrCodeBadRequest, "Method not allowed")
		}
	})

	mux.HandleFunc("/posts/", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodPatch:
			postHandlers.UpdatePost(w, r)
		case http.MethodDelete:
			postHandlers.DeletePost(w, r)
		default:
			ctx := middleware.SetErrorCode(r.Context(), api.ErrCodeBadRequest)
			api.WriteError(w, ctx, http.StatusMethodNotAllowed, api.ErrCodeBadRequest, "Method not allowed")
		}
	})

	// Upload routes (if configured)
	if uploadHandlers != nil {
		mux.HandleFunc("/uploads/sign", func(w http.ResponseWriter, r *http.Request) {
			if r.Method != http.MethodPost {
				ctx := middleware.SetErrorCode(r.Context(), api.ErrCodeBadRequest)
				api.WriteError(w, ctx, http.StatusMethodNotAllowed, api.ErrCodeBadRequest, "Method not allowed")
				return
			}
			uploadHandlers.SignUpload(w, r)
		})
	}

	// Payment routes (if configured)
	if paymentHandlers != nil {
		mux.HandleFunc("/payments/onboard", func(w http.ResponseWriter, r *http.Request) {
			if r.Method != http.MethodPost {
				ctx := middleware.SetErrorCode(r.Context(), api.ErrCodeBadRequest)
				api.WriteError(w, ctx, http.StatusMethodNotAllowed, api.ErrCodeBadRequest, "Method not allowed")
				return
			}
			paymentHandlers.OnboardScene(w, r)
		})
		
		// Wrap checkout handler with idempotency middleware
		checkoutHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method != http.MethodPost {
				ctx := middleware.SetErrorCode(r.Context(), api.ErrCodeBadRequest)
				api.WriteError(w, ctx, http.StatusMethodNotAllowed, api.ErrCodeBadRequest, "Method not allowed")
				return
			}
			paymentHandlers.CreateCheckoutSession(w, r)
		})
		
		if idempotencyMiddleware != nil {
			// Apply idempotency middleware - returns http.Handler
			mux.Handle("/payments/checkout", idempotencyMiddleware(checkoutHandler))
		} else {
			mux.Handle("/payments/checkout", checkoutHandler)
		}
		
		mux.HandleFunc("/payments/status", func(w http.ResponseWriter, r *http.Request) {
			if r.Method != http.MethodGet {
				ctx := middleware.SetErrorCode(r.Context(), api.ErrCodeBadRequest)
				api.WriteError(w, ctx, http.StatusMethodNotAllowed, api.ErrCodeBadRequest, "Method not allowed")
				return
			}
			paymentHandlers.GetPaymentStatus(w, r)
		})
	}

	// Webhook endpoint (if configured) - must be before auth middleware
	// Stripe signature verification serves as authentication
	if webhookHandlers != nil {
		mux.HandleFunc("/internal/stripe", func(w http.ResponseWriter, r *http.Request) {
			if r.Method != http.MethodPost {
				ctx := middleware.SetErrorCode(r.Context(), api.ErrCodeBadRequest)
				api.WriteError(w, ctx, http.StatusMethodNotAllowed, api.ErrCodeBadRequest, "Method not allowed")
				return
			}
			webhookHandlers.HandleStripeWebhook(w, r)
		})
	}

	// Trust score routes
	mux.HandleFunc("/trust/", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			ctx := middleware.SetErrorCode(r.Context(), api.ErrCodeBadRequest)
			api.WriteError(w, ctx, http.StatusMethodNotAllowed, api.ErrCodeBadRequest, "Method not allowed")
			return
		}
		trustHandlers.GetTrustScore(w, r)
	})

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
