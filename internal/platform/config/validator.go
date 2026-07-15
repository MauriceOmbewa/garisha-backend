package config

import (
	"errors"
	"fmt"
	"strings"
)

// Validate checks that all required configuration fields are present.
// It collects every missing field and returns them as a single error so
// the operator can fix all problems in one restart cycle.
func Validate(cfg *Config) error {
	var missing []string

	required := []struct {
		value string
		name  string
	}{
		{cfg.App.Name, "APP_NAME"},
		{cfg.App.Env, "APP_ENV"},
		{cfg.App.Port, "PORT"},

		{cfg.Database.Host, "DATABASE_HOST"},
		{cfg.Database.Port, "DATABASE_PORT"},
		{cfg.Database.User, "DATABASE_USER"},
		{cfg.Database.Password, "DATABASE_PASSWORD"},
		{cfg.Database.Name, "DATABASE_NAME"},
		{cfg.Database.SSLMode, "DATABASE_SSLMODE"},

		{cfg.JWT.Secret, "JWT_SECRET"},

		{cfg.Google.ClientID, "GOOGLE_CLIENT_ID"},

		{cfg.Redis.Host, "REDIS_HOST"},
		{cfg.Redis.Port, "REDIS_PORT"},
	}

	for _, r := range required {
		if strings.TrimSpace(r.value) == "" {
			missing = append(missing, r.name)
		}
	}

	if len(missing) > 0 {
		return errors.New(fmt.Sprintf(
			"missing required environment variables: %s",
			strings.Join(missing, ", "),
		))
	}

	return nil
}
