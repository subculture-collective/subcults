package config

import (
	"errors"
	"os"
	"strings"
	"testing"
)

// clearEnv clears all environment variables that might affect config loading tests.
func clearEnv() {
	os.Unsetenv("DATABASE_URL")
	os.Unsetenv("JWT_SECRET")
	os.Unsetenv("JWT_SECRET_CURRENT")
	os.Unsetenv("JWT_SECRET_PREVIOUS")
	os.Unsetenv("LIVEKIT_URL")
	os.Unsetenv("LIVEKIT_API_KEY")
	os.Unsetenv("LIVEKIT_API_SECRET")
	os.Unsetenv("STRIPE_API_KEY")
	os.Unsetenv("STRIPE_WEBHOOK_SECRET")
	os.Unsetenv("STRIPE_ONBOARDING_RETURN_URL")
	os.Unsetenv("STRIPE_ONBOARDING_REFRESH_URL")
	os.Unsetenv("MAPTILER_API_KEY")
	os.Unsetenv("JETSTREAM_URL")
	os.Unsetenv("R2_BUCKET_NAME")
	os.Unsetenv("R2_ACCESS_KEY_ID")
	os.Unsetenv("R2_SECRET_ACCESS_KEY")
	os.Unsetenv("R2_ENDPOINT")
	os.Unsetenv("R2_MAX_UPLOAD_SIZE_MB")
	os.Unsetenv("PORT")
	os.Unsetenv("SUBCULT_PORT")
	os.Unsetenv("ENV")
	os.Unsetenv("GO_ENV")
	os.Unsetenv("SUBCULT_ENV")
	os.Unsetenv("RANK_TRUST_ENABLED")
}

func TestLoad_MissingMandatory(t *testing.T) {
	tests := []struct {
		name             string
		envVars          map[string]string
		wantErrCount     int
		checkSpecificErr error
	}{
		{
			name:         "no environment variables set",
			envVars:      map[string]string{},
			wantErrCount: 11, // All mandatory fields missing (R2 is optional)
		},
		{
			name: "only DATABASE_URL set",
			envVars: map[string]string{
				"DATABASE_URL": "postgres://localhost/test",
			},
			wantErrCount:     10,
			checkSpecificErr: ErrMissingJWTSecret,
		},
		{
			name: "missing JWT_SECRET",
			envVars: map[string]string{
				"DATABASE_URL":                  "postgres://localhost/test",
				"LIVEKIT_URL":                   "wss://livekit.example.com",
				"LIVEKIT_API_KEY":               "api_key",
				"LIVEKIT_API_SECRET":            "api_secret",
				"STRIPE_API_KEY":                "sk_test_123",
				"STRIPE_WEBHOOK_SECRET":         "whsec_123",
				"STRIPE_ONBOARDING_RETURN_URL":  "https://example.com/return",
				"STRIPE_ONBOARDING_REFRESH_URL": "https://example.com/refresh",
				"MAPTILER_API_KEY":              "maptiler_key",
				"JETSTREAM_URL":                 "wss://jetstream.example.com",
			},
			wantErrCount:     1,
			checkSpecificErr: ErrMissingJWTSecret,
		},
		{
			name: "missing STRIPE_API_KEY",
			envVars: map[string]string{
				"DATABASE_URL":                  "postgres://localhost/test",
				"JWT_SECRET":                    "supersecret32characterlongvalue!",
				"LIVEKIT_URL":                   "wss://livekit.example.com",
				"LIVEKIT_API_KEY":               "api_key",
				"LIVEKIT_API_SECRET":            "api_secret",
				"STRIPE_WEBHOOK_SECRET":         "whsec_123",
				"STRIPE_ONBOARDING_RETURN_URL":  "https://example.com/return",
				"STRIPE_ONBOARDING_REFRESH_URL": "https://example.com/refresh",
				"MAPTILER_API_KEY":              "maptiler_key",
				"JETSTREAM_URL":                 "wss://jetstream.example.com",
			},
			wantErrCount:     1,
			checkSpecificErr: ErrMissingStripeAPIKey,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			clearEnv()
			defer clearEnv()

			for k, v := range tt.envVars {
				os.Setenv(k, v)
			}

			_, errs := Load("")

			if len(errs) != tt.wantErrCount {
				t.Errorf("Load() returned %d errors, want %d. Errors: %v", len(errs), tt.wantErrCount, errs)
			}

			if tt.checkSpecificErr != nil {
				found := false
				for _, err := range errs {
					if err == tt.checkSpecificErr {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("Load() did not return expected error %v. Got: %v", tt.checkSpecificErr, errs)
				}
			}
		})
	}
}

func TestLoad_ValidEnv(t *testing.T) {
	clearEnv()
	defer clearEnv()

	// Set all required env vars
	os.Setenv("DATABASE_URL", "postgres://user:pass@localhost/subcults")
	os.Setenv("JWT_SECRET", "supersecret32characterlongvalue!")
	os.Setenv("LIVEKIT_URL", "wss://livekit.example.com")
	os.Setenv("LIVEKIT_API_KEY", "api_key_123")
	os.Setenv("LIVEKIT_API_SECRET", "api_secret_456")
	os.Setenv("STRIPE_API_KEY", "sk_test_123456789")
	os.Setenv("STRIPE_WEBHOOK_SECRET", "whsec_123456789")
	os.Setenv("STRIPE_ONBOARDING_RETURN_URL", "https://example.com/return")
	os.Setenv("STRIPE_ONBOARDING_REFRESH_URL", "https://example.com/refresh")
	os.Setenv("MAPTILER_API_KEY", "maptiler_key_123")
	os.Setenv("JETSTREAM_URL", "wss://jetstream.example.com")
	os.Setenv("R2_BUCKET_NAME", "test-bucket")
	os.Setenv("R2_ACCESS_KEY_ID", "test-key")
	os.Setenv("R2_SECRET_ACCESS_KEY", "test-secret")
	os.Setenv("R2_ENDPOINT", "https://test.r2.cloudflarestorage.com")
	os.Setenv("PORT", "3000")
	os.Setenv("ENV", "production")

	cfg, errs := Load("")

	if len(errs) != 0 {
		t.Errorf("Load() returned errors: %v", errs)
	}

	if cfg.Port != 3000 {
		t.Errorf("cfg.Port = %d, want 3000", cfg.Port)
	}
	if cfg.Env != "production" {
		t.Errorf("cfg.Env = %s, want production", cfg.Env)
	}
	if cfg.DatabaseURL != "postgres://user:pass@localhost/subcults" {
		t.Errorf("cfg.DatabaseURL = %s, want postgres://user:pass@localhost/subcults", cfg.DatabaseURL)
	}
	if cfg.JWTSecret != "supersecret32characterlongvalue!" {
		t.Errorf("cfg.JWTSecret = %s, want supersecret32characterlongvalue!", cfg.JWTSecret)
	}
}

