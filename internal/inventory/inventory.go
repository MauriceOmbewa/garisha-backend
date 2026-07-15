// Package inventory is the domain module for parts and stock management.
// It maintains a catalogue of inventory items per tenant, tracks stock levels,
// records every stock movement (usage, adjustment, receipt), and surfaces
// items that have fallen at or below their reorder threshold.
package inventory

import "time"

// Item is a stock catalogue entry for a tenant.
type Item struct {
	ID       string
	TenantID string

	Name        string
	SKU         *string
	Description *string
	Category    *string
	Unit        string // piece | litre | kg | metre | set | box | other

	// Stock levels
	Quantity     float64
	ReorderLevel float64 // alert fires when Quantity <= ReorderLevel
	ReorderQty   float64 // suggested restock quantity

	// Pricing
	UnitCost     float64
	SellingPrice float64

	IsActive bool

	// Supplier
	SupplierName  *string
	SupplierPhone *string
	SupplierEmail *string

	Notes *string

	CreatedAt time.Time
	UpdatedAt time.Time
}

// NeedsReorder reports whether this item has reached its reorder threshold.
func (item *Item) NeedsReorder() bool {
	return item.IsActive && item.Quantity <= item.ReorderLevel
}

// MovementType classifies a stock movement record.
type MovementType string

const (
	MovementUsage      MovementType = "usage"      // consumed in a service job
	MovementAdjustment MovementType = "adjustment" // manual stock correction
	MovementReceipt    MovementType = "receipt"    // stock received from supplier
)

// UsageRecord is a single stock-movement entry.
// Quantity is signed: negative for consumption/write-off, positive for receipt/adjustment.
type UsageRecord struct {
	ID       string
	TenantID string
	ItemID   string

	Movement MovementType
	Quantity float64 // signed

	ServiceJobID     *string
	ServiceJobItemID *string

	UnitCost  float64
	Reference *string
	Notes     *string

	CreatedBy *string
	CreatedAt time.Time
}

// Unit enumerates valid stock unit types.
type Unit string

const (
	UnitPiece  Unit = "piece"
	UnitLitre  Unit = "litre"
	UnitKg     Unit = "kg"
	UnitMetre  Unit = "metre"
	UnitSet    Unit = "set"
	UnitBox    Unit = "box"
	UnitOther  Unit = "other"
)
