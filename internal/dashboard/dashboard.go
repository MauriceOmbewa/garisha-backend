// Package dashboard provides summary statistics, KPI cards, and chart data
// for the tenant home screen.  All queries are read-only aggregations over
// existing domain tables — no new tables are required.
//
// Endpoints are designed for a typical SPA dashboard:
//   - /api/v1/dashboard/summary  → KPI cards (counts + revenue totals)
//   - /api/v1/dashboard/charts   → time-series data for line/bar charts
//   - /api/v1/dashboard/activity → recent transactions across all modules
package dashboard

import "time"

// ── KPI summary ───────────────────────────────────────────────────────────────

// Summary is the top-level dashboard payload returned to the frontend.
// It contains all KPI cards needed to populate the home screen without
// making multiple round-trips.
type Summary struct {
	// Vehicles
	TotalVehicles     int `json:"total_vehicles"`
	AvailableVehicles int `json:"available_vehicles"`

	// Customers
	TotalCustomers  int `json:"total_customers"`
	ActiveCustomers int `json:"active_customers"`

	// Hire
	ActiveBookings    int     `json:"active_bookings"`    // pending + confirmed + active
	BookingsThisMonth int     `json:"bookings_this_month"`
	HireRevenueMonth  float64 `json:"hire_revenue_month"` // completed bookings this month

	// Sales
	SalesThisMonth    int     `json:"sales_this_month"`
	SalesRevenueMonth float64 `json:"sales_revenue_month"` // completed sales this month

	// Service
	OpenServiceJobs      int     `json:"open_service_jobs"` // pending + in_progress + awaiting_parts
	ServiceRevenueMonth  float64 `json:"service_revenue_month"`

	// Finance
	IncomeThisMonth   float64 `json:"income_this_month"`
	ExpenseThisMonth  float64 `json:"expense_this_month"`
	NetBalanceMonth   float64 `json:"net_balance_month"`

	// Payments
	PendingPayments   int     `json:"pending_payments"`
	CollectedThisMonth float64 `json:"collected_this_month"`

	// Inventory
	LowStockItems int `json:"low_stock_items"` // items at/below reorder level

	// Notifications
	UnreadNotifications int `json:"unread_notifications"`

	// Period context
	PeriodStart time.Time `json:"period_start"`
	PeriodEnd   time.Time `json:"period_end"`
}

// ── Charts data ───────────────────────────────────────────────────────────────

// ChartsData groups all time-series datasets for the dashboard charts.
type ChartsData struct {
	// Revenue trend: hire + sales + service combined, daily for last 30 days.
	RevenueTrend []DailyRevenue `json:"revenue_trend"`

	// Booking trend: number of bookings created per day (last 30 days).
	BookingTrend []DailyCount `json:"booking_trend"`

	// Vehicle status distribution (for a donut/pie chart).
	VehicleStatusDist []StatusShare `json:"vehicle_status_distribution"`

	// Payment method distribution (for a pie chart).
	PaymentMethodDist []MethodShare `json:"payment_method_distribution"`

	// Monthly finance comparison: income vs expenses for last 6 months.
	MonthlyFinance []MonthlyFinance `json:"monthly_finance"`
}

// ── Recent activity ───────────────────────────────────────────────────────────

// Activity is a unified recent-event item shown in the dashboard feed.
type Activity struct {
	ID           string    `json:"id"`
	Type         string    `json:"type"`         // "booking" | "sale" | "service" | "payment"
	Description  string    `json:"description"`
	Amount       *float64  `json:"amount,omitempty"`
	Status       string    `json:"status"`
	ResourceID   string    `json:"resource_id"`
	CreatedAt    time.Time `json:"created_at"`
}

// ── Chart sub-types ───────────────────────────────────────────────────────────

type DailyRevenue struct {
	Date    string  `json:"date"`    // "YYYY-MM-DD"
	Hire    float64 `json:"hire"`
	Sales   float64 `json:"sales"`
	Service float64 `json:"service"`
	Total   float64 `json:"total"`
}

type DailyCount struct {
	Date  string `json:"date"`
	Count int    `json:"count"`
}

type StatusShare struct {
	Status  string  `json:"status"`
	Count   int     `json:"count"`
	Percent float64 `json:"percent"`
}

type MethodShare struct {
	Method  string  `json:"method"`
	Count   int     `json:"count"`
	Total   float64 `json:"total"`
	Percent float64 `json:"percent"`
}

type MonthlyFinance struct {
	Month   string  `json:"month"` // "YYYY-MM"
	Income  float64 `json:"income"`
	Expense float64 `json:"expense"`
	Net     float64 `json:"net"`
}