func TestLoad_Defaults(t *testing.T) {
	clearEnv()
	defer clearEnv()

	// Set only required env vars, no PORT or ENV
	os.Setenv("DATABASE_URL", "postgres://localhost/test")
	os.Setenv("JWT_SECRET", "supersecret32characterlongvalue!")
	os.Setenv("LIVEKIT_URL", "wss://livekit.example.com")
	os.Setenv("LIVEKIT_API_KEY", "api_key")
	os.Setenv("LIVEKIT_API_SECRET", "api_secret")
	os.Setenv("STRIPE_API_KEY", "sk_test_123")
	os.Setenv("STRIPE_WEBHOOK_SECRET", "whsec_123")
	os.Setenv("STRIPE_ONBOARDING_RETURN_URL", "https://example.com/return")
	os.Setenv("STRIPE_ONBOARDING_REFRESH_URL", "https://example.com/refresh")
	os.Setenv("MAPTILER_API_KEY", "maptiler_key")
	os.Setenv("JETSTREAM_URL", "wss://jetstream.example.com")
	os.Setenv("R2_BUCKET_NAME", "test-bucket")
	os.Setenv("R2_ACCESS_KEY_ID", "test-key")
	os.Setenv("R2_SECRET_ACCESS_KEY", "test-secret")
	os.Setenv("R2_ENDPOINT", "https://test.r2.cloudflarestorage.com")

	cfg, errs := Load("")

	if len(errs) != 0 {
		t.Errorf("Load() returned errors: %v", errs)
	}

	if cfg.Port != DefaultPort {
		t.Errorf("cfg.Port = %d, want default %d", cfg.Port, DefaultPort)
	}
	if cfg.Env != DefaultEnv {
		t.Errorf("cfg.Env = %s, want default %s", cfg.Env, DefaultEnv)
	}
}

