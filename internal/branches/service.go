package branches

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

var slugRe = regexp.MustCompile(`^[a-z0-9-]+$`)

// Service implements business logic for branches.
type Service struct {
	repo *Repository
	log  *slog.Logger
}

// NewService creates a Service.
func NewService(repo *Repository, log *slog.Logger) *Service {
	return &Service{repo: repo, log: log}
}

// List returns all branches for the tenant in ctx.
func (s *Service) List(ctx context.Context) ([]*Branch, error) {
	tenantID := tenant.MustGetTenantID(ctx)
	branches, err := s.repo.ListByTenant(ctx, tenantID)
	if err != nil {
		return nil, apperr.Internal("failed to list branches", err)
	}
	return branches, nil
}

// Get returns a single branch — must belong to the tenant in ctx.
func (s *Service) Get(ctx context.Context, id string) (*Branch, error) {
	tenantID := tenant.MustGetTenantID(ctx)
	b, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return nil, apperr.Internal("failed to get branch", err)
	}
	if b == nil || b.TenantID != tenantID {
		return nil, apperr.NotFound("branch")
	}
	return b, nil
}

// CreateInput holds the fields a user provides when creating a branch.
type CreateInput struct {
	Name      string
	Slug      string
	City      *string
	Address   *string
	Phone     *string
	Email     *string
	IsDefault bool
}

// Create adds a new branch to the tenant in ctx.
func (s *Service) Create(ctx context.Context, in CreateInput) (*Branch, error) {
	tenantID := tenant.MustGetTenantID(ctx)

	// Normalise and validate slug.
	in.Slug = strings.ToLower(strings.TrimSpace(in.Slug))
	if in.Slug == "" {
		in.Slug = toSlug(in.Name)
	}
	if !slugRe.MatchString(in.Slug) {
		return nil, apperr.BadRequest("slug may only contain lowercase letters, digits, and hyphens")
	}

	b, err := s.repo.Create(ctx, CreateParams{
		TenantID:  tenantID,
		Name:      in.Name,
		Slug:      in.Slug,
		City:      in.City,
		Address:   in.Address,
		Phone:     in.Phone,
		Email:     in.Email,
		IsDefault: in.IsDefault,
	})
	if err != nil {
		if isDup(err) {
			return nil, apperr.Conflict("a branch with this slug already exists")
		}
		return nil, apperr.Internal("failed to create branch", err)
	}

	s.log.Info("branch created", "tenant_id", tenantID, "branch_id", b.ID, "name", b.Name)
	return b, nil
}

// UpdateInput holds nullable update fields.
type UpdateInput struct {
	Name      *string
	City      *string
	Address   *string
	Phone     *string
	Email     *string
	IsActive  *bool
	IsDefault *bool
}

// Update modifies a branch — must belong to the tenant in ctx.
func (s *Service) Update(ctx context.Context, id string, in UpdateInput) (*Branch, error) {
	tenantID := tenant.MustGetTenantID(ctx)

	existing, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return nil, apperr.Internal("failed to get branch", err)
	}
	if existing == nil || existing.TenantID != tenantID {
		return nil, apperr.NotFound("branch")
	}

	b, err := s.repo.Update(ctx, id, UpdateParams{
		Name:      in.Name,
		City:      in.City,
		Address:   in.Address,
		Phone:     in.Phone,
		Email:     in.Email,
		IsActive:  in.IsActive,
		IsDefault: in.IsDefault,
	})
	if err != nil {
		return nil, apperr.Internal("failed to update branch", err)
	}
	if b == nil {
		return nil, apperr.NotFound("branch")
	}
	return b, nil
}

// Delete removes a branch — must belong to the tenant in ctx.
// Records that referenced this branch will have branch_id set to NULL.
func (s *Service) Delete(ctx context.Context, id string) error {
	tenantID := tenant.MustGetTenantID(ctx)

	existing, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return apperr.Internal("failed to get branch", err)
	}
	if existing == nil || existing.TenantID != tenantID {
		return apperr.NotFound("branch")
	}

	if err := s.repo.Delete(ctx, id); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return apperr.NotFound("branch")
		}
		return apperr.Internal("failed to delete branch", err)
	}

	s.log.Info("branch deleted", "tenant_id", tenantID, "branch_id", id)
	return nil
}

// ── helpers ───────────────────────────────────────────────────────────────────

func toSlug(name string) string {
	s := strings.ToLower(strings.TrimSpace(name))
	s = regexp.MustCompile(`[^a-z0-9]+`).ReplaceAllString(s, "-")
	s = strings.Trim(s, "-")
	if len(s) > 60 {
		s = s[:60]
	}
	return s
}

func isDup(err error) bool {
	return err != nil && strings.Contains(fmt.Sprintf("%v", err), "23505")
}
