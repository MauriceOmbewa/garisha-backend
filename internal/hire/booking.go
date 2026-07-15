// Package hire is the domain module for car-hire booking management.
// A booking links a vehicle to a customer for a defined hire period,
// tracks the agreed pricing, and progresses through a status lifecycle:
//
//	pending → confirmed → active → completed
//	                    ↘ cancelled (from any non-terminal state)
package hire

import "time"

// Booking is the full hire-booking entity.
type Booking struct {
	ID         string
	TenantID   string
	VehicleID  string
	CustomerID string

	// Hire period (calendar dates, stored as DATE in postgres)
	StartDate time.Time
	EndDate   time.Time

	// Optional precise times on the start/end day
	PickupTime *string // "HH:MM"
	ReturnTime *string // "HH:MM"

	// Set when the vehicle is physically picked up / returned
	ActualStart *time.Time
	ActualEnd   *time.Time

	// Pricing
	DailyRate      float64
	TotalDays      int
	TotalAmount    float64
	DepositAmount  float64
	DiscountAmount float64
	FinalAmount    float64 // TotalAmount − DiscountAmount

	// Status lifecycle
	Status BookingStatus

	// Locations
	PickupLocation *string
	ReturnLocation *string

	// Odometer
	MileageOut *int
	MileageIn  *int

	// Staff who created the booking
	CreatedBy *string

	// Free-form notes
	Notes *string

	CreatedAt time.Time
	UpdatedAt time.Time
}

// BookingStatus enumerates the valid lifecycle states for a hire booking.
type BookingStatus string

const (
	StatusPending   BookingStatus = "pending"
	StatusConfirmed BookingStatus = "confirmed"
	StatusActive    BookingStatus = "active"
	StatusCompleted BookingStatus = "completed"
	StatusCancelled BookingStatus = "cancelled"
)

// IsTerminal reports whether a status is a final (non-changeable) state.
func (s BookingStatus) IsTerminal() bool {
	return s == StatusCompleted || s == StatusCancelled
}

// ValidTransitions maps each status to the statuses it is allowed to
// transition to.  Transitions not in this map are rejected by the service.
var ValidTransitions = map[BookingStatus][]BookingStatus{
	StatusPending:   {StatusConfirmed, StatusCancelled},
	StatusConfirmed: {StatusActive, StatusCancelled},
	StatusActive:    {StatusCompleted, StatusCancelled},
	StatusCompleted: {},
	StatusCancelled: {},
}

// CanTransitionTo reports whether a transition from s → next is allowed.
func (s BookingStatus) CanTransitionTo(next BookingStatus) bool {
	allowed := ValidTransitions[s]
	for _, a := range allowed {
		if a == next {
			return true
		}
	}
	return false
}
