package dashboard

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Repository runs all dashboard aggregate queries.
type Repository struct {
	db *pgxpool.Pool
}

// NewRepository creates a Repository.
func NewRepository(db *pgxpool.Pool) *Repository {
	return &Repository{db: db}
}

// ── Summary ───────────────────────────────────────────────────────────────────

// GetSummary queries all KPI values for the current calendar month.
func (r *Repository) GetSummary(ctx context.Context, tenantID, userID string) (Summary, error) {
	now := time.Now().UTC()
	monthStart := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC)
	monthEnd := monthStart.AddDate(0, 1, 0).Add(-time.Nanosecond)

	var s Summary
	s.PeriodStart = monthStart
	s.PeriodEnd = monthEnd

	// ── Vehicles ──────────────────────────────────────────────────────────────
	err := r.db.QueryRow(ctx, `
		SELECT
		    COUNT(*),
		    COUNT(*) FILTER (WHERE status = 'available')
		FROM vehicles WHERE tenant_id = $1`, tenantID).
		Scan(&s.TotalVehicles, &s.AvailableVehicles)
	if err != nil {
		return s, fmt.Errorf("dashboard: vehicles: %w", err)
	}

	// ── Customers ─────────────────────────────────────────────────────────────
	err = r.db.QueryRow(ctx, `
		SELECT COUNT(*), COUNT(*) FILTER (WHERE is_active = TRUE)
		FROM customers WHERE tenant_id = $1`, tenantID).
		Scan(&s.TotalCustomers, &s.ActiveCustomers)
	if err != nil {
		return s, fmt.Errorf("dashboard: customers: %w", err)
	}

	// ── Hire ──────────────────────────────────────────────────────────────────
	err = r.db.QueryRow(ctx, `
		SELECT
		    COUNT(*) FILTER (WHERE status IN ('pending','confirmed','active')),
		    COUNT(*) FILTER (WHERE start_date >= $2 AND start_date <= $3),
		    COALESCE(SUM(final_amount) FILTER (WHERE status = 'completed' AND start_date >= $2 AND start_date <= $3), 0)
		FROM hire_bookings WHERE tenant_id = $1`, tenantID, monthStart, monthEnd).
		Scan(&s.ActiveBookings, &s.BookingsThisMonth, &s.HireRevenueMonth)
	if err != nil {
		return s, fmt.Errorf("dashboard: hire: %w", err)
	}

	// ── Sales ─────────────────────────────────────────────────────────────────
	err = r.db.QueryRow(ctx, `
		SELECT
		    COUNT(*) FILTER (WHERE sale_date >= $2 AND sale_date <= $3),
		    COALESCE(SUM(final_amount) FILTER (WHERE status = 'completed' AND sale_date >= $2 AND sale_date <= $3), 0)
		FROM vehicle_sales WHERE tenant_id = $1`, tenantID, monthStart, monthEnd).
		Scan(&s.SalesThisMonth, &s.SalesRevenueMonth)
	if err != nil {
		return s, fmt.Errorf("dashboard: sales: %w", err)
	}

	// ── Service ───────────────────────────────────────────────────────────────
	err = r.db.QueryRow(ctx, `
		SELECT
		    COUNT(*) FILTER (WHERE status IN ('pending','in_progress','awaiting_parts')),
		    COALESCE(SUM(final_amount) FILTER (WHERE status = 'completed' AND received_at >= $2 AND received_at <= $3), 0)
		FROM service_jobs WHERE tenant_id = $1`, tenantID, monthStart, monthEnd).
		Scan(&s.OpenServiceJobs, &s.ServiceRevenueMonth)
	if err != nil {
		return s, fmt.Errorf("dashboard: service: %w", err)
	}

	// ── Finance ───────────────────────────────────────────────────────────────
	err = r.db.QueryRow(ctx, `
		SELECT
		    COALESCE(SUM(amount) FILTER (WHERE type = 'income'),  0),
		    COALESCE(SUM(amount) FILTER (WHERE type = 'expense'), 0)
		FROM finance_records
		WHERE tenant_id = $1 AND transaction_date >= $2 AND transaction_date <= $3`,
		tenantID, monthStart, monthEnd).
		Scan(&s.IncomeThisMonth, &s.ExpenseThisMonth)
	if err != nil {
		return s, fmt.Errorf("dashboard: finance: %w", err)
	}
	s.NetBalanceMonth = s.IncomeThisMonth - s.ExpenseThisMonth

	// ── Payments ──────────────────────────────────────────────────────────────
	err = r.db.QueryRow(ctx, `
		SELECT
		    COUNT(*) FILTER (WHERE status = 'pending'),
		    COALESCE(SUM(amount) FILTER (WHERE status = 'completed' AND created_at >= $2 AND created_at <= $3), 0)
		FROM payments WHERE tenant_id = $1`, tenantID, monthStart, monthEnd).
		Scan(&s.PendingPayments, &s.CollectedThisMonth)
	if err != nil {
		return s, fmt.Errorf("dashboard: payments: %w", err)
	}

	// ── Inventory ─────────────────────────────────────────────────────────────
	err = r.db.QueryRow(ctx, `
		SELECT COUNT(*) FROM inventory_items
		WHERE tenant_id = $1 AND is_active = TRUE AND quantity <= reorder_level`,
		tenantID).Scan(&s.LowStockItems)
	if err != nil {
		return s, fmt.Errorf("dashboard: inventory: %w", err)
	}

	// ── Unread notifications ──────────────────────────────────────────────────
	if userID != "" {
		err = r.db.QueryRow(ctx, `
			SELECT COUNT(*) FROM notifications
			WHERE tenant_id = $1 AND user_id = $2 AND is_read = FALSE`,
			tenantID, userID).Scan(&s.UnreadNotifications)
		if err != nil {
			return s, fmt.Errorf("dashboard: notifications: %w", err)
		}
	}

	return s, nil
}