func TestMaskSecret(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "empty string",
			input: "",
			want:  "<not set>",
		},
		{
			name:  "short secret (< 8 chars)",
			input: "short",
			want:  "****",
		},
		{
			name:  "exactly 8 chars",
			input: "12345678",
			want:  "1234****",
		},
		{
			name:  "long secret",
			input: "supersecretvalue123456",
			want:  "supe****",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := maskSecret(tt.input)
			if got != tt.want {
				t.Errorf("maskSecret(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestMaskStripeKey(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "empty string",
			input: "",
			want:  "<not set>",
		},
		{
			name:  "live key",
			input: "sk_live_abcdefghijk123456",
			want:  "sk_live_****",
		},
		{
			name:  "test key",
			input: "sk_test_xyz789012345",
			want:  "sk_test_****",
		},
		{
			name:  "publishable key",
			input: "pk_test_abc123",
			want:  "pk_test_****",
		},
		{
			name:  "webhook secret",
			input: "whsec_abcdefghijk",
			want:  "whse****", // Falls back to generic masking (only 2 underscores)
		},
		{
			name:  "non-stripe format",
			input: "someotherkey",
			want:  "some****",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := maskStripeKey(tt.input)
			if got != tt.want {
				t.Errorf("maskStripeKey(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestMaskDatabaseURL(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "empty string",
			input: "",
			want:  "<not set>",
		},
		{
			name:  "postgres URL with password",
			input: "postgres://user:secretpassword@localhost:5432/subcults",
			want:  "postgres://user:****@localhost:5432/subcults",
		},
		{
			name:  "postgresql URL with password",
			input: "postgresql://admin:mypass123@db.example.com:5432/mydb",
			want:  "postgresql://admin:****@db.example.com:5432/mydb",
		},
		{
			name:  "URL without password",
			input: "postgres://user@localhost/subcults",
			want:  "postgres://user@localhost/subcults",
		},
		{
			name:  "URL without credentials",
			input: "postgres://localhost/subcults",
			want:  "postgres://localhost/subcults",
		},
		{
			name:  "invalid format",
			input: "not-a-url",
			want:  "not-****",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := maskDatabaseURL(tt.input)
			if got != tt.want {
				t.Errorf("maskDatabaseURL(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestConfig_LogSummary(t *testing.T) {
	cfg := &Config{
		Port:                8080,
		Env:                 "production",
		DatabaseURL:         "postgres://user:pass@localhost/subcults",
		JWTSecret:           "supersecret32characterlongvalue!",
		LiveKitURL:          "wss://livekit.example.com",
		LiveKitAPIKey:       "api_key_123456",
		LiveKitAPISecret:    "api_secret_789",
		StripeAPIKey:        "sk_live_abcdefghijk",
		StripeWebhookSecret: "whsec_123456789",
		MapTilerAPIKey:      "maptiler_key_abc",
		JetstreamURL:        "wss://jetstream.example.com",
	}

	summary := cfg.LogSummary()

	// Check that secrets are masked
	if summary["jwt_secret"] == cfg.JWTSecret {
		t.Error("LogSummary() did not mask jwt_secret")
	}
	if summary["stripe_api_key"] == cfg.StripeAPIKey {
		t.Error("LogSummary() did not mask stripe_api_key")
	}
	if summary["database_url"] == cfg.DatabaseURL {
		t.Error("LogSummary() did not mask database_url")
	}

	// Check that non-secrets are not masked
	if summary["port"] != "8080" {
		t.Errorf("LogSummary() port = %s, want 8080", summary["port"])
	}
	if summary["env"] != "production" {
		t.Errorf("LogSummary() env = %s, want production", summary["env"])
	}
	if summary["livekit_url"] != "wss://livekit.example.com" {
		t.Errorf("LogSummary() livekit_url = %s, want wss://livekit.example.com", summary["livekit_url"])
	}

	// Check specific masked values
	if summary["stripe_api_key"] != "sk_live_****" {
		t.Errorf("LogSummary() stripe_api_key = %s, want sk_live_****", summary["stripe_api_key"])
	}
	if summary["database_url"] != "postgres://user:****@localhost/subcults" {
		t.Errorf("LogSummary() database_url = %s, want postgres://user:****@localhost/subcults", summary["database_url"])
	}
}

func TestConfig_Validate(t *testing.T) {
	tests := []struct {
		name        string
		config      Config
		wantErrs    int
		checkForErr error
	}{
		{
			name:     "empty config has all errors",
			config:   Config{},
			wantErrs: 11, // 11 required fields (R2 is optional)
		},
		{
			name: "fully valid config",
			config: Config{
				DatabaseURL:                "postgres://localhost/test",
				JWTSecret:                  "secret",
				LiveKitURL:                 "wss://livekit.example.com",
				LiveKitAPIKey:              "key",
				LiveKitAPISecret:           "secret",
				StripeAPIKey:               "sk_test_123",
				StripeWebhookSecret:        "whsec_123",
				StripeOnboardingReturnURL:  "https://example.com/return",
				StripeOnboardingRefreshURL: "https://example.com/refresh",
				MapTilerAPIKey:             "key",
				JetstreamURL:               "wss://jetstream.example.com",
			},
			wantErrs: 0,
		},
		{
			name: "fully valid config with R2",
			config: Config{
				DatabaseURL:                "postgres://localhost/test",
				JWTSecret:                  "secret",
				LiveKitURL:                 "wss://livekit.example.com",
				LiveKitAPIKey:              "key",
				LiveKitAPISecret:           "secret",
				StripeAPIKey:               "sk_test_123",
				StripeWebhookSecret:        "whsec_123",
				StripeOnboardingReturnURL:  "https://example.com/return",
				StripeOnboardingRefreshURL: "https://example.com/refresh",
				MapTilerAPIKey:             "key",
				JetstreamURL:               "wss://jetstream.example.com",
				R2BucketName:               "test-bucket",
				R2AccessKeyID:              "test-key",
				R2SecretAccessKey:          "test-secret",
				R2Endpoint:                 "https://test.r2.cloudflarestorage.com",
			},
			wantErrs: 0,
		},
		{
			name: "missing only LiveKitURL",
			config: Config{
				DatabaseURL:                "postgres://localhost/test",
				JWTSecret:                  "secret",
				LiveKitAPIKey:              "key",
				LiveKitAPISecret:           "secret",
				StripeAPIKey:               "sk_test_123",
				StripeWebhookSecret:        "whsec_123",
				StripeOnboardingReturnURL:  "https://example.com/return",
				StripeOnboardingRefreshURL: "https://example.com/refresh",
				MapTilerAPIKey:             "key",
				JetstreamURL:               "wss://jetstream.example.com",
			},
			wantErrs:    1,
			checkForErr: ErrMissingLiveKitURL,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			errs := tt.config.Validate()
			if len(errs) != tt.wantErrs {
				t.Errorf("Validate() returned %d errors, want %d. Errors: %v", len(errs), tt.wantErrs, errs)
			}

			if tt.checkForErr != nil {
				found := false
				for _, err := range errs {
					if err == tt.checkForErr {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("Validate() did not return expected error %v. Got: %v", tt.checkForErr, errs)
				}
			}
		})
	}
}

func TestLoad_FromYAMLFile(t *testing.T) {
	clearEnv()
	defer clearEnv()

	// Create a temporary YAML config file
	yamlContent := `port: 3000
env: staging
database_url: postgres://fileuser:filepass@localhost/filedb
jwt_secret: file_jwt_secret_value_32_chars!
livekit_url: wss://file-livekit.example.com
livekit_api_key: file_livekit_key
livekit_api_secret: file_livekit_secret
stripe_api_key: sk_test_file_key
stripe_webhook_secret: whsec_file_secret
stripe_onboarding_return_url: https://example.com/return
stripe_onboarding_refresh_url: https://example.com/refresh
maptiler_api_key: file_maptiler_key
jetstream_url: wss://file-jetstream.example.com
r2_bucket_name: file-bucket
r2_access_key_id: file-key
r2_secret_access_key: file-secret
r2_endpoint: https://file.r2.cloudflarestorage.com
`
	tmpFile, err := os.CreateTemp("", "config-*.yaml")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())

	if _, err := tmpFile.WriteString(yamlContent); err != nil {
		t.Fatalf("Failed to write to temp file: %v", err)
	}
	if err := tmpFile.Close(); err != nil {
		t.Fatalf("Failed to close temp file: %v", err)
	}

	cfg, errs := Load(tmpFile.Name())

	if len(errs) != 0 {
		t.Errorf("Load() returned errors: %v", errs)
	}

	if cfg.Port != 3000 {
		t.Errorf("cfg.Port = %d, want 3000", cfg.Port)
	}
	if cfg.Env != "staging" {
		t.Errorf("cfg.Env = %s, want staging", cfg.Env)
	}
	if cfg.DatabaseURL != "postgres://fileuser:filepass@localhost/filedb" {
		t.Errorf("cfg.DatabaseURL = %s, want postgres://fileuser:filepass@localhost/filedb", cfg.DatabaseURL)
	}
}

func TestLoad_EnvOverridesFile(t *testing.T) {
	clearEnv()
	defer clearEnv()

	// Create a temporary YAML config file
	yamlContent := `port: 3000
env: staging
database_url: postgres://fileuser:filepass@localhost/filedb
jwt_secret: file_jwt_secret_value_32_chars!
livekit_url: wss://file-livekit.example.com
livekit_api_key: file_livekit_key
livekit_api_secret: file_livekit_secret
stripe_api_key: sk_test_file_key
stripe_webhook_secret: whsec_file_secret
stripe_onboarding_return_url: https://example.com/return
stripe_onboarding_refresh_url: https://example.com/refresh
maptiler_api_key: file_maptiler_key
jetstream_url: wss://file-jetstream.example.com
r2_bucket_name: file-bucket
r2_access_key_id: file-key
r2_secret_access_key: file-secret
r2_endpoint: https://file.r2.cloudflarestorage.com
`
	tmpFile, err := os.CreateTemp("", "config-*.yaml")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())

	if _, err := tmpFile.WriteString(yamlContent); err != nil {
		t.Fatalf("Failed to write to temp file: %v", err)
	}
	if err := tmpFile.Close(); err != nil {
		t.Fatalf("Failed to close temp file: %v", err)
	}

	// Set env vars that should override file values
	os.Setenv("PORT", "9000")
	os.Setenv("DATABASE_URL", "postgres://envuser:envpass@envhost/envdb")

	cfg, errs := Load(tmpFile.Name())

	if len(errs) != 0 {
		t.Errorf("Load() returned errors: %v", errs)
	}

	// Env should override file
	if cfg.Port != 9000 {
		t.Errorf("cfg.Port = %d, want 9000 (env should override file)", cfg.Port)
	}
	if cfg.DatabaseURL != "postgres://envuser:envpass@envhost/envdb" {
		t.Errorf("cfg.DatabaseURL = %s, want postgres://envuser:envpass@envhost/envdb (env should override file)", cfg.DatabaseURL)
	}

	// File values should be used where env not set
	if cfg.Env != "staging" {
		t.Errorf("cfg.Env = %s, want staging (from file)", cfg.Env)
	}
}

func TestLoad_InvalidPort(t *testing.T) {
	clearEnv()
	defer clearEnv()

	// Set all required env vars
	os.Setenv("DATABASE_URL", "postgres://localhost/test")
	os.Setenv("JWT_SECRET", "supersecret32characterlongvalue!")
	os.Setenv("LIVEKIT_URL", "wss://livekit.example.com")
	os.Setenv("LIVEKIT_API_KEY", "api_key")
	os.Setenv("LIVEKIT_API_SECRET", "api_secret")
	os.Setenv("STRIPE_API_KEY", "sk_test_123")
	os.Setenv("STRIPE_WEBHOOK_SECRET", "whsec_123")
	os.Setenv("STRIPE_ONBOARDING_RETURN_URL", "https://example.com/return")
	os.Setenv("STRIPE_ONBOARDING_REFRESH_URL", "https://example.com/refresh")
	os.Setenv("MAPTILER_API_KEY", "maptiler_key")
	os.Setenv("JETSTREAM_URL", "wss://jetstream.example.com")
	os.Setenv("R2_BUCKET_NAME", "test-bucket")
	os.Setenv("R2_ACCESS_KEY_ID", "test-key")
	os.Setenv("R2_SECRET_ACCESS_KEY", "test-secret")
	os.Setenv("R2_ENDPOINT", "https://test.r2.cloudflarestorage.com")

	tests := []struct {
		name    string
		portVal string
		wantErr bool
	}{
		{
			name:    "non-numeric port",
			portVal: "abc",
			wantErr: true,
		},
		{
			name:    "port with suffix",
			portVal: "8080x",
			wantErr: true,
		},
		{
			name:    "empty port uses default",
			portVal: "",
			wantErr: false,
		},
		{
			name:    "valid port",
			portVal: "3000",
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.portVal != "" {
				os.Setenv("PORT", tt.portVal)
			} else {
				os.Unsetenv("PORT")
			}

			_, errs := Load("")

			hasPortErr := false
			for _, err := range errs {
				if errors.Is(err, ErrInvalidPort) {
					hasPortErr = true
					break
				}
			}

			if tt.wantErr && !hasPortErr {
				t.Errorf("Load() with PORT=%q should return port error, got errors: %v", tt.portVal, errs)
			}
			if !tt.wantErr && hasPortErr {
				t.Errorf("Load() with PORT=%q should not return port error, got errors: %v", tt.portVal, errs)
			}
		})
	}
}

func TestLoad_NonExistentConfigFile(t *testing.T) {
	clearEnv()
	defer clearEnv()

	_, errs := Load("/nonexistent/path/config.yaml")

	if len(errs) == 0 {
		t.Error("Load() with non-existent file should return error")
	}

	// Check that the error mentions the file
	found := false
	for _, err := range errs {
		if err != nil && strings.Contains(err.Error(), "failed to load config file") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("Load() error should mention 'failed to load config file', got: %v", errs)
	}
}

func TestLoad_InvalidYAMLSyntax(t *testing.T) {
	clearEnv()
	defer clearEnv()

	// Create a temporary file with invalid YAML (unclosed bracket)
	invalidYAML := `port: 3000
env: staging
database_url: [unclosed bracket
`
	tmpFile, err := os.CreateTemp("", "invalid-config-*.yaml")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())

	if _, err := tmpFile.WriteString(invalidYAML); err != nil {
		t.Fatalf("Failed to write to temp file: %v", err)
	}
	if err := tmpFile.Close(); err != nil {
		t.Fatalf("Failed to close temp file: %v", err)
	}

	_, errs := Load(tmpFile.Name())

	if len(errs) == 0 {
		t.Error("Load() with invalid YAML should return error")
	}

	// Check that the error mentions the file
	found := false
	for _, err := range errs {
		if err != nil && strings.Contains(err.Error(), "failed to load config file") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("Load() error should mention 'failed to load config file', got: %v", errs)
	}
}

func TestLoad_SubcultEnvAliases(t *testing.T) {
	tests := []struct {
		name     string
		envVars  map[string]string
		wantPort int
		wantEnv  string
	}{
		{
			name: "SUBCULT_PORT and SUBCULT_ENV take precedence",
			envVars: map[string]string{
				"SUBCULT_PORT":                  "9000",
				"PORT":                          "8080",
				"SUBCULT_ENV":                   "production",
				"ENV":                           "development",
				"GO_ENV":                        "staging",
				"DATABASE_URL":                  "postgres://localhost/test",
				"JWT_SECRET":                    "supersecret32characterlongvalue!",
				"LIVEKIT_URL":                   "wss://livekit.example.com",
				"LIVEKIT_API_KEY":               "api_key",
				"LIVEKIT_API_SECRET":            "api_secret",
				"STRIPE_API_KEY":                "sk_test_123",
				"STRIPE_WEBHOOK_SECRET":         "whsec_123",
				"STRIPE_ONBOARDING_RETURN_URL":  "https://example.com/return",
				"STRIPE_ONBOARDING_REFRESH_URL": "https://example.com/refresh",
				"MAPTILER_API_KEY":              "maptiler_key",
				"JETSTREAM_URL":                 "wss://jetstream.example.com",
				"R2_BUCKET_NAME":                "test-bucket",
				"R2_ACCESS_KEY_ID":              "test-key",
				"R2_SECRET_ACCESS_KEY":          "test-secret",
				"R2_ENDPOINT":                   "https://test.r2.cloudflarestorage.com",
			},
			wantPort: 9000,
			wantEnv:  "production",
		},
		{
			name: "PORT fallback when SUBCULT_PORT not set",
			envVars: map[string]string{
				"PORT":                          "3000",
				"ENV":                           "staging",
				"DATABASE_URL":                  "postgres://localhost/test",
				"JWT_SECRET":                    "supersecret32characterlongvalue!",
				"LIVEKIT_URL":                   "wss://livekit.example.com",
				"LIVEKIT_API_KEY":               "api_key",
				"LIVEKIT_API_SECRET":            "api_secret",
				"STRIPE_API_KEY":                "sk_test_123",
				"STRIPE_WEBHOOK_SECRET":         "whsec_123",
				"STRIPE_ONBOARDING_RETURN_URL":  "https://example.com/return",
				"STRIPE_ONBOARDING_REFRESH_URL": "https://example.com/refresh",
				"MAPTILER_API_KEY":              "maptiler_key",
				"JETSTREAM_URL":                 "wss://jetstream.example.com",
				"R2_BUCKET_NAME":                "test-bucket",
				"R2_ACCESS_KEY_ID":              "test-key",
				"R2_SECRET_ACCESS_KEY":          "test-secret",
				"R2_ENDPOINT":                   "https://test.r2.cloudflarestorage.com",
			},
			wantPort: 3000,
			wantEnv:  "staging",
		},
		{
			name: "GO_ENV fallback when SUBCULT_ENV and ENV not set",
			envVars: map[string]string{
				"GO_ENV":                        "testing",
				"DATABASE_URL":                  "postgres://localhost/test",
				"JWT_SECRET":                    "supersecret32characterlongvalue!",
				"LIVEKIT_URL":                   "wss://livekit.example.com",
				"LIVEKIT_API_KEY":               "api_key",
				"LIVEKIT_API_SECRET":            "api_secret",
				"STRIPE_API_KEY":                "sk_test_123",
				"STRIPE_WEBHOOK_SECRET":         "whsec_123",
				"STRIPE_ONBOARDING_RETURN_URL":  "https://example.com/return",
				"STRIPE_ONBOARDING_REFRESH_URL": "https://example.com/refresh",
				"MAPTILER_API_KEY":              "maptiler_key",
				"JETSTREAM_URL":                 "wss://jetstream.example.com",
				"R2_BUCKET_NAME":                "test-bucket",
				"R2_ACCESS_KEY_ID":              "test-key",
				"R2_SECRET_ACCESS_KEY":          "test-secret",
				"R2_ENDPOINT":                   "https://test.r2.cloudflarestorage.com",
			},
			wantPort: DefaultPort,
			wantEnv:  "testing",
		},
		{
			name: "defaults when no env vars set for port and env",
			envVars: map[string]string{
				"DATABASE_URL":                  "postgres://localhost/test",
				"JWT_SECRET":                    "supersecret32characterlongvalue!",
				"LIVEKIT_URL":                   "wss://livekit.example.com",
				"LIVEKIT_API_KEY":               "api_key",
				"LIVEKIT_API_SECRET":            "api_secret",
				"STRIPE_API_KEY":                "sk_test_123",
				"STRIPE_WEBHOOK_SECRET":         "whsec_123",
				"STRIPE_ONBOARDING_RETURN_URL":  "https://example.com/return",
				"STRIPE_ONBOARDING_REFRESH_URL": "https://example.com/refresh",
				"MAPTILER_API_KEY":              "maptiler_key",
				"JETSTREAM_URL":                 "wss://jetstream.example.com",
				"R2_BUCKET_NAME":                "test-bucket",
				"R2_ACCESS_KEY_ID":              "test-key",
				"R2_SECRET_ACCESS_KEY":          "test-secret",
				"R2_ENDPOINT":                   "https://test.r2.cloudflarestorage.com",
			},
			wantPort: DefaultPort,
			wantEnv:  DefaultEnv,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			clearEnv()
			defer clearEnv()

			for k, v := range tt.envVars {
				os.Setenv(k, v)
			}

			cfg, errs := Load("")

			if len(errs) != 0 {
				t.Errorf("Load() returned errors: %v", errs)
			}

			if cfg.Port != tt.wantPort {
				t.Errorf("cfg.Port = %d, want %d", cfg.Port, tt.wantPort)
			}
			if cfg.Env != tt.wantEnv {
				t.Errorf("cfg.Env = %s, want %s", cfg.Env, tt.wantEnv)
			}
		})
	}
}

func TestLoad_InvalidSubcultPort(t *testing.T) {
	clearEnv()
	defer clearEnv()

	// Set all required env vars
	os.Setenv("DATABASE_URL", "postgres://localhost/test")
	os.Setenv("JWT_SECRET", "supersecret32characterlongvalue!")
	os.Setenv("LIVEKIT_URL", "wss://livekit.example.com")
	os.Setenv("LIVEKIT_API_KEY", "api_key")
	os.Setenv("LIVEKIT_API_SECRET", "api_secret")
	os.Setenv("STRIPE_API_KEY", "sk_test_123")
	os.Setenv("STRIPE_WEBHOOK_SECRET", "whsec_123")
	os.Setenv("STRIPE_ONBOARDING_RETURN_URL", "https://example.com/return")
	os.Setenv("STRIPE_ONBOARDING_REFRESH_URL", "https://example.com/refresh")
	os.Setenv("MAPTILER_API_KEY", "maptiler_key")
	os.Setenv("JETSTREAM_URL", "wss://jetstream.example.com")
	os.Setenv("R2_BUCKET_NAME", "test-bucket")
	os.Setenv("R2_ACCESS_KEY_ID", "test-key")
	os.Setenv("R2_SECRET_ACCESS_KEY", "test-secret")
	os.Setenv("R2_ENDPOINT", "https://test.r2.cloudflarestorage.com")

	tests := []struct {
		name    string
		portVal string
		wantErr bool
	}{
		{
			name:    "invalid SUBCULT_PORT",
			portVal: "not-a-number",
			wantErr: true,
		},
		{
			name:    "valid SUBCULT_PORT",
			portVal: "9090",
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			os.Setenv("SUBCULT_PORT", tt.portVal)
			defer os.Unsetenv("SUBCULT_PORT")

			_, errs := Load("")

			hasPortErr := false
			for _, err := range errs {
				if errors.Is(err, ErrInvalidPort) {
					hasPortErr = true
					break
				}
			}

			if tt.wantErr && !hasPortErr {
				t.Errorf("Load() with SUBCULT_PORT=%q should return port error, got errors: %v", tt.portVal, errs)
			}
			if !tt.wantErr && hasPortErr {
				t.Errorf("Load() with SUBCULT_PORT=%q should not return port error, got errors: %v", tt.portVal, errs)
			}
		})
	}
}

