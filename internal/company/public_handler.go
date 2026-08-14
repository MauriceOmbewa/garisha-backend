package company

// PublicHandler serves the white-label tenant bootstrap endpoint.
// It is deliberately separate from Handler so it can be registered WITHOUT
// authentication middleware — any browser hitting a subdomain can call it.

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"strings"

	apperr "github.com/MauriceOmbewa/garisha-backend/internal/platform/errors"
	"github.com/MauriceOmbewa/garisha-backend/internal/platform/response"
	"github.com/MauriceOmbewa/garisha-backend/internal/platform/tenant"
)

// TenantFinder is the minimal interface PublicHandler needs to load a tenant
// by its URL slug.  tenants.Repository satisfies this.
type TenantFinder interface {
	FindBySlug(ctx context.Context, slug string) (*tenant.Record, error)
}

// PublicHandler handles the public (no-auth) tenant config endpoint.
type PublicHandler struct {
	finder TenantFinder
	repo   *Repository // company profile repo — reused
	log    *slog.Logger
}

// NewPublicHandler creates a PublicHandler.
func NewPublicHandler(finder TenantFinder, repo *Repository, log *slog.Logger) *PublicHandler {
	return &PublicHandler{finder: finder, repo: repo, log: log}
}

// ─── Response DTO ──────────────────────────────────────────────────────────────

// publicTenantDTO is the shape returned to the customer frontend.
// It merges the tenant record with the company branding profile.
type publicTenantDTO struct {
	// From tenant record
	ID       string `json:"id"`
	Slug     string `json:"slug"`
	Name     string `json:"name"`
	Initials string `json:"initials"`
	Plan     string `json:"plan"`

	// From company profile — branding
	Tagline      string  `json:"tagline"`
	Description  *string `json:"description,omitempty"`
	LogoURL      *string `json:"logo_url,omitempty"`
	FaviconURL   *string `json:"favicon_url,omitempty"`
	HeroImageURL *string `json:"hero_image_url,omitempty"`
	HeroEyebrow  *string `json:"hero_eyebrow,omitempty"`
	CoverImageURL *string `json:"cover_image_url,omitempty"`

	// Branding colours (derived / explicit)
	PrimaryColor  string `json:"primary_color"`
	PrimaryDark   string `json:"primary_dark"`
	PrimaryLight  string `json:"primary_light"`
	PrimaryRgb    string `json:"primary_rgb"`
	AccentColor   string `json:"accent_color"`

	// Services enabled flags
	ServicesEnabled servicesDTO `json:"services_enabled"`

	// Contact
	Contact contactDTO `json:"contact"`

	// Social links
	SocialLinks map[string]string `json:"social_links"`

	// Locale
	Currency string `json:"currency"`
	Timezone string `json:"timezone"`
}

type servicesDTO struct {
	Hire    bool `json:"hire"`
	Sales   bool `json:"sales"`
	Service bool `json:"service"`
}

type contactDTO struct {
	Phone    *string `json:"phone,omitempty"`
	Email    *string `json:"email,omitempty"`
	WhatsApp *string `json:"whatsapp,omitempty"`
	Location *string `json:"location,omitempty"`
}

// ─── Handler ──────────────────────────────────────────────────────────────────

// GetPublicTenant godoc
// GET /api/v1/public/tenant/{slug}
//
// Returns the white-label configuration for the requested tenant slug.
// No authentication required — this is called by the customer frontend
// on every page load to bootstrap branding and service flags.
func (h *PublicHandler) GetPublicTenant(w http.ResponseWriter, r *http.Request) {
	slug := r.PathValue("slug")
	if slug == "" {
		apperr.Handle(w, r, apperr.BadRequest("slug is required"), h.log)
		return
	}

	// 1. Load tenant record
	rec, err := h.finder.FindBySlug(r.Context(), slug)
	if err != nil {
		h.log.Error("public tenant lookup failed", "slug", slug, "error", err)
		apperr.Handle(w, r, apperr.Internal("failed to look up tenant", err), h.log)
		return
	}
	if rec == nil {
		apperr.Handle(w, r, apperr.NotFound(fmt.Sprintf("no business found for slug %q", slug)), h.log)
		return
	}
	if !rec.IsActive {
		apperr.Handle(w, r, apperr.Forbidden("this account is suspended"), h.log)
		return
	}

	// 2. Load company profile (best-effort — not all tenants have one yet)
	profile, err := h.repo.FindByTenantID(r.Context(), rec.ID)
	if err != nil {
		h.log.Warn("could not load company profile for public tenant", "tenant_id", rec.ID, "error", err)
		// Non-fatal — fall through with nil profile
	}

	// 3. Build response
	dto := buildPublicDTO(rec, profile)
	response.Success(w, http.StatusOK, "tenant config retrieved", dto, h.log)
}

// ─── Builder ──────────────────────────────────────────────────────────────────

