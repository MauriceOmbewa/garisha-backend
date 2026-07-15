// Package finance is the domain module for the tenant financial ledger.
// It manages income and expense records grouped by categories, with optional
// traceability back to the hire, sales, and service modules.
package finance

import "time"

// ── Enums ─────────────────────────────────────────────────────────────────────

// EntryType distinguishes income from expense records and categories.
type EntryType string

const (
	EntryTypeIncome  EntryType = "income"
	EntryTypeExpense EntryType = "expense"
)

// PaymentMethod enumerates accepted payment channels for a finance record.
type PaymentMethod string

const (
	PaymentMethodCash         PaymentMethod = "cash"
	PaymentMethodMpesa        PaymentMethod = "mpesa"
	PaymentMethodBankTransfer PaymentMethod = "bank_transfer"
	PaymentMethodCard         PaymentMethod = "card"
	PaymentMethodOther        PaymentMethod = "other"
)

// ── Category entity ───────────────────────────────────────────────────────────

// Category is a tenant-defined label used to group income/expense records.
type Category struct {
	ID          string
	TenantID    string
	Name        string
	Type        EntryType // income | expense
	Description *string
	IsActive    bool
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

// ── Record entity ─────────────────────────────────────────────────────────────

// Record is a single income or expense entry in the tenant ledger.
type Record struct {
	ID         string
	TenantID   string
	CategoryID string

	Type   EntryType // income | expense
	Amount float64   // always positive

	// Optional source-transaction references (at most one should be set)
	HireBookingID *string
	SaleID        *string
	ServiceJobID  *string

	Description     string
	TransactionDate time.Time // the date the money moved

	PaymentMethod *string
	Reference     *string // receipt number, M-PESA code, bank ref, etc.

	CreatedBy *string
	Notes     *string

	CreatedAt time.Time
	UpdatedAt time.Time
}

// ── Summary ───────────────────────────────────────────────────────────────────

// LedgerSummary holds aggregated totals for a given filter period.
type LedgerSummary struct {
	TotalIncome   float64 `json:"total_income"`
	TotalExpenses float64 `json:"total_expenses"`
	NetBalance    float64 `json:"net_balance"` // income − expenses
}
