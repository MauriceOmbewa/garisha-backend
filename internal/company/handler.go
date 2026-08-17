package company

import (
	"log/slog"
	"net/http"

	apperr "github.com/MauriceOmbewa/garisha-backend/internal/platform/errors"
	"github.com/MauriceOmbewa/garisha-backend/internal/platform/response"
	"github.com/MauriceOmbewa/garisha-backend/internal/platform/tenant"
	"github.com/MauriceOmbewa/garisha-backend/internal/platform/validation"
)

// Handler holds the HTTP handlers for the company domain.
type Handler struct {
	svc *Service
	log *slog.Logger
}

// NewHandler creates a Handler.
func NewHandler(svc *Service, log *slog.Logger) *Handler {
	return &Handler{svc: svc, log: log}
}

// ─── Request DTOs ─────────────────────────────────────────────────────────────

// dayScheduleInput mirrors DaySchedule for JSON binding.
type dayScheduleInput struct {
	Open   string `json:"open"`
	Close  string `json:"close"`
	Closed bool   `json:"closed"`
}

type updateProfileRequest struct {
	LegalName      *string `json:"legal_name"`
	BusinessType   *string `json:"business_type"    validate:"omitempty,oneof=car_yard dealership rental service_center mixed"`
	RegistrationNo *string `json:"registration_no"`
	TaxPIN         *string `json:"tax_pin"`
	Description    *string `json:"description"      validate:"omitempty,max=2000"`
	Tagline        *string `json:"tagline"          validate:"omitempty,max=120"`
	LogoURL        *string `json:"logo_url"`
	HeroImageURL   *string `json:"hero_image_url"`
	HeroEyebrow    *string `json:"hero_eyebrow"     validate:"omitempty,max=80"`

	Country      *string `json:"country"`
	City         *string `json:"city"`
	AddressLine1 *string `json:"address_line1"`
	AddressLine2 *string `json:"address_line2"`
	PostalCode   *string `json:"postal_code"`

	SupportEmail *string `json:"support_email" validate:"omitempty,email"`
	SupportPhone *string `json:"support_phone"`
	WhatsApp     *string `json:"whatsapp"`

	SocialLinks map[string]string `json:"social_links"`

	PrimaryColor   *string `json:"primary_color"`
	SecondaryColor *string `json:"secondary_color"`
	FontFamily     *string `json:"font_family"`

	OperatingHours map[string]dayScheduleInput `json:"operating_hours"`

	EnableHire    bool `json:"enable_hire"`
	EnableSales   bool `json:"enable_sales"`
	EnableService bool `json:"enable_service"`

	Currency *string `json:"currency" validate:"omitempty,len=3"`
	Timezone *string `json:"timezone"`
}

// ─── Response DTO ─────────────────────────────────────────────────────────────

type profileDTO struct {
	ID       string `json:"id,omitempty"`
	TenantID string `json:"tenant_id"`

	// Tenant identity — included so the admin frontend can construct the
	// customer portal URL without needing a separate /auth/me round-trip.
	TenantSlug string `json:"tenant_slug"`
	TenantName string `json:"tenant_name"`

	LegalName      *string `json:"legal_name"`
	BusinessType   *string `json:"business_type"`
	RegistrationNo *string `json:"registration_no"`
	TaxPIN         *string `json:"tax_pin"`
	Description    *string `json:"description"`
	Tagline        *string `json:"tagline"`
	LogoURL        *string `json:"logo_url"`
	HeroImageURL   *string `json:"hero_image_url"`
	HeroEyebrow    *string `json:"hero_eyebrow"`

	Country      *string `json:"country"`
	City         *string `json:"city"`
	AddressLine1 *string `json:"address_line1"`
	AddressLine2 *string `json:"address_line2"`
	PostalCode   *string `json:"postal_code"`

	SupportEmail *string `json:"support_email"`
	SupportPhone *string `json:"support_phone"`
	WhatsApp     *string `json:"whatsapp"`

	SocialLinks map[string]string `json:"social_links"`

	PrimaryColor   *string `json:"primary_color"`
	SecondaryColor *string `json:"secondary_color"`
	FontFamily     *string `json:"font_family"`

	OperatingHours map[string]DaySchedule `json:"operating_hours"`

	EnableHire    bool `json:"enable_hire"`
	EnableSales   bool `json:"enable_sales"`
	EnableService bool `json:"enable_service"`

	Currency  string `json:"currency"`
	Timezone  string `json:"timezone"`
	UpdatedAt string `json:"updated_at,omitempty"`
}

// ─── Handlers ─────────────────────────────────────────────────────────────────