func TestLoad_RankTrustEnabled(t *testing.T) {
	tests := []struct {
		name     string
		envValue string
		want     bool
	}{
		{
			name:     "true (lowercase)",
			envValue: "true",
			want:     true,
		},
		{
			name:     "True (mixed case)",
			envValue: "True",
			want:     true,
		},
		{
			name:     "TRUE (uppercase)",
			envValue: "TRUE",
			want:     true,
		},
		{
			name:     "1",
			envValue: "1",
			want:     true,
		},
		{
			name:     "yes",
			envValue: "yes",
			want:     true,
		},
		{
			name:     "YES",
			envValue: "YES",
			want:     true,
		},
		{
			name:     "on",
			envValue: "on",
			want:     true,
		},
		{
			name:     "ON",
			envValue: "ON",
			want:     true,
		},
		{
			name:     "false (lowercase)",
			envValue: "false",
			want:     false,
		},
		{
			name:     "False (mixed case)",
			envValue: "False",
			want:     false,
		},
		{
			name:     "FALSE (uppercase)",
			envValue: "FALSE",
			want:     false,
		},
		{
			name:     "0",
			envValue: "0",
			want:     false,
		},
		{
			name:     "no",
			envValue: "no",
			want:     false,
		},
		{
			name:     "NO",
			envValue: "NO",
			want:     false,
		},
		{
			name:     "off",
			envValue: "off",
			want:     false,
		},
		{
			name:     "OFF",
			envValue: "OFF",
			want:     false,
		},
		{
			name:     "invalid value defaults to false",
			envValue: "invalid",
			want:     false,
		},
		{
			name:     "empty string defaults to false",
			envValue: "",
			want:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			clearEnv()
			defer clearEnv()

			// Set all required env vars
			os.Setenv("DATABASE_URL", "postgres://localhost/test")
			os.Setenv("JWT_SECRET", "supersecret32characterlongvalue!")
			os.Setenv("LIVEKIT_URL", "wss://livekit.example.com")
			os.Setenv("LIVEKIT_API_KEY", "api_key")
			os.Setenv("LIVEKIT_API_SECRET", "api_secret")
			os.Setenv("STRIPE_API_KEY", "sk_test_123")
			os.Setenv("STRIPE_WEBHOOK_SECRET", "whsec_123")
			os.Setenv("STRIPE_ONBOARDING_RETURN_URL", "https://example.com/return")
			os.Setenv("STRIPE_ONBOARDING_REFRESH_URL", "https://example.com/refresh")
			os.Setenv("MAPTILER_API_KEY", "maptiler_key")
			os.Setenv("JETSTREAM_URL", "wss://jetstream.example.com")
			os.Setenv("R2_BUCKET_NAME", "test-bucket")
			os.Setenv("R2_ACCESS_KEY_ID", "test-key")
			os.Setenv("R2_SECRET_ACCESS_KEY", "test-secret")
			os.Setenv("R2_ENDPOINT", "https://test.r2.cloudflarestorage.com")

			if tt.envValue != "" {
				os.Setenv("RANK_TRUST_ENABLED", tt.envValue)
			}

			cfg, errs := Load("")

			if len(errs) != 0 {
				t.Errorf("Load() returned errors: %v", errs)
			}

			if cfg.RankTrustEnabled != tt.want {
				t.Errorf("cfg.RankTrustEnabled = %t, want %t", cfg.RankTrustEnabled, tt.want)
			}
		})
	}
}

