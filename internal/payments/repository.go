package payments

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Repository handles all database operations for the payments domain.
type Repository struct {
	db *pgxpool.Pool
}

// NewRepository creates a Repository backed by the given pool.
func NewRepository(db *pgxpool.Pool) *Repository {
	return &Repository{db: db}
}

// ── Column list ───────────────────────────────────────────────────────────────

const paymentCols = `
	id, tenant_id,
	hire_booking_id, sale_id, service_job_id,
	customer_id,
	payment_method, amount, currency, status,
	mpesa_phone, mpesa_checkout_req_id, mpesa_receipt_number,
	mpesa_result_code, mpesa_result_desc,
	reference, failure_reason,
	paid_at, created_by, notes,
	created_at, updated_at`

// ── Filters ───────────────────────────────────────────────────────────────────

// ListFilters narrows the payments list query.
type ListFilters struct {
	Status        *string
	Method        *string
	CustomerID    *string
	HireBookingID *string
	SaleID        *string
	ServiceJobID  *string
	FromDate      *time.Time
	ToDate        *time.Time
}

// ── Queries ───────────────────────────────────────────────────────────────────

// List returns payments for tenantID, optionally filtered.
func (r *Repository) List(ctx context.Context, tenantID string, f ListFilters) ([]*Payment, error) {
	args := []any{tenantID}
	conditions := []string{"tenant_id = $1"}

	if f.Status != nil {
		args = append(args, *f.Status)
		conditions = append(conditions, fmt.Sprintf("status = $%d", len(args)))
	}
	if f.Method != nil {
		args = append(args, *f.Method)
		conditions = append(conditions, fmt.Sprintf("payment_method = $%d", len(args)))
	}
	if f.CustomerID != nil {
		args = append(args, *f.CustomerID)
		conditions = append(conditions, fmt.Sprintf("customer_id = $%d", len(args)))
	}
	if f.HireBookingID != nil {
		args = append(args, *f.HireBookingID)
		conditions = append(conditions, fmt.Sprintf("hire_booking_id = $%d", len(args)))
	}
	if f.SaleID != nil {
		args = append(args, *f.SaleID)
		conditions = append(conditions, fmt.Sprintf("sale_id = $%d", len(args)))
	}
	if f.ServiceJobID != nil {
		args = append(args, *f.ServiceJobID)
		conditions = append(conditions, fmt.Sprintf("service_job_id = $%d", len(args)))
	}
	if f.FromDate != nil {
		args = append(args, *f.FromDate)
		conditions = append(conditions, fmt.Sprintf("created_at >= $%d", len(args)))
	}
	if f.ToDate != nil {
		args = append(args, *f.ToDate)
		conditions = append(conditions, fmt.Sprintf("created_at <= $%d", len(args)))
	}

	q := `SELECT ` + paymentCols + `
	      FROM   payments
	      WHERE  ` + strings.Join(conditions, " AND ") + `
	      ORDER  BY created_at DESC`

	rows, err := r.db.Query(ctx, q, args...)
	if err != nil {
		return nil, fmt.Errorf("payments: list: %w", err)
	}
	defer rows.Close()

	var result []*Payment
	for rows.Next() {
		p, err := scanPayment(rows)
		if err != nil {
			return nil, fmt.Errorf("payments: list scan: %w", err)
		}
		result = append(result, p)
	}

	return result, rows.Err()
}

// FindByID returns a payment by UUID.  Returns (nil, nil) when not found.
func (r *Repository) FindByID(ctx context.Context, id string) (*Payment, error) {
	q := `SELECT ` + paymentCols + ` FROM payments WHERE id = $1 LIMIT 1`

	p, err := scanPayment(r.db.QueryRow(ctx, q, id))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("payments: find by id: %w", err)
	}

	return p, nil
}

// FindByCheckoutRequestID finds a payment by its M-PESA CheckoutRequestID.
// Used to match incoming Daraja callbacks.  Returns (nil, nil) when not found.
func (r *Repository) FindByCheckoutRequestID(ctx context.Context, checkoutReqID string) (*Payment, error) {
	q := `SELECT ` + paymentCols + `
	      FROM   payments
	      WHERE  mpesa_checkout_req_id = $1
	      LIMIT  1`

	p, err := scanPayment(r.db.QueryRow(ctx, q, checkoutReqID))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("payments: find by checkout req id: %w", err)
	}

	return p, nil
}

