package customers

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"github.com/jackc/pgx/v5"

	apperr "github.com/MauriceOmbewa/garisha-backend/internal/platform/errors"
	"github.com/MauriceOmbewa/garisha-backend/internal/platform/tenant"
)

// Service implements business logic for customer management.
type Service struct {
	repo *Repository
	log  *slog.Logger
}

// NewService creates a Service.
func NewService(repo *Repository, log *slog.Logger) *Service {
	return &Service{repo: repo, log: log}
}

// ── Input types ───────────────────────────────────────────────────────────────

// CreateInput carries the fields a caller provides when creating a customer.
type CreateInput struct {
	UserID   *string
	FullName string
	Email    *string
	Phone    *string
	IDNumber *string
	IDType   *string
	Country  *string
	City     *string
	Address  *string
	Notes    *string
}

// UpdateInput carries all mutable fields for a partial update.
// nil pointer means "leave the existing value unchanged".
type UpdateInput struct {
	FullName *string
	Email    *string
	Phone    *string
	IDNumber *string
	IDType   *string
	Country  *string
	City     *string
	Address  *string
	IsActive *bool
	Notes    *string
}

// ── Service methods ───────────────────────────────────────────────────────────

// List returns all customers for the tenant in ctx.
func (s *Service) List(ctx context.Context, f ListFilters) ([]*Customer, error) {
	tenantID := tenant.MustGetTenantID(ctx)

	customers, err := s.repo.List(ctx, tenantID, f)
	if err != nil {
		return nil, apperr.Internal("failed to list customers", err)
	}

	return customers, nil
}

// GetByID returns a single customer scoped to the tenant in ctx.
func (s *Service) GetByID(ctx context.Context, id string) (*Customer, error) {
	tenantID := tenant.MustGetTenantID(ctx)

	c, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return nil, apperr.Internal("failed to get customer", err)
	}

	if c == nil || c.TenantID != tenantID {
		return nil, apperr.NotFound("customer")
	}

	return c, nil
}

// Create adds a new customer profile to the tenant.
func (s *Service) Create(ctx context.Context, in CreateInput) (*Customer, error) {
	tenantID := tenant.MustGetTenantID(ctx)

	if err := validateIDType(in.IDType); err != nil {
		return nil, err
	}

	// Guard against duplicate email within the same tenant.
	if in.Email != nil && *in.Email != "" {
		existing, err := s.repo.FindByEmail(ctx, tenantID, *in.Email)
		if err != nil {
			return nil, apperr.Internal("failed to check for duplicate email", err)
		}
		if existing != nil {
			return nil, apperr.Conflict("a customer with this email already exists")
		}
	}

	c, err := s.repo.Create(ctx, CreateParams{
		TenantID: tenantID,
		UserID:   in.UserID,
		FullName: in.FullName,
		Email:    in.Email,
		Phone:    in.Phone,
		IDNumber: in.IDNumber,
		IDType:   in.IDType,
		Country:  in.Country,
		City:     in.City,
		Address:  in.Address,
		Notes:    in.Notes,
	})
	if err != nil {
		if isDuplicateError(err) {
			return nil, apperr.Conflict("a customer with this email already exists")
		}
		return nil, apperr.Internal("failed to create customer", err)
	}

	s.log.Info("customer created",
		"customer_id", c.ID,
		"tenant_id",   tenantID,
	)

	return c, nil
}

// Update applies a partial update to a customer owned by the tenant in ctx.
func (s *Service) Update(ctx context.Context, id string, in UpdateInput) (*Customer, error) {
	tenantID := tenant.MustGetTenantID(ctx)

	if err := validateIDType(in.IDType); err != nil {
		return nil, err
	}

	existing, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return nil, apperr.Internal("failed to get customer", err)
	}

	if existing == nil || existing.TenantID != tenantID {
		return nil, apperr.NotFound("customer")
	}

	// If changing the email, ensure it's not already taken by another customer.
	if in.Email != nil && *in.Email != "" {
		dup, err := s.repo.FindByEmail(ctx, tenantID, *in.Email)
		if err != nil {
			return nil, apperr.Internal("failed to check for duplicate email", err)
		}
		if dup != nil && dup.ID != id {
			return nil, apperr.Conflict("a customer with this email already exists")
		}
	}

	c, err := s.repo.Update(ctx, id, UpdateParams{
		FullName: in.FullName,
		Email:    in.Email,
		Phone:    in.Phone,
		IDNumber: in.IDNumber,
		IDType:   in.IDType,
		Country:  in.Country,
		City:     in.City,
		Address:  in.Address,
		IsActive: in.IsActive,
		Notes:    in.Notes,
	})
	if err != nil {
		if isDuplicateError(err) {
			return nil, apperr.Conflict("a customer with this email already exists")
		}
		return nil, apperr.Internal("failed to update customer", err)
	}

	if c == nil {
		return nil, apperr.NotFound("customer")
	}

	s.log.Info("customer updated", "customer_id", id, "tenant_id", tenantID)
	return c, nil
}

// Delete hard-deletes a customer.  Prefer deactivation via Update for
// customers with existing booking / sale / service history.
func (s *Service) Delete(ctx context.Context, id string) error {
	tenantID := tenant.MustGetTenantID(ctx)

	existing, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return apperr.Internal("failed to get customer", err)
	}

	if existing == nil || existing.TenantID != tenantID {
		return apperr.NotFound("customer")
	}

	if err := s.repo.Delete(ctx, id); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return apperr.NotFound("customer")
		}
		return apperr.Internal("failed to delete customer", err)
	}

	s.log.Info("customer deleted", "customer_id", id, "tenant_id", tenantID)
	return nil
}

// ── Validator ─────────────────────────────────────────────────────────────────

var validIDTypes = map[string]struct{}{
	string(IDTypeNationalID):     {},
	string(IDTypePassport):       {},
	string(IDTypeDrivingLicense): {},
	string(IDTypeOther):          {},
}

func validateIDType(t *string) error {
	if t == nil || *t == "" {
		return nil // optional field
	}
	if _, ok := validIDTypes[*t]; !ok {
		return apperr.BadRequest(fmt.Sprintf(
			"invalid id_type %q — must be one of: national_id, passport, driving_license, other", *t,
		))
	}
	return nil
}
