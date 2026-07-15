// Package payments is the domain module for payment transaction management.
// It supports manual payment recording (cash, bank transfer, card) and
// M-PESA STK Push payments via the Safaricom Daraja API.
//
// Status lifecycle:
//
//	pending → completed | failed | cancelled
package payments

import "time"

// Payment is the full payment entity.
type Payment struct {
	ID       string
	TenantID string

	// Source transaction references (at most one set)
	HireBookingID *string
	SaleID        *string
	ServiceJobID  *string

	CustomerID *string

	Method   PaymentMethod
	Amount   float64
	Currency string

	Status PaymentStatus

	// M-PESA fields (nil for non-M-PESA payments)
	MpesaPhone          *string
	MpesaCheckoutReqID  *string // CheckoutRequestID from STK push
	MpesaReceiptNumber  *string
	MpesaResultCode     *int
	MpesaResultDesc     *string

	Reference     *string // generic ref (bank ref, cash receipt, etc.)
	FailureReason *string

	PaidAt    *time.Time
	CreatedBy *string
	Notes     *string

	CreatedAt time.Time
	UpdatedAt time.Time
}

// PaymentMethod enumerates the supported payment channels.
type PaymentMethod string

const (
	MethodMpesa        PaymentMethod = "mpesa"
	MethodCash         PaymentMethod = "cash"
	MethodBankTransfer PaymentMethod = "bank_transfer"
	MethodCard         PaymentMethod = "card"
	MethodOther        PaymentMethod = "other"
)

// PaymentStatus enumerates the lifecycle states of a payment.
type PaymentStatus string

const (
	StatusPending   PaymentStatus = "pending"
	StatusCompleted PaymentStatus = "completed"
	StatusFailed    PaymentStatus = "failed"
	StatusCancelled PaymentStatus = "cancelled"
)

// IsTerminal reports whether a status is final.
func (s PaymentStatus) IsTerminal() bool {
	return s == StatusCompleted || s == StatusFailed || s == StatusCancelled
}
