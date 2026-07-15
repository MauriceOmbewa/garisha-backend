// Package tenants is the domain module that manages tenant (business) records.
// Super-admins use this module to onboard and manage businesses on the platform.
package tenants

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/MauriceOmbewa/garisha-backend/internal/platform/tenant"
)

// Repository handles all database operations for the tenants domain.
type Repository struct {
	db *pgxpool.Pool
}

// NewRepository creates a Repository backed by the given pool.
func NewRepository(db *pgxpool.Pool) *Repository {
	return &Repository{db: db}
}

// ── TenantResolver interface (used by ResolveTenant middleware) ───────────────

// FindByID returns a tenant by UUID, or nil if not found.
func (r *Repository) FindByID(ctx context.Context, id string) (*tenant.Record, error) {
	const q = `
		SELECT id, name, slug, email, phone, logo_url, website_url,
		       plan, is_active, created_at, updated_at
		FROM   tenants
		WHERE  id = $1
		LIMIT  1`

	rec, err := scanRecord(r.db.QueryRow(ctx, q, id))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("tenants: find by id: %w", err)
	}

	return rec, nil
}

// FindBySlug returns a tenant by slug, or nil if not found.
func (r *Repository) FindBySlug(ctx context.Context, slug string) (*tenant.Record, error) {
	const q = `
		SELECT id, name, slug, email, phone, logo_url, website_url,
		       plan, is_active, created_at, updated_at
		FROM   tenants
		WHERE  slug = $1
		LIMIT  1`

	rec, err := scanRecord(r.db.QueryRow(ctx, q, slug))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("tenants: find by slug: %w", err)
	}

	return rec, nil
}

// ── Domain CRUD ───────────────────────────────────────────────────────────────

// List returns all tenants ordered by creation date.
func (r *Repository) List(ctx context.Context) ([]*tenant.Record, error) {
	const q = `
		SELECT id, name, slug, email, phone, logo_url, website_url,
		       plan, is_active, created_at, updated_at
		FROM   tenants
		ORDER  BY created_at DESC`

	rows, err := r.db.Query(ctx, q)
	if err != nil {
		return nil, fmt.Errorf("tenants: list: %w", err)
	}
	defer rows.Close()

	var records []*tenant.Record
	for rows.Next() {
		rec, err := scanRecord(rows)
		if err != nil {
			return nil, fmt.Errorf("tenants: list scan: %w", err)
		}
		records = append(records, rec)
	}

	return records, rows.Err()
}

// Create inserts a new tenant and returns the persisted record.
func (r *Repository) Create(ctx context.Context, p CreateParams) (*tenant.Record, error) {
	const q = `
		INSERT INTO tenants (name, slug, email, phone, logo_url, website_url, plan)
		VALUES              ($1,   $2,   $3,    $4,    $5,       $6,          $7)
		RETURNING id, name, slug, email, phone, logo_url, website_url,
		          plan, is_active, created_at, updated_at`

	rec, err := scanRecord(r.db.QueryRow(ctx, q,
		p.Name, p.Slug, p.Email, p.Phone, p.LogoURL, p.WebsiteURL, p.Plan,
	))
	if err != nil {
		return nil, fmt.Errorf("tenants: create: %w", err)
	}

	return rec, nil
}

// Update modifies mutable fields on an existing tenant.
func (r *Repository) Update(ctx context.Context, id string, p UpdateParams) (*tenant.Record, error) {
	const q = `
		UPDATE tenants
		SET    name        = COALESCE($2, name),
		       email       = COALESCE($3, email),
		       phone       = COALESCE($4, phone),
		       logo_url    = COALESCE($5, logo_url),
		       website_url = COALESCE($6, website_url),
		       plan        = COALESCE($7, plan),
		       is_active   = COALESCE($8, is_active),
		       updated_at  = NOW()
		WHERE  id = $1
		RETURNING id, name, slug, email, phone, logo_url, website_url,
		          plan, is_active, created_at, updated_at`

	rec, err := scanRecord(r.db.QueryRow(ctx, q,
		id, p.Name, p.Email, p.Phone, p.LogoURL, p.WebsiteURL, p.Plan, p.IsActive,
	))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("tenants: update: %w", err)
	}

	return rec, nil
}

// Delete hard-deletes a tenant by ID. Cascades to all tenant-scoped rows.
func (r *Repository) Delete(ctx context.Context, id string) error {
	const q = `DELETE FROM tenants WHERE id = $1`

	ct, err := r.db.Exec(ctx, q, id)
	if err != nil {
		return fmt.Errorf("tenants: delete: %w", err)
	}

	if ct.RowsAffected() == 0 {
		return pgx.ErrNoRows
	}

	return nil
}

// ── Params ────────────────────────────────────────────────────────────────────

// CreateParams holds the fields required to create a new tenant.
type CreateParams struct {
	Name       string
	Slug       string
	Email      string
	Phone      *string
	LogoURL    *string
	WebsiteURL *string
	Plan       string
}

// UpdateParams holds nullable update fields. nil means "leave unchanged".
type UpdateParams struct {
	Name       *string
	Email      *string
	Phone      *string
	LogoURL    *string
	WebsiteURL *string
	Plan       *string
	IsActive   *bool
}

// ── Scanner ───────────────────────────────────────────────────────────────────

type scanner interface {
	Scan(dest ...any) error
}

func scanRecord(row scanner) (*tenant.Record, error) {
	var rec tenant.Record

	err := row.Scan(
		&rec.ID,
		&rec.Name,
		&rec.Slug,
		&rec.Email,
		&rec.Phone,
		&rec.LogoURL,
		&rec.WebsiteURL,
		&rec.Plan,
		&rec.IsActive,
		&rec.CreatedAt,
		&rec.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}

	return &rec, nil
}