func TestLoad_RankTrustEnabled_YAMLOverride(t *testing.T) {
	clearEnv()
	defer clearEnv()

	// Create a temporary YAML config file with rank_trust_enabled set to true
	yamlContent := `port: 3000
env: staging
database_url: postgres://localhost/filedb
jwt_secret: file_jwt_secret_value_32_chars!
livekit_url: wss://file-livekit.example.com
livekit_api_key: file_livekit_key
livekit_api_secret: file_livekit_secret
stripe_api_key: sk_test_file_key
stripe_webhook_secret: whsec_file_secret
stripe_onboarding_return_url: https://example.com/return
stripe_onboarding_refresh_url: https://example.com/refresh
maptiler_api_key: file_maptiler_key
jetstream_url: wss://file-jetstream.example.com
r2_bucket_name: file-bucket
r2_access_key_id: file-key
r2_secret_access_key: file-secret
r2_endpoint: https://file.r2.cloudflarestorage.com
rank_trust_enabled: true
`
	tmpFile, err := os.CreateTemp("", "config-*.yaml")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())

	if _, err := tmpFile.WriteString(yamlContent); err != nil {
		t.Fatalf("Failed to write to temp file: %v", err)
	}
	if err := tmpFile.Close(); err != nil {
		t.Fatalf("Failed to close temp file: %v", err)
	}

	cfg, errs := Load(tmpFile.Name())

	if len(errs) != 0 {
		t.Errorf("Load() returned errors: %v", errs)
	}

	// Should read true from YAML file
	if !cfg.RankTrustEnabled {
		t.Error("cfg.RankTrustEnabled = false, want true from YAML file")
	}

	// Now test that env var overrides YAML file
	os.Setenv("RANK_TRUST_ENABLED", "false")

	cfg2, errs2 := Load(tmpFile.Name())

	if len(errs2) != 0 {
		t.Errorf("Load() returned errors: %v", errs2)
	}

	// Env var should override YAML file
	if cfg2.RankTrustEnabled {
		t.Error("cfg.RankTrustEnabled = true, want false (env should override YAML)")
	}
}

