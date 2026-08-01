// Package tenant provides context helpers for propagating the resolved
// tenant throughout a request's lifetime.
//
// Flow:
//  1. ResolveTenant middleware loads the tenant from the DB and calls SetTenant.
//  2. EnforceTenantScope middleware cross-checks it against the JWT claims.
//  3. Every repository calls MustGetTenantID to scope its queries.
package tenant

import (
	"context"
	"errors"
	"time"
)

// Tenant is the lightweight representation of a platform tenant that is
// stored in the request context.  It is intentionally minimal — only the
// fields needed by middleware and repository scoping are included here.
// The full domain model lives in internal/tenants.
type Tenant struct {
	ID       string
	Slug     string
	Name     string
	Plan     string
	IsActive bool
}

// contextKey is unexported to prevent collisions with other packages.
type contextKey struct{}

// SetTenant stores t in ctx and returns the derived context.
func SetTenant(ctx context.Context, t *Tenant) context.Context {
	return context.WithValue(ctx, contextKey{}, t)
}

// GetTenant retrieves the tenant stored by SetTenant.
// Returns nil if no tenant is present in ctx.
func GetTenant(ctx context.Context) *Tenant {
	t, _ := ctx.Value(contextKey{}).(*Tenant)
	return t
}

// MustGetTenantID returns the tenant ID from ctx.
// Panics if no tenant is present — this indicates a programming error
// (a repository was called on a route that skipped ResolveTenant).
func MustGetTenantID(ctx context.Context) string {
	t := GetTenant(ctx)
	if t == nil {
		panic("tenant: MustGetTenantID called on context with no tenant — " +
			"ensure ResolveTenant middleware is applied to this route")
	}
	return t.ID
}

// BranchIDKey is the context key under which the optional branch filter is stored.
type branchKey struct{}

// SetBranchID stores an optional branch_id filter in ctx.
// Pass empty string to clear it (all-branches view).
func SetBranchID(ctx context.Context, branchID string) context.Context {
	return context.WithValue(ctx, branchKey{}, branchID)
}

// GetBranchID retrieves the optional branch_id filter from ctx.
// Returns empty string when no branch is selected (aggregate view).
func GetBranchID(ctx context.Context) string {
	id, _ := ctx.Value(branchKey{}).(string)
	return id
}

// ─── Tenant domain entity ─────────────────────────────────────────────────────
// The full entity (with all DB columns) lives here so both the middleware
// and the tenants domain module share the same type without a circular import.

// Record is the full tenant record as stored in the database.
type Record struct {
	ID         string
	Name       string
	Slug       string
	Email      string
	Phone      *string
	LogoURL    *string
	WebsiteURL *string
	Plan       string
	IsActive   bool
	CreatedAt  time.Time
	UpdatedAt  time.Time
}

// ErrNotFound is returned by the tenant resolver when no tenant matches
// the supplied identifier.
var ErrNotFound = errors.New("tenant not found")

// ErrInactive is returned when the matched tenant has is_active = false.
var ErrInactive = errors.New("tenant is suspended")
