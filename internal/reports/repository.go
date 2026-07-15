package reports

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Repository runs all aggregate report queries.
type Repository struct {
	db *pgxpool.Pool
}

// NewRepository creates a Repository.
func NewRepository(db *pgxpool.Pool) *Repository {
	return &Repository{db: db}
}

// ── Hire ─────────────────────────────────────────────────────────────────────

func (r *Repository) HireSummary(ctx context.Context, tenantID string, dr DateRange) (HireSummary, error) {
	args, where := buildDateWhere("start_date", tenantID, dr)

	// Totals
	var summary HireSummary
	err := r.db.QueryRow(ctx, fmt.Sprintf(`
		SELECT
		    COUNT(*)                                                            AS total_bookings,
		    COALESCE(SUM(final_amount) FILTER (WHERE status = 'completed'), 0) AS total_revenue,
		    COALESCE(AVG(total_days),  0)                                       AS avg_duration
		FROM hire_bookings WHERE %s`, where), args...).
		Scan(&summary.TotalBookings, &summary.TotalRevenue, &summary.AverageDuration)
	if err != nil {
		return HireSummary{}, fmt.Errorf("reports: hire totals: %w", err)
	}

	// By status
	rows, err := r.db.Query(ctx, fmt.Sprintf(`
		SELECT status, COUNT(*) FROM hire_bookings
		WHERE %s GROUP BY status ORDER BY status`, where), args...)
	if err != nil {
		return HireSummary{}, fmt.Errorf("reports: hire by status: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var sc StatusCount
		if err := rows.Scan(&sc.Status, &sc.Count); err != nil {
			return HireSummary{}, err
		}
		summary.ByStatus = append(summary.ByStatus, sc)
	}
	if err := rows.Err(); err != nil {
		return HireSummary{}, err
	}

	// Top vehicles (by number of bookings)
	vargs, vwhere := buildDateWhere("hb.start_date", tenantID, dr)
	vrows, err := r.db.Query(ctx, fmt.Sprintf(`
		SELECT hb.vehicle_id,
		       CONCAT(v.make, ' ', v.model, ' ', v.year) AS label,
		       COUNT(*) AS cnt
		FROM   hire_bookings hb
		JOIN   vehicles v ON v.id = hb.vehicle_id
		WHERE  %s
		GROUP  BY hb.vehicle_id, v.make, v.model, v.year
		ORDER  BY cnt DESC
		LIMIT  5`, vwhere), vargs...)
	if err != nil {
		return HireSummary{}, fmt.Errorf("reports: hire top vehicles: %w", err)
	}
	defer vrows.Close()
	for vrows.Next() {
		var vc VehicleCount
		if err := vrows.Scan(&vc.VehicleID, &vc.Label, &vc.Count); err != nil {
			return HireSummary{}, err
		}
		summary.TopVehicles = append(summary.TopVehicles, vc)
	}

	return summary, vrows.Err()
}

// ── Sales ─────────────────────────────────────────────────────────────────────

func (r *Repository) SalesSummary(ctx context.Context, tenantID string, dr DateRange) (SalesSummary, error) {
	args, where := buildDateWhere("sale_date", tenantID, dr)

	var summary SalesSummary
	err := r.db.QueryRow(ctx, fmt.Sprintf(`
		SELECT
		    COUNT(*)                                                            AS total_sales,
		    COALESCE(SUM(final_amount) FILTER (WHERE status = 'completed'), 0) AS total_revenue,
		    COALESCE(AVG(agreed_price), 0)                                     AS avg_price
		FROM vehicle_sales WHERE %s`, where), args...).
		Scan(&summary.TotalSales, &summary.TotalRevenue, &summary.AveragePrice)
	if err != nil {
		return SalesSummary{}, fmt.Errorf("reports: sales totals: %w", err)
	}

	rows, err := r.db.Query(ctx, fmt.Sprintf(`
		SELECT status, COUNT(*) FROM vehicle_sales
		WHERE %s GROUP BY status ORDER BY status`, where), args...)
	if err != nil {
		return SalesSummary{}, fmt.Errorf("reports: sales by status: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var sc StatusCount
		if err := rows.Scan(&sc.Status, &sc.Count); err != nil {
			return SalesSummary{}, err
		}
		summary.ByStatus = append(summary.ByStatus, sc)
	}
	if err := rows.Err(); err != nil {
		return SalesSummary{}, err
	}

	// Top sold vehicles
	vargs, vwhere := buildDateWhere("vs.sale_date", tenantID, dr)
	vrows, err := r.db.Query(ctx, fmt.Sprintf(`
		SELECT vs.vehicle_id,
		       CONCAT(v.make, ' ', v.model, ' ', v.year) AS label,
		       COUNT(*) AS cnt
		FROM   vehicle_sales vs
		JOIN   vehicles v ON v.id = vs.vehicle_id
		WHERE  %s
		GROUP  BY vs.vehicle_id, v.make, v.model, v.year
		ORDER  BY cnt DESC
		LIMIT  5`, vwhere), vargs...)
	if err != nil {
		return SalesSummary{}, fmt.Errorf("reports: sales top vehicles: %w", err)
	}
	defer vrows.Close()
	for vrows.Next() {
		var vc VehicleCount
		if err := vrows.Scan(&vc.VehicleID, &vc.Label, &vc.Count); err != nil {
			return SalesSummary{}, err
		}
		summary.TopVehicles = append(summary.TopVehicles, vc)
	}

	return summary, vrows.Err()
}

// ── Service ───────────────────────────────────────────────────────────────────

func (r *Repository) ServiceSummary(ctx context.Context, tenantID string, dr DateRange) (ServiceSummary, error) {
	args, where := buildDateWhere("received_at", tenantID, dr)

	var summary ServiceSummary
	err := r.db.QueryRow(ctx, fmt.Sprintf(`
		SELECT
		    COUNT(*)                                                            AS total_jobs,
		    COALESCE(SUM(final_amount) FILTER (WHERE status = 'completed'), 0) AS total_revenue,
		    COALESCE(AVG(final_amount), 0)                                     AS avg_value
		FROM service_jobs WHERE %s`, where), args...).
		Scan(&summary.TotalJobs, &summary.TotalRevenue, &summary.AverageJobValue)
	if err != nil {
		return ServiceSummary{}, fmt.Errorf("reports: service totals: %w", err)
	}

	// By status
	rows, err := r.db.Query(ctx, fmt.Sprintf(`
		SELECT status, COUNT(*) FROM service_jobs
		WHERE %s GROUP BY status ORDER BY status`, where), args...)
	if err != nil {
		return ServiceSummary{}, fmt.Errorf("reports: service by status: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var sc StatusCount
		if err := rows.Scan(&sc.Status, &sc.Count); err != nil {
			return ServiceSummary{}, err
		}
		summary.ByStatus = append(summary.ByStatus, sc)
	}
	if err := rows.Err(); err != nil {
		return ServiceSummary{}, err
	}

	// By job type
	trows, err := r.db.Query(ctx, fmt.Sprintf(`
		SELECT job_type, COUNT(*) FROM service_jobs
		WHERE %s GROUP BY job_type ORDER BY job_type`, where), args...)
	if err != nil {
		return ServiceSummary{}, fmt.Errorf("reports: service by type: %w", err)
	}
	defer trows.Close()
	for trows.Next() {
		var tc TypeCount
		if err := trows.Scan(&tc.Type, &tc.Count); err != nil {
			return ServiceSummary{}, err
		}
		summary.ByJobType = append(summary.ByJobType, tc)
	}
	if err := trows.Err(); err != nil {
		return ServiceSummary{}, err
	}

	// Top mechanics
	margs, mwhere := buildDateWhere("sj.received_at", tenantID, dr)
	mrows, err := r.db.Query(ctx, fmt.Sprintf(`
		SELECT sj.mechanic_id,
		       COALESCE(u.name, 'Unknown') AS mechanic_name,
		       COUNT(*) AS job_count
		FROM   service_jobs sj
		LEFT   JOIN users u ON u.id = sj.mechanic_id
		WHERE  %s AND sj.mechanic_id IS NOT NULL
		GROUP  BY sj.mechanic_id, u.name
		ORDER  BY job_count DESC
		LIMIT  5`, mwhere), margs...)
	if err != nil {
		return ServiceSummary{}, fmt.Errorf("reports: service top mechanics: %w", err)
	}
	defer mrows.Close()
	for mrows.Next() {
		var mc MechanicCount
		if err := mrows.Scan(&mc.MechanicID, &mc.Name, &mc.JobCount); err != nil {
			return ServiceSummary{}, err
		}
		summary.TopMechanics = append(summary.TopMechanics, mc)
	}

	return summary, mrows.Err()
}

// ── Finance ───────────────────────────────────────────────────────────────────

func (r *Repository) FinanceSummary(ctx context.Context, tenantID string, dr DateRange) (FinanceSummary, error) {
	args, where := buildDateWhere("transaction_date", tenantID, dr)

	var summary FinanceSummary
	err := r.db.QueryRow(ctx, fmt.Sprintf(`
		SELECT
		    COALESCE(SUM(amount) FILTER (WHERE type = 'income'),  0) AS total_income,
		    COALESCE(SUM(amount) FILTER (WHERE type = 'expense'), 0) AS total_expenses
		FROM finance_records WHERE %s`, where), args...).
		Scan(&summary.TotalIncome, &summary.TotalExpenses)
	if err != nil {
		return FinanceSummary{}, fmt.Errorf("reports: finance totals: %w", err)
	}

	summary.NetBalance = summary.TotalIncome - summary.TotalExpenses

	// By category
	cargs, cwhere := buildDateWhere("fr.transaction_date", tenantID, dr)
	crows, err := r.db.Query(ctx, fmt.Sprintf(`
		SELECT fr.category_id, fc.name, fc.type,
		       COALESCE(SUM(fr.amount), 0) AS total
		FROM   finance_records fr
		JOIN   finance_categories fc ON fc.id = fr.category_id
		WHERE  %s
		GROUP  BY fr.category_id, fc.name, fc.type
		ORDER  BY total DESC`, cwhere), cargs...)
	if err != nil {
		return FinanceSummary{}, fmt.Errorf("reports: finance by category: %w", err)
	}
	defer crows.Close()
	for crows.Next() {
		var ca CategoryAmount
		if err := crows.Scan(&ca.CategoryID, &ca.CategoryName, &ca.Type, &ca.Total); err != nil {
			return FinanceSummary{}, err
		}
		summary.ByCategory = append(summary.ByCategory, ca)
	}
	if err := crows.Err(); err != nil {
		return FinanceSummary{}, err
	}

	// Monthly trend
	margs, mwhere := buildDateWhere("transaction_date", tenantID, dr)
	mrows, err := r.db.Query(ctx, fmt.Sprintf(`
		SELECT
		    TO_CHAR(transaction_date, 'YYYY-MM') AS month,
		    COALESCE(SUM(amount) FILTER (WHERE type = 'income'),  0) AS income,
		    COALESCE(SUM(amount) FILTER (WHERE type = 'expense'), 0) AS expense
		FROM finance_records
		WHERE %s
		GROUP BY month
		ORDER BY month ASC`, mwhere), margs...)
	if err != nil {
		return FinanceSummary{}, fmt.Errorf("reports: finance monthly: %w", err)
	}
	defer mrows.Close()
	for mrows.Next() {
		var ma MonthlyAmount
		if err := mrows.Scan(&ma.Month, &ma.Income, &ma.Expense); err != nil {
			return FinanceSummary{}, err
		}
		ma.Net = ma.Income - ma.Expense
		summary.ByMonth = append(summary.ByMonth, ma)
	}

	return summary, mrows.Err()
}

// ── Payments ──────────────────────────────────────────────────────────────────

func (r *Repository) PaymentSummary(ctx context.Context, tenantID string, dr DateRange) (PaymentSummary, error) {
	args, where := buildDateWhere("created_at", tenantID, dr)

	var summary PaymentSummary
	err := r.db.QueryRow(ctx, fmt.Sprintf(`
		SELECT
		    COALESCE(SUM(amount) FILTER (WHERE status = 'completed'), 0) AS total_collected,
		    COALESCE(COUNT(*)    FILTER (WHERE payment_method = 'mpesa' AND status = 'completed'), 0) AS mpesa_success,
		    COALESCE(COUNT(*)    FILTER (WHERE payment_method = 'mpesa' AND status = 'failed'),    0) AS mpesa_failure
		FROM payments WHERE %s`, where), args...).
		Scan(&summary.TotalCollected, &summary.MpesaSuccess, &summary.MpesaFailure)
	if err != nil {
		return PaymentSummary{}, fmt.Errorf("reports: payment totals: %w", err)
	}

	// By status
	srows, err := r.db.Query(ctx, fmt.Sprintf(`
		SELECT status, COUNT(*) FROM payments
		WHERE %s GROUP BY status ORDER BY status`, where), args...)
	if err != nil {
		return PaymentSummary{}, fmt.Errorf("reports: payment by status: %w", err)
	}
	defer srows.Close()
	for srows.Next() {
		var sc StatusCount
		if err := srows.Scan(&sc.Status, &sc.Count); err != nil {
			return PaymentSummary{}, err
		}
		summary.ByStatus = append(summary.ByStatus, sc)
	}
	if err := srows.Err(); err != nil {
		return PaymentSummary{}, err
	}

	// By payment method
	mrows, err := r.db.Query(ctx, fmt.Sprintf(`
		SELECT payment_method,
		       COALESCE(SUM(amount) FILTER (WHERE status = 'completed'), 0) AS total,
		       COUNT(*) AS cnt
		FROM   payments
		WHERE  %s
		GROUP  BY payment_method
		ORDER  BY total DESC`, where), args...)
	if err != nil {
		return PaymentSummary{}, fmt.Errorf("reports: payment by method: %w", err)
	}
	defer mrows.Close()
	for mrows.Next() {
		var ma MethodAmount
		if err := mrows.Scan(&ma.Method, &ma.Total, &ma.Count); err != nil {
			return PaymentSummary{}, err
		}
		summary.ByMethod = append(summary.ByMethod, ma)
	}

	return summary, mrows.Err()
}

// ── Helper ────────────────────────────────────────────────────────────────────

// buildDateWhere constructs a WHERE clause for tenant + optional date range.
// dateCol is the column used for date filtering (e.g. "start_date", "created_at").
func buildDateWhere(dateCol, tenantID string, dr DateRange) (args []any, where string) {
	args = []any{tenantID}
	where = "tenant_id = $1"

	if dr.From != nil {
		args = append(args, dr.From.UTC().Truncate(24*time.Hour))
		where += fmt.Sprintf(" AND %s >= $%d", dateCol, len(args))
	}

	if dr.To != nil {
		// Include the full To day.
		end := dr.To.UTC().Truncate(24 * time.Hour).Add(24*time.Hour - time.Nanosecond)
		args = append(args, end)
		where += fmt.Sprintf(" AND %s <= $%d", dateCol, len(args))
	}

	return args, where
}