func TestLoad_RankTrustEnabled_YAMLFalseValue(t *testing.T) {
	clearEnv()
	defer clearEnv()

	// Create a temporary YAML config file with rank_trust_enabled explicitly set to false
	yamlContent := `port: 3000
env: staging
database_url: postgres://localhost/filedb
jwt_secret: file_jwt_secret_value_32_chars!
livekit_url: wss://file-livekit.example.com
livekit_api_key: file_livekit_key
livekit_api_secret: file_livekit_secret
stripe_api_key: sk_test_file_key
stripe_webhook_secret: whsec_file_secret
stripe_onboarding_return_url: https://example.com/return
stripe_onboarding_refresh_url: https://example.com/refresh
maptiler_api_key: file_maptiler_key
jetstream_url: wss://file-jetstream.example.com
r2_bucket_name: file-bucket
r2_access_key_id: file-key
r2_secret_access_key: file-secret
r2_endpoint: https://file.r2.cloudflarestorage.com
rank_trust_enabled: false
`
	tmpFile, err := os.CreateTemp("", "config-*.yaml")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())

	if _, err := tmpFile.WriteString(yamlContent); err != nil {
		t.Fatalf("Failed to write to temp file: %v", err)
	}
	if err := tmpFile.Close(); err != nil {
		t.Fatalf("Failed to close temp file: %v", err)
	}

	cfg, errs := Load(tmpFile.Name())

	if len(errs) != 0 {
		t.Errorf("Load() returned errors: %v", errs)
	}

	// Should respect explicit false from YAML file (not use default)
	if cfg.RankTrustEnabled {
		t.Error("cfg.RankTrustEnabled = true, want false from YAML file")
	}
}

