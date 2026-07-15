package reports

import (
	"context"
	"log/slog"

	apperr "github.com/MauriceOmbewa/garisha-backend/internal/platform/errors"
	"github.com/MauriceOmbewa/garisha-backend/internal/platform/tenant"
)

// Service implements business logic for reports.
type Service struct {
	repo *Repository
	log  *slog.Logger
}

// NewService creates a Service.
func NewService(repo *Repository, log *slog.Logger) *Service {
	return &Service{repo: repo, log: log}
}

func (s *Service) HireSummary(ctx context.Context, dr DateRange) (HireSummary, error) {
	tenantID := tenant.MustGetTenantID(ctx)

	summary, err := s.repo.HireSummary(ctx, tenantID, dr)
	if err != nil {
		return HireSummary{}, apperr.Internal("failed to generate hire report", err)
	}

	return summary, nil
}

func (s *Service) SalesSummary(ctx context.Context, dr DateRange) (SalesSummary, error) {
	tenantID := tenant.MustGetTenantID(ctx)

	summary, err := s.repo.SalesSummary(ctx, tenantID, dr)
	if err != nil {
		return SalesSummary{}, apperr.Internal("failed to generate sales report", err)
	}

	return summary, nil
}

func (s *Service) ServiceSummary(ctx context.Context, dr DateRange) (ServiceSummary, error) {
	tenantID := tenant.MustGetTenantID(ctx)

	summary, err := s.repo.ServiceSummary(ctx, tenantID, dr)
	if err != nil {
		return ServiceSummary{}, apperr.Internal("failed to generate service report", err)
	}

	return summary, nil
}

func (s *Service) FinanceSummary(ctx context.Context, dr DateRange) (FinanceSummary, error) {
	tenantID := tenant.MustGetTenantID(ctx)

	summary, err := s.repo.FinanceSummary(ctx, tenantID, dr)
	if err != nil {
		return FinanceSummary{}, apperr.Internal("failed to generate finance report", err)
	}

	return summary, nil
}

func (s *Service) PaymentSummary(ctx context.Context, dr DateRange) (PaymentSummary, error) {
	tenantID := tenant.MustGetTenantID(ctx)

	summary, err := s.repo.PaymentSummary(ctx, tenantID, dr)
	if err != nil {
		return PaymentSummary{}, apperr.Internal("failed to generate payment report", err)
	}

	return summary, nil
}
