package users

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"github.com/jackc/pgx/v5"

	apperr "github.com/MauriceOmbewa/garisha-backend/internal/platform/errors"
	"github.com/MauriceOmbewa/garisha-backend/internal/platform/rbac"
	"github.com/MauriceOmbewa/garisha-backend/internal/platform/tenant"
)

// Service implements business logic for tenant-scoped user management.
type Service struct {
	repo *Repository
	log  *slog.Logger
}

// NewService creates a Service.
func NewService(repo *Repository, log *slog.Logger) *Service {
	return &Service{repo: repo, log: log}
}

// List returns all users belonging to the tenant in ctx.
func (s *Service) List(ctx context.Context) ([]*User, error) {
	tenantID := tenant.MustGetTenantID(ctx)

	users, err := s.repo.List(ctx, tenantID)
	if err != nil {
		return nil, apperr.Internal("failed to list users", err)
	}

	return users, nil
}

// GetByID returns a single user, scoped to the tenant in ctx.
// Returns NotFound if the user does not exist or belongs to a different tenant.
func (s *Service) GetByID(ctx context.Context, id string) (*User, error) {
	tenantID := tenant.MustGetTenantID(ctx)

	u, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return nil, apperr.Internal("failed to get user", err)
	}

	if u == nil || !belongsToTenant(u, tenantID) {
		return nil, apperr.NotFound("user")
	}

	return u, nil
}

// AssignRole changes a user's role.  The caller must hold PermUserUpdate.
// Super-admin role cannot be assigned through this endpoint — that would
// break the single-tenant isolation model.
func (s *Service) AssignRole(ctx context.Context, id, role string) (*User, error) {
	tenantID := tenant.MustGetTenantID(ctx)

	if !rbac.IsValidRole(role) {
		return nil, apperr.BadRequest(fmt.Sprintf("invalid role: %q", role))
	}

	if rbac.Role(role) == rbac.RoleSuperAdmin {
		return nil, apperr.Forbidden("super_admin role cannot be assigned through this endpoint")
	}

	// Verify the user belongs to this tenant before modifying.
	existing, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return nil, apperr.Internal("failed to get user", err)
	}

	if existing == nil || !belongsToTenant(existing, tenantID) {
		return nil, apperr.NotFound("user")
	}

	u, err := s.repo.UpdateRole(ctx, id, role)
	if err != nil {
		return nil, apperr.Internal("failed to update user role", err)
	}

	s.log.Info("user role updated",
		"user_id",   id,
		"role",      role,
		"tenant_id", tenantID,
	)

	return u, nil
}

// Activate enables a previously suspended user account.
func (s *Service) Activate(ctx context.Context, id string) (*User, error) {
	return s.setActive(ctx, id, true)
}

// Suspend disables a user account.  The user will be denied login until
// reactivated.
func (s *Service) Suspend(ctx context.Context, id string) (*User, error) {
	return s.setActive(ctx, id, false)
}

// UpdatePermissions replaces the per-user permission overrides.
// Pass an empty slice to clear all overrides and rely solely on role defaults.
func (s *Service) UpdatePermissions(ctx context.Context, id string, perms []string) (*User, error) {
	tenantID := tenant.MustGetTenantID(ctx)

	// Validate that every permission string is a known permission.
	for _, p := range perms {
		if !isKnownPermission(p) {
			return nil, apperr.BadRequest(fmt.Sprintf("unknown permission: %q", p))
		}
	}

	existing, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return nil, apperr.Internal("failed to get user", err)
	}

	if existing == nil || !belongsToTenant(existing, tenantID) {
		return nil, apperr.NotFound("user")
	}

	if perms == nil {
		perms = []string{}
	}

	u, err := s.repo.UpdatePermissions(ctx, id, perms)
	if err != nil {
		return nil, apperr.Internal("failed to update user permissions", err)
	}

	s.log.Info("user permissions updated",
		"user_id",      id,
		"permissions",  perms,
		"tenant_id",    tenantID,
	)

	return u, nil
}

// Delete hard-deletes a user.  This is an admin-level operation and should
// be used with care — prefer Suspend for reversible deactivation.
func (s *Service) Delete(ctx context.Context, id string) error {
	tenantID := tenant.MustGetTenantID(ctx)

	existing, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return apperr.Internal("failed to get user", err)
	}

	if existing == nil || !belongsToTenant(existing, tenantID) {
		return apperr.NotFound("user")
	}

	if err := s.repo.Delete(ctx, id); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return apperr.NotFound("user")
		}
		return apperr.Internal("failed to delete user", err)
	}

	s.log.Info("user deleted", "user_id", id, "tenant_id", tenantID)
	return nil
}

// ── Internal helpers ─────────────────────────────────────────────────────────

// setActive is the shared implementation for Activate and Suspend.
func (s *Service) setActive(ctx context.Context, id string, active bool) (*User, error) {
	tenantID := tenant.MustGetTenantID(ctx)

	existing, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return nil, apperr.Internal("failed to get user", err)
	}

	if existing == nil || !belongsToTenant(existing, tenantID) {
		return nil, apperr.NotFound("user")
	}

	u, err := s.repo.SetActive(ctx, id, active)
	if err != nil {
		return nil, apperr.Internal("failed to update user status", err)
	}

	action := "activated"
	if !active {
		action = "suspended"
	}

	s.log.Info("user "+action, "user_id", id, "tenant_id", tenantID)
	return u, nil
}

// belongsToTenant checks that u.TenantID matches tenantID.
func belongsToTenant(u *User, tenantID string) bool {
	return u.TenantID != nil && *u.TenantID == tenantID
}

// isKnownPermission validates a permission string against the rbac package.
func isKnownPermission(p string) bool {
	// PermissionsFor(admin) returns all known permissions since admin has them all.
	// We cross-check our string against every permission set instead.
	all := rbac.PermissionsFor(rbac.RoleAdmin)
	// Admin doesn't get super_admin-only perms, so also include those.
	all = append(all, rbac.PermTenantCreate, rbac.PermTenantUpdate, rbac.PermTenantDelete, rbac.PermTenantView)

	for _, perm := range all {
		if string(perm) == p {
			return true
		}
	}
	return false
}
