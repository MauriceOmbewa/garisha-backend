// Package company is the domain module for business profile management.
// Each tenant owns exactly one company profile, created on first access
// and updated by the business admin through the settings surface.
package company

import "time"

// Profile is the full company profile entity.
type Profile struct {
	ID       string
	TenantID string

	// Business identity
	LegalName      *string
	BusinessType   *string
	RegistrationNo *string
	TaxPIN         *string
	Description    *string

	// Address
	Country      *string
	City         *string
	AddressLine1 *string
	AddressLine2 *string
	PostalCode   *string

	// Contact
	SupportEmail *string
	SupportPhone *string
	WhatsApp     *string

	// Social links: map of platform → URL, e.g. {"facebook":"https://..."}
	SocialLinks map[string]string

	// Branding
	PrimaryColor   *string
	SecondaryColor *string
	FontFamily     *string

	// Visual branding for customer portal
	Tagline       *string
	LogoURL       *string // stored here for company-profile-driven branding
	FaviconURL    *string
	HeroImageURL  *string
	HeroEyebrow   *string
	CoverImageURL *string

	// Operating hours: map of weekday → DaySchedule
	OperatingHours map[string]DaySchedule

	// Module flags
	EnableHire    bool
	EnableSales   bool
	EnableService bool

	// Locale
	Currency string
	Timezone string

	CreatedAt time.Time
	UpdatedAt time.Time
}

// DaySchedule describes opening hours for a single day.
type DaySchedule struct {
	Open   string `json:"open"`   // e.g. "08:00"
	Close  string `json:"close"`  // e.g. "17:00"
	Closed bool   `json:"closed"` // true = closed all day
}

// BusinessType enumerates valid business type values.
type BusinessType string

const (
	BusinessTypeCarYard       BusinessType = "car_yard"
	BusinessTypeDealership    BusinessType = "dealership"
	BusinessTypeRental        BusinessType = "rental"
	BusinessTypeServiceCenter BusinessType = "service_center"
	BusinessTypeMixed         BusinessType = "mixed"
)
