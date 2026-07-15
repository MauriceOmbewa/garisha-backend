package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/MauriceOmbewa/garisha-backend/internal/auth"
	"github.com/MauriceOmbewa/garisha-backend/internal/company"
	"github.com/MauriceOmbewa/garisha-backend/internal/customers"
	"github.com/MauriceOmbewa/garisha-backend/internal/finance"
	"github.com/MauriceOmbewa/garisha-backend/internal/hire"
	"github.com/MauriceOmbewa/garisha-backend/internal/sales"
	"github.com/MauriceOmbewa/garisha-backend/internal/service"
	"github.com/MauriceOmbewa/garisha-backend/internal/tenants"
	"github.com/MauriceOmbewa/garisha-backend/internal/users"
	"github.com/MauriceOmbewa/garisha-backend/internal/vehicles"
	platformauth "github.com/MauriceOmbewa/garisha-backend/internal/platform/auth"
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
		slog.New(slog.NewTextHandler(os.Stderr, nil)).
			Error("failed to load configuration", "error", err)
		os.Exit(1)
	}

	// -------------------------------------------------------------------------
	// Logger
	// -------------------------------------------------------------------------

	log := logger.New(cfg.App.Env, os.Stdout)
	log.Info("configuration loaded",
		"app",  cfg.App.Name,
		"env",  cfg.App.Env,
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
	// Platform infrastructure
	// -------------------------------------------------------------------------

	jwtManager, err := platformauth.NewManager(
		cfg.JWT.Secret,
		cfg.JWT.AccessTTL,
		cfg.JWT.RefreshTTL,
	)
	if err != nil {
		log.Error("failed to create JWT manager", "error", err)
		os.Exit(1)
	}

	googleVerifier := platformauth.NewGoogleVerifier(cfg.Google.ClientID)

	// -------------------------------------------------------------------------
	// Tenants domain (also serves as the TenantResolver for middleware)
	// -------------------------------------------------------------------------

	tenantsRepo    := tenants.NewRepository(db)
	tenantsService := tenants.NewService(tenantsRepo, log)
	tenantsHandler := tenants.NewHandler(tenantsService, log)

	// -------------------------------------------------------------------------
	// Auth domain
	// -------------------------------------------------------------------------

	authRepo    := auth.NewRepository(db)
	authService := auth.NewService(authRepo, jwtManager, googleVerifier, log)
	authHandler := auth.NewHandler(authService, log)

	// -------------------------------------------------------------------------
	// Company domain
	// -------------------------------------------------------------------------

	companyRepo    := company.NewRepository(db)
	companyService := company.NewService(companyRepo, log)
	companyHandler := company.NewHandler(companyService, log)

	// -------------------------------------------------------------------------
	// Users domain
	// -------------------------------------------------------------------------

	usersRepo    := users.NewRepository(db)
	usersService := users.NewService(usersRepo, log)
	usersHandler := users.NewHandler(usersService, log)

	// -------------------------------------------------------------------------
	// Vehicles domain
	// -------------------------------------------------------------------------

	vehiclesRepo    := vehicles.NewRepository(db)
	vehiclesService := vehicles.NewService(vehiclesRepo, log)
	vehiclesHandler := vehicles.NewHandler(vehiclesService, log)

	// -------------------------------------------------------------------------
	// Customers domain
	// -------------------------------------------------------------------------

	customersRepo    := customers.NewRepository(db)
	customersService := customers.NewService(customersRepo, log)
	customersHandler := customers.NewHandler(customersService, log)

	// -------------------------------------------------------------------------
	// Hire domain
	// -------------------------------------------------------------------------

	hireRepo    := hire.NewRepository(db)
	hireService := hire.NewService(hireRepo, log)
	hireHandler := hire.NewHandler(hireService, log)

	// -------------------------------------------------------------------------
	// Sales domain
	// -------------------------------------------------------------------------

	salesRepo    := sales.NewRepository(db)
	salesService := sales.NewService(salesRepo, log)
	salesHandler := sales.NewHandler(salesService, log)

	// -------------------------------------------------------------------------
	// Service domain
	// -------------------------------------------------------------------------

	serviceRepo    := service.NewRepository(db)
	serviceService := service.NewService(serviceRepo, log)
	serviceHandler := service.NewHandler(serviceService, log)

	// -------------------------------------------------------------------------
	// Finance domain
	// -------------------------------------------------------------------------

	financeRepo    := finance.NewRepository(db)
	financeService := finance.NewService(financeRepo, log)
	financeHandler := finance.NewHandler(financeService, log)

	// -------------------------------------------------------------------------
	// Router
	// -------------------------------------------------------------------------

	handler := router.New(router.Dependencies{
		Log:              log,
		JWTManager:       jwtManager,
		TenantResolver:   tenantsRepo,
		AuthHandler:      authHandler,
		TenantsHandler:   tenantsHandler,
		CompanyHandler:   companyHandler,
		UsersHandler:     usersHandler,
		VehiclesHandler:  vehiclesHandler,
		CustomersHandler: customersHandler,
		HireHandler:      hireHandler,
		SalesHandler:     salesHandler,
		ServiceHandler:   serviceHandler,
		FinanceHandler:   financeHandler,
	})

	// -------------------------------------------------------------------------
	// HTTP Server
	// -------------------------------------------------------------------------

	srv := router.NewServer(cfg.App.Port, handler, log)

	serverErr := make(chan error, 1)
	go func() {
		serverErr <- srv.Start()
	}()

	select {
	case <-ctx.Done():
		log.Info("shutdown signal received")
	case err := <-serverErr:
		log.Error("server error", "error", err)
	}

	srv.Shutdown()

	log.Info("server stopped")
}
