package vehicles

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Repository handles all database operations for the vehicles domain.
type Repository struct {
	db *pgxpool.Pool
}

// NewRepository creates a Repository backed by the given pool.
func NewRepository(db *pgxpool.Pool) *Repository {
	return &Repository{db: db}
}

// ── ListFilters ───────────────────────────────────────────────────────────────

// ListFilters optionally narrows the vehicle list query.
// Nil / zero-value fields are ignored (all vehicles are returned).
type ListFilters struct {
	Status      *string // filter by lifecycle status
	VehicleType *string // filter by body type
}

// ── Queries ───────────────────────────────────────────────────────────────────

// List returns vehicles for tenantID, optionally filtered.
func (r *Repository) List(ctx context.Context, tenantID string, f ListFilters) ([]*Vehicle, error) {
	// Build a dynamic WHERE clause so we don't send unnecessary params.
	args := []any{tenantID}
	where := "tenant_id = $1"

	if f.Status != nil {
		args = append(args, *f.Status)
		where += fmt.Sprintf(" AND status = $%d", len(args))
	}

	if f.VehicleType != nil {
		args = append(args, *f.VehicleType)
		where += fmt.Sprintf(" AND vehicle_type = $%d", len(args))
	}

	q := `SELECT id, tenant_id,
	             make, model, year, color, vin, plate_no,
	             vehicle_type, status,
	             mileage, fuel_type, daily_rate, sale_price,
	             images, notes, created_at, updated_at
	      FROM   vehicles
	      WHERE  ` + where + `
	      ORDER  BY created_at DESC`

	rows, err := r.db.Query(ctx, q, args...)
	if err != nil {
		return nil, fmt.Errorf("vehicles: list: %w", err)
	}
	defer rows.Close()

	var result []*Vehicle
	for rows.Next() {
		v, err := scanVehicle(rows)
		if err != nil {
			return nil, fmt.Errorf("vehicles: list scan: %w", err)
		}
		result = append(result, v)
	}

	return result, rows.Err()
}

// FindByID returns a vehicle by UUID.  Returns (nil, nil) when not found.
func (r *Repository) FindByID(ctx context.Context, id string) (*Vehicle, error) {
	const q = `
		SELECT id, tenant_id,
		       make, model, year, color, vin, plate_no,
		       vehicle_type, status,
		       mileage, fuel_type, daily_rate, sale_price,
		       images, notes, created_at, updated_at
		FROM   vehicles
		WHERE  id = $1
		LIMIT  1`

	v, err := scanVehicle(r.db.QueryRow(ctx, q, id))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("vehicles: find by id: %w", err)
	}

	return v, nil
}

// Create inserts a new vehicle and returns the persisted record.
func (r *Repository) Create(ctx context.Context, p CreateParams) (*Vehicle, error) {
	imagesJSON, err := json.Marshal(p.Images)
	if err != nil {
		return nil, fmt.Errorf("vehicles: marshal images: %w", err)
	}

	const q = `
		INSERT INTO vehicles (
		    tenant_id, make, model, year, color, vin, plate_no,
		    vehicle_type, status, mileage, fuel_type,
		    daily_rate, sale_price, images, notes
		) VALUES (
		    $1, $2, $3, $4, $5, $6, $7,
		    $8, $9, $10, $11,
		    $12, $13, $14, $15
		)
		RETURNING id, tenant_id,
		          make, model, year, color, vin, plate_no,
		          vehicle_type, status,
		          mileage, fuel_type, daily_rate, sale_price,
		          images, notes, created_at, updated_at`

	v, err := scanVehicle(r.db.QueryRow(ctx, q,
		p.TenantID, p.Make, p.Model, p.Year, p.Color, p.VIN, p.PlateNo,
		p.VehicleType, p.Status, p.Mileage, p.FuelType,
		p.DailyRate, p.SalePrice, imagesJSON, p.Notes,
	))
	if err != nil {
		return nil, fmt.Errorf("vehicles: create: %w", err)
	}

	return v, nil
}

