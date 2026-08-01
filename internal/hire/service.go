package hire

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"math"
	"time"

	"github.com/jackc/pgx/v5"

	apperr "github.com/MauriceOmbewa/garisha-backend/internal/platform/errors"
	"github.com/MauriceOmbewa/garisha-backend/internal/platform/tenant"
)

// Service implements business logic for car-hire booking management.
type Service struct {
	repo *Repository
	log  *slog.Logger
}

// NewService creates a Service.
func NewService(repo *Repository, log *slog.Logger) *Service {
	return &Service{repo: repo, log: log}
}

// ── Input types ───────────────────────────────────────────────────────────────

// CreateInput carries the fields a caller provides when creating a booking.
type CreateInput struct {
	VehicleID  string
	CustomerID string

	StartDate  time.Time
	EndDate    time.Time
	PickupTime *string // "HH:MM", optional
	ReturnTime *string // "HH:MM", optional

	// Pricing — DailyRate is required; deposit and discount are optional.
	DailyRate      float64
	DepositAmount  float64
	DiscountAmount float64

	PickupLocation *string
	ReturnLocation *string

	// Staff who created the booking (pulled from JWT claims by the handler).
	CreatedBy *string
	Notes     *string
}

// UpdateInput carries mutable fields for a partial booking update.
// nil pointer = leave existing value unchanged.
type UpdateInput struct {
	StartDate  *time.Time
	EndDate    *time.Time
	PickupTime *string
	ReturnTime *string

	ActualStart *time.Time
	ActualEnd   *time.Time

	DailyRate      *float64
	DepositAmount  *float64
	DiscountAmount *float64

	PickupLocation *string
	ReturnLocation *string

	MileageOut *int
	MileageIn  *int

	Notes *string
}

// ── Service methods ───────────────────────────────────────────────────────────

// ListEnriched returns bookings joined with customer and vehicle data.
func (s *Service) ListEnriched(ctx context.Context, f ListFilters) ([]*BookingEnriched, error) {
	tenantID := tenant.MustGetTenantID(ctx)
	bookings, err := s.repo.ListEnriched(ctx, tenantID, f)
	if err != nil {
		return nil, apperr.Internal("failed to list bookings", err)
	}
	return bookings, nil
}

// List returns bookings for the tenant in ctx, optionally filtered.
func (s *Service) List(ctx context.Context, f ListFilters) ([]*Booking, error) {
	tenantID := tenant.MustGetTenantID(ctx)

	bookings, err := s.repo.List(ctx, tenantID, f)
	if err != nil {
		return nil, apperr.Internal("failed to list bookings", err)
	}

	return bookings, nil
}

// GetByID returns a single booking scoped to the tenant in ctx.
func (s *Service) GetByID(ctx context.Context, id string) (*Booking, error) {
	tenantID := tenant.MustGetTenantID(ctx)

	b, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return nil, apperr.Internal("failed to get booking", err)
	}

	if b == nil || b.TenantID != tenantID {
		return nil, apperr.NotFound("booking")
	}

	return b, nil
}

// CheckAvailability returns true when a vehicle is free for the given dates.
func (s *Service) CheckAvailability(ctx context.Context, vehicleID string, start, end time.Time) (bool, error) {
	tenant.MustGetTenantID(ctx) // ensure tenant context is set

	if err := validateDateRange(start, end); err != nil {
		return false, err
	}

	conflict, err := s.repo.HasConflict(ctx, vehicleID, start, end, nil)
	if err != nil {
		return false, apperr.Internal("failed to check availability", err)
	}

	return !conflict, nil
}