// TestJWTSecretRotation tests the dual-key JWT rotation feature.
func TestJWTSecretRotation(t *testing.T) {
	clearEnv()
	defer clearEnv()

	t.Run("legacy JWT_SECRET still works", func(t *testing.T) {
		clearEnv()
		os.Setenv("DATABASE_URL", "postgres://localhost/test")
		os.Setenv("JWT_SECRET", "supersecret32characterlongvalue!")
		os.Setenv("LIVEKIT_URL", "wss://livekit.example.com")
		os.Setenv("LIVEKIT_API_KEY", "api_key")
		os.Setenv("LIVEKIT_API_SECRET", "api_secret")
		os.Setenv("STRIPE_API_KEY", "sk_test_123")
		os.Setenv("STRIPE_WEBHOOK_SECRET", "whsec_123")
		os.Setenv("STRIPE_ONBOARDING_RETURN_URL", "https://example.com/return")
		os.Setenv("STRIPE_ONBOARDING_REFRESH_URL", "https://example.com/refresh")
		os.Setenv("MAPTILER_API_KEY", "maptiler_key")
		os.Setenv("JETSTREAM_URL", "wss://jetstream.example.com")

		cfg, errs := Load("")
		if len(errs) != 0 {
			t.Errorf("Load() returned errors: %v", errs)
		}
		if cfg.JWTSecret != "supersecret32characterlongvalue!" {
			t.Errorf("cfg.JWTSecret = %s, want supersecret32characterlongvalue!", cfg.JWTSecret)
		}
	})

	t.Run("JWT_SECRET_CURRENT without previous secret", func(t *testing.T) {
		clearEnv()
		os.Setenv("DATABASE_URL", "postgres://localhost/test")
		os.Setenv("JWT_SECRET_CURRENT", "current-secret-key-32-characters!")
		os.Setenv("LIVEKIT_URL", "wss://livekit.example.com")
		os.Setenv("LIVEKIT_API_KEY", "api_key")
		os.Setenv("LIVEKIT_API_SECRET", "api_secret")
		os.Setenv("STRIPE_API_KEY", "sk_test_123")
		os.Setenv("STRIPE_WEBHOOK_SECRET", "whsec_123")
		os.Setenv("STRIPE_ONBOARDING_RETURN_URL", "https://example.com/return")
		os.Setenv("STRIPE_ONBOARDING_REFRESH_URL", "https://example.com/refresh")
		os.Setenv("MAPTILER_API_KEY", "maptiler_key")
		os.Setenv("JETSTREAM_URL", "wss://jetstream.example.com")

		cfg, errs := Load("")
		if len(errs) != 0 {
			t.Errorf("Load() returned errors: %v", errs)
		}
		if cfg.JWTSecretCurrent != "current-secret-key-32-characters!" {
			t.Errorf("cfg.JWTSecretCurrent = %s, want current-secret-key-32-characters!", cfg.JWTSecretCurrent)
		}
		if cfg.JWTSecretPrevious != "" {
			t.Errorf("cfg.JWTSecretPrevious = %s, want empty", cfg.JWTSecretPrevious)
		}
	})

	t.Run("both JWT_SECRET_CURRENT and JWT_SECRET_PREVIOUS", func(t *testing.T) {
		clearEnv()
		os.Setenv("DATABASE_URL", "postgres://localhost/test")
		os.Setenv("JWT_SECRET_CURRENT", "current-secret-key-32-characters!")
		os.Setenv("JWT_SECRET_PREVIOUS", "previous-secret-key-32-chars!!")
		os.Setenv("LIVEKIT_URL", "wss://livekit.example.com")
		os.Setenv("LIVEKIT_API_KEY", "api_key")
		os.Setenv("LIVEKIT_API_SECRET", "api_secret")
		os.Setenv("STRIPE_API_KEY", "sk_test_123")
		os.Setenv("STRIPE_WEBHOOK_SECRET", "whsec_123")
		os.Setenv("STRIPE_ONBOARDING_RETURN_URL", "https://example.com/return")
		os.Setenv("STRIPE_ONBOARDING_REFRESH_URL", "https://example.com/refresh")
		os.Setenv("MAPTILER_API_KEY", "maptiler_key")
		os.Setenv("JETSTREAM_URL", "wss://jetstream.example.com")

		cfg, errs := Load("")
		if len(errs) != 0 {
			t.Errorf("Load() returned errors: %v", errs)
		}
		if cfg.JWTSecretCurrent != "current-secret-key-32-characters!" {
			t.Errorf("cfg.JWTSecretCurrent = %s, want current-secret-key-32-characters!", cfg.JWTSecretCurrent)
		}
		if cfg.JWTSecretPrevious != "previous-secret-key-32-chars!!" {
			t.Errorf("cfg.JWTSecretPrevious = %s, want previous-secret-key-32-chars!!", cfg.JWTSecretPrevious)
		}
	})

	t.Run("missing both JWT_SECRET and JWT_SECRET_CURRENT fails", func(t *testing.T) {
		clearEnv()
		os.Setenv("DATABASE_URL", "postgres://localhost/test")
		os.Setenv("LIVEKIT_URL", "wss://livekit.example.com")
		os.Setenv("LIVEKIT_API_KEY", "api_key")
		os.Setenv("LIVEKIT_API_SECRET", "api_secret")
		os.Setenv("STRIPE_API_KEY", "sk_test_123")
		os.Setenv("STRIPE_WEBHOOK_SECRET", "whsec_123")
		os.Setenv("STRIPE_ONBOARDING_RETURN_URL", "https://example.com/return")
		os.Setenv("STRIPE_ONBOARDING_REFRESH_URL", "https://example.com/refresh")
		os.Setenv("MAPTILER_API_KEY", "maptiler_key")
		os.Setenv("JETSTREAM_URL", "wss://jetstream.example.com")

		_, errs := Load("")
		if len(errs) == 0 {
			t.Error("Load() expected errors, got none")
		}

		foundJWTError := false
		for _, err := range errs {
			if errors.Is(err, ErrMissingJWTSecret) {
				foundJWTError = true
				break
			}
		}
		if !foundJWTError {
			t.Errorf("Load() errors = %v, want ErrMissingJWTSecret", errs)
		}
	})

	t.Run("JWT_SECRET takes precedence over legacy behavior", func(t *testing.T) {
		clearEnv()
		os.Setenv("DATABASE_URL", "postgres://localhost/test")
		os.Setenv("JWT_SECRET", "legacy-secret-key-32-characters!")
		os.Setenv("JWT_SECRET_CURRENT", "current-secret-key-32-characters!")
		os.Setenv("LIVEKIT_URL", "wss://livekit.example.com")
		os.Setenv("LIVEKIT_API_KEY", "api_key")
		os.Setenv("LIVEKIT_API_SECRET", "api_secret")
		os.Setenv("STRIPE_API_KEY", "sk_test_123")
		os.Setenv("STRIPE_WEBHOOK_SECRET", "whsec_123")
		os.Setenv("STRIPE_ONBOARDING_RETURN_URL", "https://example.com/return")
		os.Setenv("STRIPE_ONBOARDING_REFRESH_URL", "https://example.com/refresh")
		os.Setenv("MAPTILER_API_KEY", "maptiler_key")
		os.Setenv("JETSTREAM_URL", "wss://jetstream.example.com")

		cfg, errs := Load("")
		if len(errs) != 0 {
			t.Errorf("Load() returned errors: %v", errs)
		}
		// Both should be populated
		if cfg.JWTSecret != "legacy-secret-key-32-characters!" {
			t.Errorf("cfg.JWTSecret = %s, want legacy-secret-key-32-characters!", cfg.JWTSecret)
		}
		if cfg.JWTSecretCurrent != "current-secret-key-32-characters!" {
			t.Errorf("cfg.JWTSecretCurrent = %s, want current-secret-key-32-characters!", cfg.JWTSecretCurrent)
		}
	})
}

