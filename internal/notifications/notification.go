// Package notifications is the domain module for in-app notification management.
// Notifications are created by the system when significant events occur
// (booking confirmed, payment received, reorder alert, etc.) and are delivered
// to specific users within a tenant.  Users can mark them as read individually
// or in bulk.
package notifications

import "time"

// Notification is the full notification entity.
type Notification struct {
	ID       string
	TenantID string
	UserID   string

	Type  string // e.g. "booking_confirmed", "payment_received", "reorder_alert"
	Title string
	Body  string

	// Optional deep-link to the source record.
	ResourceType *string // e.g. "hire_booking", "sale", "service_job"
	ResourceID   *string // UUID of the related record

	IsRead bool
	ReadAt *time.Time

	CreatedAt time.Time
}

// Well-known notification type constants.
// The system uses these when creating notifications programmatically.
const (
	TypeBookingConfirmed  = "booking_confirmed"
	TypeBookingCancelled  = "booking_cancelled"
	TypePaymentReceived   = "payment_received"
	TypePaymentFailed     = "payment_failed"
	TypeSaleCompleted     = "sale_completed"
	TypeServiceCompleted  = "service_completed"
	TypeReorderAlert      = "reorder_alert"
	TypeGeneral           = "general"
)

// UnreadCount is a summary returned by the unread-count endpoint.
type UnreadCount struct {
	Count int `json:"count"`
}
