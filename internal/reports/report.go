// Package reports provides aggregated data endpoints for hire bookings,
// vehicle sales, service jobs, and finance records.
// All queries are read-only views over existing domain tables — no new
// tables are created for this module.
package reports

import "time"

// ── Shared types ──────────────────────────────────────────────────────────────

// DateRange is a common filter applied to all report queries.
type DateRange struct {
	From *time.Time
	To   *time.Time
}

// ── Hire report ───────────────────────────────────────────────────────────────

// HireSummary aggregates hire booking data for a period.
type HireSummary struct {
	TotalBookings    int     `json:"total_bookings"`
	ByStatus         []StatusCount `json:"by_status"`
	TotalRevenue     float64 `json:"total_revenue"`    // sum of final_amount for completed
	AverageDuration  float64 `json:"avg_duration_days"` // avg total_days
	TopVehicles      []VehicleCount `json:"top_vehicles"`
}

// ── Sales report ──────────────────────────────────────────────────────────────

// SalesSummary aggregates vehicle sale data for a period.
type SalesSummary struct {
	TotalSales       int     `json:"total_sales"`
	ByStatus         []StatusCount `json:"by_status"`
	TotalRevenue     float64 `json:"total_revenue"` // sum of final_amount for completed
	AveragePrice     float64 `json:"avg_agreed_price"`
	TopVehicles      []VehicleCount `json:"top_vehicles"`
}

// ── Service report ────────────────────────────────────────────────────────────

// ServiceSummary aggregates service job data for a period.
type ServiceSummary struct {
	TotalJobs        int     `json:"total_jobs"`
	ByStatus         []StatusCount `json:"by_status"`
	ByJobType        []TypeCount   `json:"by_job_type"`
	TotalRevenue     float64 `json:"total_revenue"` // sum of final_amount for completed
	AverageJobValue  float64 `json:"avg_job_value"`
	TopMechanics     []MechanicCount `json:"top_mechanics"`
}

// ── Finance report ────────────────────────────────────────────────────────────

// FinanceSummary aggregates ledger data for a period.
type FinanceSummary struct {
	TotalIncome   float64 `json:"total_income"`
	TotalExpenses float64 `json:"total_expenses"`
	NetBalance    float64 `json:"net_balance"`
	ByCategory    []CategoryAmount `json:"by_category"`
	ByMonth       []MonthlyAmount  `json:"by_month"`
}

// ── Payment report ────────────────────────────────────────────────────────────

// PaymentSummary aggregates payment data for a period.
type PaymentSummary struct {
	TotalCollected   float64 `json:"total_collected"` // sum of completed payments
	ByMethod         []MethodAmount `json:"by_method"`
	ByStatus         []StatusCount  `json:"by_status"`
	MpesaSuccess     int     `json:"mpesa_success_count"`
	MpesaFailure     int     `json:"mpesa_failure_count"`
}

// ── Shared sub-types ──────────────────────────────────────────────────────────

type StatusCount struct {
	Status string `json:"status"`
	Count  int    `json:"count"`
}

type TypeCount struct {
	Type  string `json:"type"`
	Count int    `json:"count"`
}

type VehicleCount struct {
	VehicleID string `json:"vehicle_id"`
	Label     string `json:"label"` // "Make Model Year"
	Count     int    `json:"count"`
}

type MechanicCount struct {
	MechanicID string `json:"mechanic_id"`
	Name       string `json:"name"`
	JobCount   int    `json:"job_count"`
}

type CategoryAmount struct {
	CategoryID   string  `json:"category_id"`
	CategoryName string  `json:"category_name"`
	Type         string  `json:"type"` // income | expense
	Total        float64 `json:"total"`
}

type MonthlyAmount struct {
	Month   string  `json:"month"` // "YYYY-MM"
	Income  float64 `json:"income"`
	Expense float64 `json:"expense"`
	Net     float64 `json:"net"`
}

type MethodAmount struct {
	Method string  `json:"method"`
	Total  float64 `json:"total"`
	Count  int     `json:"count"`
}
