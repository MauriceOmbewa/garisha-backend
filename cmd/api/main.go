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
	"github.com/MauriceOmbewa/garisha-backend/internal/platform/router"
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
	// Migrations
	// -------------------------------------------------------------------------

	if err := database.Migrate(cfg.Database, "file://migrations", log); err != nil {
		log.Error("failed to run migrations", "error", err)
		os.Exit(1)
	}

	// -------------------------------------------------------------------------
	// Router
	// -------------------------------------------------------------------------

	handler := router.New(log)

	// -------------------------------------------------------------------------
	// HTTP Server
	// -------------------------------------------------------------------------

	srv := router.NewServer(cfg.App.Port, handler, log)

	// Run the server in a goroutine so it does not block the shutdown logic.
	serverErr := make(chan error, 1)
	go func() {
		serverErr <- srv.Start()
	}()

	// Block until a shutdown signal or a fatal server error.
	select {
	case <-ctx.Done():
		log.Info("shutdown signal received")
	case err := <-serverErr:
		log.Error("server error", "error", err)
	}

	// Stop accepting new requests and drain in-flight ones.
	srv.Shutdown()

	log.Info("server stopped")
}
