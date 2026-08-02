package service

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Repository handles all database operations for the service domain.
type Repository struct {
	db *pgxpool.Pool
}

// NewRepository creates a Repository backed by the given pool.
func NewRepository(db *pgxpool.Pool) *Repository {
	return &Repository{db: db}
}

// ── ListFilters ───────────────────────────────────────────────────────────────

// ListFilters narrows the service job list query.
type ListFilters struct {
	Status     *string
	VehicleID  *string
	CustomerID *string
	MechanicID *string
	JobType    *string
	FromDate   *time.Time // received_at >= FromDate
	ToDate     *time.Time // received_at <= ToDate
}

// ── Column lists ──────────────────────────────────────────────────────────────

const jobCols = `
	id, tenant_id, vehicle_id, customer_id, mechanic_id,
	job_type, status,
	received_at, due_date, completed_at,
	mileage_in,
	labour_total, parts_total, total_amount, discount_amount, final_amount,
	created_by, customer_notes, internal_notes,
	created_at, updated_at`

const itemCols = `
	id, job_id, tenant_id,
	item_type, description,
	quantity, unit_price, total_price,
	part_number,
	created_at, updated_at`

// ── Job Queries ───────────────────────────────────────────────────────────────

// List returns service jobs for tenantID, optionally filtered.
func (r *Repository) List(ctx context.Context, tenantID string, f ListFilters) ([]*Job, error) {
	args := []any{tenantID}
	conditions := []string{"tenant_id = $1"}

	if f.Status != nil {
		args = append(args, *f.Status)
		conditions = append(conditions, fmt.Sprintf("status = $%d", len(args)))
	}
	if f.VehicleID != nil {
		args = append(args, *f.VehicleID)
		conditions = append(conditions, fmt.Sprintf("vehicle_id = $%d", len(args)))
	}
	if f.CustomerID != nil {
		args = append(args, *f.CustomerID)
		conditions = append(conditions, fmt.Sprintf("customer_id = $%d", len(args)))
	}
	if f.MechanicID != nil {
		args = append(args, *f.MechanicID)
		conditions = append(conditions, fmt.Sprintf("mechanic_id = $%d", len(args)))
	}
	if f.JobType != nil {
		args = append(args, *f.JobType)
		conditions = append(conditions, fmt.Sprintf("job_type = $%d", len(args)))
	}
	if f.FromDate != nil {
		args = append(args, *f.FromDate)
		conditions = append(conditions, fmt.Sprintf("received_at >= $%d", len(args)))
	}
	if f.ToDate != nil {
		args = append(args, *f.ToDate)
		conditions = append(conditions, fmt.Sprintf("received_at <= $%d", len(args)))
	}

	q := `SELECT ` + jobCols + `
	      FROM   service_jobs
	      WHERE  ` + strings.Join(conditions, " AND ") + `
	      ORDER  BY received_at DESC, created_at DESC`

	rows, err := r.db.Query(ctx, q, args...)
	if err != nil {
		return nil, fmt.Errorf("service: list: %w", err)
	}
	defer rows.Close()

	var result []*Job
	for rows.Next() {
		j, err := scanJob(rows)
		if err != nil {
			return nil, fmt.Errorf("service: list scan: %w", err)
		}
		result = append(result, j)
	}

	return result, rows.Err()
}

// ── Enriched list ─────────────────────────────────────────────────────────────

// JobEnriched augments a Job with denormalised customer and vehicle fields.
type JobEnriched struct {
	Job
	CustomerName string
	VehicleMake  string
	VehicleModel string
	VehiclePlate *string
	VehicleType  string
}

