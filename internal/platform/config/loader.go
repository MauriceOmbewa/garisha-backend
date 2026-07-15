package config

import (
	"fmt"
	"os"
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
			ClientID: getEnv("GOOGLE_CLIENT_ID"),
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
