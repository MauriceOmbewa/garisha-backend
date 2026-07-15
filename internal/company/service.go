package company

import (
	"context"
	"log/slog"

	apperr "github.com/MauriceOmbewa/garisha-backend/internal/platform/errors"
	"github.com/MauriceOmbewa/garisha-backend/internal/platform/tenant"
)

// Service implements business logic for company profile management.
type Service struct {
	repo *Repository
	log  *slog.Logger
}

// NewService creates a Service.
func NewService(repo *Repository, log *slog.Logger) *Service {
	return &Service{repo: repo, log: log}
}

// Get returns the company profile for the tenant in ctx.
// If no profile has been saved yet it returns a default empty profile so
// the frontend always receives a consistent shape.
func (s *Service) Get(ctx context.Context) (*Profile, error) {
	tenantID := tenant.MustGetTenantID(ctx)

	profile, err := s.repo.FindByTenantID(ctx, tenantID)
	if err != nil {
		return nil, apperr.Internal("failed to retrieve company profile", err)
	}

	// First-time access — return a zeroed profile so callers don't have to
	// handle nil.  The profile will be persisted on first Update call.
	if profile == nil {
		profile = &Profile{
			TenantID:       tenantID,
			SocialLinks:    map[string]string{},
			OperatingHours: map[string]DaySchedule{},
			Currency:       "KES",
			Timezone:       "Africa/Nairobi",
		}
	}

	return profile, nil
}

// Update creates or updates the company profile for the tenant in ctx.
func (s *Service) Update(ctx context.Context, p UpdateInput) (*Profile, error) {
	tenantID := tenant.MustGetTenantID(ctx)

	// Apply defaults for required locale fields if not provided.
	currency := "KES"
	if p.Currency != nil && *p.Currency != "" {
		currency = *p.Currency
	}

	timezone := "Africa/Nairobi"
	if p.Timezone != nil && *p.Timezone != "" {
		timezone = *p.Timezone
	}

	socialLinks := p.SocialLinks
	if socialLinks == nil {
		socialLinks = map[string]string{}
	}

	operatingHours := p.OperatingHours
	if operatingHours == nil {
		operatingHours = map[string]DaySchedule{}
	}

	params := UpsertParams{
		TenantID:       tenantID,
		LegalName:      p.LegalName,
		BusinessType:   p.BusinessType,
		RegistrationNo: p.RegistrationNo,
		TaxPIN:         p.TaxPIN,
		Description:    p.Description,
		Country:        p.Country,
		City:           p.City,
		AddressLine1:   p.AddressLine1,
		AddressLine2:   p.AddressLine2,
		PostalCode:     p.PostalCode,
		SupportEmail:   p.SupportEmail,
		SupportPhone:   p.SupportPhone,
		WhatsApp:       p.WhatsApp,
		SocialLinks:    socialLinks,
		PrimaryColor:   p.PrimaryColor,
		SecondaryColor: p.SecondaryColor,
		FontFamily:     p.FontFamily,
		OperatingHours: operatingHours,
		EnableHire:     p.EnableHire,
		EnableSales:    p.EnableSales,
		EnableService:  p.EnableService,
		Currency:       currency,
		Timezone:       timezone,
	}

	profile, err := s.repo.Upsert(ctx, params)
	if err != nil {
		return nil, apperr.Internal("failed to update company profile", err)
	}

	s.log.Info("company profile updated", "tenant_id", tenantID)
	return profile, nil
}

// UpdateInput carries the fields a business admin may change.
// All fields are optional — only non-nil values are applied.
type UpdateInput struct {
	LegalName      *string
	BusinessType   *string
	RegistrationNo *string
	TaxPIN         *string
	Description    *string
	Country        *string
	City           *string
	AddressLine1   *string
	AddressLine2   *string
	PostalCode     *string
	SupportEmail   *string
	SupportPhone   *string
	WhatsApp       *string
	SocialLinks    map[string]string
	PrimaryColor   *string
	SecondaryColor *string
	FontFamily     *string
	OperatingHours map[string]DaySchedule
	EnableHire     bool
	EnableSales    bool
	EnableService  bool
	Currency       *string
	Timezone       *string
}