func buildPublicDTO(rec *tenant.Record, p *Profile) publicTenantDTO {
	dto := publicTenantDTO{
		ID:       rec.ID,
		Slug:     rec.Slug,
		Name:     rec.Name,
		Initials: nameInitials(rec.Name),
		Plan:     rec.Plan,

		// Defaults — overridden below if company profile exists
		PrimaryColor: "#0F766E",
		PrimaryDark:  "#0C5F58",
		PrimaryLight: "#E6F4F2",
		PrimaryRgb:   "15, 118, 110",
		AccentColor:  "#C97A40",

		ServicesEnabled: servicesDTO{Hire: true, Sales: true, Service: true},
		SocialLinks:     map[string]string{},
		Currency:        "KSh",
		Timezone:        "Africa/Nairobi",
	}

	// Logo comes from the tenant record
	if rec.LogoURL != nil && *rec.LogoURL != "" {
		dto.LogoURL = rec.LogoURL
	}

	if p == nil {
		// No company profile yet — use tenant name as tagline
		dto.Tagline = "Welcome to " + rec.Name
		return dto
	}

	// ── Branding colours ──────────────────────────────────────────────────────
	primary := "#0F766E"
	if p.PrimaryColor != nil && *p.PrimaryColor != "" {
		primary = *p.PrimaryColor
	}
	dto.PrimaryColor = primary
	dto.PrimaryDark  = darkenHex(primary, 0.08)
	dto.PrimaryLight = lightenHex(primary, 0.92)
	dto.PrimaryRgb   = hexToRGB(primary)

	if p.SecondaryColor != nil && *p.SecondaryColor != "" {
		dto.AccentColor = *p.SecondaryColor
	}

	// ── Text content ──────────────────────────────────────────────────────────
	if p.Description != nil {
		dto.Description = p.Description
		// Use first sentence as tagline if no dedicated tagline field
		dto.Tagline = firstSentence(*p.Description)
	} else {
		dto.Tagline = "Welcome to " + rec.Name
	}

	// ── Service flags ─────────────────────────────────────────────────────────
	dto.ServicesEnabled = servicesDTO{
		Hire:    p.EnableHire,
		Sales:   p.EnableSales,
		Service: p.EnableService,
	}

	// ── Contact ───────────────────────────────────────────────────────────────
	dto.Contact = contactDTO{
		Phone:    p.SupportPhone,
		Email:    p.SupportEmail,
		WhatsApp: p.WhatsApp,
	}
	if p.City != nil {
		loc := *p.City
		if p.Country != nil {
			loc = loc + ", " + *p.Country
		}
		dto.Contact.Location = &loc
	}

	// ── Socials ───────────────────────────────────────────────────────────────
	if p.SocialLinks != nil {
		dto.SocialLinks = p.SocialLinks
	}

	// ── Locale ────────────────────────────────────────────────────────────────
	if p.Currency != "" {
		dto.Currency = p.Currency
	}
	if p.Timezone != "" {
		dto.Timezone = p.Timezone
	}

	return dto
}

// ─── Colour helpers ───────────────────────────────────────────────────────────

// hexToRGB converts "#RRGGBB" to "r, g, b" for use in rgba() CSS.
func hexToRGB(hex string) string {
	hex = strings.TrimPrefix(hex, "#")
	if len(hex) != 6 {
		return "15, 118, 110"
	}
	r := hexByte(hex[0:2])
	g := hexByte(hex[2:4])
	b := hexByte(hex[4:6])
	return fmt.Sprintf("%d, %d, %d", r, g, b)
}

func hexByte(s string) int {
	var n int
	fmt.Sscanf(s, "%02x", &n)
	return n
}

// darkenHex darkens a hex colour by the given fraction (0–1).
func darkenHex(hex string, amount float64) string {
	hex = strings.TrimPrefix(hex, "#")
	if len(hex) != 6 {
		return "#0C5F58"
	}
	r := clamp(int(float64(hexByte(hex[0:2])) * (1 - amount)))
	g := clamp(int(float64(hexByte(hex[2:4])) * (1 - amount)))
	b := clamp(int(float64(hexByte(hex[4:6])) * (1 - amount)))
	return fmt.Sprintf("#%02x%02x%02x", r, g, b)
}

// lightenHex blends hex with white at (1-alpha) to produce a light tint.
func lightenHex(hex string, alpha float64) string {
	hex = strings.TrimPrefix(hex, "#")
	if len(hex) != 6 {
		return "#E6F4F2"
	}
	r := clamp(int(float64(hexByte(hex[0:2]))*alpha + 255*(1-alpha)))
	g := clamp(int(float64(hexByte(hex[2:4]))*alpha + 255*(1-alpha)))
	b := clamp(int(float64(hexByte(hex[4:6]))*alpha + 255*(1-alpha)))
	return fmt.Sprintf("#%02x%02x%02x", r, g, b)
}

func clamp(v int) int {
	if v < 0 { return 0 }
	if v > 255 { return 255 }
	return v
}

// ─── Text helpers ─────────────────────────────────────────────────────────────

// nameInitials extracts up to 2 capital initials from a business name.
func nameInitials(name string) string {
	words := strings.Fields(name)
	initials := ""
	for _, w := range words {
		if len(w) > 0 {
			initials += strings.ToUpper(string([]rune(w)[0]))
		}
		if len(initials) >= 2 {
			break
		}
	}
	return initials
}

// firstSentence returns the first sentence of s (up to 120 chars).
func firstSentence(s string) string {
	s = strings.TrimSpace(s)
	for _, sep := range []string{". ", "! ", "? ", ".\n"} {
		if i := strings.Index(s, sep); i > 0 {
			return s[:i+1]
		}
	}
	if len(s) > 120 {
		return s[:120] + "…"
	}
	return s
}
