package workers

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
)

// HandlerFunc is the signature every job handler must implement.
// Return a non-nil error to mark the job as failed (and trigger retry).
type HandlerFunc func(ctx context.Context, job *Job) error

// ── Handler registry ──────────────────────────────────────────────────────────

// Registry maps job types to their handler functions.
type Registry map[string]HandlerFunc

// ── Built-in stub handlers ────────────────────────────────────────────────────
// These are no-op implementations that log the intent.
// Replace them with real integrations (SendGrid, Africa's Talking, etc.)
// by registering a different handler for the same job type.

// NewEmailHandler returns a stub email handler.
// Swap this out for a real SendGrid / AWS SES handler without touching
// any other code — just register a different HandlerFunc for TypeSendEmail.
func NewEmailHandler(log *slog.Logger) HandlerFunc {
	return func(ctx context.Context, job *Job) error {
		var p EmailPayload
		if err := decodePayload(job.Payload, &p); err != nil {
			return fmt.Errorf("email handler: %w", err)
		}

		log.Info("email job",
			"job_id",  job.ID,
			"to",      p.To,
			"subject", p.Subject,
		)

		// TODO: integrate real email provider (SendGrid, AWS SES, etc.)
		// Example:
		//   return emailClient.Send(ctx, p.To, p.Subject, p.Body)
		return nil
	}
}

// NewSMSHandler returns a stub SMS handler.
// Swap for Africa's Talking, Twilio, etc.
func NewSMSHandler(log *slog.Logger) HandlerFunc {
	return func(ctx context.Context, job *Job) error {
		var p SMSPayload
		if err := decodePayload(job.Payload, &p); err != nil {
			return fmt.Errorf("sms handler: %w", err)
		}

		log.Info("sms job",
			"job_id",  job.ID,
			"phone",   p.Phone,
			"message", p.Message,
		)

		// TODO: integrate real SMS provider (Africa's Talking, Twilio, etc.)
		return nil
	}
}

// NewMpesaPollHandler returns a handler that checks whether a pending
// M-PESA payment has been settled by querying the Daraja Query API.
// The real implementation should call daraja.QuerySTKPush and update
// the payment record's status accordingly.
func NewMpesaPollHandler(log *slog.Logger) HandlerFunc {
	return func(ctx context.Context, job *Job) error {
		var p MpesaPollPayload
		if err := decodePayload(job.Payload, &p); err != nil {
			return fmt.Errorf("mpesa poll handler: %w", err)
		}

		log.Info("mpesa poll job",
			"job_id",              job.ID,
			"payment_id",          p.PaymentID,
			"checkout_request_id", p.CheckoutRequestID,
		)

		// TODO: call mpesa.QuerySTKPush(ctx, p.CheckoutRequestID)
		// and update payments.status based on the result.
		return nil
	}
}

// NewReminderHandler returns a handler that sends hire or service reminders.
func NewReminderHandler(log *slog.Logger) HandlerFunc {
	return func(ctx context.Context, job *Job) error {
		var p ReminderPayload
		if err := decodePayload(job.Payload, &p); err != nil {
			return fmt.Errorf("reminder handler: %w", err)
		}

		log.Info("reminder job",
			"job_id",        job.ID,
			"user_id",       p.UserID,
			"resource_type", p.ResourceType,
			"resource_id",   p.ResourceID,
		)

		// TODO: dispatch push / in-app notification via notifications.Service.Send
		return nil
	}
}

// NewNotificationHandler creates in-app notification records.
// Receives a NotificationPayload and calls the notifications service.
func NewNotificationHandler(log *slog.Logger) HandlerFunc {
	return func(ctx context.Context, job *Job) error {
		var p NotificationPayload
		if err := decodePayload(job.Payload, &p); err != nil {
			return fmt.Errorf("notification handler: %w", err)
		}

		log.Info("notification job",
			"job_id",   job.ID,
			"user_id",  p.UserID,
			"type",     p.Type,
		)

		// TODO: call notificationsSvc.Send(ctx, notifications.SendInput{...})
		return nil
	}
}

// NewWebhookHandler returns a handler that fires HTTP webhooks.
func NewWebhookHandler(log *slog.Logger) HandlerFunc {
	return func(ctx context.Context, job *Job) error {
		var p WebhookPayload
		if err := decodePayload(job.Payload, &p); err != nil {
			return fmt.Errorf("webhook handler: %w", err)
		}

		log.Info("webhook job",
			"job_id", job.ID,
			"url",    p.URL,
			"method", p.Method,
		)

		// TODO: make HTTP call with retries
		return nil
	}
}

// ── Helper ────────────────────────────────────────────────────────────────────

// decodePayload re-encodes the raw map[string]any into the target struct.
// This avoids maintaining a separate raw JSON field on Job.
func decodePayload(raw map[string]any, dst any) error {
	b, err := json.Marshal(raw)
	if err != nil {
		return fmt.Errorf("re-marshal payload: %w", err)
	}

	if err := json.Unmarshal(b, dst); err != nil {
		return fmt.Errorf("unmarshal payload into %T: %w", dst, err)
	}

	return nil
}
