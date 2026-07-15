package hire

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Repository handles all database operations for the hire domain.
type Repository struct {
	db *pgxpool.Pool
}

// NewRepository creates a Repository backed by the given pool.
func NewRepository(db *pgxpool.Pool) *Repository {
	return &Repository{db: db}
}

// ── ListFilters ───────────────────────────────────────────────────────────────

// ListFilters narrows the bookings list query.  Zero/nil fields are ignored.
type ListFilters struct {
	Status     *string    // filter by booking status
	VehicleID  *string    // filter by vehicle
	CustomerID *string    // filter by customer
	FromDate   *time.Time // bookings whose start_date >= FromDate
	ToDate     *time.Time // bookings whose start_date <= ToDate
}

// ── Queries ───────────────────────────────────────────────────────────────────

// List returns bookings for tenantID, optionally filtered.
func (r *Repository) List(ctx context.Context, tenantID string, f ListFilters) ([]*Booking, error) {
	args := []any{tenantID}
	conditions := []string{"tenant_id = $1"}

	if f.Status != nil {
		args = append(args, *f.Status)
		conditions = append(conditions, fmt.Sprintf("status = $%d", len(args)))
	}
	if f.VehicleID != nil {
		args = append(args, *f.VehicleID)
		conditions = append(conditions, fmt.Sprintf("vehicle_id = $%d", len(args)))
	}
	if f.CustomerID != nil {
		args = append(args, *f.CustomerID)
		conditions = append(conditions, fmt.Sprintf("customer_id = $%d", len(args)))
	}
	if f.FromDate != nil {
		args = append(args, *f.FromDate)
		conditions = append(conditions, fmt.Sprintf("start_date >= $%d", len(args)))
	}
	if f.ToDate != nil {
		args = append(args, *f.ToDate)
		conditions = append(conditions, fmt.Sprintf("start_date <= $%d", len(args)))
	}

	q := `SELECT id, tenant_id, vehicle_id, customer_id,
	             start_date, end_date, pickup_time, return_time,
	             actual_start, actual_end,
	             daily_rate, total_days, total_amount,
	             deposit_amount, discount_amount, final_amount,
	             status,
	             pickup_location, return_location,
	             mileage_out, mileage_in,
	             created_by, notes, created_at, updated_at
	      FROM   hire_bookings
	      WHERE  ` + strings.Join(conditions, " AND ") + `
	      ORDER  BY start_date DESC, created_at DESC`

	rows, err := r.db.Query(ctx, q, args...)
	if err != nil {
		return nil, fmt.Errorf("hire: list: %w", err)
	}
	defer rows.Close()

	var result []*Booking
	for rows.Next() {
		b, err := scanBooking(rows)
		if err != nil {
			return nil, fmt.Errorf("hire: list scan: %w", err)
		}
		result = append(result, b)
	}

	return result, rows.Err()
}

// FindByID returns a booking by UUID.  Returns (nil, nil) when not found.
func (r *Repository) FindByID(ctx context.Context, id string) (*Booking, error) {
	const q = `
		SELECT id, tenant_id, vehicle_id, customer_id,
		       start_date, end_date, pickup_time, return_time,
		       actual_start, actual_end,
		       daily_rate, total_days, total_amount,
		       deposit_amount, discount_amount, final_amount,
		       status,
		       pickup_location, return_location,
		       mileage_out, mileage_in,
		       created_by, notes, created_at, updated_at
		FROM   hire_bookings
		WHERE  id = $1
		LIMIT  1`

	b, err := scanBooking(r.db.QueryRow(ctx, q, id))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("hire: find by id: %w", err)
	}

	return b, nil
}

// HasConflict reports whether a vehicle already has an active/confirmed/pending
// booking that overlaps the given date range, optionally excluding a booking ID
// (used when updating an existing booking).
func (r *Repository) HasConflict(ctx context.Context, vehicleID string, start, end time.Time, excludeID *string) (bool, error) {
	args := []any{vehicleID, start, end}
	excludeClause := ""

	if excludeID != nil {
		args = append(args, *excludeID)
		excludeClause = fmt.Sprintf("AND id <> $%d", len(args))
	}

	q := fmt.Sprintf(`
		SELECT EXISTS (
		    SELECT 1 FROM hire_bookings
		    WHERE  vehicle_id = $1
		    AND    status     NOT IN ('cancelled', 'completed')
		    AND    start_date <= $3
		    AND    end_date   >= $2
		    %s
		)`, excludeClause)

	var exists bool
	if err := r.db.QueryRow(ctx, q, args...).Scan(&exists); err != nil {
		return false, fmt.Errorf("hire: conflict check: %w", err)
	}

	return exists, nil
}

// Create inserts a new booking and returns the persisted record.
func (r *Repository) Create(ctx context.Context, p CreateParams) (*Booking, error) {
	const q = `
		INSERT INTO hire_bookings (
		    tenant_id, vehicle_id, customer_id,
		    start_date, end_date, pickup_time, return_time,
		    daily_rate, total_days, total_amount,
		    deposit_amount, discount_amount, final_amount,
		    status,
		    pickup_location, return_location,
		    created_by, notes
		) VALUES (
		    $1, $2, $3,
		    $4, $5, $6, $7,
		    $8, $9, $10,
		    $11, $12, $13,
		    $14,
		    $15, $16,
		    $17, $18
		)
		RETURNING id, tenant_id, vehicle_id, customer_id,
		          start_date, end_date, pickup_time, return_time,
		          actual_start, actual_end,
		          daily_rate, total_days, total_amount,
		          deposit_amount, discount_amount, final_amount,
		          status,
		          pickup_location, return_location,
		          mileage_out, mileage_in,
		          created_by, notes, created_at, updated_at`

	b, err := scanBooking(r.db.QueryRow(ctx, q,
		p.TenantID, p.VehicleID, p.CustomerID,
		p.StartDate, p.EndDate, p.PickupTime, p.ReturnTime,
		p.DailyRate, p.TotalDays, p.TotalAmount,
		p.DepositAmount, p.DiscountAmount, p.FinalAmount,
		p.Status,
		p.PickupLocation, p.ReturnLocation,
		p.CreatedBy, p.Notes,
	))
	if err != nil {
		return nil, fmt.Errorf("hire: create: %w", err)
	}

	return b, nil
}