// TestGetJWTSecrets tests the helper method for retrieving JWT secrets.
func TestGetJWTSecrets(t *testing.T) {
	tests := []struct {
		name             string
		jwtSecret        string
		jwtSecretCurrent string
		jwtSecretPrev    string
		wantCurrent      string
		wantPrevious     string
	}{
		{
			name:             "legacy JWT_SECRET only",
			jwtSecret:        "legacy-secret",
			jwtSecretCurrent: "",
			jwtSecretPrev:    "",
			wantCurrent:      "legacy-secret",
			wantPrevious:     "",
		},
		{
			name:             "JWT_SECRET_CURRENT only",
			jwtSecret:        "",
			jwtSecretCurrent: "current-secret",
			jwtSecretPrev:    "",
			wantCurrent:      "current-secret",
			wantPrevious:     "",
		},
		{
			name:             "JWT_SECRET_CURRENT with previous",
			jwtSecret:        "",
			jwtSecretCurrent: "current-secret",
			jwtSecretPrev:    "previous-secret",
			wantCurrent:      "current-secret",
			wantPrevious:     "previous-secret",
		},
		{
			name:             "JWT_SECRET_CURRENT takes precedence over JWT_SECRET",
			jwtSecret:        "legacy-secret",
			jwtSecretCurrent: "current-secret",
			jwtSecretPrev:    "previous-secret",
			wantCurrent:      "current-secret",
			wantPrevious:     "previous-secret",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &Config{
				JWTSecret:         tt.jwtSecret,
				JWTSecretCurrent:  tt.jwtSecretCurrent,
				JWTSecretPrevious: tt.jwtSecretPrev,
			}

			current, previous := cfg.GetJWTSecrets()
			if current != tt.wantCurrent {
				t.Errorf("GetJWTSecrets() current = %v, want %v", current, tt.wantCurrent)
			}
			if previous != tt.wantPrevious {
				t.Errorf("GetJWTSecrets() previous = %v, want %v", previous, tt.wantPrevious)
			}
		})
	}
}