// ListEnriched returns service jobs joined with customer and vehicle data.
func (r *Repository) ListEnriched(ctx context.Context, tenantID string, f ListFilters) ([]*JobEnriched, error) {
	args := []any{tenantID}
	conditions := []string{"sj.tenant_id = $1"}

	if f.Status != nil {
		args = append(args, *f.Status)
		conditions = append(conditions, fmt.Sprintf("sj.status = $%d", len(args)))
	}
	if f.VehicleID != nil {
		args = append(args, *f.VehicleID)
		conditions = append(conditions, fmt.Sprintf("sj.vehicle_id = $%d", len(args)))
	}
	if f.CustomerID != nil {
		args = append(args, *f.CustomerID)
		conditions = append(conditions, fmt.Sprintf("sj.customer_id = $%d", len(args)))
	}
	if f.MechanicID != nil {
		args = append(args, *f.MechanicID)
		conditions = append(conditions, fmt.Sprintf("sj.mechanic_id = $%d", len(args)))
	}
	if f.JobType != nil {
		args = append(args, *f.JobType)
		conditions = append(conditions, fmt.Sprintf("sj.job_type = $%d", len(args)))
	}
	if f.FromDate != nil {
		args = append(args, *f.FromDate)
		conditions = append(conditions, fmt.Sprintf("sj.received_at >= $%d", len(args)))
	}
	if f.ToDate != nil {
		args = append(args, *f.ToDate)
		conditions = append(conditions, fmt.Sprintf("sj.received_at <= $%d", len(args)))
	}

	q := `SELECT
		sj.id, sj.tenant_id, sj.vehicle_id, sj.customer_id, sj.mechanic_id,
		sj.job_type, sj.status,
		sj.received_at, sj.due_date, sj.completed_at,
		sj.mileage_in,
		sj.labour_total, sj.parts_total, sj.total_amount, sj.discount_amount, sj.final_amount,
		sj.created_by, sj.customer_notes, sj.internal_notes,
		sj.created_at, sj.updated_at,
		COALESCE(c.full_name, '') AS customer_name,
		COALESCE(v.make, '')      AS vehicle_make,
		COALESCE(v.model, '')     AS vehicle_model,
		v.plate_no                AS vehicle_plate,
		COALESCE(v.vehicle_type, '') AS vehicle_type
	FROM   service_jobs sj
	LEFT   JOIN customers c ON c.id = sj.customer_id
	LEFT   JOIN vehicles  v ON v.id = sj.vehicle_id
	WHERE  ` + strings.Join(conditions, " AND ") + `
	ORDER  BY sj.received_at DESC, sj.created_at DESC`

	rows, err := r.db.Query(ctx, q, args...)
	if err != nil {
		return nil, fmt.Errorf("service: list enriched: %w", err)
	}
	defer rows.Close()

	var result []*JobEnriched
	for rows.Next() {
		var je JobEnriched
		j := &je.Job
		var jobType, status string
		err := rows.Scan(
			&j.ID, &j.TenantID, &j.VehicleID, &j.CustomerID, &j.MechanicID,
			&jobType, &status,
			&j.ReceivedAt, &j.DueDate, &j.CompletedAt,
			&j.MileageIn,
			&j.LabourTotal, &j.PartsTotal, &j.TotalAmount, &j.DiscountAmount, &j.FinalAmount,
			&j.CreatedBy, &j.CustomerNotes, &j.InternalNotes,
			&j.CreatedAt, &j.UpdatedAt,
			&je.CustomerName, &je.VehicleMake, &je.VehicleModel,
			&je.VehiclePlate, &je.VehicleType,
		)
		if err != nil {
			return nil, fmt.Errorf("service: list enriched scan: %w", err)
		}
		j.JobType = JobType(jobType)
		j.Status = JobStatus(status)
		j.Items = []*JobItem{}
		result = append(result, &je)
	}
	return result, rows.Err()
}

// FindByID returns a job by UUID with its items.  Returns (nil, nil) when not found.
func (r *Repository) FindByID(ctx context.Context, id string) (*Job, error) {
	q := `SELECT ` + jobCols + `
	      FROM   service_jobs
	      WHERE  id = $1
	      LIMIT  1`

	j, err := scanJob(r.db.QueryRow(ctx, q, id))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("service: find by id: %w", err)
	}

	items, err := r.ListItems(ctx, id)
	if err != nil {
		return nil, err
	}

	j.Items = items
	return j, nil
}

