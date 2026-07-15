// Package database manages the PostgreSQL connection pool for the application.
// It uses pgx/v5 via the pgxpool package which is safe for concurrent use
// and handles connection health checks automatically.
package database

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/MauriceOmbewa/garisha-backend/internal/platform/config"
)

const (
	// defaultMaxConns is the upper bound on open connections in the pool.
	defaultMaxConns = 25

	// defaultMinConns keeps a handful of connections warm so the first
	// requests after an idle period don't pay a dial penalty.
	defaultMinConns = 5

	// defaultMaxConnLifetime rotates connections to avoid hitting
	// server-side idle timeouts.
	defaultMaxConnLifetime = 30 * time.Minute

	// defaultMaxConnIdleTime closes connections that have been unused for
	// this duration.
	defaultMaxConnIdleTime = 10 * time.Minute

	// connectTimeout is the maximum time we wait for the initial pool to
	// become healthy on startup.
	connectTimeout = 10 * time.Second
)

// New parses the database configuration, creates a pgxpool connection pool,
// and verifies connectivity with a Ping before returning.
// The caller owns the pool and is responsible for calling pool.Close() on
// shutdown.
func New(ctx context.Context, cfg config.DatabaseConfig) (*pgxpool.Pool, error) {
	dsn := buildDSN(cfg)

	poolCfg, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		return nil, fmt.Errorf("database: parse config: %w", err)
	}

	poolCfg.MaxConns = defaultMaxConns
	poolCfg.MinConns = defaultMinConns
	poolCfg.MaxConnLifetime = defaultMaxConnLifetime
	poolCfg.MaxConnIdleTime = defaultMaxConnIdleTime

	// Use a bounded context for the initial connect so a misconfigured DB
	// doesn't block startup indefinitely.
	connectCtx, cancel := context.WithTimeout(ctx, connectTimeout)
	defer cancel()

	pool, err := pgxpool.NewWithConfig(connectCtx, poolCfg)
	if err != nil {
		return nil, fmt.Errorf("database: create pool: %w", err)
	}

	if err := pool.Ping(connectCtx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("database: ping: %w", err)
	}

	return pool, nil
}

// buildDSN constructs a PostgreSQL connection string from the supplied config.
func buildDSN(cfg config.DatabaseConfig) string {
	return fmt.Sprintf(
		"host=%s port=%s user=%s password=%s dbname=%s sslmode=%s",
		cfg.Host,
		cfg.Port,
		cfg.User,
		cfg.Password,
		cfg.Name,
		cfg.SSLMode,
	)
}
