// Package config provides configuration loading and validation for the API server.
// It uses koanf to merge environment variables with optional file overrides.
package config

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/knadh/koanf/parsers/yaml"
	"github.com/knadh/koanf/providers/file"
	"github.com/knadh/koanf/v2"
)

// Config holds all configuration values for the API server.
type Config struct {
	// Server settings
	Port int    `koanf:"port"`
	Env  string `koanf:"env"`

	// Database
	DatabaseURL string `koanf:"database_url"`

	// JWT Authentication
	JWTSecret string `koanf:"jwt_secret"`

	// LiveKit (WebRTC)
	LiveKitURL       string `koanf:"livekit_url"`
	LiveKitAPIKey    string `koanf:"livekit_api_key"`
	LiveKitAPISecret string `koanf:"livekit_api_secret"`

	// Stripe
	StripeAPIKey              string  `koanf:"stripe_api_key"`
	StripeWebhookSecret       string  `koanf:"stripe_webhook_secret"`
	StripeOnboardingReturnURL string  `koanf:"stripe_onboarding_return_url"`
	StripeOnboardingRefreshURL string `koanf:"stripe_onboarding_refresh_url"`
	StripeApplicationFeePercent float64 `koanf:"stripe_application_fee_percent"` // Platform fee as percentage (e.g., 5.0 for 5%)

	// MapTiler
	MapTilerAPIKey string `koanf:"maptiler_api_key"`

	// Jetstream (AT Protocol)
	JetstreamURL string `koanf:"jetstream_url"`

	// R2 (Cloudflare Object Storage)
	R2BucketName      string `koanf:"r2_bucket_name"`
	R2AccessKeyID     string `koanf:"r2_access_key_id"`
	R2SecretAccessKey string `koanf:"r2_secret_access_key"`
	R2Endpoint        string `koanf:"r2_endpoint"`
	R2MaxUploadSizeMB int    `koanf:"r2_max_upload_size_mb"` // Default: 15MB

	// Feature Flags
	RankTrustEnabled bool `koanf:"rank_trust_enabled"` // Enable trust-weighted ranking in search/feed
}

// Configuration validation errors.
var (
	ErrMissingDatabaseURL                = errors.New("DATABASE_URL is required")
	ErrMissingJWTSecret                  = errors.New("JWT_SECRET is required")
	ErrMissingLiveKitURL                 = errors.New("LIVEKIT_URL is required")
	ErrMissingLiveKitAPIKey              = errors.New("LIVEKIT_API_KEY is required")
	ErrMissingLiveKitAPISecret           = errors.New("LIVEKIT_API_SECRET is required")
	ErrMissingStripeAPIKey               = errors.New("STRIPE_API_KEY is required")
	ErrMissingStripeWebhookSecret        = errors.New("STRIPE_WEBHOOK_SECRET is required")
	ErrMissingStripeOnboardingReturnURL  = errors.New("STRIPE_ONBOARDING_RETURN_URL is required")
	ErrMissingStripeOnboardingRefreshURL = errors.New("STRIPE_ONBOARDING_REFRESH_URL is required")
	ErrMissingMapTilerAPIKey             = errors.New("MAPTILER_API_KEY is required")
	ErrMissingJetstreamURL               = errors.New("JETSTREAM_URL is required")
	ErrMissingR2BucketName               = errors.New("R2_BUCKET_NAME is required")
	ErrMissingR2AccessKeyID              = errors.New("R2_ACCESS_KEY_ID is required")
	ErrMissingR2SecretAccessKey          = errors.New("R2_SECRET_ACCESS_KEY is required")
	ErrMissingR2Endpoint                 = errors.New("R2_ENDPOINT is required")
	ErrInvalidPort                       = errors.New("PORT must be a valid integer")
)

// Default values for non-secret configuration.
const (
	DefaultPort                      = 8080
	DefaultEnv                       = "development"
	DefaultR2MaxUploadSizeMB         = 15
	DefaultRankTrustEnabled          = false
	DefaultStripeApplicationFeePercent = 5.0 // 5% platform fee by default
)

