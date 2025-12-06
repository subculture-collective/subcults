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
	StripeAPIKey       string `koanf:"stripe_api_key"`
	StripeWebhookSecret string `koanf:"stripe_webhook_secret"`

	// MapTiler
	MapTilerAPIKey string `koanf:"maptiler_api_key"`

	// Jetstream (AT Protocol)
	JetstreamURL string `koanf:"jetstream_url"`
}

// Configuration validation errors.
var (
	ErrMissingDatabaseURL       = errors.New("DATABASE_URL is required")
	ErrMissingJWTSecret         = errors.New("JWT_SECRET is required")
	ErrMissingLiveKitURL        = errors.New("LIVEKIT_URL is required")
	ErrMissingLiveKitAPIKey     = errors.New("LIVEKIT_API_KEY is required")
	ErrMissingLiveKitAPISecret  = errors.New("LIVEKIT_API_SECRET is required")
	ErrMissingStripeAPIKey      = errors.New("STRIPE_API_KEY is required")
	ErrMissingStripeWebhookSecret = errors.New("STRIPE_WEBHOOK_SECRET is required")
	ErrMissingMapTilerAPIKey    = errors.New("MAPTILER_API_KEY is required")
	ErrMissingJetstreamURL      = errors.New("JETSTREAM_URL is required")
)

// Default values for non-secret configuration.
const (
	DefaultPort = 8080
	DefaultEnv  = "development"
)

// Load reads configuration from environment variables and an optional config file.
// Environment variables take precedence over file values.
// Returns the loaded config and a slice of validation errors (empty if valid).
func Load(configFilePath string) (*Config, []error) {
	k := koanf.New(".")

	// Load from YAML file first if provided (lower precedence)
	if configFilePath != "" {
		if err := k.Load(file.Provider(configFilePath), yaml.Parser()); err != nil {
			// File loading error is not fatal - continue with env vars
			// but we could log this if desired
		}
	}

	// Build config struct, with env vars taking precedence over file values
	cfg := &Config{
		Port:                getEnvIntOrDefault("PORT", k.Int("port"), DefaultPort),
		Env:                 getEnvOrDefault("ENV", k.String("env"), DefaultEnv),
		DatabaseURL:         getEnvOrKoanf("DATABASE_URL", k, "database_url"),
		JWTSecret:           getEnvOrKoanf("JWT_SECRET", k, "jwt_secret"),
		LiveKitURL:          getEnvOrKoanf("LIVEKIT_URL", k, "livekit_url"),
		LiveKitAPIKey:       getEnvOrKoanf("LIVEKIT_API_KEY", k, "livekit_api_key"),
		LiveKitAPISecret:    getEnvOrKoanf("LIVEKIT_API_SECRET", k, "livekit_api_secret"),
		StripeAPIKey:        getEnvOrKoanf("STRIPE_API_KEY", k, "stripe_api_key"),
		StripeWebhookSecret: getEnvOrKoanf("STRIPE_WEBHOOK_SECRET", k, "stripe_webhook_secret"),
		MapTilerAPIKey:      getEnvOrKoanf("MAPTILER_API_KEY", k, "maptiler_api_key"),
		JetstreamURL:        getEnvOrKoanf("JETSTREAM_URL", k, "jetstream_url"),
	}

	// Validate and collect errors
	errs := cfg.Validate()

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

// getEnvIntOrDefault returns the environment variable as int if set, otherwise the koanf value, or default.
func getEnvIntOrDefault(envKey string, koanfVal int, defaultVal int) int {
	if val := os.Getenv(envKey); val != "" {
		if i, err := strconv.Atoi(val); err == nil {
			return i
		}
	}
	if koanfVal != 0 {
		return koanfVal
	}
	return defaultVal
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
	if c.MapTilerAPIKey == "" {
		errs = append(errs, ErrMissingMapTilerAPIKey)
	}
	if c.JetstreamURL == "" {
		errs = append(errs, ErrMissingJetstreamURL)
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
		"stripe_api_key":        maskStripeKey(c.StripeAPIKey),
		"stripe_webhook_secret": maskSecret(c.StripeWebhookSecret),
		"maptiler_api_key":      maskSecret(c.MapTilerAPIKey),
		"jetstream_url":         c.JetstreamURL,
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
