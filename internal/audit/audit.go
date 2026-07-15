// Package audit is the domain module for the immutable system audit log.
// Every significant action (create, update, delete, status change, login,
// payment, etc.) is recorded here for compliance, debugging, and security
// investigations.
//
// The audit log is append-only: no record is ever updated or deleted.
// The database enforces this with NO UPDATE / NO DELETE rules.
package audit

import "time"

// Log is a single audit log entry.
type Log struct {
	ID       string
	TenantID string

	// Actor — who performed the action (nil for system-generated events).
	ActorID    *string
	ActorEmail *string
	ActorRole  *string

	// Event classification
	Action       string // e.g. "hire_booking.created", "payment.completed"
	ResourceType string // e.g. "hire_booking", "vehicle_sale"
	ResourceID   *string

	// Data snapshot at the time of the action.
	// Shape: {"before": {...}, "after": {...}} for mutations,
	//        {"data": {...}}                  for creates.
	Changes map[string]any

	// Request context
	IPAddress *string
	UserAgent *string
	RequestID *string

	// Outcome
	Status       LogStatus
	ErrorMessage *string

	CreatedAt time.Time
}

// LogStatus indicates whether the action succeeded or failed.
type LogStatus string

const (
	StatusSuccess LogStatus = "success"
	StatusFailure LogStatus = "failure"
)

// ── Well-known action constants ───────────────────────────────────────────────
// Using dot-notation: "<resource>.<verb>"

const (
	// Auth
	ActionUserLogin          = "user.login"
	ActionUserLogout         = "user.logout"
	ActionTokenRefreshed     = "user.token_refreshed"

	// Users
	ActionUserCreated         = "user.created"
	ActionUserUpdated         = "user.updated"
	ActionUserDeleted         = "user.deleted"
	ActionUserRoleChanged     = "user.role_changed"
	ActionUserActivated       = "user.activated"
	ActionUserSuspended       = "user.suspended"

	// Vehicles
	ActionVehicleCreated      = "vehicle.created"
	ActionVehicleUpdated      = "vehicle.updated"
	ActionVehicleDeleted      = "vehicle.deleted"

	// Customers
	ActionCustomerCreated     = "customer.created"
	ActionCustomerUpdated     = "customer.updated"
	ActionCustomerDeleted     = "customer.deleted"

	// Hire bookings
	ActionBookingCreated      = "hire_booking.created"
	ActionBookingUpdated      = "hire_booking.updated"
	ActionBookingStatusChanged = "hire_booking.status_changed"
	ActionBookingDeleted      = "hire_booking.deleted"

	// Vehicle sales
	ActionSaleCreated         = "vehicle_sale.created"
	ActionSaleUpdated         = "vehicle_sale.updated"
	ActionSaleStatusChanged   = "vehicle_sale.status_changed"
	ActionSaleDeleted         = "vehicle_sale.deleted"

	// Service jobs
	ActionServiceJobCreated       = "service_job.created"
	ActionServiceJobUpdated       = "service_job.updated"
	ActionServiceJobStatusChanged = "service_job.status_changed"
	ActionServiceJobDeleted       = "service_job.deleted"
	ActionServiceItemAdded        = "service_job.item_added"
	ActionServiceItemUpdated      = "service_job.item_updated"
	ActionServiceItemDeleted      = "service_job.item_deleted"

	// Payments
	ActionPaymentCreated      = "payment.created"
	ActionPaymentCompleted    = "payment.completed"
	ActionPaymentFailed       = "payment.failed"
	ActionPaymentCancelled    = "payment.cancelled"

	// Finance
	ActionFinanceRecordCreated = "finance_record.created"
	ActionFinanceRecordUpdated = "finance_record.updated"
	ActionFinanceRecordDeleted = "finance_record.deleted"

	// Inventory
	ActionInventoryItemCreated  = "inventory_item.created"
	ActionInventoryItemUpdated  = "inventory_item.updated"
	ActionInventoryItemDeleted  = "inventory_item.deleted"
	ActionInventoryStockAdjusted = "inventory_item.stock_adjusted"
	ActionInventoryUsageRecorded = "inventory_item.usage_recorded"

	// Tenants (super admin)
	ActionTenantCreated       = "tenant.created"
	ActionTenantUpdated       = "tenant.updated"
	ActionTenantDeleted       = "tenant.deleted"

	// Company profile
	ActionCompanyProfileUpserted = "company_profile.upserted"
)

// ListFilters narrows the audit log query.
type ListFilters struct {
	ActorID      *string
	Action       *string
	ResourceType *string
	ResourceID   *string
	Status       *string
	FromDate     *time.Time
	ToDate       *time.Time
	Limit        int
	Offset       int
}
