package dashboard

import (
	"context"
	"log/slog"

	apperr "github.com/MauriceOmbewa/garisha-backend/internal/platform/errors"
	"github.com/MauriceOmbewa/garisha-backend/internal/platform/middleware"
	"github.com/MauriceOmbewa/garisha-backend/internal/platform/tenant"
)

// Service implements business logic for the dashboard.
type Service struct {
	repo *Repository
	log  *slog.Logger
}

// NewService creates a Service.
func NewService(repo *Repository, log *slog.Logger) *Service {
	return &Service{repo: repo, log: log}
}

// GetSummary returns all KPI card values for the current calendar month.
func (s *Service) GetSummary(ctx context.Context) (Summary, error) {
	tenantID := tenant.MustGetTenantID(ctx)

	// Extract the caller's user ID for the unread notification count.
	userID := ""
	if claims := middleware.GetClaims(ctx); claims != nil {
		userID = claims.UserID
	}

	summary, err := s.repo.GetSummary(ctx, tenantID, userID, tenant.GetBranchID(ctx))
	if err != nil {
		return Summary{}, apperr.Internal("failed to load dashboard summary", err)
	}

	return summary, nil
}

// GetCharts returns time-series and distribution datasets for dashboard charts.
func (s *Service) GetCharts(ctx context.Context) (ChartsData, error) {
	tenantID := tenant.MustGetTenantID(ctx)

	charts, err := s.repo.GetCharts(ctx, tenantID)
	if err != nil {
		return ChartsData{}, apperr.Internal("failed to load dashboard charts", err)
	}

	return charts, nil
}

// GetActivity returns the 20 most recent transactions across all modules.
func (s *Service) GetActivity(ctx context.Context) ([]Activity, error) {
	tenantID := tenant.MustGetTenantID(ctx)

	activity, err := s.repo.GetActivity(ctx, tenantID)
	if err != nil {
		return nil, apperr.Internal("failed to load recent activity", err)
	}

	if activity == nil {
		activity = []Activity{}
	}

	return activity, nil
}
