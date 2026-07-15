// Package customers is the domain module for customer profile management.
// Customers are tenant-scoped contacts that appear across hire bookings,
// vehicle sales, and service jobs.  They may optionally be linked to a
// platform user account (for self-service portals), but walk-in customers
// created by staff have no login.
package customers

import "time"

// Customer is the full customer entity.
type Customer struct {
	ID       string
	TenantID string

	// Optional link to a platform user account.
	UserID *string

	// Identity
	FullName string
	Email    *string
	Phone    *string
	IDNumber *string // national ID / passport number
	IDType   *string // 'national_id' | 'passport' | 'driving_license' | 'other'

	// Address
	Country *string
	City    *string
	Address *string

	// Lifecycle
	IsActive bool

	// Free-form notes
	Notes *string

	CreatedAt time.Time
	UpdatedAt time.Time
}

// IDType enumerates the valid identification document types.
type IDType string

const (
	IDTypeNationalID      IDType = "national_id"
	IDTypePassport        IDType = "passport"
	IDTypeDrivingLicense  IDType = "driving_license"
	IDTypeOther           IDType = "other"
)
