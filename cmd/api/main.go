package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/MauriceOmbewa/garisha-backend/internal/platform/config"
	"github.com/MauriceOmbewa/garisha-backend/internal/platform/database"
	"github.com/MauriceOmbewa/garisha-backend/internal/platform/logger"
)

func main() {
	// -------------------------------------------------------------------------
	// Configuration
	// -------------------------------------------------------------------------

	cfg, err := config.Load()
	if err != nil {
		// Logger is not yet available; fall back to stdlib for this one error.
		slog.New(slog.NewTextHandler(os.Stderr, nil)).
			Error("failed to load configuration", "error", err)
		os.Exit(1)
	}

	// -------------------------------------------------------------------------
	// Logger
	// -------------------------------------------------------------------------

	log := logger.New(cfg.App.Env, os.Stdout)
	log.Info("configuration loaded",
		"app", cfg.App.Name,
		"env", cfg.App.Env,
		"port", cfg.App.Port,
	)

	// -------------------------------------------------------------------------
	// Database
	// -------------------------------------------------------------------------

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	db, err := database.New(ctx, cfg.Database)
	if err != nil {
		log.Error("failed to connect to database", "error", err)
		os.Exit(1)
	}
	defer db.Close()

	log.Info("database connected",
		"host", cfg.Database.Host,
		"port", cfg.Database.Port,
		"name", cfg.Database.Name,
	)

	// -------------------------------------------------------------------------
	// Placeholder — router and server will be wired here in a later phase
	// -------------------------------------------------------------------------

	log.Info("startup complete — waiting for shutdown signal")

	// Block until SIGINT / SIGTERM.
	<-ctx.Done()

	log.Info("shutdown signal received, stopping gracefully")
}
