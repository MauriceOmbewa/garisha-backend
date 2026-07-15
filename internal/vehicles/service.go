package vehicles

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5"

	apperr "github.com/MauriceOmbewa/garisha-backend/internal/platform/errors"
	"github.com/MauriceOmbewa/garisha-backend/internal/platform/tenant"
)

// Service implements business logic for vehicle management.
type Service struct {
	repo *Repository
	log  *slog.Logger
}

// NewService creates a Service.
func NewService(repo *Repository, log *slog.Logger) *Service {
	return &Service{repo: repo, log: log}
}

// ── Input types ───────────────────────────────────────────────────────────────

// CreateInput carries the fields a caller provides when adding a vehicle.
type CreateInput struct {
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

// UpdateInput carries all mutable fields for a partial update.
// nil pointer = leave the existing value unchanged.
type UpdateInput struct {
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
	Images      []string
	Notes       *string
}

// ── Service methods ───────────────────────────────────────────────────────────

// List returns all vehicles for the tenant in ctx, optionally filtered.
func (s *Service) List(ctx context.Context, f ListFilters) ([]*Vehicle, error) {
	tenantID := tenant.MustGetTenantID(ctx)

	vehicles, err := s.repo.List(ctx, tenantID, f)
	if err != nil {
		return nil, apperr.Internal("failed to list vehicles", err)
	}

	return vehicles, nil
}

// GetByID returns a single vehicle scoped to the tenant in ctx.
func (s *Service) GetByID(ctx context.Context, id string) (*Vehicle, error) {
	tenantID := tenant.MustGetTenantID(ctx)

	v, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return nil, apperr.Internal("failed to get vehicle", err)
	}

	if v == nil || v.TenantID != tenantID {
		return nil, apperr.NotFound("vehicle")
	}

	return v, nil
}

// Create adds a new vehicle to the tenant's inventory.
func (s *Service) Create(ctx context.Context, in CreateInput) (*Vehicle, error) {
	tenantID := tenant.MustGetTenantID(ctx)

	if err := validateVehicleType(in.VehicleType); err != nil {
		return nil, err
	}

	if err := validateStatus(in.Status); err != nil {
		return nil, err
	}

	if err := validateYear(in.Year); err != nil {
		return nil, err
	}

	if in.Status == "" {
		in.Status = string(VehicleStatusAvailable)
	}

	if in.VehicleType == "" {
		in.VehicleType = string(VehicleTypeOther)
	}

	if in.Images == nil {
		in.Images = []string{}
	}

	v, err := s.repo.Create(ctx, CreateParams{
		TenantID:    tenantID,
		Make:        in.Make,
		Model:       in.Model,
		Year:        in.Year,
		Color:       in.Color,
		VIN:         in.VIN,
		PlateNo:     in.PlateNo,
		VehicleType: in.VehicleType,
		Status:      in.Status,
		Mileage:     in.Mileage,
		FuelType:    in.FuelType,
		DailyRate:   in.DailyRate,
		SalePrice:   in.SalePrice,
		Images:      in.Images,
		Notes:       in.Notes,
	})
	if err != nil {
		return nil, apperr.Internal("failed to create vehicle", err)
	}

	s.log.Info("vehicle created",
		"vehicle_id", v.ID,
		"make",       v.Make,
		"model",      v.Model,
		"tenant_id",  tenantID,
	)

	return v, nil
}

// Update applies a partial update to a vehicle owned by the tenant in ctx.
func (s *Service) Update(ctx context.Context, id string, in UpdateInput) (*Vehicle, error) {
	tenantID := tenant.MustGetTenantID(ctx)

	// Validate optional fields if provided.
	if in.VehicleType != nil {
		if err := validateVehicleType(*in.VehicleType); err != nil {
			return nil, err
		}
	}

	if in.Status != nil {
		if err := validateStatus(*in.Status); err != nil {
			return nil, err
		}
	}

	if in.Year != nil {
		if err := validateYear(*in.Year); err != nil {
			return nil, err
		}
	}

	// Confirm the vehicle exists and belongs to this tenant.
	existing, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return nil, apperr.Internal("failed to get vehicle", err)
	}

	if existing == nil || existing.TenantID != tenantID {
		return nil, apperr.NotFound("vehicle")
	}

	if in.Images == nil {
		in.Images = []string{}
	}

	v, err := s.repo.Update(ctx, id, UpdateParams{
		Make:        in.Make,
		Model:       in.Model,
		Year:        in.Year,
		Color:       in.Color,
		VIN:         in.VIN,
		PlateNo:     in.PlateNo,
		VehicleType: in.VehicleType,
		Status:      in.Status,
		Mileage:     in.Mileage,
		FuelType:    in.FuelType,
		DailyRate:   in.DailyRate,
		SalePrice:   in.SalePrice,
		Images:      in.Images,
		Notes:       in.Notes,
	})
	if err != nil {
		return nil, apperr.Internal("failed to update vehicle", err)
	}

	if v == nil {
		return nil, apperr.NotFound("vehicle")
	}

	s.log.Info("vehicle updated", "vehicle_id", id, "tenant_id", tenantID)
	return v, nil
}

// UpdateStatus changes only the lifecycle status of a vehicle.
// This is a convenience wrapper used by hire/sales/service modules.
func (s *Service) UpdateStatus(ctx context.Context, id string, status VehicleStatus) (*Vehicle, error) {
	st := string(status)
	return s.Update(ctx, id, UpdateInput{Status: &st})
}

// Delete hard-deletes a vehicle.  Use UpdateStatus(inactive) for soft removal.
func (s *Service) Delete(ctx context.Context, id string) error {
	tenantID := tenant.MustGetTenantID(ctx)

	existing, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return apperr.Internal("failed to get vehicle", err)
	}

	if existing == nil || existing.TenantID != tenantID {
		return apperr.NotFound("vehicle")
	}

	if err := s.repo.Delete(ctx, id); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return apperr.NotFound("vehicle")
		}
		return apperr.Internal("failed to delete vehicle", err)
	}

	s.log.Info("vehicle deleted", "vehicle_id", id, "tenant_id", tenantID)
	return nil
}

// ── Validators ────────────────────────────────────────────────────────────────

var validVehicleTypes = map[string]struct{}{
	string(VehicleTypeSedan):      {},
	string(VehicleTypeSUV):        {},
	string(VehicleTypeTruck):      {},
	string(VehicleTypeVan):        {},
	string(VehicleTypeBus):        {},
	string(VehicleTypePickup):     {},
	string(VehicleTypeMotorcycle): {},
	string(VehicleTypeOther):      {},
}

var validStatuses = map[string]struct{}{
	string(VehicleStatusAvailable):    {},
	string(VehicleStatusHired):        {},
	string(VehicleStatusSold):         {},
	string(VehicleStatusUnderService): {},
	string(VehicleStatusInactive):     {},
}

func validateVehicleType(t string) error {
	if _, ok := validVehicleTypes[t]; !ok {
		return apperr.BadRequest(fmt.Sprintf(
			"invalid vehicle_type %q — must be one of: sedan, suv, truck, van, bus, pickup, motorcycle, other", t,
		))
	}
	return nil
}

func validateStatus(s string) error {
	if _, ok := validStatuses[s]; !ok {
		return apperr.BadRequest(fmt.Sprintf(
			"invalid status %q — must be one of: available, hired, sold, under_service, inactive", s,
		))
	}
	return nil
}

func validateYear(y int) error {
	current := time.Now().Year()
	if y < 1900 || y > current+1 {
		return apperr.BadRequest(fmt.Sprintf("year must be between 1900 and %d", current+1))
	}
	return nil
}