// CreateJob inserts a new service job and returns the persisted record.
func (r *Repository) CreateJob(ctx context.Context, p CreateJobParams) (*Job, error) {
	q := `
		INSERT INTO service_jobs (
		    tenant_id, vehicle_id, customer_id, mechanic_id,
		    job_type, status,
		    received_at, due_date,
		    mileage_in,
		    created_by, customer_notes, internal_notes
		) VALUES (
		    $1, $2, $3, $4,
		    $5, $6,
		    $7, $8,
		    $9,
		    $10, $11, $12
		)
		RETURNING ` + jobCols

	j, err := scanJob(r.db.QueryRow(ctx, q,
		p.TenantID, p.VehicleID, p.CustomerID, p.MechanicID,
		p.JobType, p.Status,
		p.ReceivedAt, p.DueDate,
		p.MileageIn,
		p.CreatedBy, p.CustomerNotes, p.InternalNotes,
	))
	if err != nil {
		return nil, fmt.Errorf("service: create job: %w", err)
	}

	j.Items = []*JobItem{}
	return j, nil
}

// UpdateJob applies a partial update to a service job.
func (r *Repository) UpdateJob(ctx context.Context, id string, p UpdateJobParams) (*Job, error) {
	q := `
		UPDATE service_jobs
		SET    customer_id     = COALESCE($2,  customer_id),
		       mechanic_id     = COALESCE($3,  mechanic_id),
		       job_type        = COALESCE($4,  job_type),
		       status          = COALESCE($5,  status),
		       due_date        = COALESCE($6,  due_date),
		       completed_at    = COALESCE($7,  completed_at),
		       mileage_in      = COALESCE($8,  mileage_in),
		       labour_total    = COALESCE($9,  labour_total),
		       parts_total     = COALESCE($10, parts_total),
		       total_amount    = COALESCE($11, total_amount),
		       discount_amount = COALESCE($12, discount_amount),
		       final_amount    = COALESCE($13, final_amount),
		       customer_notes  = COALESCE($14, customer_notes),
		       internal_notes  = COALESCE($15, internal_notes),
		       updated_at      = NOW()
		WHERE  id = $1
		RETURNING ` + jobCols

	j, err := scanJob(r.db.QueryRow(ctx, q,
		id,
		p.CustomerID, p.MechanicID,
		p.JobType, p.Status,
		p.DueDate, p.CompletedAt,
		p.MileageIn,
		p.LabourTotal, p.PartsTotal, p.TotalAmount, p.DiscountAmount, p.FinalAmount,
		p.CustomerNotes, p.InternalNotes,
	))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("service: update job: %w", err)
	}

	return j, nil
}