// Create inserts a new payment record.
func (r *Repository) Create(ctx context.Context, p CreateParams) (*Payment, error) {
	q := `
		INSERT INTO payments (
		    tenant_id,
		    hire_booking_id, sale_id, service_job_id,
		    customer_id,
		    payment_method, amount, currency, status,
		    mpesa_phone, mpesa_checkout_req_id,
		    reference, created_by, notes
		) VALUES (
		    $1,
		    $2, $3, $4,
		    $5,
		    $6, $7, $8, $9,
		    $10, $11,
		    $12, $13, $14
		)
		RETURNING ` + paymentCols

	pay, err := scanPayment(r.db.QueryRow(ctx, q,
		p.TenantID,
		p.HireBookingID, p.SaleID, p.ServiceJobID,
		p.CustomerID,
		p.Method, p.Amount, p.Currency, p.Status,
		p.MpesaPhone, p.MpesaCheckoutReqID,
		p.Reference, p.CreatedBy, p.Notes,
	))
	if err != nil {
		return nil, fmt.Errorf("payments: create: %w", err)
	}

	return pay, nil
}

// Update applies a partial update to a payment.
func (r *Repository) Update(ctx context.Context, id string, p UpdateParams) (*Payment, error) {
	q := `
		UPDATE payments
		SET    status                = COALESCE($2,  status),
		       mpesa_receipt_number  = COALESCE($3,  mpesa_receipt_number),
		       mpesa_result_code     = COALESCE($4,  mpesa_result_code),
		       mpesa_result_desc     = COALESCE($5,  mpesa_result_desc),
		       reference             = COALESCE($6,  reference),
		       failure_reason        = COALESCE($7,  failure_reason),
		       paid_at               = COALESCE($8,  paid_at),
		       notes                 = COALESCE($9,  notes),
		       updated_at            = NOW()
		WHERE  id = $1
		RETURNING ` + paymentCols

	pay, err := scanPayment(r.db.QueryRow(ctx, q,
		id,
		p.Status,
		p.MpesaReceiptNumber, p.MpesaResultCode, p.MpesaResultDesc,
		p.Reference, p.FailureReason,
		p.PaidAt,
		p.Notes,
	))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("payments: update: %w", err)
	}

	return pay, nil
}

// SetCheckoutRequestID stores the M-PESA CheckoutRequestID on a payment record.
func (r *Repository) SetCheckoutRequestID(ctx context.Context, id, checkoutReqID string) error {
	_, err := r.db.Exec(ctx,
		`UPDATE payments SET mpesa_checkout_req_id = $2, updated_at = NOW() WHERE id = $1`,
		id, checkoutReqID,
	)
	if err != nil {
		return fmt.Errorf("payments: set checkout request id: %w", err)
	}

	return nil
}

type CreateParams struct {
	TenantID       string
	HireBookingID  *string
	SaleID         *string
	ServiceJobID   *string
	CustomerID     *string
	Method         PaymentMethod
	Amount         float64
	Currency       string
	Status         PaymentStatus
	MpesaPhone         *string
	MpesaCheckoutReqID *string
	Reference      *string
	CreatedBy      *string
	Notes          *string
}

type UpdateParams struct {
	Status             *PaymentStatus
	MpesaReceiptNumber *string
	MpesaResultCode    *int
	MpesaResultDesc    *string
	Reference          *string
	FailureReason      *string
	PaidAt             *time.Time
	Notes              *string
}

// ── Scanner ───────────────────────────────────────────────────────────────────

type rowScanner interface {
	Scan(dest ...any) error
}

func scanPayment(row rowScanner) (*Payment, error) {
	var p Payment
	var method, status string

	err := row.Scan(
		&p.ID, &p.TenantID,
		&p.HireBookingID, &p.SaleID, &p.ServiceJobID,
		&p.CustomerID,
		&method, &p.Amount, &p.Currency, &status,
		&p.MpesaPhone, &p.MpesaCheckoutReqID, &p.MpesaReceiptNumber,
		&p.MpesaResultCode, &p.MpesaResultDesc,
		&p.Reference, &p.FailureReason,
		&p.PaidAt, &p.CreatedBy, &p.Notes,
		&p.CreatedAt, &p.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}

	p.Method = PaymentMethod(method)
	p.Status = PaymentStatus(status)
	return &p, nil
}
