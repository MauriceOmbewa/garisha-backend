package tenants

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"regexp"
	"strings"

	"github.com/jackc/pgx/v5"

	apperr "github.com/MauriceOmbewa/garisha-backend/internal/platform/errors"
	"github.com/MauriceOmbewa/garisha-backend/internal/platform/tenant"
)

// slugPattern allows only lowercase letters, digits, and hyphens.
var slugPattern = regexp.MustCompile(`^[a-z0-9-]+$`)

// Service implements business logic for tenant management.
type Service struct {
	repo *Repository
	log  *slog.Logger
}

// NewService creates a Service.
func NewService(repo *Repository, log *slog.Logger) *Service {
	return &Service{repo: repo, log: log}
}

// List returns every tenant on the platform. Super-admin only.
func (s *Service) List(ctx context.Context) ([]*tenant.Record, error) {
	records, err := s.repo.List(ctx)
	if err != nil {
		return nil, apperr.Internal("failed to list tenants", err)
	}
	return records, nil
}

// GetByID returns a single tenant by UUID.
func (s *Service) GetByID(ctx context.Context, id string) (*tenant.Record, error) {
	rec, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return nil, apperr.Internal("failed to get tenant", err)
	}
	if rec == nil {
		return nil, apperr.NotFound("tenant")
	}
	return rec, nil
}

// Create provisions a new tenant on the platform. Super-admin only.
func (s *Service) Create(ctx context.Context, p CreateParams) (*tenant.Record, error) {
	// Normalise slug.
	p.Slug = strings.ToLower(strings.TrimSpace(p.Slug))

	if !slugPattern.MatchString(p.Slug) {
		return nil, apperr.BadRequest("slug may only contain lowercase letters, digits, and hyphens")
	}

	if p.Plan == "" {
		p.Plan = "trial"
	}

	rec, err := s.repo.Create(ctx, p)
	if err != nil {
		if isDuplicateError(err) {
			return nil, apperr.Conflict("a tenant with this slug or email already exists")
		}
		return nil, apperr.Internal("failed to create tenant", err)
	}

	s.log.Info("tenant created", "tenant_id", rec.ID, "slug", rec.Slug)
	return rec, nil
}

// Update applies a partial update to a tenant.
func (s *Service) Update(ctx context.Context, id string, p UpdateParams) (*tenant.Record, error) {
	rec, err := s.repo.Update(ctx, id, p)
	if err != nil {
		if isDuplicateError(err) {
			return nil, apperr.Conflict("email is already used by another tenant")
		}
		return nil, apperr.Internal("failed to update tenant", err)
	}
	if rec == nil {
		return nil, apperr.NotFound("tenant")
	}

	s.log.Info("tenant updated", "tenant_id", id)
	return rec, nil
}

// Delete hard-deletes a tenant and all its cascaded data. Super-admin only.
func (s *Service) Delete(ctx context.Context, id string) error {
	if err := s.repo.Delete(ctx, id); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return apperr.NotFound("tenant")
		}
		return apperr.Internal("failed to delete tenant", err)
	}

	s.log.Info("tenant deleted", "tenant_id", id)
	return nil
}

// isDuplicateError detects PostgreSQL unique constraint violations.
func isDuplicateError(err error) bool {
	return err != nil && strings.Contains(fmt.Sprintf("%v", err), "23505")
}
