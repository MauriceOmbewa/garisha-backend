package company

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Repository handles all database operations for the company domain.
type Repository struct {
	db *pgxpool.Pool
}

// NewRepository creates a Repository backed by the given pool.
func NewRepository(db *pgxpool.Pool) *Repository {
	return &Repository{db: db}
}

// FindByTenantID returns the profile for tenantID, or nil if none exists yet.
func (r *Repository) FindByTenantID(ctx context.Context, tenantID string) (*Profile, error) {
	const q = `
		SELECT id, tenant_id,
		       legal_name, business_type, registration_no, tax_pin, description,
		       country, city, address_line1, address_line2, postal_code,
		       support_email, support_phone, whatsapp,
		       social_links, primary_color, secondary_color, font_family,
		       operating_hours, enable_hire, enable_sales, enable_service,
		       currency, timezone,
		       tagline, logo_url, favicon_url, hero_image_url, hero_eyebrow, cover_image_url,
		       created_at, updated_at
		FROM   company_profiles
		WHERE  tenant_id = $1
		LIMIT  1`

	profile, err := scanProfile(r.db.QueryRow(ctx, q, tenantID))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("company: find by tenant: %w", err)
	}

	return profile, nil
}

// Upsert inserts or updates the company profile for the given tenant.
// It uses INSERT … ON CONFLICT to guarantee exactly one profile per tenant.
func (r *Repository) Upsert(ctx context.Context, p UpsertParams) (*Profile, error) {
	socialJSON, err := json.Marshal(p.SocialLinks)
	if err != nil {
		return nil, fmt.Errorf("company: marshal social links: %w", err)
	}

	hoursJSON, err := json.Marshal(p.OperatingHours)
	if err != nil {
		return nil, fmt.Errorf("company: marshal operating hours: %w", err)
	}

	const q = `
		INSERT INTO company_profiles (
		    tenant_id,
		    legal_name, business_type, registration_no, tax_pin, description,
		    tagline, logo_url, hero_image_url, hero_eyebrow,
		    country, city, address_line1, address_line2, postal_code,
		    support_email, support_phone, whatsapp,
		    social_links, primary_color, secondary_color, font_family,
		    operating_hours, enable_hire, enable_sales, enable_service,
		    currency, timezone
		) VALUES (
		    $1,
		    $2,  $3,  $4,  $5,  $6,
		    $7,  $8,  $9,  $10,
		    $11, $12, $13, $14, $15,
		    $16, $17, $18,
		    $19, $20, $21, $22,
		    $23, $24, $25, $26,
		    $27, $28
		)
		ON CONFLICT (tenant_id) DO UPDATE SET
		    legal_name      = COALESCE(EXCLUDED.legal_name,      company_profiles.legal_name),
		    business_type   = COALESCE(EXCLUDED.business_type,   company_profiles.business_type),
		    registration_no = COALESCE(EXCLUDED.registration_no, company_profiles.registration_no),
		    tax_pin         = COALESCE(EXCLUDED.tax_pin,         company_profiles.tax_pin),
		    description     = COALESCE(EXCLUDED.description,     company_profiles.description),
		    tagline         = COALESCE(EXCLUDED.tagline,         company_profiles.tagline),
		    logo_url        = COALESCE(EXCLUDED.logo_url,        company_profiles.logo_url),
		    hero_image_url  = COALESCE(EXCLUDED.hero_image_url,  company_profiles.hero_image_url),
		    hero_eyebrow    = COALESCE(EXCLUDED.hero_eyebrow,    company_profiles.hero_eyebrow),
		    country         = COALESCE(EXCLUDED.country,         company_profiles.country),
		    city            = COALESCE(EXCLUDED.city,            company_profiles.city),
		    address_line1   = COALESCE(EXCLUDED.address_line1,   company_profiles.address_line1),
		    address_line2   = COALESCE(EXCLUDED.address_line2,   company_profiles.address_line2),
		    postal_code     = COALESCE(EXCLUDED.postal_code,     company_profiles.postal_code),
		    support_email   = COALESCE(EXCLUDED.support_email,   company_profiles.support_email),
		    support_phone   = COALESCE(EXCLUDED.support_phone,   company_profiles.support_phone),
		    whatsapp        = COALESCE(EXCLUDED.whatsapp,        company_profiles.whatsapp),
		    social_links    = CASE WHEN $19::jsonb = '{}'::jsonb
		                          THEN company_profiles.social_links
		                          ELSE EXCLUDED.social_links END,
		    primary_color   = COALESCE(EXCLUDED.primary_color,   company_profiles.primary_color),
		    secondary_color = COALESCE(EXCLUDED.secondary_color, company_profiles.secondary_color),
		    font_family     = COALESCE(EXCLUDED.font_family,     company_profiles.font_family),
		    operating_hours = CASE WHEN $23::jsonb = '{}'::jsonb
		                          THEN company_profiles.operating_hours
		                          ELSE EXCLUDED.operating_hours END,
		    enable_hire     = EXCLUDED.enable_hire,
		    enable_sales    = EXCLUDED.enable_sales,
		    enable_service  = EXCLUDED.enable_service,
		    currency        = COALESCE(EXCLUDED.currency,        company_profiles.currency),
		    timezone        = COALESCE(EXCLUDED.timezone,        company_profiles.timezone),
		    updated_at      = NOW()
		RETURNING
		    id, tenant_id,
		    legal_name, business_type, registration_no, tax_pin, description,
		    country, city, address_line1, address_line2, postal_code,
		    support_email, support_phone, whatsapp,
		    social_links, primary_color, secondary_color, font_family,
		    operating_hours, enable_hire, enable_sales, enable_service,
		    currency, timezone,
		    tagline, logo_url, favicon_url, hero_image_url, hero_eyebrow, cover_image_url,
		    created_at, updated_at`

	profile, err := scanProfile(r.db.QueryRow(ctx, q,
		p.TenantID,
		p.LegalName, p.BusinessType, p.RegistrationNo, p.TaxPIN, p.Description,
		p.Tagline, p.LogoURL, p.HeroImageURL, p.HeroEyebrow,
		p.Country, p.City, p.AddressLine1, p.AddressLine2, p.PostalCode,
		p.SupportEmail, p.SupportPhone, p.WhatsApp,
		socialJSON, p.PrimaryColor, p.SecondaryColor, p.FontFamily,
		hoursJSON, p.EnableHire, p.EnableSales, p.EnableService,
		p.Currency, p.Timezone,
	))
	if err != nil {
		return nil, fmt.Errorf("company: upsert: %w", err)
	}

	return profile, nil
}

