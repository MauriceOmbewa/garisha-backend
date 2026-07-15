// Package sales is the domain module for vehicle-sale transaction management.
// A sale record links a vehicle to a buyer (customer), captures agreed
// pricing, and progresses through a status lifecycle:
//
//	pending → reserved → completed
//	        ↘ cancelled (from pending or reserved)
package sales

import "time"

// Sale is the full vehicle-sale entity.
type Sale struct {
	ID         string
	TenantID   string
	VehicleID  string
	CustomerID string

	// Pricing
	AskingPrice    float64 // listed price at time of sale
	AgreedPrice    float64 // negotiated price
	DepositAmount  float64
	DiscountAmount float64
	FinalAmount    float64 // AgreedPrice − DiscountAmount

	// Payment
	PaymentMethod *string // 'cash' | 'mpesa' | 'bank_transfer' | 'finance' | 'other'
	PaymentTerms  *string

	// Key dates
	SaleDate   time.Time  // when the deal was agreed (DATE)
	HandoverAt *time.Time // when keys were physically handed over

	// Lifecycle
	Status SaleStatus

	// Reference numbers
	InvoiceNumber *string
	ContractRef   *string

	// Staff who recorded the sale
	CreatedBy *string

	Notes *string

	CreatedAt time.Time
	UpdatedAt time.Time
}

// SaleStatus enumerates the valid lifecycle states for a vehicle sale.
type SaleStatus string

const (
	SaleStatusPending   SaleStatus = "pending"
	SaleStatusReserved  SaleStatus = "reserved"
	SaleStatusCompleted SaleStatus = "completed"
	SaleStatusCancelled SaleStatus = "cancelled"
)

// IsTerminal reports whether a status is a final (non-changeable) state.
func (s SaleStatus) IsTerminal() bool {
	return s == SaleStatusCompleted || s == SaleStatusCancelled
}

// validTransitions maps each status to the statuses it is allowed to
// transition to.
var validTransitions = map[SaleStatus][]SaleStatus{
	SaleStatusPending:   {SaleStatusReserved, SaleStatusCancelled},
	SaleStatusReserved:  {SaleStatusCompleted, SaleStatusCancelled},
	SaleStatusCompleted: {},
	SaleStatusCancelled: {},
}

// CanTransitionTo reports whether a transition from s → next is allowed.
func (s SaleStatus) CanTransitionTo(next SaleStatus) bool {
	for _, a := range validTransitions[s] {
		if a == next {
			return true
		}
	}
	return false
}

// PaymentMethod enumerates accepted payment methods.
type PaymentMethod string

const (
	PaymentMethodCash         PaymentMethod = "cash"
	PaymentMethodMpesa        PaymentMethod = "mpesa"
	PaymentMethodBankTransfer PaymentMethod = "bank_transfer"
	PaymentMethodFinance      PaymentMethod = "finance"
	PaymentMethodOther        PaymentMethod = "other"
)