// Update applies a partial update to a vehicle.
// Only non-nil pointer fields are changed; primitive fields are always updated.
func (r *Repository) Update(ctx context.Context, id string, p UpdateParams) (*Vehicle, error) {
	imagesJSON, err := json.Marshal(p.Images)
	if err != nil {
		return nil, fmt.Errorf("vehicles: marshal images: %w", err)
	}

	const q = `
		UPDATE vehicles
		SET    make         = COALESCE($2,  make),
		       model        = COALESCE($3,  model),
		       year         = COALESCE($4,  year),
		       color        = COALESCE($5,  color),
		       vin          = COALESCE($6,  vin),
		       plate_no     = COALESCE($7,  plate_no),
		       vehicle_type = COALESCE($8,  vehicle_type),
		       status       = COALESCE($9,  status),
		       mileage      = COALESCE($10, mileage),
		       fuel_type    = COALESCE($11, fuel_type),
		       daily_rate   = COALESCE($12, daily_rate),
		       sale_price   = COALESCE($13, sale_price),
		       images       = CASE WHEN $14::jsonb = '[]'::jsonb
		                          THEN images
		                          ELSE $14::jsonb END,
		       notes        = COALESCE($15, notes),
		       updated_at   = NOW()
		WHERE  id = $1
		RETURNING id, tenant_id,
		          make, model, year, color, vin, plate_no,
		          vehicle_type, status,
		          mileage, fuel_type, daily_rate, sale_price,
		          images, notes, created_at, updated_at`

	v, err := scanVehicle(r.db.QueryRow(ctx, q,
		id,
		p.Make, p.Model, p.Year, p.Color, p.VIN, p.PlateNo,
		p.VehicleType, p.Status, p.Mileage, p.FuelType,
		p.DailyRate, p.SalePrice, imagesJSON, p.Notes,
	))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("vehicles: update: %w", err)
	}

	return v, nil
}

// Delete hard-deletes a vehicle by ID.
func (r *Repository) Delete(ctx context.Context, id string) error {
	const q = `DELETE FROM vehicles WHERE id = $1`

	ct, err := r.db.Exec(ctx, q, id)
	if err != nil {
		return fmt.Errorf("vehicles: delete: %w", err)
	}

	if ct.RowsAffected() == 0 {
		return pgx.ErrNoRows
	}

	return nil
}

// ── Params ────────────────────────────────────────────────────────────────────

// CreateParams holds all fields required to insert a new vehicle.
type CreateParams struct {
	TenantID    string
	Make        string
	Model       string
	Year        int
	Color       *string
	VIN         *string
	PlateNo     *string
	VehicleType string
	Status      string
	Mileage     *int
	FuelType    *string
	DailyRate   *float64
	SalePrice   *float64
	Images      []string
	Notes       *string
}

// UpdateParams mirrors CreateParams but every field is optional (pointer).
// nil means "leave unchanged" for pointer fields.
type UpdateParams struct {
	Make        *string
	Model       *string
	Year        *int
	Color       *string
	VIN         *string
	PlateNo     *string
	VehicleType *string
	Status      *string
	Mileage     *int
	FuelType    *string
	DailyRate   *float64
	SalePrice   *float64
	// Images replaces the full list when non-empty; empty slice leaves it untouched.
	Images []string
	Notes  *string
}

// ── Scanner ───────────────────────────────────────────────────────────────────

type rowScanner interface {
	Scan(dest ...any) error
}

func scanVehicle(row rowScanner) (*Vehicle, error) {
	var v Vehicle
	var imagesRaw []byte

	err := row.Scan(
		&v.ID,
		&v.TenantID,
		&v.Make,
		&v.Model,
		&v.Year,
		&v.Color,
		&v.VIN,
		&v.PlateNo,
		&v.VehicleType,
		&v.Status,
		&v.Mileage,
		&v.FuelType,
		&v.DailyRate,
		&v.SalePrice,
		&imagesRaw,
		&v.Notes,
		&v.CreatedAt,
		&v.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}

	if err := json.Unmarshal(imagesRaw, &v.Images); err != nil {
		v.Images = []string{}
	}

	return &v, nil
}
