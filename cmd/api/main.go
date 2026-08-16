// Package main is the entry point for the API server.
package main

import (
	"context"
	"database/sql"
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
	"github.com/redis/go-redis/v9"

	"github.com/onnwee/subcults/internal/alliance"
	"github.com/onnwee/subcults/internal/api"
	"github.com/onnwee/subcults/internal/atprotocol"
	"github.com/onnwee/subcults/internal/attachment"
	"github.com/onnwee/subcults/internal/audience"
	"github.com/onnwee/subcults/internal/audit"
	"github.com/onnwee/subcults/internal/auth"
	"github.com/onnwee/subcults/internal/config"
	"github.com/onnwee/subcults/internal/db"
	"github.com/onnwee/subcults/internal/health"
	"github.com/onnwee/subcults/internal/idempotency"
	"github.com/onnwee/subcults/internal/identity"
	"github.com/onnwee/subcults/internal/jobs"
	"github.com/onnwee/subcults/internal/livekit"
	"github.com/onnwee/subcults/internal/locationaccess"
	"github.com/onnwee/subcults/internal/membership"
	"github.com/onnwee/subcults/internal/middleware"
	"github.com/onnwee/subcults/internal/notification"
	"github.com/onnwee/subcults/internal/payment"
	"github.com/onnwee/subcults/internal/post"
	"github.com/onnwee/subcults/internal/ranking"
	"github.com/onnwee/subcults/internal/retention"
	"github.com/onnwee/subcults/internal/scene"
	domainsignal "github.com/onnwee/subcults/internal/signal"
	"github.com/onnwee/subcults/internal/stream"
	"github.com/onnwee/subcults/internal/telemetry"
	"github.com/onnwee/subcults/internal/touring"
	"github.com/onnwee/subcults/internal/tracing"
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

	// In-memory repositories are deliberately opt-in. They are useful for
	// local fixtures, but must never be mistaken for durable production state.
	inMemoryRepositories := strings.EqualFold(strings.TrimSpace(os.Getenv("SUBCULT_IN_MEMORY_REPOSITORIES")), "true")
	var runtimeRepositories *db.RuntimeRepositories
	if inMemoryRepositories {
		logger.Warn("using explicitly enabled in-memory repositories; API state will be lost on restart",
			"environment", env,
			"switch", "SUBCULT_IN_MEMORY_REPOSITORIES",
		)
	} else {
		startupContext, cancelStartup := context.WithTimeout(context.Background(), 10*time.Second)
		var err error
		runtimeRepositories, err = db.NewRuntimeRepositories(startupContext, os.Getenv("DATABASE_URL"))
		if err != nil {
			cancelStartup()
			logger.Error("durable repository startup prerequisite failed", "error", err)
			os.Exit(1)
		}
		defer func() {
			if err := runtimeRepositories.Close(); err != nil {
				logger.Warn("failed to close runtime database", "error", err)
			}
		}()

		checker := db.NewSchemaVersionChecker(runtimeRepositories.DB, logger)
		if err := checker.EnsureCompatible(startupContext); err != nil {
			cancelStartup()
			logger.Error("database schema is not compatible", "error", err)
			os.Exit(1)
		}
		cancelStartup()
	}

	// Initialize OpenTelemetry tracing
	tracingEnabled := false
	if val := os.Getenv("TRACING_ENABLED"); val != "" {
		valLower := strings.ToLower(val)
		switch valLower {
		case "true", "1", "yes", "on":
			tracingEnabled = true
		}
	}

	var tracerProvider *tracing.Provider
	if tracingEnabled {
		// Parse tracing configuration
		exporterType := os.Getenv("TRACING_EXPORTER_TYPE")
		if exporterType == "" {
			exporterType = "otlp-http"
		}

		sampleRateStr := os.Getenv("TRACING_SAMPLE_RATE")
		sampleRate := 0.1 // Default 10%
		if sampleRateStr != "" {
			if parsed, err := strconv.ParseFloat(sampleRateStr, 64); err == nil {
				sampleRate = parsed
			} else {
				logger.Warn("invalid TRACING_SAMPLE_RATE value, using default",
					"value", sampleRateStr,
					"error", err,
					"default_sample_rate", sampleRate,
				)
			}
		}

		insecureMode := false
		if val := os.Getenv("TRACING_INSECURE"); val != "" {
			valLower := strings.ToLower(val)
			insecureMode = valLower == "true" || valLower == "1" || valLower == "yes" || valLower == "on"
		}

		tracingConfig := tracing.Config{
			ServiceName:  "subcults-api",
			Enabled:      true,
			Environment:  env,
			ExporterType: exporterType,
			OTLPEndpoint: os.Getenv("TRACING_OTLP_ENDPOINT"),
			SamplingRate: sampleRate,
			InsecureMode: insecureMode,
		}

		var err error
		tracerProvider, err = tracing.NewProvider(tracingConfig)
		if err != nil {
			logger.Error("failed to initialize tracing", "error", err)
			os.Exit(1)
		}
		logger.Info("tracing initialized",
			"exporter", exporterType,
			"endpoint", tracingConfig.OTLPEndpoint,
			"sample_rate", sampleRate,
		)
	} else {
		logger.Info("tracing disabled")
	}

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

	// Load ranking calibration if file path is provided
	rankingCalibrationPath := os.Getenv("RANKING_CALIBRATION_PATH")
	if rankingCalibrationPath != "" {
		// Load calibration weights from file
		weights, err := ranking.LoadCalibration(rankingCalibrationPath)
		if err != nil {
			logger.Warn("failed to load ranking calibration file, using defaults",
				"path", rankingCalibrationPath,
				"error", err)
		} else {
			// Apply loaded weights as the active ranking configuration
			ranking.SetActiveWeights(weights)
			// Log loaded weights for verification
			logger.Info("ranking calibration loaded",
				"path", rankingCalibrationPath,
				"scene_weights", map[string]float64{
					"text_match": weights.Scene.TextMatch,
					"proximity":  weights.Scene.Proximity,
					"trust":      weights.Scene.Trust,
				},
				"event_weights", map[string]float64{
					"recency":    weights.Event.Recency,
					"text_match": weights.Event.TextMatch,
					"proximity":  weights.Event.Proximity,
					"trust":      weights.Event.Trust,
				})
		}
	} else {
		logger.Info("ranking calibration path not set, using default weights",
			"help", "Set RANKING_CALIBRATION_PATH environment variable to load custom weights")
	}

	// Initialize durable beta repositories, retaining memory only for explicit
	// local fixture mode.
	var eventRepo scene.EventRepository
	var sceneRepo scene.SceneRepository
	auditRepo := audit.NewInMemoryRepository()
	var rsvpRepo scene.RSVPRepository
	var streamRepo stream.SessionRepository
	var participantRepo stream.ParticipantRepository
	var analyticsRepo stream.AnalyticsRepository
	var postRepo post.PostRepository
	var membershipRepo membership.MembershipRepository
	var allianceRepo alliance.AllianceRepository
	var touringRepo touring.Repository
	var audienceRepo audience.Repository
	if runtimeRepositories != nil {
		sceneRepositories := scene.NewSQLRepositories(runtimeRepositories.DB)
		sceneRepo, eventRepo, rsvpRepo = sceneRepositories.Scenes, sceneRepositories.Events, sceneRepositories.RSVPs
		touringRepo = touring.NewSQLRepository(runtimeRepositories.DB)
		audienceRepo = audience.NewSQLRepository(runtimeRepositories.DB)
		streamRepo = stream.NewSQLSessionRepository(runtimeRepositories.DB)
		participantRepo = stream.NewSQLParticipantRepository(runtimeRepositories.DB)
		analyticsRepo = stream.NewSQLAnalyticsRepository(runtimeRepositories.DB)
		postRepo = post.NewSQLPostRepository(runtimeRepositories.DB)
		membershipRepo = membership.NewSQLMembershipRepository(runtimeRepositories.DB)
		allianceRepo = alliance.NewSQLAllianceRepository(runtimeRepositories.DB)
	} else {
		eventRepo = scene.NewInMemoryEventRepository()
		sceneRepo = scene.NewInMemorySceneRepository()
		rsvpRepo = scene.NewInMemoryRSVPRepository()
		touringRepo = touring.NewInMemoryRepository()
		audienceRepo = audience.NewInMemoryRepository()
		inMemoryStreamRepo := stream.NewInMemorySessionRepository()
		streamRepo = inMemoryStreamRepo
		participantRepo = stream.NewInMemoryParticipantRepository(inMemoryStreamRepo)
		analyticsRepo = stream.NewInMemoryAnalyticsRepository(inMemoryStreamRepo)
		postRepo = post.NewInMemoryPostRepository()
		membershipRepo = membership.NewInMemoryMembershipRepository()
		allianceRepo = alliance.NewInMemoryAllianceRepository()
	}
	audienceService := audience.NewService(audienceRepo)
	var signalRepo domainsignal.Repository
	var locationRepository locationaccess.Repository
	if runtimeRepositories != nil {
		locationRepository = locationaccess.NewSQLRepository(runtimeRepositories.DB)
	} else {
		signalRepo = domainsignal.NewInMemoryRepository()
		locationRepository = locationaccess.NewInMemoryRepository()
	}
	protectedLocationHandlers := api.NewProtectedLocationHandlers(locationRepository, eventRepo, sceneRepo)

	jwtSecret := strings.TrimSpace(os.Getenv("JWT_SECRET_CURRENT"))
	if jwtSecret == "" {
		jwtSecret = strings.TrimSpace(os.Getenv("JWT_SECRET"))
	}
	if jwtSecret == "" && inMemoryRepositories {
		jwtSecret = "development-only-jwt-secret-change-me"
	}
	if len(jwtSecret) < 32 {
		logger.Error("JWT secret must contain at least 32 characters")
		os.Exit(1)
	}
	jwtService := auth.NewJWTServiceWithRotation(jwtSecret, strings.TrimSpace(os.Getenv("JWT_SECRET_PREVIOUS")))

	var contactProtector *identity.ContactProtector
	var identityMailer identity.Mailer
	var err error
	if inMemoryRepositories {
		contactProtector, err = identity.NewEphemeralContactProtector()
		identityMailer = identity.DevelopmentMailer{Logger: logger}
	} else {
		contactProtector, err = identity.NewContactProtectorFromBase64(os.Getenv("CONTACT_ENCRYPTION_KEY"), os.Getenv("CONTACT_HMAC_KEY"))
		if err == nil && env == "development" {
			identityMailer = identity.DevelopmentMailer{Logger: logger}
		} else if err == nil {
			identityMailer, err = identity.NewPostmarkMailer(os.Getenv("POSTMARK_TRANSACTIONAL_TOKEN"), os.Getenv("POSTMARK_FROM"), os.Getenv("POSTMARK_TRANSACTIONAL_STREAM"))
		}
	}
	if err != nil {
		logger.Error("identity security configuration failed", "error", err)
		os.Exit(1)
	}
	if runtimeRepositories != nil {
		signalRepo = domainsignal.NewSQLRepository(runtimeRepositories.DB, contactProtector)
	}
	signalService := domainsignal.NewService(signalRepo)
	identityRepo := identity.Repository(identity.NewInMemoryRepository())
	if runtimeRepositories != nil {
		identityRepo = identity.NewSQLRepository(runtimeRepositories.DB)
	}
	publicWebURL := strings.TrimSpace(os.Getenv("PUBLIC_WEB_URL"))
	if publicWebURL == "" && inMemoryRepositories {
		publicWebURL = "http://localhost:5173"
	}
	identityService, err := identity.NewService(identityRepo, identityMailer, contactProtector, jwtService, publicWebURL)
	if err != nil {
		logger.Error("identity service initialization failed", "error", err)
		os.Exit(1)
	}
	identityHandlers := api.NewIdentityAuthHandlers(identityService, env != "development")
	notificationRepository := notification.Repository(notification.NewInMemoryRepository())
	if runtimeRepositories != nil {
		notificationRepository = notification.NewSQLRepository(runtimeRepositories.DB)
	}
	notificationService := notification.NewService(notificationRepository, contactProtector, os.Getenv("VAPID_PUBLIC_KEY"), os.Getenv("VAPID_PRIVATE_KEY"), os.Getenv("VAPID_SUBJECT"))
	notificationHandlers := api.NewNotificationHandlers(notificationService)

	// Initialize event broadcaster for WebSocket participant updates
	eventBroadcaster := stream.NewEventBroadcaster()

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

	// Initialize job metrics
	jobMetrics := jobs.NewMetrics()
	if err := jobMetrics.Register(promRegistry); err != nil {
		logger.Error("failed to register job metrics", "error", err)
		os.Exit(1)
	}
	logger.Info("job metrics registered")

	// Initialize trust metrics
	trustMetrics := trust.NewMetrics()
	if err := trustMetrics.Register(promRegistry); err != nil {
		logger.Error("failed to register trust metrics", "error", err)
		os.Exit(1)
	}
	logger.Info("trust metrics registered")

	// Parse trust recompute job configuration
	recomputeInterval := trust.DefaultRecomputeInterval
	if val := os.Getenv("TRUST_RECOMPUTE_INTERVAL"); val != "" {
		if duration, err := time.ParseDuration(val); err == nil {
			recomputeInterval = duration
		} else {
			logger.Warn("invalid TRUST_RECOMPUTE_INTERVAL, using default",
				"value", val,
				"error", err,
				"default", recomputeInterval)
		}
	}

	recomputeTimeout := trust.DefaultRecomputeTimeout
	if val := os.Getenv("TRUST_RECOMPUTE_TIMEOUT"); val != "" {
		if duration, err := time.ParseDuration(val); err == nil {
			recomputeTimeout = duration
		} else {
			logger.Warn("invalid TRUST_RECOMPUTE_TIMEOUT, using default",
				"value", val,
				"error", err,
				"default", recomputeTimeout)
		}
	}

	// Initialize trust recompute job
	trustRecomputeJob := trust.NewRecomputeJob(
		trust.RecomputeJobConfig{
			Interval:   recomputeInterval,
			Logger:     logger,
			Metrics:    trustMetrics,
			JobMetrics: jobMetrics,
			Timeout:    recomputeTimeout,
		},
		trustDirtyTracker,
		trustDataSource,
		trustScoreStore,
	)
	logger.Info("trust recompute job initialized",
		"interval", recomputeInterval,
		"timeout", recomputeTimeout)

	// Initialize database slow query metrics
	dbMetrics := db.NewSlowQueryMetrics()
	if err := dbMetrics.Register(promRegistry); err != nil {
		logger.Error("failed to register db metrics", "error", err)
		os.Exit(1)
	}
	logger.Info("database slow query metrics registered")

	// Initialize telemetry store and metrics
	telemetryStore := telemetry.NewInMemoryStore()
	telemetryMetrics := telemetry.NewMetrics()
	if err := telemetryMetrics.Register(promRegistry); err != nil {
		logger.Error("failed to register telemetry metrics", "error", err)
		os.Exit(1)
	}
	logger.Info("telemetry metrics registered")

	// Initialize HTTP and rate limiting metrics
	rateLimitMetrics := middleware.NewMetrics()
	if err := rateLimitMetrics.Register(promRegistry); err != nil {
		logger.Error("failed to register middleware metrics", "error", err)
		os.Exit(1)
	}
	logger.Info("middleware metrics registered (HTTP request metrics and rate limiting)")

	// Load canary deployment configuration from Config struct
	cfg, configErrs := config.Load("")
	if len(configErrs) > 0 {
		// Log config errors but continue - some errors may be non-critical
		for _, err := range configErrs {
			logger.Warn("config validation warning", "error", err)
		}
	}

	canaryConfig := middleware.CanaryConfig{
		Enabled:          cfg.CanaryEnabled,
		TrafficPercent:   cfg.CanaryTrafficPercent,
		ErrorThreshold:   cfg.CanaryErrorThreshold,
		LatencyThreshold: cfg.CanaryLatencyThreshold,
		AutoRollback:     cfg.CanaryAutoRollback,
		MonitoringWindow: cfg.CanaryMonitoringWindow,
		Version:          cfg.CanaryVersion,
	}

	canaryRouter := middleware.NewCanaryRouter(canaryConfig, logger)
	canaryRouter.SetPrometheusMetrics(rateLimitMetrics)

	if cfg.CanaryEnabled {
		logger.Info("canary deployment initialized",
			"traffic_percent", cfg.CanaryTrafficPercent,
			"error_threshold", cfg.CanaryErrorThreshold,
			"latency_threshold", cfg.CanaryLatencyThreshold,
			"auto_rollback", cfg.CanaryAutoRollback,
			"version", cfg.CanaryVersion,
		)
	} else {
		logger.Info("canary deployment disabled")
	}

	// Initialize rate limiting
	// Check if Redis URL is configured for distributed rate limiting
	redisURL := os.Getenv("REDIS_URL")
	var rateLimitStore middleware.RateLimitStore
	var redisClient *redis.Client
	if redisURL != "" {
		// Use Redis for distributed rate limiting
		opt, err := redis.ParseURL(redisURL)
		if err != nil {
			logger.Error("failed to parse Redis URL", "error", err)
			os.Exit(1)
		}
		redisClient = redis.NewClient(opt)

		// Test connection
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := redisClient.Ping(ctx).Err(); err != nil {
			logger.Error("failed to connect to Redis", "error", err)
			os.Exit(1)
		}

		rateLimitStore = middleware.NewRedisRateLimitStoreWithMetrics(redisClient, rateLimitMetrics)
		logger.Info("rate limiting initialized with Redis backend")
	} else {
		// Use in-memory rate limiting for single-instance deployments
		inMemStore := middleware.NewInMemoryRateLimitStore()
		rateLimitStore = inMemStore

		// Start periodic cleanup to prevent unbounded memory growth from expired buckets
		cleanupInterval := 5 * time.Minute // Clean up every 5 minutes
		go func() {
			ticker := time.NewTicker(cleanupInterval)
			defer ticker.Stop()
			for range ticker.C {
				inMemStore.Cleanup()
				logger.Debug("cleaned up expired rate limit buckets")
			}
		}()

		logger.Warn("rate limiting initialized with in-memory backend (not suitable for distributed deployments)")
	}

	// Initialize LiveKit token service
	// Get credentials from environment variables
	livekitAPIKey := os.Getenv("LIVEKIT_API_KEY")
	livekitAPISecret := os.Getenv("LIVEKIT_API_SECRET")
	livekitURL := os.Getenv("LIVEKIT_URL")

	var livekitHandlers *api.LiveKitHandlers
	var roomService *livekit.RoomService
	if livekitAPIKey != "" && livekitAPISecret != "" {
		tokenService, err := livekit.NewTokenService(livekitAPIKey, livekitAPISecret)
		if err != nil {
			logger.Error("failed to initialize LiveKit token service", "error", err)
			os.Exit(1)
		}
		livekitHandlers = api.NewLiveKitHandlers(tokenService, auditRepo)
		logger.Info("LiveKit token service initialized")

		// Initialize LiveKit room service for organizer controls
		if livekitURL != "" {
			roomService = livekit.NewRoomService(livekitURL, livekitAPIKey, livekitAPISecret)
			if roomService != nil {
				logger.Info("LiveKit room service initialized for organizer controls")
			}
		} else {
			logger.Warn("LIVEKIT_URL not configured, organizer controls will not be available")
		}
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
	sceneService := scene.NewService(sceneRepo, eventRepo, rsvpRepo)
	membershipService := membership.NewService(membershipRepo)
	allianceService := alliance.NewService(allianceRepo)
	sceneHandlers := api.NewSceneHandlers(sceneService, membershipRepo, streamRepo)
	membershipHandlers := api.NewMembershipHandlers(membershipService, sceneRepo, auditRepo)
	eventHandlers := api.NewEventHandlers(sceneService, auditRepo, streamRepo, trustStoreAdapter)
	rsvpHandlers := api.NewRSVPHandlers(rsvpRepo, eventRepo)
	streamService := stream.NewService(streamRepo, participantRepo, analyticsRepo, streamMetrics)
	streamHandlers := api.NewStreamHandlers(streamService, sceneRepo, eventRepo, auditRepo, streamMetrics, eventBroadcaster, roomService)
	postService := post.NewService(postRepo)
	postHandlers := api.NewPostHandlers(postService, sceneRepo, membershipRepo, metadataService)
	trustHandlers := api.NewTrustHandlers(sceneRepo, trustDataSource, trustScoreStore, trustDirtyTracker)
	allianceHandlers := api.NewAllianceHandlers(allianceService, sceneRepo, trustDataSource, trustDirtyTracker)
	searchHandlers := api.NewSearchHandlers(sceneRepo, postRepo, trustStoreAdapter, eventRepo)
	touringService := touring.NewService(touringRepo)
	touringHandlers := api.NewTouringHandlers(touringService, eventRepo, sceneRepo)
	signalHandlers := api.NewSignalHandlers(signalService, audienceService)
	var atprotoOAuthHandlers *api.ATProtoOAuthHandlers
	var stopATProtoReconciler context.CancelFunc
	atprotoOAuthEnabled := strings.EqualFold(strings.TrimSpace(os.Getenv("ATPROTO_OAUTH_ENABLED")), "true")
	if atprotoOAuthEnabled {
		if runtimeRepositories == nil {
			logger.Error("AT Protocol OAuth requires durable PostgreSQL repositories")
			os.Exit(1)
		}
		if redisClient == nil {
			logger.Error("AT Protocol OAuth requires Redis for refresh and publication locking")
			os.Exit(1)
		}
		sessionCipher, err := atprotocol.NewSessionCipherFromBase64(os.Getenv("ATPROTO_SESSION_ENCRYPTION_KEY"))
		if err != nil {
			logger.Error("AT Protocol OAuth session encryption configuration failed", "error", err)
			os.Exit(1)
		}
		atprotoStore := atprotocol.NewSQLStore(runtimeRepositories.DB, sessionCipher)
		atprotoOAuthService, err := atprotocol.NewOAuthService(atprotocol.OAuthConfig{
			ClientID:      strings.TrimSpace(os.Getenv("ATPROTO_OAUTH_CLIENT_ID")),
			CallbackURL:   strings.TrimSpace(os.Getenv("ATPROTO_OAUTH_CALLBACK_URL")),
			JWKSURL:       strings.TrimSpace(os.Getenv("ATPROTO_OAUTH_JWKS_URL")),
			PrivateKey:    strings.TrimSpace(os.Getenv("ATPROTO_OAUTH_CLIENT_PRIVATE_KEY")),
			KeyID:         strings.TrimSpace(os.Getenv("ATPROTO_OAUTH_CLIENT_KEY_ID")),
			PublicWebURL:  publicWebURL,
			DefaultPDSURL: strings.TrimSpace(os.Getenv("ATPROTO_DEFAULT_PDS_URL")),
			ClientName:    "Subcults",
			PrivacyURL:    strings.TrimRight(publicWebURL, "/") + "/privacy",
			TermsURL:      strings.TrimRight(publicWebURL, "/") + "/terms",
		}, atprotoStore)
		if err != nil {
			logger.Error("AT Protocol OAuth initialization failed", "error", err)
			os.Exit(1)
		}
		atprotoOAuthService.SetPublicationLocker(atprotocol.NewRedisPublicationLocker(redisClient))
		atprotoOAuthService.ConfigureTap(strings.TrimSpace(os.Getenv("ATPROTO_TAP_URL")), strings.TrimSpace(os.Getenv("ATPROTO_TAP_ADMIN_PASSWORD")))
		identityHandlers.SetATProtoStatusResolver(func(ctx context.Context, userID string) (map[string]any, error) {
			link, linkErr := atprotoOAuthService.Status(ctx, userID)
			if linkErr != nil {
				return nil, linkErr
			}
			return map[string]any{
				"atproto_did": link.AccountDID, "atproto_handle": link.Handle,
				"atproto_link_status": link.Status, "atproto_granted_scopes": link.GrantedScopes,
			}, nil
		})
		reconcileContext, cancelReconciler := context.WithCancel(context.Background())
		stopATProtoReconciler = cancelReconciler
		go atprotocol.NewReconciler(atprotoStore).Run(reconcileContext)
		dailyCap := 25
		if configured, parseErr := strconv.Atoi(strings.TrimSpace(os.Getenv("PDS_SIGNUP_DAILY_CAP"))); parseErr == nil && configured > 0 {
			dailyCap = configured
		}
		termsVersion := strings.TrimSpace(os.Getenv("ATPROTO_TERMS_VERSION"))
		if termsVersion == "" {
			termsVersion = "2026-08-09"
		}
		provisioningService, err := atprotocol.NewProvisioningService(atprotoStore, atprotocol.ProvisioningConfig{
			Enabled:          strings.EqualFold(strings.TrimSpace(os.Getenv("PDS_SIGNUP_ENABLED")), "true"),
			DefaultPDSURL:    strings.TrimSpace(os.Getenv("ATPROTO_DEFAULT_PDS_URL")),
			ProvisionerURL:   strings.TrimSpace(os.Getenv("PDS_PROVISIONER_URL")),
			ProvisionerToken: strings.TrimSpace(os.Getenv("PDS_PROVISIONER_TOKEN")),
			TurnstileSecret:  strings.TrimSpace(os.Getenv("ATPROTO_TURNSTILE_SECRET")),
			TermsVersion:     termsVersion,
			HandleDomain:     strings.TrimSpace(os.Getenv("ATPROTO_HANDLE_DOMAIN")),
			DailyCap:         dailyCap,
		})
		if err != nil {
			logger.Error("AT Protocol provisioning initialization failed", "error", err)
			os.Exit(1)
		}
		atprotoOAuthHandlers = api.NewATProtoOAuthHandlers(atprotoOAuthService, provisioningService)
		atprotoOAuthHandlers.SetSyncPassword(strings.TrimSpace(os.Getenv("ATPROTO_TAP_ADMIN_PASSWORD")))
		logger.Info("AT Protocol OAuth initialized", "default_pds", os.Getenv("ATPROTO_DEFAULT_PDS_URL"))
	}

	// Initialize retention and account handlers
	retentionRepo := retention.NewInMemoryRepository(logger)
	accountHandlers := api.NewAccountHandlers(retentionRepo, 30*24*time.Hour)

	// Define rate limit configurations per endpoint
	telemetryLimit := middleware.RateLimitConfig{
		RequestsPerWindow: 100, // Allow 100 metrics submissions per minute (generous for legitimate use)
		WindowDuration:    time.Minute,
	}
	generalLimit := middleware.RateLimitConfig{
		RequestsPerWindow: 1000,
		WindowDuration:    time.Minute,
	}

	// Create HTTP server with routes
	mux := http.NewServeMux()
	deps := &api.RouteDeps{RateLimitStore: rateLimitStore, RateLimitMetrics: rateLimitMetrics}
	requireCreator := api.RequireCreator(identityService)

	// Domain route registrars (each Register* function lives in its handler file).
	api.RegisterIdentityRoutes(mux, deps, identityHandlers)
	if atprotoOAuthHandlers != nil {
		api.RegisterATProtoRoutes(mux, deps, atprotoOAuthHandlers, identityService)
	}
	api.RegisterEventRoutes(mux, deps, eventHandlers, rsvpHandlers, postHandlers, protectedLocationHandlers)
	api.RegisterSceneRoutes(mux, deps, sceneHandlers, postHandlers, membershipHandlers)
	api.RegisterSearchRoutes(mux, deps, searchHandlers, eventHandlers)
	api.RegisterStreamRoutes(mux, deps, streamHandlers)
	api.RegisterTouringRoutes(mux, deps, touringHandlers, identityService)
	api.RegisterPostRoutes(mux, deps, postHandlers)
	api.RegisterAllianceRoutes(mux, deps, allianceHandlers)
	api.RegisterSignalRoutes(mux, deps, signalHandlers)

	// Studio routes shared across handler domains
	mux.HandleFunc("/api/v1/studio/scenes", requireCreator(sceneHandlers.CreateScene))
	mux.HandleFunc("/api/v1/studio/events", requireCreator(eventHandlers.CreateEvent))
	mux.HandleFunc("/api/v1/studio/signals", requireCreator(signalHandlers.CreateDraft))

	// Notification routes
	mux.HandleFunc("/api/v1/notifications/subscribe", notificationHandlers.Subscribe)
	// Compatibility endpoint used by the existing notification settings client.
	mux.HandleFunc("/api/notifications/subscribe", notificationHandlers.Subscribe)

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

	// Canary deployment management endpoints
	canaryHandler := api.NewCanaryHandler(canaryRouter, logger)
	mux.HandleFunc("/canary/metrics", canaryHandler.GetMetrics)
	mux.HandleFunc("/canary/rollback", canaryHandler.Rollback)
	mux.HandleFunc("/canary/metrics/reset", canaryHandler.ResetMetrics)

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

	// Health check endpoints for Kubernetes probes
	// Initialize health checkers for external dependencies
	var redisHealthChecker api.HealthChecker
	if redisClient != nil {
		redisHealthChecker = health.NewRedisChecker(redisClient)
	}

	var livekitHealthChecker api.HealthChecker
	if livekitURL != "" {
		livekitHealthChecker = health.NewLiveKitChecker(livekitURL)
	}

	var dbHealthChecker api.HealthChecker
	if runtimeRepositories != nil {
		dbHealthChecker = health.NewDBChecker(runtimeRepositories.DB)
	}

	healthHandlers := api.NewHealthHandlers(api.HealthHandlersConfig{
		DBChecker:      dbHealthChecker,
		RedisChecker:   redisHealthChecker,
		LiveKitChecker: livekitHealthChecker,
		StripeChecker:  nil, // Will be configured when Stripe health check is implemented
		MetricsEnabled: true,
	})
	mux.HandleFunc("/health/live", healthHandlers.Health)
	mux.HandleFunc("/health/ready", healthHandlers.Ready)

	// Profiling status endpoint (always available, reports whether profiling is enabled)
	mux.Handle("/debug/profiling/status", middleware.ProfilingStatus(middleware.ProfilingConfig{
		Enabled:     cfg.ProfilingEnabled,
		Environment: cfg.Env,
	}))

	// CSP violation report endpoint (no auth required — browsers send these automatically)
	mux.HandleFunc("/api/csp-report", api.CSPReportHandler())

	// Account data export and deletion endpoints
	mux.HandleFunc("/api/account/export", accountHandlers.ExportAccountData)
	mux.HandleFunc("/api/account/delete", accountHandlers.DeleteAccount)

	// Telemetry endpoints for frontend performance metrics and event batching
	telemetryHandlers := api.NewTelemetryHandlers(telemetryStore, telemetryMetrics)
	telemetryMetricsHandler := middleware.RateLimiter(rateLimitStore, telemetryLimit, middleware.IPKeyFunc(), rateLimitMetrics)(
		http.HandlerFunc(telemetryHandlers.PostMetrics),
	)
	mux.Handle("/api/telemetry/metrics", telemetryMetricsHandler)

	// Telemetry event batch endpoint (same rate limiting as metrics)
	telemetryEventsHandler := middleware.RateLimiter(rateLimitStore, telemetryLimit, middleware.IPKeyFunc(), rateLimitMetrics)(
		http.HandlerFunc(telemetryHandlers.PostEvents),
	)
	mux.Handle("/api/telemetry", telemetryEventsHandler)

	// Client-side error logging endpoint (10 errors/min per IP)
	clientErrorLimit := middleware.RateLimitConfig{
		RequestsPerWindow: 10,
		WindowDuration:    time.Minute,
	}
	errorLoggerHandlers := api.NewErrorLoggerHandlers(telemetryStore, telemetryMetrics)
	clientErrorHandler := middleware.RateLimiter(rateLimitStore, clientErrorLimit, middleware.IPKeyFunc(), rateLimitMetrics)(
		http.HandlerFunc(errorLoggerHandlers.HandleClientError),
	)
	mux.Handle("/api/log/client-error", clientErrorHandler)

	// Schema version endpoint for service compatibility checks
	var schemaVersionDB *sql.DB
	if runtimeRepositories != nil {
		schemaVersionDB = runtimeRepositories.DB
	}
	schemaVersionStore := db.NewSchemaVersionStore(schemaVersionDB, logger)
	mux.HandleFunc("/internal/db/schema-version", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			api.WriteError(w, r.Context(), http.StatusMethodNotAllowed, "method_not_allowed", "Only GET is allowed")
			return
		}
		info, err := schemaVersionStore.GetCurrentVersion(r.Context())
		if err != nil {
			api.WriteError(w, r.Context(), http.StatusInternalServerError, "schema_version_error", "Failed to get schema version")
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		resp := fmt.Sprintf(`{"version":%d,"description":%q,"applied_at":%q,"min_version":%d}`,
			info.Version, info.Description, info.AppliedAt, db.MinSchemaVersion)
		if _, err := w.Write([]byte(resp)); err != nil {
			slog.Error("failed to write schema version response", "error", err)
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

	// Apply middleware chain:
	// The following middleware are applied in reverse order (innermost to outermost).
	// This means the request flows through them in the order listed below (1→6),
	// but they are applied to the handler in reverse order (6→1).
	//
	// Request flow (what executes first to last):
	// 1. Tracing - OpenTelemetry instrumentation (if enabled)
	// 2. CORS - Cross-origin resource sharing (if configured)
	// 3. General rate limiting (1000 req/min per IP) - blocks excessive requests early
	// 4. HTTP metrics - captures request duration, sizes, and counts
	// 5. RequestID - generates/extracts request IDs for tracing
	// 6. Logging - logs requests with all context
	var handler http.Handler = mux
	if runtimeRepositories != nil {
		handler = middleware.DurableBetaSurface(handler)
	}

	// Apply middleware in reverse order of execution
	// Logging is applied first (innermost, executes last)
	handler = middleware.Logging(logger)(handler)

	// Verify optional bearer credentials before logging and route authorization.
	// Public routes remain accessible without a token; malformed tokens never
	// silently degrade to anonymous access.
	handler = middleware.JWTAuth(jwtService)(handler)

	// Then RequestID
	handler = middleware.RequestID(handler)

	// Then HTTP metrics
	handler = middleware.HTTPMetrics(rateLimitMetrics)(handler)

	// Then request body size limits (1MB JSON, 15MB uploads)
	handler = middleware.MaxBodySize(1<<20, 15<<20)(handler)

	// Then security headers (defense-in-depth, duplicates Caddy headers)
	handler = middleware.SecurityHeaders(handler)

	// Then rate limiting
	handler = middleware.RateLimiter(rateLimitStore, generalLimit, middleware.IPKeyFunc(), rateLimitMetrics)(handler)

	// Then canary routing (if enabled)
	if cfg.CanaryEnabled {
		handler = canaryRouter.Middleware(handler)
	}

	// Then CORS (if configured)
	if cfg.CORSAllowedOrigins != "" {
		// Parse comma-separated origins
		origins := strings.Split(cfg.CORSAllowedOrigins, ",")
		for i, origin := range origins {
			origins[i] = strings.TrimSpace(origin)
		}

		// Parse comma-separated methods
		methods := strings.Split(cfg.CORSAllowedMethods, ",")
		for i, method := range methods {
			methods[i] = strings.TrimSpace(method)
		}

		// Parse comma-separated headers
		headers := strings.Split(cfg.CORSAllowedHeaders, ",")
		for i, header := range headers {
			headers[i] = strings.TrimSpace(header)
		}

		corsConfig := middleware.CORSConfig{
			AllowedOrigins:   origins,
			AllowedMethods:   methods,
			AllowedHeaders:   headers,
			AllowCredentials: cfg.CORSAllowCredentials,
			MaxAge:           cfg.CORSMaxAge,
		}

		handler = middleware.CORS(corsConfig)(handler)

		slog.Info("CORS enabled",
			"origins", origins,
			"methods", methods,
			"headers", headers,
			"allow_credentials", cfg.CORSAllowCredentials,
			"max_age", cfg.CORSMaxAge,
		)
	} else {
		slog.Info("CORS disabled - no origins configured")
	}

	// Add profiling middleware (DEVELOPMENT ONLY)
	// Profiling is applied near the top of the middleware stack so profiling endpoints
	// are accessible without going through rate limiting or other restrictive middleware
	if cfg.ProfilingEnabled {
		handler = middleware.Profiling(middleware.ProfilingConfig{
			Enabled:     true,
			Environment: cfg.Env,
		})(handler)
		logger.Info("profiling enabled", "env", cfg.Env, "endpoints", "/debug/pprof/*")
	} else {
		logger.Info("profiling disabled")
	}

	// Finally, tracing (outermost, executes first) - only if enabled
	if tracingEnabled {
		handler = middleware.Tracing("subcults-api")(handler)
	}

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

	// Start trust recompute job
	if err := trustRecomputeJob.Start(context.Background()); err != nil {
		logger.Error("failed to start trust recompute job", "error", err)
		os.Exit(1)
	}
	logger.Info("trust recompute job started")

	// Wait for interrupt signal for graceful shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	logger.Info("shutting down server...")
	if stopATProtoReconciler != nil {
		stopATProtoReconciler()
	}

	// Stop trust recompute job
	trustRecomputeJob.Stop()
	logger.Info("trust recompute job stopped")

	// Create context with timeout for shutdown
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Shutdown tracer provider first to flush pending spans
	if tracerProvider != nil {
		if err := tracerProvider.Shutdown(ctx); err != nil {
			logger.Error("failed to shutdown tracer provider", "error", err)
		}
	}

	if err := server.Shutdown(ctx); err != nil {
		logger.Error("server forced to shutdown", "error", err)
		os.Exit(1)
	}

	// Close Redis client if it was initialized
	if redisClient != nil {
		if err := redisClient.Close(); err != nil {
			logger.Error("failed to close Redis client", "error", err)
		} else {
			logger.Info("Redis client closed")
		}
	}

	logger.Info("server stopped")
}