// ── Params ─────────────────────────────────────────────────────────────────

// UpsertParams holds all updatable fields for a company profile.
type UpsertParams struct {
	TenantID       string
	LegalName      *string
	BusinessType   *string
	RegistrationNo *string
	TaxPIN         *string
	Description    *string
	Tagline        *string
	LogoURL        *string
	HeroImageURL   *string
	HeroEyebrow    *string
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
	Currency       string
	Timezone       string
}

// ── Scanner ────────────────────────────────────────────────────────────────

func scanProfile(row pgx.Row) (*Profile, error) {
	var p Profile
	var socialRaw  []byte
	var hoursRaw   []byte

	err := row.Scan(
		&p.ID,
		&p.TenantID,
		&p.LegalName,
		&p.BusinessType,
		&p.RegistrationNo,
		&p.TaxPIN,
		&p.Description,
		&p.Country,
		&p.City,
		&p.AddressLine1,
		&p.AddressLine2,
		&p.PostalCode,
		&p.SupportEmail,
		&p.SupportPhone,
		&p.WhatsApp,
		&socialRaw,
		&p.PrimaryColor,
		&p.SecondaryColor,
		&p.FontFamily,
		&hoursRaw,
		&p.EnableHire,
		&p.EnableSales,
		&p.EnableService,
		&p.Currency,
		&p.Timezone,
		&p.Tagline,
		&p.LogoURL,
		&p.FaviconURL,
		&p.HeroImageURL,
		&p.HeroEyebrow,
		&p.CoverImageURL,
		&p.CreatedAt,
		&p.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}

	// Decode JSONB columns.
	if err := json.Unmarshal(socialRaw, &p.SocialLinks); err != nil {
		p.SocialLinks = map[string]string{}
	}

	if err := json.Unmarshal(hoursRaw, &p.OperatingHours); err != nil {
		p.OperatingHours = map[string]DaySchedule{}
	}

	return &p, nil
}
