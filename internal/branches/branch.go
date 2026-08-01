// Package branches manages the physical locations (branches / yards) that
// belong to a tenant.  Every core data table carries an optional branch_id
// so records can be scoped to a specific location.
package branches

import "time"

// Branch is a physical location owned by a tenant.
type Branch struct {
	ID        string
	TenantID  string
	Name      string
	Slug      string
	City      *string
	Address   *string
	Phone     *string
	Email     *string
	IsActive  bool
	IsDefault bool
	CreatedAt time.Time
	UpdatedAt time.Time
}