// DeleteJob hard-deletes a service job (cascades to items).
func (r *Repository) DeleteJob(ctx context.Context, id string) error {
	ct, err := r.db.Exec(ctx, `DELETE FROM service_jobs WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("service: delete job: %w", err)
	}

	if ct.RowsAffected() == 0 {
		return pgx.ErrNoRows
	}

	return nil
}

// ── Item Queries ──────────────────────────────────────────────────────────────

// ListItems returns all line-items for a job.
func (r *Repository) ListItems(ctx context.Context, jobID string) ([]*JobItem, error) {
	q := `SELECT ` + itemCols + `
	      FROM   service_job_items
	      WHERE  job_id = $1
	      ORDER  BY created_at ASC`

	rows, err := r.db.Query(ctx, q, jobID)
	if err != nil {
		return nil, fmt.Errorf("service: list items: %w", err)
	}
	defer rows.Close()

	var items []*JobItem
	for rows.Next() {
		item, err := scanItem(rows)
		if err != nil {
			return nil, fmt.Errorf("service: list items scan: %w", err)
		}
		items = append(items, item)
	}

	if items == nil {
		items = []*JobItem{}
	}

	return items, rows.Err()
}

// FindItemByID returns a single job item.  Returns (nil, nil) when not found.
func (r *Repository) FindItemByID(ctx context.Context, itemID string) (*JobItem, error) {
	q := `SELECT ` + itemCols + `
	      FROM   service_job_items
	      WHERE  id = $1
	      LIMIT  1`

	item, err := scanItem(r.db.QueryRow(ctx, q, itemID))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("service: find item by id: %w", err)
	}

	return item, nil
}

// AddItem inserts a job item and recalculates the job's pricing totals
// atomically within a single transaction.
func (r *Repository) AddItem(ctx context.Context, p AddItemParams) (*JobItem, error) {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("service: add item begin tx: %w", err)
	}
	defer tx.Rollback(ctx) //nolint:errcheck

	totalPrice := roundPrice(p.Quantity * p.UnitPrice)

	insertQ := `
		INSERT INTO service_job_items (job_id, tenant_id, item_type, description, quantity, unit_price, total_price, part_number)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		RETURNING ` + itemCols

	item, err := scanItem(tx.QueryRow(ctx, insertQ,
		p.JobID, p.TenantID, p.ItemType, p.Description,
		p.Quantity, p.UnitPrice, totalPrice, p.PartNumber,
	))
	if err != nil {
		return nil, fmt.Errorf("service: insert item: %w", err)
	}

	if err := recalcTotals(ctx, tx, p.JobID, p.DiscountAmount); err != nil {
		return nil, err
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("service: add item commit: %w", err)
	}

	return item, nil
}

// UpdateItem updates a job item's details and recalculates job totals.
func (r *Repository) UpdateItem(ctx context.Context, itemID string, p UpdateItemParams) (*JobItem, error) {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("service: update item begin tx: %w", err)
	}
	defer tx.Rollback(ctx) //nolint:errcheck

	updateQ := `
		UPDATE service_job_items
		SET    item_type   = COALESCE($2, item_type),
		       description = COALESCE($3, description),
		       quantity    = COALESCE($4, quantity),
		       unit_price  = COALESCE($5, unit_price),
		       total_price = COALESCE($4, quantity) * COALESCE($5, unit_price),
		       part_number = COALESCE($6, part_number),
		       updated_at  = NOW()
		WHERE  id = $1
		RETURNING ` + itemCols

	item, err := scanItem(tx.QueryRow(ctx, updateQ,
		itemID,
		p.ItemType, p.Description,
		p.Quantity, p.UnitPrice,
		p.PartNumber,
	))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("service: update item: %w", err)
	}

	if err := recalcTotals(ctx, tx, item.JobID, p.DiscountAmount); err != nil {
		return nil, err
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("service: update item commit: %w", err)
	}

	return item, nil
}

// DeleteItem removes a job item and recalculates job totals.
func (r *Repository) DeleteItem(ctx context.Context, itemID string, discountAmount *float64) error {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("service: delete item begin tx: %w", err)
	}
	defer tx.Rollback(ctx) //nolint:errcheck

	// Get job_id before deleting.
	var jobID string
	if err := tx.QueryRow(ctx, `SELECT job_id FROM service_job_items WHERE id = $1`, itemID).Scan(&jobID); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return pgx.ErrNoRows
		}
		return fmt.Errorf("service: delete item get job id: %w", err)
	}

	if _, err := tx.Exec(ctx, `DELETE FROM service_job_items WHERE id = $1`, itemID); err != nil {
		return fmt.Errorf("service: delete item exec: %w", err)
	}

	if err := recalcTotals(ctx, tx, jobID, discountAmount); err != nil {
		return err
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("service: delete item commit: %w", err)
	}

	return nil
}

// ── Params ────────────────────────────────────────────────────────────────────

// CreateJobParams holds all fields required to insert a new service job.
type CreateJobParams struct {
	TenantID   string
	VehicleID  string
	CustomerID *string
	MechanicID *string
	JobType    JobType
	Status     JobStatus
	ReceivedAt time.Time
	DueDate    *time.Time
	MileageIn  *int
	CreatedBy  *string
	CustomerNotes *string
	InternalNotes *string
}

// UpdateJobParams holds nullable fields for a partial job update.
type UpdateJobParams struct {
	CustomerID *string
	MechanicID *string
	JobType    *JobType
	Status     *JobStatus
	DueDate    *time.Time
	CompletedAt *time.Time
	MileageIn  *int
	LabourTotal    *float64
	PartsTotal     *float64
	TotalAmount    *float64
	DiscountAmount *float64
	FinalAmount    *float64
	CustomerNotes *string
	InternalNotes *string
}

// AddItemParams holds all fields required to add an item to a job.
type AddItemParams struct {
	JobID       string
	TenantID    string
	ItemType    ItemType
	Description string
	Quantity    float64
	UnitPrice   float64
	PartNumber  *string
	// DiscountAmount is the current job-level discount for total recalc.
	DiscountAmount *float64
}

// UpdateItemParams holds nullable fields for a partial item update.
type UpdateItemParams struct {
	ItemType    *ItemType
	Description *string
	Quantity    *float64
	UnitPrice   *float64
	PartNumber  *string
	// DiscountAmount is the current job-level discount for total recalc.
	DiscountAmount *float64
}

// ── Scanners ──────────────────────────────────────────────────────────────────

type rowScanner interface {
	Scan(dest ...any) error
}

func scanJob(row rowScanner) (*Job, error) {
	var j Job
	var jobType, status string

	err := row.Scan(
		&j.ID, &j.TenantID, &j.VehicleID, &j.CustomerID, &j.MechanicID,
		&jobType, &status,
		&j.ReceivedAt, &j.DueDate, &j.CompletedAt,
		&j.MileageIn,
		&j.LabourTotal, &j.PartsTotal, &j.TotalAmount, &j.DiscountAmount, &j.FinalAmount,
		&j.CreatedBy, &j.CustomerNotes, &j.InternalNotes,
		&j.CreatedAt, &j.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}

	j.JobType = JobType(jobType)
	j.Status = JobStatus(status)
	return &j, nil
}

func scanItem(row rowScanner) (*JobItem, error) {
	var item JobItem
	var itemType string

	err := row.Scan(
		&item.ID, &item.JobID, &item.TenantID,
		&itemType, &item.Description,
		&item.Quantity, &item.UnitPrice, &item.TotalPrice,
		&item.PartNumber,
		&item.CreatedAt, &item.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}

	item.ItemType = ItemType(itemType)
	return &item, nil
}

// ── Internal helpers ──────────────────────────────────────────────────────────

type txQuerier interface {
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
	Exec(ctx context.Context, sql string, args ...any) (interface{ RowsAffected() int64 }, error)
}

// recalcTotals recomputes labour_total, parts_total, total_amount, and
// final_amount on the parent job by summing item rows — all within the
// caller's transaction.
func recalcTotals(ctx context.Context, tx pgx.Tx, jobID string, discountAmount *float64) error {
	const q = `
		UPDATE service_jobs j
		SET    labour_total    = COALESCE((SELECT SUM(total_price) FROM service_job_items WHERE job_id = j.id AND item_type = 'labour'),     0),
		       parts_total     = COALESCE((SELECT SUM(total_price) FROM service_job_items WHERE job_id = j.id AND item_type IN ('part','consumable')), 0),
		       total_amount    = COALESCE((SELECT SUM(total_price) FROM service_job_items WHERE job_id = j.id), 0),
		       discount_amount = COALESCE($2, discount_amount),
		       final_amount    = GREATEST(0,
		           COALESCE((SELECT SUM(total_price) FROM service_job_items WHERE job_id = j.id), 0)
		           - COALESCE($2, discount_amount)
		       ),
		       updated_at = NOW()
		WHERE  j.id = $1`

	if _, err := tx.Exec(ctx, q, jobID, discountAmount); err != nil {
		return fmt.Errorf("service: recalc totals: %w", err)
	}

	return nil
}

func roundPrice(v float64) float64 {
	// Round to 2 decimal places.
	// Uses integer arithmetic to avoid floating-point drift.
	return float64(int64(v*100+0.5)) / 100
}
