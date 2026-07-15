package config

import (
	"fmt"
	"os"

	"github.com/joho/godotenv"
)

// Load reads environment variables from .env and returns
// the application's configuration.
func Load() (*Config, error) {

	// Load .env file.
	// If it doesn't exist (e.g. production), we continue because
	// production environments usually provide environment variables directly.
	_ = godotenv.Load()

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
			Secret: getEnv("JWT_SECRET"),
		},

		Google: GoogleConfig{
			ClientID: getEnv("GOOGLE_CLIENT_ID"),
		},

		Redis: RedisConfig{
			Host: getEnv("REDIS_HOST"),
			Port: getEnv("REDIS_PORT"),
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