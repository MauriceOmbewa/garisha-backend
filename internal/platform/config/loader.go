package config

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/joho/godotenv"
)

// Load reads environment variables from .env (if present) and returns the
// application's fully validated configuration.
func Load() (*Config, error) {
	// .env is optional — production environments supply vars directly.
	_ = godotenv.Load()

	accessTTL, err := parseDuration("JWT_ACCESS_TTL", "15m")
	if err != nil {
		return nil, err
	}

	refreshTTL, err := parseDuration("JWT_REFRESH_TTL", "168h")
	if err != nil {
		return nil, err
	}

	cfg := &Config{
		App: AppConfig{
			Name: getEnv("APP_NAME"),
			Env:  getEnv("APP_ENV"),
			Port: getEnv("PORT"),
		},

		Database: DatabaseConfig{
			Host:     getEnv("DATABASE_HOST"),
			Port:     getEnv("DATABASE_PORT"),
			User:     getEnv("DATABASE_USER"),
			Password: getEnv("DATABASE_PASSWORD"),
			Name:     getEnv("DATABASE_NAME"),
			SSLMode:  getEnv("DATABASE_SSLMODE"),
		},

		JWT: JWTConfig{
			Secret:     getEnv("JWT_SECRET"),
			AccessTTL:  accessTTL,
			RefreshTTL: refreshTTL,
		},

		Google: GoogleConfig{
			ClientID:        getEnv("GOOGLE_CLIENT_ID"),
			ClientSecret:    getEnv("GOOGLE_CLIENT_SECRET"),
			RedirectURL:     getEnv("GOOGLE_REDIRECT_URL"),
			AllowedOrigins:  splitCSV(getEnv("GOOGLE_ALLOWED_ORIGINS")),
			AndroidClientID: getEnv("GOOGLE_ANDROID_CLIENT_ID"),
			IOSClientID:     getEnv("GOOGLE_IOS_CLIENT_ID"),
		},

		Redis: RedisConfig{
			Host: getEnv("REDIS_HOST"),
			Port: getEnv("REDIS_PORT"),
		},

		Mpesa: MpesaConfig{
			ConsumerKey:    getEnv("MPESA_CONSUMER_KEY"),
			ConsumerSecret: getEnv("MPESA_CONSUMER_SECRET"),
			Passkey:        getEnv("MPESA_PASSKEY"),
			ShortCode:      getEnv("MPESA_SHORT_CODE"),
			CallbackURL:    getEnv("MPESA_CALLBACK_URL"),
			Environment:    getEnv("MPESA_ENVIRONMENT"),
		},

		Storage: StorageConfig{
			Endpoint:        getEnv("STORAGE_ENDPOINT"),
			Region:          getEnv("STORAGE_REGION"),
			Bucket:          getEnv("STORAGE_BUCKET"),
			AccessKeyID:     getEnv("STORAGE_ACCESS_KEY_ID"),
			SecretAccessKey: getEnv("STORAGE_SECRET_ACCESS_KEY"),
			UseSSL:          getEnv("STORAGE_USE_SSL") != "false",
			PublicBaseURL:   getEnv("STORAGE_PUBLIC_BASE_URL"),
		},
	}

	if err := Validate(cfg); err != nil {
		return nil, err
	}

	return cfg, nil
}

// getEnv returns the value of an environment variable.
func getEnv(key string) string {
	return os.Getenv(key)
}

// splitCSV splits a comma-separated string into a trimmed slice.
// Returns nil if the value is empty.
func splitCSV(s string) []string {
	if s == "" {
		return nil
	}
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if v := strings.TrimSpace(p); v != "" {
			out = append(out, v)
		}
	}
	return out
}

// parseDuration reads key from the environment and parses it as a
// time.Duration.  If the key is unset, defaultVal is used.
func parseDuration(key, defaultVal string) (time.Duration, error) {
	raw := os.Getenv(key)
	if raw == "" {
		raw = defaultVal
	}

	d, err := time.ParseDuration(raw)
	if err != nil {
		return 0, fmt.Errorf("config: %s: invalid duration %q: %w", key, raw, err)
	}

	return d, nil
}
