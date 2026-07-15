// Package vehicles is the domain module for vehicle inventory management.
// Vehicles are tenant-scoped and can be referenced by the hire, sales, and
// service modules.
package vehicles

import "time"

// Vehicle is the full vehicle entity.
type Vehicle struct {
	ID       string
	TenantID string

	// Identity
	Make    string
	Model   string
	Year    int
	Color   *string
	VIN     *string  // Vehicle Identification Number
	PlateNo *string  // registration / number plate

	// Classification
	VehicleType VehicleType

	// Lifecycle
	Status VehicleStatus

	// Odometer / fuel
	Mileage  *int
	FuelType *string

	// Pricing hints
	DailyRate *float64
	SalePrice *float64

	// Images — ordered list of public URLs
	Images []string

	// Free-form notes
	Notes *string

	CreatedAt time.Time
	UpdatedAt time.Time
}

// VehicleType enumerates the valid vehicle body types.
type VehicleType string

const (
	VehicleTypeSedan      VehicleType = "sedan"
	VehicleTypeSUV        VehicleType = "suv"
	VehicleTypeTruck      VehicleType = "truck"
	VehicleTypeVan        VehicleType = "van"
	VehicleTypeBus        VehicleType = "bus"
	VehicleTypePickup     VehicleType = "pickup"
	VehicleTypeMotorcycle VehicleType = "motorcycle"
	VehicleTypeOther      VehicleType = "other"
)

// VehicleStatus enumerates the valid lifecycle states a vehicle can be in.
type VehicleStatus string

const (
	VehicleStatusAvailable    VehicleStatus = "available"
	VehicleStatusHired        VehicleStatus = "hired"
	VehicleStatusSold         VehicleStatus = "sold"
	VehicleStatusUnderService VehicleStatus = "under_service"
	VehicleStatusInactive     VehicleStatus = "inactive"
)
