// Package workers provides a PostgreSQL-backed persistent async job queue.
//
// Architecture:
//   - Enqueue() adds a job to the `jobs` table.
//   - Worker goroutines poll the table using SELECT … FOR UPDATE SKIP LOCKED
//     so multiple workers never claim the same row.
//   - Each job type is handled by a registered Handler function.
//   - Failed jobs are retried with exponential backoff up to max_attempts.
//   - After max_attempts the job moves to 'dead' for manual inspection.
//
// Usage — enqueue from a domain service:
//
//	err := queue.Enqueue(ctx, workers.Job{
//	    Type:    workers.TypeSendEmail,
//	    Payload: workers.EmailPayload{...},
//	})
//
// Usage — register a handler and start workers:
//
//	queue.Register(workers.TypeSendEmail, emailHandler)
//	queue.Start(ctx)    // non-blocking — starts goroutines
//	defer queue.Stop()  // blocks until in-flight jobs finish
package workers

import "time"

// ── Job types ─────────────────────────────────────────────────────────────────

// Well-known job type constants.
const (
	TypeSendEmail         = "send_email"
	TypeSendSMS           = "send_sms"
	TypePollMpesaPayment  = "poll_mpesa_payment"
	TypeSendReminder      = "send_reminder"
	TypeSendNotification  = "send_notification"
	TypeSendWebhook       = "send_webhook"
)

// ── Job status ────────────────────────────────────────────────────────────────

type JobStatus string

const (
	StatusPending   JobStatus = "pending"
	StatusRunning   JobStatus = "running"
	StatusCompleted JobStatus = "completed"
	StatusFailed    JobStatus = "failed"
	StatusDead      JobStatus = "dead"
)

// ── Job record ────────────────────────────────────────────────────────────────

// Job is a queued unit of work as stored in the database.
type Job struct {
	ID       string
	TenantID *string

	Type    string
	Payload map[string]any

	RunAt       time.Time
	Status      JobStatus
	Attempts    int
	MaxAttempts int
	LastError   *string

	StartedAt   *time.Time
	CompletedAt *time.Time

	IdempotencyKey *string
	CreatedBy      *string

	CreatedAt time.Time
	UpdatedAt time.Time
}

// ── Enqueue options ───────────────────────────────────────────────────────────

// EnqueueOptions controls how a job is enqueued.
type EnqueueOptions struct {
	// TenantID scopes the job to a specific tenant (nil for system jobs).
	TenantID *string

	// RunAt schedules the job for a future time.  Zero value = immediate.
	RunAt time.Time

	// MaxAttempts overrides the default retry count (3).
	MaxAttempts int

	// IdempotencyKey prevents duplicate jobs of the same type.
	// If a pending/running job with the same type+key exists, Enqueue is a no-op.
	IdempotencyKey *string

	// CreatedBy records which service or user created the job.
	CreatedBy string
}

// ── Well-known payload structs ────────────────────────────────────────────────
// These are marshalled into jobs.payload JSONB.

// EmailPayload is the payload for TypeSendEmail jobs.
type EmailPayload struct {
	To      string `json:"to"`
	Subject string `json:"subject"`
	Body    string `json:"body"`
	IsHTML  bool   `json:"is_html"`
}

// SMSPayload is the payload for TypeSendSMS jobs.
type SMSPayload struct {
	Phone   string `json:"phone"`
	Message string `json:"message"`
}

// MpesaPollPayload is the payload for TypePollMpesaPayment jobs.
// Used to check the status of a payment that has not received a callback.
type MpesaPollPayload struct {
	PaymentID         string `json:"payment_id"`
	CheckoutRequestID string `json:"checkout_request_id"`
}

// ReminderPayload is the payload for TypeSendReminder jobs.
type ReminderPayload struct {
	TenantID   string `json:"tenant_id"`
	UserID     string `json:"user_id"`
	ResourceType string `json:"resource_type"` // "hire_booking" | "service_job"
	ResourceID string `json:"resource_id"`
	Message    string `json:"message"`
}

// NotificationPayload is the payload for TypeSendNotification jobs.
type NotificationPayload struct {
	TenantID     string  `json:"tenant_id"`
	UserID       string  `json:"user_id"`
	Type         string  `json:"type"`
	Title        string  `json:"title"`
	Body         string  `json:"body"`
	ResourceType *string `json:"resource_type,omitempty"`
	ResourceID   *string `json:"resource_id,omitempty"`
}

// WebhookPayload is the payload for TypeSendWebhook jobs.
type WebhookPayload struct {
	URL     string            `json:"url"`
	Method  string            `json:"method"`
	Headers map[string]string `json:"headers"`
	Body    string            `json:"body"`
}