// ── Charts ────────────────────────────────────────────────────────────────────

// GetCharts returns time-series and distribution data for dashboard charts.
func (r *Repository) GetCharts(ctx context.Context, tenantID string) (ChartsData, error) {
	var charts ChartsData

	// ── Daily revenue (last 30 days) ──────────────────────────────────────────
	rows, err := r.db.Query(ctx, `
		WITH dates AS (
		    SELECT generate_series(
		        CURRENT_DATE - INTERVAL '29 days',
		        CURRENT_DATE,
		        INTERVAL '1 day'
		    )::DATE AS d
		),
		hire_rev AS (
		    SELECT start_date::DATE AS d, COALESCE(SUM(final_amount), 0) AS rev
		    FROM   hire_bookings
		    WHERE  tenant_id = $1 AND status = 'completed'
		    AND    start_date >= CURRENT_DATE - INTERVAL '29 days'
		    GROUP  BY d
		),
		sale_rev AS (
		    SELECT sale_date::DATE AS d, COALESCE(SUM(final_amount), 0) AS rev
		    FROM   vehicle_sales
		    WHERE  tenant_id = $1 AND status = 'completed'
		    AND    sale_date >= CURRENT_DATE - INTERVAL '29 days'
		    GROUP  BY d
		),
		svc_rev AS (
		    SELECT completed_at::DATE AS d, COALESCE(SUM(final_amount), 0) AS rev
		    FROM   service_jobs
		    WHERE  tenant_id = $1 AND status = 'completed'
		    AND    completed_at >= CURRENT_DATE - INTERVAL '29 days'
		    GROUP  BY d
		)
		SELECT
		    dates.d::TEXT,
		    COALESCE(hire_rev.rev, 0),
		    COALESCE(sale_rev.rev, 0),
		    COALESCE(svc_rev.rev, 0)
		FROM   dates
		LEFT   JOIN hire_rev ON hire_rev.d = dates.d
		LEFT   JOIN sale_rev ON sale_rev.d = dates.d
		LEFT   JOIN svc_rev  ON svc_rev.d  = dates.d
		ORDER  BY dates.d`, tenantID)
	if err != nil {
		return charts, fmt.Errorf("dashboard: revenue trend: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var dr DailyRevenue
		if err := rows.Scan(&dr.Date, &dr.Hire, &dr.Sales, &dr.Service); err != nil {
			return charts, err
		}
		dr.Total = dr.Hire + dr.Sales + dr.Service
		charts.RevenueTrend = append(charts.RevenueTrend, dr)
	}
	if err := rows.Err(); err != nil {
		return charts, err
	}

	// ── Daily booking count (last 30 days) ────────────────────────────────────
	brows, err := r.db.Query(ctx, `
		WITH dates AS (
		    SELECT generate_series(
		        CURRENT_DATE - INTERVAL '29 days',
		        CURRENT_DATE,
		        INTERVAL '1 day'
		    )::DATE AS d
		),
		counts AS (
		    SELECT created_at::DATE AS d, COUNT(*) AS cnt
		    FROM   hire_bookings
		    WHERE  tenant_id = $1
		    AND    created_at >= CURRENT_DATE - INTERVAL '29 days'
		    GROUP  BY d
		)
		SELECT dates.d::TEXT, COALESCE(counts.cnt, 0)
		FROM   dates
		LEFT   JOIN counts ON counts.d = dates.d
		ORDER  BY dates.d`, tenantID)
	if err != nil {
		return charts, fmt.Errorf("dashboard: booking trend: %w", err)
	}
	defer brows.Close()
	for brows.Next() {
		var dc DailyCount
		if err := brows.Scan(&dc.Date, &dc.Count); err != nil {
			return charts, err
		}
		charts.BookingTrend = append(charts.BookingTrend, dc)
	}
	if err := brows.Err(); err != nil {
		return charts, err
	}

	// ── Vehicle status distribution ───────────────────────────────────────────
	vrows, err := r.db.Query(ctx, `
		SELECT status, COUNT(*) AS cnt
		FROM   vehicles
		WHERE  tenant_id = $1
		GROUP  BY status`, tenantID)
	if err != nil {
		return charts, fmt.Errorf("dashboard: vehicle status dist: %w", err)
	}
	defer vrows.Close()

	var totalVehicles int
	var vshares []StatusShare
	for vrows.Next() {
		var ss StatusShare
		if err := vrows.Scan(&ss.Status, &ss.Count); err != nil {
			return charts, err
		}
		totalVehicles += ss.Count
		vshares = append(vshares, ss)
	}
	if err := vrows.Err(); err != nil {
		return charts, err
	}
	for i := range vshares {
		if totalVehicles > 0 {
			vshares[i].Percent = round2(float64(vshares[i].Count) / float64(totalVehicles) * 100)
		}
	}
	charts.VehicleStatusDist = vshares

	// ── Payment method distribution (completed, last 30 days) ─────────────────
	prows, err := r.db.Query(ctx, `
		SELECT payment_method,
		       COUNT(*) AS cnt,
		       COALESCE(SUM(amount), 0) AS total
		FROM   payments
		WHERE  tenant_id = $1
		AND    status = 'completed'
		AND    created_at >= CURRENT_DATE - INTERVAL '30 days'
		GROUP  BY payment_method
		ORDER  BY total DESC`, tenantID)
	if err != nil {
		return charts, fmt.Errorf("dashboard: payment method dist: %w", err)
	}
	defer prows.Close()

	var totalPayments float64
	var mshares []MethodShare
	for prows.Next() {
		var ms MethodShare
		if err := prows.Scan(&ms.Method, &ms.Count, &ms.Total); err != nil {
			return charts, err
		}
		totalPayments += ms.Total
		mshares = append(mshares, ms)
	}
	if err := prows.Err(); err != nil {
		return charts, err
	}
	for i := range mshares {
		if totalPayments > 0 {
			mshares[i].Percent = round2(mshares[i].Total / totalPayments * 100)
		}
	}
	charts.PaymentMethodDist = mshares

	// ── Monthly finance (last 6 months) ───────────────────────────────────────
	mrows, err := r.db.Query(ctx, `
		SELECT
		    TO_CHAR(transaction_date, 'YYYY-MM') AS month,
		    COALESCE(SUM(amount) FILTER (WHERE type = 'income'),  0) AS income,
		    COALESCE(SUM(amount) FILTER (WHERE type = 'expense'), 0) AS expense
		FROM finance_records
		WHERE tenant_id = $1
		AND   transaction_date >= DATE_TRUNC('month', CURRENT_DATE) - INTERVAL '5 months'
		GROUP BY month
		ORDER BY month ASC`, tenantID)
	if err != nil {
		return charts, fmt.Errorf("dashboard: monthly finance: %w", err)
	}
	defer mrows.Close()
	for mrows.Next() {
		var mf MonthlyFinance
		if err := mrows.Scan(&mf.Month, &mf.Income, &mf.Expense); err != nil {
			return charts, err
		}
		mf.Net = mf.Income - mf.Expense
		charts.MonthlyFinance = append(charts.MonthlyFinance, mf)
	}

	return charts, mrows.Err()
}

// ── Recent activity ───────────────────────────────────────────────────────────

// GetActivity returns the 20 most recent transactions across all modules.
func (r *Repository) GetActivity(ctx context.Context, tenantID string) ([]Activity, error) {
	// UNION of recent events from all four transaction domains.
	rows, err := r.db.Query(ctx, `
		(SELECT id::TEXT, 'booking' AS type,
		        'Hire booking ' || status AS description,
		        final_amount AS amount,
		        status, id::TEXT AS resource_id, created_at
		 FROM   hire_bookings WHERE tenant_id = $1
		 ORDER  BY created_at DESC LIMIT 5)
		UNION ALL
		(SELECT id::TEXT, 'sale',
		        'Vehicle sale ' || status,
		        final_amount, status, id::TEXT, created_at
		 FROM   vehicle_sales WHERE tenant_id = $1
		 ORDER  BY created_at DESC LIMIT 5)
		UNION ALL
		(SELECT id::TEXT, 'service',
		        'Service job (' || job_type || ') ' || status,
		        final_amount, status, id::TEXT, created_at
		 FROM   service_jobs WHERE tenant_id = $1
		 ORDER  BY created_at DESC LIMIT 5)
		UNION ALL
		(SELECT id::TEXT, 'payment',
		        'Payment via ' || payment_method || ' ' || status,
		        amount, status, id::TEXT, created_at
		 FROM   payments WHERE tenant_id = $1
		 ORDER  BY created_at DESC LIMIT 5)
		ORDER  BY created_at DESC
		LIMIT  20`, tenantID)
	if err != nil {
		return nil, fmt.Errorf("dashboard: activity: %w", err)
	}
	defer rows.Close()

	var result []Activity
	for rows.Next() {
		var a Activity
		var amount float64
		if err := rows.Scan(
			&a.ID, &a.Type, &a.Description,
			&amount, &a.Status, &a.ResourceID, &a.CreatedAt,
		); err != nil {
			return nil, err
		}
		a.Amount = &amount
		result = append(result, a)
	}

	return result, rows.Err()
}

// ── Helper ────────────────────────────────────────────────────────────────────

func round2(v float64) float64 {
	// Round to 2 decimal places.
	shifted := v * 100
	rounded := float64(int64(shifted + 0.5))
	return rounded / 100
}