// Update applies a partial update to a booking.
func (r *Repository) Update(ctx context.Context, id string, p UpdateParams) (*Booking, error) {
	const q = `
		UPDATE hire_bookings
		SET    vehicle_id       = COALESCE($2,  vehicle_id),
		       customer_id      = COALESCE($3,  customer_id),
		       start_date       = COALESCE($4,  start_date),
		       end_date         = COALESCE($5,  end_date),
		       pickup_time      = COALESCE($6,  pickup_time),
		       return_time      = COALESCE($7,  return_time),
		       actual_start     = COALESCE($8,  actual_start),
		       actual_end       = COALESCE($9,  actual_end),
		       daily_rate       = COALESCE($10, daily_rate),
		       total_days       = COALESCE($11, total_days),
		       total_amount     = COALESCE($12, total_amount),
		       deposit_amount   = COALESCE($13, deposit_amount),
		       discount_amount  = COALESCE($14, discount_amount),
		       final_amount     = COALESCE($15, final_amount),
		       status           = COALESCE($16, status),
		       pickup_location  = COALESCE($17, pickup_location),
		       return_location  = COALESCE($18, return_location),
		       mileage_out      = COALESCE($19, mileage_out),
		       mileage_in       = COALESCE($20, mileage_in),
		       notes            = COALESCE($21, notes),
		       updated_at       = NOW()
		WHERE  id = $1
		RETURNING id, tenant_id, vehicle_id, customer_id,
		          start_date, end_date, pickup_time, return_time,
		          actual_start, actual_end,
		          daily_rate, total_days, total_amount,
		          deposit_amount, discount_amount, final_amount,
		          status,
		          pickup_location, return_location,
		          mileage_out, mileage_in,
		          created_by, notes, created_at, updated_at`

	b, err := scanBooking(r.db.QueryRow(ctx, q,
		id,
		p.VehicleID, p.CustomerID,
		p.StartDate, p.EndDate, p.PickupTime, p.ReturnTime,
		p.ActualStart, p.ActualEnd,
		p.DailyRate, p.TotalDays, p.TotalAmount,
		p.DepositAmount, p.DiscountAmount, p.FinalAmount,
		p.Status,
		p.PickupLocation, p.ReturnLocation,
		p.MileageOut, p.MileageIn,
		p.Notes,
	))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("hire: update: %w", err)
	}

	return b, nil
}

// Delete hard-deletes a booking by ID.
// Only pending or cancelled bookings should be hard-deleted in practice.
func (r *Repository) Delete(ctx context.Context, id string) error {
	const q = `DELETE FROM hire_bookings WHERE id = $1`

	ct, err := r.db.Exec(ctx, q, id)
	if err != nil {
		return fmt.Errorf("hire: delete: %w", err)
	}

	if ct.RowsAffected() == 0 {
		return pgx.ErrNoRows
	}

	return nil
}

// ── Params ────────────────────────────────────────────────────────────────────

// CreateParams holds all fields required to insert a new booking.
type CreateParams struct {
	TenantID   string
	VehicleID  string
	CustomerID string

	StartDate  time.Time
	EndDate    time.Time
	PickupTime *string
	ReturnTime *string

	DailyRate      float64
	TotalDays      int
	TotalAmount    float64
	DepositAmount  float64
	DiscountAmount float64
	FinalAmount    float64

	Status BookingStatus

	PickupLocation *string
	ReturnLocation *string

	CreatedBy *string
	Notes     *string
}

// UpdateParams holds nullable fields for a partial booking update.
type UpdateParams struct {
	VehicleID  *string
	CustomerID *string

	StartDate  *time.Time
	EndDate    *time.Time
	PickupTime *string
	ReturnTime *string

	ActualStart *time.Time
	ActualEnd   *time.Time

	DailyRate      *float64
	TotalDays      *int
	TotalAmount    *float64
	DepositAmount  *float64
	DiscountAmount *float64
	FinalAmount    *float64

	Status *BookingStatus

	PickupLocation *string
	ReturnLocation *string

	MileageOut *int
	MileageIn  *int

	Notes *string
}

// ── Scanner ───────────────────────────────────────────────────────────────────

type rowScanner interface {
	Scan(dest ...any) error
}

func scanBooking(row rowScanner) (*Booking, error) {
	var b Booking
	var status string

	err := row.Scan(
		&b.ID,
		&b.TenantID,
		&b.VehicleID,
		&b.CustomerID,
		&b.StartDate,
		&b.EndDate,
		&b.PickupTime,
		&b.ReturnTime,
		&b.ActualStart,
		&b.ActualEnd,
		&b.DailyRate,
		&b.TotalDays,
		&b.TotalAmount,
		&b.DepositAmount,
		&b.DiscountAmount,
		&b.FinalAmount,
		&status,
		&b.PickupLocation,
		&b.ReturnLocation,
		&b.MileageOut,
		&b.MileageIn,
		&b.CreatedBy,
		&b.Notes,
		&b.CreatedAt,
		&b.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}

	b.Status = BookingStatus(status)
	return &b, nil
}