// Create adds a new hire booking.  It validates dates, checks availability,
// calculates derived pricing fields, and persists the record.
func (s *Service) Create(ctx context.Context, in CreateInput) (*Booking, error) {
	tenantID := tenant.MustGetTenantID(ctx)

	// ── Validation ────────────────────────────────────────────────────────────
	if err := validateDateRange(in.StartDate, in.EndDate); err != nil {
		return nil, err
	}

	if in.DailyRate <= 0 {
		return nil, apperr.BadRequest("daily_rate must be greater than 0")
	}

	if err := validateTimeFormat(in.PickupTime); err != nil {
		return nil, err
	}

	if err := validateTimeFormat(in.ReturnTime); err != nil {
		return nil, err
	}

	// ── Availability check ────────────────────────────────────────────────────
	conflict, err := s.repo.HasConflict(ctx, in.VehicleID, in.StartDate, in.EndDate, nil)
	if err != nil {
		return nil, apperr.Internal("failed to check vehicle availability", err)
	}

	if conflict {
		return nil, apperr.Conflict("vehicle is not available for the selected dates")
	}

	// ── Pricing calculation ───────────────────────────────────────────────────
	totalDays := computeTotalDays(in.StartDate, in.EndDate)
	totalAmount := roundAmount(float64(totalDays) * in.DailyRate)
	finalAmount := roundAmount(math.Max(0, totalAmount-in.DiscountAmount))

	// ── Persist ───────────────────────────────────────────────────────────────
	b, err := s.repo.Create(ctx, CreateParams{
		TenantID:   tenantID,
		VehicleID:  in.VehicleID,
		CustomerID: in.CustomerID,

		StartDate:  in.StartDate,
		EndDate:    in.EndDate,
		PickupTime: in.PickupTime,
		ReturnTime: in.ReturnTime,

		DailyRate:      in.DailyRate,
		TotalDays:      totalDays,
		TotalAmount:    totalAmount,
		DepositAmount:  in.DepositAmount,
		DiscountAmount: in.DiscountAmount,
		FinalAmount:    finalAmount,

		Status:         StatusPending,
		PickupLocation: in.PickupLocation,
		ReturnLocation: in.ReturnLocation,

		CreatedBy: in.CreatedBy,
		Notes:     in.Notes,
	})
	if err != nil {
		return nil, apperr.Internal("failed to create booking", err)
	}

	s.log.Info("hire booking created",
		"booking_id",  b.ID,
		"vehicle_id",  in.VehicleID,
		"customer_id", in.CustomerID,
		"tenant_id",   tenantID,
	)

	return b, nil
}

// Update applies a partial update to a booking.  Dates and pricing are
// recalculated automatically when start/end/rate are changed.
func (s *Service) Update(ctx context.Context, id string, in UpdateInput) (*Booking, error) {
	tenantID := tenant.MustGetTenantID(ctx)

	existing, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return nil, apperr.Internal("failed to get booking", err)
	}

	if existing == nil || existing.TenantID != tenantID {
		return nil, apperr.NotFound("booking")
	}

	if existing.Status.IsTerminal() {
		return nil, apperr.BadRequest(fmt.Sprintf(
			"cannot update a booking that is %s", existing.Status,
		))
	}

	// Validate optional date / time fields.
	if in.StartDate != nil || in.EndDate != nil {
		start := existing.StartDate
		end := existing.EndDate
		if in.StartDate != nil {
			start = *in.StartDate
		}
		if in.EndDate != nil {
			end = *in.EndDate
		}

		if err := validateDateRange(start, end); err != nil {
			return nil, err
		}

		// Re-check availability excluding this booking.
		conflict, err := s.repo.HasConflict(ctx, existing.VehicleID, start, end, &id)
		if err != nil {
			return nil, apperr.Internal("failed to check vehicle availability", err)
		}

		if conflict {
			return nil, apperr.Conflict("vehicle is not available for the updated dates")
		}
	}

	if err := validateTimeFormat(in.PickupTime); err != nil {
		return nil, err
	}

	if err := validateTimeFormat(in.ReturnTime); err != nil {
		return nil, err
	}

	// Recalculate derived pricing if relevant fields changed.
	var (
		totalDays      = existing.TotalDays
		totalAmount    = existing.TotalAmount
		dailyRate      = existing.DailyRate
		discountAmount = existing.DiscountAmount
		finalAmount    = existing.FinalAmount
	)

	if in.DailyRate != nil {
		dailyRate = *in.DailyRate
	}
	if in.DiscountAmount != nil {
		discountAmount = *in.DiscountAmount
	}

	if in.StartDate != nil || in.EndDate != nil || in.DailyRate != nil {
		start := existing.StartDate
		end := existing.EndDate
		if in.StartDate != nil {
			start = *in.StartDate
		}
		if in.EndDate != nil {
			end = *in.EndDate
		}

		totalDays = computeTotalDays(start, end)
		totalAmount = roundAmount(float64(totalDays) * dailyRate)
		finalAmount = roundAmount(math.Max(0, totalAmount-discountAmount))
	} else if in.DiscountAmount != nil {
		// Only discount changed.
		finalAmount = roundAmount(math.Max(0, totalAmount-discountAmount))
	}

	days := totalDays
	ta := totalAmount
	fa := finalAmount

	var statusPtr *BookingStatus
	_ = statusPtr // status changes go through UpdateStatus, not here

	b, err := s.repo.Update(ctx, id, UpdateParams{
		StartDate:      in.StartDate,
		EndDate:        in.EndDate,
		PickupTime:     in.PickupTime,
		ReturnTime:     in.ReturnTime,
		ActualStart:    in.ActualStart,
		ActualEnd:      in.ActualEnd,
		DailyRate:      &dailyRate,
		TotalDays:      &days,
		TotalAmount:    &ta,
		DepositAmount:  in.DepositAmount,
		DiscountAmount: &discountAmount,
		FinalAmount:    &fa,
		PickupLocation: in.PickupLocation,
		ReturnLocation: in.ReturnLocation,
		MileageOut:     in.MileageOut,
		MileageIn:      in.MileageIn,
		Notes:          in.Notes,
	})
	if err != nil {
		return nil, apperr.Internal("failed to update booking", err)
	}

	if b == nil {
		return nil, apperr.NotFound("booking")
	}

	s.log.Info("hire booking updated", "booking_id", id, "tenant_id", tenantID)
	return b, nil
}