// Load reads configuration from environment variables and an optional config file.
// Environment variables take precedence over file values.
// Returns the loaded config and a slice of validation errors (empty if valid).
// If a config file path is provided and the file cannot be loaded, an error is returned.
func Load(configFilePath string) (*Config, []error) {
	k := koanf.New(".")
	var loadErrs []error

	// Load from YAML file first if provided (lower precedence)
	if configFilePath != "" {
		if err := k.Load(file.Provider(configFilePath), yaml.Parser()); err != nil {
			return nil, []error{fmt.Errorf("failed to load config file %s: %w", configFilePath, err)}
		}
	}

	// Parse port from env, collecting error if invalid
	// Try SUBCULT_PORT first, then PORT for backward compatibility
	port, portErr := getEnvIntOrDefaultMulti([]string{"SUBCULT_PORT", "PORT"}, k.Int("port"), DefaultPort)
	if portErr != nil {
		loadErrs = append(loadErrs, portErr)
	}

	// Parse R2 max upload size from env with default
	maxUploadSize, uploadSizeErr := getEnvIntOrDefault("R2_MAX_UPLOAD_SIZE_MB", k.Int("r2_max_upload_size_mb"), DefaultR2MaxUploadSizeMB)
	if uploadSizeErr != nil {
		loadErrs = append(loadErrs, uploadSizeErr)
	}

	// Parse trust ranking feature flag from env with default
	rankTrustEnabled := DefaultRankTrustEnabled
	if k.Exists("rank_trust_enabled") {
		rankTrustEnabled = k.Bool("rank_trust_enabled")
	}
	if val := os.Getenv("RANK_TRUST_ENABLED"); val != "" {
		// Env var takes precedence over file config
		valLower := strings.ToLower(val)
		switch valLower {
		case "true", "1", "yes", "on":
			rankTrustEnabled = true
		case "false", "0", "no", "off":
			rankTrustEnabled = false
		}
	}

	// Parse Stripe application fee percentage with default
	stripeFeePercent, stripeFeeErr := getEnvFloatOrDefault("STRIPE_APPLICATION_FEE_PERCENT", k.Float64("stripe_application_fee_percent"), DefaultStripeApplicationFeePercent)
	if stripeFeeErr != nil {
		loadErrs = append(loadErrs, stripeFeeErr)
	}

	// Build config struct, with env vars taking precedence over file values
	cfg := &Config{
		Port:                port,
		Env:                 getEnvOrDefaultMulti([]string{"SUBCULT_ENV", "ENV", "GO_ENV"}, k.String("env"), DefaultEnv),
		DatabaseURL:         getEnvOrKoanf("DATABASE_URL", k, "database_url"),
		JWTSecret:           getEnvOrKoanf("JWT_SECRET", k, "jwt_secret"),
		LiveKitURL:          getEnvOrKoanf("LIVEKIT_URL", k, "livekit_url"),
		LiveKitAPIKey:       getEnvOrKoanf("LIVEKIT_API_KEY", k, "livekit_api_key"),
		LiveKitAPISecret:    getEnvOrKoanf("LIVEKIT_API_SECRET", k, "livekit_api_secret"),
		StripeAPIKey:              getEnvOrKoanf("STRIPE_API_KEY", k, "stripe_api_key"),
		StripeWebhookSecret:       getEnvOrKoanf("STRIPE_WEBHOOK_SECRET", k, "stripe_webhook_secret"),
		StripeOnboardingReturnURL:  getEnvOrKoanf("STRIPE_ONBOARDING_RETURN_URL", k, "stripe_onboarding_return_url"),
		StripeOnboardingRefreshURL: getEnvOrKoanf("STRIPE_ONBOARDING_REFRESH_URL", k, "stripe_onboarding_refresh_url"),
		StripeApplicationFeePercent: stripeFeePercent,
		MapTilerAPIKey:            getEnvOrKoanf("MAPTILER_API_KEY", k, "maptiler_api_key"),
		JetstreamURL:        getEnvOrKoanf("JETSTREAM_URL", k, "jetstream_url"),
		R2BucketName:        getEnvOrKoanf("R2_BUCKET_NAME", k, "r2_bucket_name"),
		R2AccessKeyID:       getEnvOrKoanf("R2_ACCESS_KEY_ID", k, "r2_access_key_id"),
		R2SecretAccessKey:   getEnvOrKoanf("R2_SECRET_ACCESS_KEY", k, "r2_secret_access_key"),
		R2Endpoint:          getEnvOrKoanf("R2_ENDPOINT", k, "r2_endpoint"),
		R2MaxUploadSizeMB:   maxUploadSize,
		RankTrustEnabled:    rankTrustEnabled,
	}

	// Validate and collect errors
	errs := cfg.Validate()
	errs = append(loadErrs, errs...)

	return cfg, errs
}

