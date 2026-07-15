package database

import (
	"errors"
	"fmt"
	"log/slog"

	"github.com/golang-migrate/migrate/v4"
	// pgx v5 driver for golang-migrate.
	_ "github.com/golang-migrate/migrate/v4/database/pgx/v5"
	// File-system source for migration SQL files.
	_ "github.com/golang-migrate/migrate/v4/source/file"

	"github.com/MauriceOmbewa/garisha-backend/internal/platform/config"
)

// Migrate runs all pending UP migrations found in migrationsPath against
// the database described by cfg.  It is safe to call on every startup —
// when the schema is already up to date, migrate.ErrNoChange is silently
// ignored so the application continues normally.
//
// migrationsPath must be a valid file-source URL understood by
// golang-migrate, e.g. "file://migrations".
func Migrate(cfg config.DatabaseConfig, migrationsPath string, log *slog.Logger) error {
	dsn := buildMigrateDSN(cfg)

	m, err := migrate.New(migrationsPath, dsn)
	if err != nil {
		return fmt.Errorf("migrate: initialise: %w", err)
	}
	defer func() {
		srcErr, dbErr := m.Close()
		if srcErr != nil {
			log.Warn("migrate: error closing source", "error", srcErr)
		}
		if dbErr != nil {
			log.Warn("migrate: error closing db connection", "error", dbErr)
		}
	}()

	log.Info("running database migrations")

	if err := m.Up(); err != nil {
		if errors.Is(err, migrate.ErrNoChange) {
			log.Info("database schema is up to date")
			return nil
		}
		return fmt.Errorf("migrate: up: %w", err)
	}

	version, dirty, err := m.Version()
	if err != nil {
		return fmt.Errorf("migrate: version: %w", err)
	}

	log.Info("migrations applied successfully", "version", version, "dirty", dirty)
	return nil
}

// buildMigrateDSN constructs the DSN string expected by golang-migrate's
// pgx/v5 driver (pgx5://user:password@host:port/dbname?sslmode=...).
func buildMigrateDSN(cfg config.DatabaseConfig) string {
	return fmt.Sprintf(
		"pgx5://%s:%s@%s:%s/%s?sslmode=%s",
		cfg.User,
		cfg.Password,
		cfg.Host,
		cfg.Port,
		cfg.Name,
		cfg.SSLMode,
	)
}
