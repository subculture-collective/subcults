package config

import (
	"os"
	"testing"
)

func TestLoad_MissingMandatory(t *testing.T) {
	// Clear all environment variables that might affect the test
	clearEnv := func() {
		os.Unsetenv("DATABASE_URL")
		os.Unsetenv("JWT_SECRET")
		os.Unsetenv("LIVEKIT_URL")
		os.Unsetenv("LIVEKIT_API_KEY")
		os.Unsetenv("LIVEKIT_API_SECRET")
		os.Unsetenv("STRIPE_API_KEY")
		os.Unsetenv("STRIPE_WEBHOOK_SECRET")
		os.Unsetenv("MAPTILER_API_KEY")
		os.Unsetenv("JETSTREAM_URL")
		os.Unsetenv("PORT")
		os.Unsetenv("ENV")
	}

	tests := []struct {
		name           string
		envVars        map[string]string
		wantErrs       []error
		wantErrCount   int
		checkSpecificErr error
	}{
		{
			name:         "no environment variables set",
			envVars:      map[string]string{},
			wantErrCount: 9, // All mandatory fields missing
		},
		{
			name: "only DATABASE_URL set",
			envVars: map[string]string{
				"DATABASE_URL": "postgres://localhost/test",
			},
			wantErrCount: 8,
			checkSpecificErr: ErrMissingJWTSecret,
		},
		{
			name: "missing JWT_SECRET",
			envVars: map[string]string{
				"DATABASE_URL":          "postgres://localhost/test",
				"LIVEKIT_URL":           "wss://livekit.example.com",
				"LIVEKIT_API_KEY":       "api_key",
				"LIVEKIT_API_SECRET":    "api_secret",
				"STRIPE_API_KEY":        "sk_test_123",
				"STRIPE_WEBHOOK_SECRET": "whsec_123",
				"MAPTILER_API_KEY":      "maptiler_key",
				"JETSTREAM_URL":         "wss://jetstream.example.com",
			},
			wantErrCount:     1,
			checkSpecificErr: ErrMissingJWTSecret,
		},
		{
			name: "missing STRIPE_API_KEY",
			envVars: map[string]string{
				"DATABASE_URL":          "postgres://localhost/test",
				"JWT_SECRET":            "supersecret32characterlongvalue!",
				"LIVEKIT_URL":           "wss://livekit.example.com",
				"LIVEKIT_API_KEY":       "api_key",
				"LIVEKIT_API_SECRET":    "api_secret",
				"STRIPE_WEBHOOK_SECRET": "whsec_123",
				"MAPTILER_API_KEY":      "maptiler_key",
				"JETSTREAM_URL":         "wss://jetstream.example.com",
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
	// Clear all environment variables that might affect the test
	clearEnv := func() {
		os.Unsetenv("DATABASE_URL")
		os.Unsetenv("JWT_SECRET")
		os.Unsetenv("LIVEKIT_URL")
		os.Unsetenv("LIVEKIT_API_KEY")
		os.Unsetenv("LIVEKIT_API_SECRET")
		os.Unsetenv("STRIPE_API_KEY")
		os.Unsetenv("STRIPE_WEBHOOK_SECRET")
		os.Unsetenv("MAPTILER_API_KEY")
		os.Unsetenv("JETSTREAM_URL")
		os.Unsetenv("PORT")
		os.Unsetenv("ENV")
	}

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
	os.Setenv("MAPTILER_API_KEY", "maptiler_key_123")
	os.Setenv("JETSTREAM_URL", "wss://jetstream.example.com")
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
	// Clear all environment variables that might affect the test
	clearEnv := func() {
		os.Unsetenv("DATABASE_URL")
		os.Unsetenv("JWT_SECRET")
		os.Unsetenv("LIVEKIT_URL")
		os.Unsetenv("LIVEKIT_API_KEY")
		os.Unsetenv("LIVEKIT_API_SECRET")
		os.Unsetenv("STRIPE_API_KEY")
		os.Unsetenv("STRIPE_WEBHOOK_SECRET")
		os.Unsetenv("MAPTILER_API_KEY")
		os.Unsetenv("JETSTREAM_URL")
		os.Unsetenv("PORT")
		os.Unsetenv("ENV")
	}

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
	os.Setenv("MAPTILER_API_KEY", "maptiler_key")
	os.Setenv("JETSTREAM_URL", "wss://jetstream.example.com")

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
		Port:               8080,
		Env:                "production",
		DatabaseURL:        "postgres://user:pass@localhost/subcults",
		JWTSecret:          "supersecret32characterlongvalue!",
		LiveKitURL:         "wss://livekit.example.com",
		LiveKitAPIKey:      "api_key_123456",
		LiveKitAPISecret:   "api_secret_789",
		StripeAPIKey:       "sk_live_abcdefghijk",
		StripeWebhookSecret: "whsec_123456789",
		MapTilerAPIKey:     "maptiler_key_abc",
		JetstreamURL:       "wss://jetstream.example.com",
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
		name       string
		config     Config
		wantErrs   int
		checkForErr error
	}{
		{
			name:     "empty config has all errors",
			config:   Config{},
			wantErrs: 9,
		},
		{
			name: "fully valid config",
			config: Config{
				DatabaseURL:        "postgres://localhost/test",
				JWTSecret:          "secret",
				LiveKitURL:         "wss://livekit.example.com",
				LiveKitAPIKey:      "key",
				LiveKitAPISecret:   "secret",
				StripeAPIKey:       "sk_test_123",
				StripeWebhookSecret: "whsec_123",
				MapTilerAPIKey:     "key",
				JetstreamURL:       "wss://jetstream.example.com",
			},
			wantErrs: 0,
		},
		{
			name: "missing only LiveKitURL",
			config: Config{
				DatabaseURL:        "postgres://localhost/test",
				JWTSecret:          "secret",
				LiveKitAPIKey:      "key",
				LiveKitAPISecret:   "secret",
				StripeAPIKey:       "sk_test_123",
				StripeWebhookSecret: "whsec_123",
				MapTilerAPIKey:     "key",
				JetstreamURL:       "wss://jetstream.example.com",
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
	// Clear all environment variables that might affect the test
	clearEnv := func() {
		os.Unsetenv("DATABASE_URL")
		os.Unsetenv("JWT_SECRET")
		os.Unsetenv("LIVEKIT_URL")
		os.Unsetenv("LIVEKIT_API_KEY")
		os.Unsetenv("LIVEKIT_API_SECRET")
		os.Unsetenv("STRIPE_API_KEY")
		os.Unsetenv("STRIPE_WEBHOOK_SECRET")
		os.Unsetenv("MAPTILER_API_KEY")
		os.Unsetenv("JETSTREAM_URL")
		os.Unsetenv("PORT")
		os.Unsetenv("ENV")
	}

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
maptiler_api_key: file_maptiler_key
jetstream_url: wss://file-jetstream.example.com
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
	// Clear all environment variables that might affect the test
	clearEnv := func() {
		os.Unsetenv("DATABASE_URL")
		os.Unsetenv("JWT_SECRET")
		os.Unsetenv("LIVEKIT_URL")
		os.Unsetenv("LIVEKIT_API_KEY")
		os.Unsetenv("LIVEKIT_API_SECRET")
		os.Unsetenv("STRIPE_API_KEY")
		os.Unsetenv("STRIPE_WEBHOOK_SECRET")
		os.Unsetenv("MAPTILER_API_KEY")
		os.Unsetenv("JETSTREAM_URL")
		os.Unsetenv("PORT")
		os.Unsetenv("ENV")
	}

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
maptiler_api_key: file_maptiler_key
jetstream_url: wss://file-jetstream.example.com
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