// getEnvOrKoanf returns the environment variable value if set, otherwise the koanf value.
func getEnvOrKoanf(envKey string, k *koanf.Koanf, koanfKey string) string {
	if val := os.Getenv(envKey); val != "" {
		return val
	}
	return k.String(koanfKey)
}

// getEnvOrDefault returns the environment variable value if set, otherwise the koanf value, or default.
func getEnvOrDefault(envKey string, koanfVal string, defaultVal string) string {
	if val := os.Getenv(envKey); val != "" {
		return val
	}
	if koanfVal != "" {
		return koanfVal
	}
	return defaultVal
}

// getEnvOrDefaultMulti tries multiple environment variable keys in order.
// Returns the first non-empty value found, otherwise the koanf value, or default.
func getEnvOrDefaultMulti(envKeys []string, koanfVal string, defaultVal string) string {
	for _, key := range envKeys {
		if val := os.Getenv(key); val != "" {
			return val
		}
	}
	if koanfVal != "" {
		return koanfVal
	}
	return defaultVal
}

// getEnvIntOrDefault returns the environment variable as int if set, otherwise the koanf value, or default.
// Returns an error if the environment variable is set but cannot be parsed as an integer.
// Note: A port value of 0 from a YAML file will fall back to the default; port 0 is not supported in YAML files.
func getEnvIntOrDefault(envKey string, koanfVal int, defaultVal int) (int, error) {
	if val := os.Getenv(envKey); val != "" {
		i, err := strconv.Atoi(val)
		if err != nil {
			return 0, fmt.Errorf("%s must be a valid integer: %w", envKey, ErrInvalidPort)
		}
		return i, nil
	}
	if koanfVal != 0 {
		return koanfVal, nil
	}
	return defaultVal, nil
}

// getEnvIntOrDefaultMulti tries multiple environment variable keys in order.
// Returns the first valid integer value found, otherwise the koanf value, or default.
// Returns an error if any environment variable is set but cannot be parsed as an integer.
func getEnvIntOrDefaultMulti(envKeys []string, koanfVal int, defaultVal int) (int, error) {
	for _, key := range envKeys {
		if val := os.Getenv(key); val != "" {
			i, err := strconv.Atoi(val)
			if err != nil {
				return 0, fmt.Errorf("%s must be a valid integer: %w", key, ErrInvalidPort)
			}
			return i, nil
		}
	}
	if koanfVal != 0 {
		return koanfVal, nil
	}
	return defaultVal, nil
}

// getEnvFloatOrDefault returns the environment variable as float64 if set, otherwise the koanf value, or default.
// Returns an error if the environment variable is set but cannot be parsed as a float.
func getEnvFloatOrDefault(envKey string, koanfVal float64, defaultVal float64) (float64, error) {
	if val := os.Getenv(envKey); val != "" {
		f, err := strconv.ParseFloat(val, 64)
		if err != nil {
			return 0, fmt.Errorf("%s must be a valid float: %w", envKey, err)
		}
		return f, nil
	}
	if koanfVal != 0 {
		return koanfVal, nil
	}
	return defaultVal, nil
}

// Validate checks that all required configuration values are present.
// Returns a slice of validation errors (empty if valid).
func (c *Config) Validate() []error {
	var errs []error

	if c.DatabaseURL == "" {
		errs = append(errs, ErrMissingDatabaseURL)
	}
	if c.JWTSecret == "" {
		errs = append(errs, ErrMissingJWTSecret)
	}
	if c.LiveKitURL == "" {
		errs = append(errs, ErrMissingLiveKitURL)
	}
	if c.LiveKitAPIKey == "" {
		errs = append(errs, ErrMissingLiveKitAPIKey)
	}
	if c.LiveKitAPISecret == "" {
		errs = append(errs, ErrMissingLiveKitAPISecret)
	}
	if c.StripeAPIKey == "" {
		errs = append(errs, ErrMissingStripeAPIKey)
	}
	if c.StripeWebhookSecret == "" {
		errs = append(errs, ErrMissingStripeWebhookSecret)
	}
	if c.StripeOnboardingReturnURL == "" {
		errs = append(errs, ErrMissingStripeOnboardingReturnURL)
	}
	if c.StripeOnboardingRefreshURL == "" {
		errs = append(errs, ErrMissingStripeOnboardingRefreshURL)
	}
	if c.MapTilerAPIKey == "" {
		errs = append(errs, ErrMissingMapTilerAPIKey)
	}
	if c.JetstreamURL == "" {
		errs = append(errs, ErrMissingJetstreamURL)
	}

	// R2 configuration is optional. Only validate fields if any R2 value is set.
	if c.R2BucketName != "" || c.R2AccessKeyID != "" || c.R2SecretAccessKey != "" || c.R2Endpoint != "" {
		if c.R2BucketName == "" {
			errs = append(errs, ErrMissingR2BucketName)
		}
		if c.R2AccessKeyID == "" {
			errs = append(errs, ErrMissingR2AccessKeyID)
		}
		if c.R2SecretAccessKey == "" {
			errs = append(errs, ErrMissingR2SecretAccessKey)
		}
		if c.R2Endpoint == "" {
			errs = append(errs, ErrMissingR2Endpoint)
		}
	}

	return errs
}

