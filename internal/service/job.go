// Package service is the domain module for vehicle service job management.
// A service job records a vehicle being brought in for inspection, repair,
// or maintenance.  It carries a list of job items (labour + parts) and
// progresses through a status lifecycle:
//
//	pending → in_progress → awaiting_parts → completed
//	        ↘ cancelled (from any non-terminal state)
package service

import "time"

// Job is the full service-job entity.
type Job struct {
	ID         string
	TenantID   string
	VehicleID  string
	CustomerID *string // nil for internal / fleet jobs
	MechanicID *string // assigned workshop user

	JobType JobType
	Status  JobStatus

	// Key dates
	ReceivedAt  time.Time
	DueDate     *time.Time
	CompletedAt *time.Time

	// Odometer at intake
	MileageIn *int

	// Pricing totals (kept in sync with job items)
	LabourTotal    float64
	PartsTotal     float64
	TotalAmount    float64
	DiscountAmount float64
	FinalAmount    float64

	CreatedBy *string

	CustomerNotes *string
	InternalNotes *string

	// Resolved job items (populated by service layer when needed)
	Items []*JobItem

	CreatedAt time.Time
	UpdatedAt time.Time
}

// JobItem is a single line-item within a service job (a task or a part).
type JobItem struct {
	ID         string
	JobID      string
	TenantID   string
	ItemType   ItemType
	Description string
	Quantity   float64
	UnitPrice  float64
	TotalPrice float64 // Quantity × UnitPrice
	PartNumber *string
	CreatedAt  time.Time
	UpdatedAt  time.Time
}

// ── Status ────────────────────────────────────────────────────────────────────

// JobStatus enumerates the valid lifecycle states for a service job.
type JobStatus string

const (
	JobStatusPending       JobStatus = "pending"
	JobStatusInProgress    JobStatus = "in_progress"
	JobStatusAwaitingParts JobStatus = "awaiting_parts"
	JobStatusCompleted     JobStatus = "completed"
	JobStatusCancelled     JobStatus = "cancelled"
)

// IsTerminal reports whether a status is a final (non-changeable) state.
func (s JobStatus) IsTerminal() bool {
	return s == JobStatusCompleted || s == JobStatusCancelled
}

// validTransitions maps each status to the statuses it may transition to.
var validTransitions = map[JobStatus][]JobStatus{
	JobStatusPending:       {JobStatusInProgress, JobStatusCancelled},
	JobStatusInProgress:    {JobStatusAwaitingParts, JobStatusCompleted, JobStatusCancelled},
	JobStatusAwaitingParts: {JobStatusInProgress, JobStatusCancelled},
	JobStatusCompleted:     {},
	JobStatusCancelled:     {},
}

// CanTransitionTo reports whether a transition from s → next is allowed.
func (s JobStatus) CanTransitionTo(next JobStatus) bool {
	for _, a := range validTransitions[s] {
		if a == next {
			return true
		}
	}
	return false
}

// ── JobType ───────────────────────────────────────────────────────────────────

// JobType classifies the nature of the service work.
type JobType string

const (
	JobTypeGeneral     JobType = "general"
	JobTypeRepair      JobType = "repair"
	JobTypeMaintenance JobType = "maintenance"
	JobTypeInspection  JobType = "inspection"
	JobTypeBodywork    JobType = "bodywork"
	JobTypeElectrical  JobType = "electrical"
	JobTypeOther       JobType = "other"
)

// ── ItemType ──────────────────────────────────────────────────────────────────

// ItemType classifies a single job line-item.
type ItemType string

const (
	ItemTypeLabour      ItemType = "labour"
	ItemTypePart        ItemType = "part"
	ItemTypeConsumable  ItemType = "consumable"
	ItemTypeOther       ItemType = "other"
)
