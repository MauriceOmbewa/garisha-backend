// Package users is the domain module for tenant-scoped user management.
// It handles listing, role assignment, activation/suspension, and permission
// overrides for users within a tenant.
//
// Authentication (login / token issuance) lives in internal/auth, which owns
// the login flow.  This module owns everything that happens after a user
// exists: who can see them, what role they carry, and whether they're active.
package users

import "time"

// User is the full user entity as managed by this domain.
// It mirrors the auth.User struct intentionally — both map to the same DB
// row. Having a local type avoids a cross-package import and lets us add
// user-management-specific methods here without polluting the auth package.
type User struct {
	ID          string
	TenantID    *string  // nil only for super-admins
	GoogleSub   string
	Email       string
	Name        string
	AvatarURL   *string
	Role        string
	Permissions []string // per-user permission overrides on top of role defaults
	IsActive    bool
	CreatedAt   time.Time
	UpdatedAt   time.Time
}