// GetProfile godoc
// GET /api/v1/company/profile
// Returns the company profile for the resolved tenant.
// Accessible to any authenticated tenant member.
func (h *Handler) GetProfile(w http.ResponseWriter, r *http.Request) {
	profile, err := h.svc.Get(r.Context())
	if err != nil {
		apperr.Handle(w, r, err, h.log)
		return
	}

	t := tenant.GetTenant(r.Context())
	response.Success(w, http.StatusOK, "company profile retrieved", toDTO(profile, t), h.log)
}

// UpdateProfile godoc
// PUT /api/v1/company/profile
// Creates or updates the company profile. Admin only.
func (h *Handler) UpdateProfile(w http.ResponseWriter, r *http.Request) {
	var req updateProfileRequest
	if err := validation.DecodeJSON(r, &req); err != nil {
		apperr.Handle(w, r, err, h.log)
		return
	}

	// Convert operating hours input to domain type.
	var operatingHours map[string]DaySchedule
	if len(req.OperatingHours) > 0 {
		operatingHours = make(map[string]DaySchedule, len(req.OperatingHours))
		for day, s := range req.OperatingHours {
			operatingHours[day] = DaySchedule{
				Open:   s.Open,
				Close:  s.Close,
				Closed: s.Closed,
			}
		}
	}

	profile, err := h.svc.Update(r.Context(), UpdateInput{
		LegalName:      req.LegalName,
		BusinessType:   req.BusinessType,
		RegistrationNo: req.RegistrationNo,
		TaxPIN:         req.TaxPIN,
		Description:    req.Description,
		Tagline:        req.Tagline,
		LogoURL:        req.LogoURL,
		HeroImageURL:   req.HeroImageURL,
		HeroEyebrow:    req.HeroEyebrow,
		Country:        req.Country,
		City:           req.City,
		AddressLine1:   req.AddressLine1,
		AddressLine2:   req.AddressLine2,
		PostalCode:     req.PostalCode,
		SupportEmail:   req.SupportEmail,
		SupportPhone:   req.SupportPhone,
		WhatsApp:       req.WhatsApp,
		SocialLinks:    req.SocialLinks,
		PrimaryColor:   req.PrimaryColor,
		SecondaryColor: req.SecondaryColor,
		FontFamily:     req.FontFamily,
		OperatingHours: operatingHours,
		EnableHire:     req.EnableHire,
		EnableSales:    req.EnableSales,
		EnableService:  req.EnableService,
		Currency:       req.Currency,
		Timezone:       req.Timezone,
	})
	if err != nil {
		apperr.Handle(w, r, err, h.log)
		return
	}

	response.Success(w, http.StatusOK, "company profile updated", toDTO(profile, tenant.GetTenant(r.Context())), h.log)
}

// ─── Helpers ──────────────────────────────────────────────────────────────────

func toDTO(p *Profile, t *tenant.Tenant) profileDTO {
	dto := profileDTO{
		ID:             p.ID,
		TenantID:       p.TenantID,
		LegalName:      p.LegalName,
		BusinessType:   p.BusinessType,
		RegistrationNo: p.RegistrationNo,
		TaxPIN:         p.TaxPIN,
		Description:    p.Description,
		Tagline:        p.Tagline,
		LogoURL:        p.LogoURL,
		HeroImageURL:   p.HeroImageURL,
		HeroEyebrow:    p.HeroEyebrow,
		Country:        p.Country,
		City:           p.City,
		AddressLine1:   p.AddressLine1,
		AddressLine2:   p.AddressLine2,
		PostalCode:     p.PostalCode,
		SupportEmail:   p.SupportEmail,
		SupportPhone:   p.SupportPhone,
		WhatsApp:       p.WhatsApp,
		SocialLinks:    p.SocialLinks,
		PrimaryColor:   p.PrimaryColor,
		SecondaryColor: p.SecondaryColor,
		FontFamily:     p.FontFamily,
		OperatingHours: p.OperatingHours,
		EnableHire:     p.EnableHire,
		EnableSales:    p.EnableSales,
		EnableService:  p.EnableService,
		Currency:       p.Currency,
		Timezone:       p.Timezone,
	}

	// Enrich with tenant identity from the request context so the admin
	// frontend can build the customer portal URL without a second API call.
	if t != nil {
		dto.TenantSlug = t.Slug
		dto.TenantName = t.Name
	}

	if !p.UpdatedAt.IsZero() {
		dto.UpdatedAt = p.UpdatedAt.UTC().Format("2006-01-02T15:04:05Z")
	}

	// Ensure maps are never null in the response.
	if dto.SocialLinks == nil {
		dto.SocialLinks = map[string]string{}
	}
	if dto.OperatingHours == nil {
		dto.OperatingHours = map[string]DaySchedule{}
	}

	return dto
}