// UpdateStatus transitions a booking to a new status, enforcing the lifecycle
// rules and setting actual timestamps when the vehicle is collected/returned.
func (s *Service) UpdateStatus(ctx context.Context, id string, next BookingStatus) (*Booking, error) {
	tenantID := tenant.MustGetTenantID(ctx)

	existing, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return nil, apperr.Internal("failed to get booking", err)
	}

	if existing == nil || existing.TenantID != tenantID {
		return nil, apperr.NotFound("booking")
	}

	if !existing.Status.CanTransitionTo(next) {
		return nil, apperr.BadRequest(fmt.Sprintf(
			"cannot transition booking from %s to %s", existing.Status, next,
		))
	}

	now := time.Now().UTC()

	p := UpdateParams{Status: &next}

	// Set actual timestamps automatically on key transitions.
	switch next {
	case StatusActive:
		p.ActualStart = &now
	case StatusCompleted:
		p.ActualEnd = &now
	}

	b, err := s.repo.Update(ctx, id, p)
	if err != nil {
		return nil, apperr.Internal("failed to update booking status", err)
	}

	if b == nil {
		return nil, apperr.NotFound("booking")
	}

	s.log.Info("hire booking status updated",
		"booking_id", id,
		"from",       existing.Status,
		"to",         next,
		"tenant_id",  tenantID,
	)

	return b, nil
}

// Delete hard-deletes a booking.  Only pending or cancelled bookings may
// be deleted; others must be cancelled first.
func (s *Service) Delete(ctx context.Context, id string) error {
	tenantID := tenant.MustGetTenantID(ctx)

	existing, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return apperr.Internal("failed to get booking", err)
	}

	if existing == nil || existing.TenantID != tenantID {
		return apperr.NotFound("booking")
	}

	if existing.Status != StatusPending && existing.Status != StatusCancelled {
		return apperr.BadRequest("only pending or cancelled bookings can be deleted")
	}

	if err := s.repo.Delete(ctx, id); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return apperr.NotFound("booking")
		}
		return apperr.Internal("failed to delete booking", err)
	}

	s.log.Info("hire booking deleted", "booking_id", id, "tenant_id", tenantID)
	return nil
}

// ── Helpers ───────────────────────────────────────────────────────────────────

// computeTotalDays returns the inclusive number of hire days between two dates.
func computeTotalDays(start, end time.Time) int {
	days := int(end.Truncate(24*time.Hour).Sub(start.Truncate(24*time.Hour)).Hours()/24) + 1
	if days < 1 {
		return 1
	}
	return days
}

// roundAmount rounds a currency amount to 2 decimal places.
func roundAmount(v float64) float64 {
	return math.Round(v*100) / 100
}

// validateDateRange ensures start <= end and start is not in the past (date-only).
func validateDateRange(start, end time.Time) error {
	today := time.Now().UTC().Truncate(24 * time.Hour)
	s := start.UTC().Truncate(24 * time.Hour)
	e := end.UTC().Truncate(24 * time.Hour)

	if s.Before(today) {
		return apperr.BadRequest("start_date cannot be in the past")
	}

	if e.Before(s) {
		return apperr.BadRequest("end_date must be on or after start_date")
	}

	return nil
}

// validateTimeFormat checks that a time string is "HH:MM" if provided.
func validateTimeFormat(t *string) error {
	if t == nil || *t == "" {
		return nil
	}

	var h, m int
	if n, _ := fmt.Sscanf(*t, "%d:%d", &h, &m); n != 2 || h < 0 || h > 23 || m < 0 || m > 59 {
		return apperr.BadRequest(fmt.Sprintf("invalid time %q — expected HH:MM (e.g. 08:30)", *t))
	}

	return nil
}
