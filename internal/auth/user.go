// Package auth is the domain module responsible for authentication and
// session management.  It owns the User entity and the login flow.
package auth

import (
	"time"
)

// User represents an authenticated platform user.
type User struct {
	ID        string
	TenantID  *string // nil for super-admins
	GoogleSub string
	Email     string
	Name      string
	AvatarURL *string
	Role      string
	IsActive  bool
	CreatedAt time.Time
	UpdatedAt time.Time
}