// LogSummary returns a summary of the configuration suitable for logging.
// All secrets are masked to prevent accidental exposure.
func (c *Config) LogSummary() map[string]string {
	return map[string]string{
		"port":                  fmt.Sprintf("%d", c.Port),
		"env":                   c.Env,
		"database_url":          maskDatabaseURL(c.DatabaseURL),
		"jwt_secret":            maskSecret(c.JWTSecret),
		"livekit_url":           c.LiveKitURL,
		"livekit_api_key":       maskSecret(c.LiveKitAPIKey),
		"livekit_api_secret":    maskSecret(c.LiveKitAPISecret),
		"stripe_api_key":                 maskStripeKey(c.StripeAPIKey),
		"stripe_webhook_secret":          maskSecret(c.StripeWebhookSecret),
		"stripe_onboarding_return_url":   c.StripeOnboardingReturnURL,
		"stripe_onboarding_refresh_url":  c.StripeOnboardingRefreshURL,
		"maptiler_api_key":               maskSecret(c.MapTilerAPIKey),
		"jetstream_url":         c.JetstreamURL,
		"r2_bucket_name":        c.R2BucketName,
		"r2_access_key_id":      maskSecret(c.R2AccessKeyID),
		"r2_secret_access_key":  maskSecret(c.R2SecretAccessKey),
		"r2_endpoint":           c.R2Endpoint,
		"r2_max_upload_size_mb": fmt.Sprintf("%d", c.R2MaxUploadSizeMB),
		"rank_trust_enabled":    fmt.Sprintf("%t", c.RankTrustEnabled),
	}
}

// maskSecret masks a secret value, showing only the first 4 characters followed by ****
// If the secret is shorter than 8 characters, it's fully masked.
func maskSecret(s string) string {
	if s == "" {
		return "<not set>"
	}
	if len(s) < 8 {
		return "****"
	}
	return s[:4] + "****"
}

// maskStripeKey masks a Stripe API key, preserving the prefix (sk_live_, sk_test_, etc.)
func maskStripeKey(s string) string {
	if s == "" {
		return "<not set>"
	}

	// Stripe keys have format like sk_live_..., sk_test_..., pk_live_..., etc.
	parts := strings.SplitN(s, "_", 3)
	if len(parts) == 3 {
		return parts[0] + "_" + parts[1] + "_****"
	}

	// Fallback to generic masking
	return maskSecret(s)
}

// maskDatabaseURL masks the password in a database URL.
// Supports both postgres:// and postgresql:// schemes.
func maskDatabaseURL(s string) string {
	if s == "" {
		return "<not set>"
	}

	// Look for password pattern: user:password@host
	// Simple approach: find :// and then mask between : and @
	schemeEnd := strings.Index(s, "://")
	if schemeEnd == -1 {
		return maskSecret(s)
	}

	rest := s[schemeEnd+3:]
	atIndex := strings.Index(rest, "@")
	if atIndex == -1 {
		return s // No credentials in URL
	}

	colonIndex := strings.Index(rest[:atIndex], ":")
	if colonIndex == -1 {
		return s // No password (only username)
	}

	// Reconstruct URL with masked password
	scheme := s[:schemeEnd+3]
	user := rest[:colonIndex]
	hostAndPath := rest[atIndex:]

	return scheme + user + ":****" + hostAndPath
}
