package service

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5"

	apperr "github.com/MauriceOmbewa/garisha-backend/internal/platform/errors"
	"github.com/MauriceOmbewa/garisha-backend/internal/platform/tenant"
)

// Service implements business logic for vehicle service job management.
type Service struct {
	repo *Repository
	log  *slog.Logger
}

// NewService creates a Service.
func NewService(repo *Repository, log *slog.Logger) *Service {
	return &Service{repo: repo, log: log}
}

// ── Job input types ───────────────────────────────────────────────────────────

// CreateJobInput carries the fields a caller provides when opening a service job.
type CreateJobInput struct {
	VehicleID  string
	CustomerID *string
	MechanicID *string
	JobType    string
	ReceivedAt *time.Time // defaults to now
	DueDate    *time.Time
	MileageIn  *int
	CreatedBy  *string
	CustomerNotes *string
	InternalNotes *string
}

// UpdateJobInput carries mutable fields for a partial job update.
// nil = leave existing value unchanged.
type UpdateJobInput struct {
	CustomerID    *string
	MechanicID    *string
	JobType       *string
	DueDate       *time.Time
	MileageIn     *int
	DiscountAmount *float64
	CustomerNotes *string
	InternalNotes *string
}

// ── Item input types ──────────────────────────────────────────────────────────

// AddItemInput carries the fields required to add a line-item to a job.
type AddItemInput struct {
	ItemType    string
	Description string
	Quantity    float64
	UnitPrice   float64
	PartNumber  *string
}

// UpdateItemInput carries mutable fields for a partial item update.
type UpdateItemInput struct {
	ItemType    *string
	Description *string
	Quantity    *float64
	UnitPrice   *float64
	PartNumber  *string
}

// ── Job service methods ───────────────────────────────────────────────────────

// ListEnriched returns service jobs joined with customer and vehicle data.
func (s *Service) ListEnriched(ctx context.Context, f ListFilters) ([]*JobEnriched, error) {
	tenantID := tenant.MustGetTenantID(ctx)
	jobs, err := s.repo.ListEnriched(ctx, tenantID, f)
	if err != nil {
		return nil, apperr.Internal("failed to list service jobs", err)
	}
	return jobs, nil
}

// List returns service jobs for the tenant in ctx, optionally filtered.
func (s *Service) List(ctx context.Context, f ListFilters) ([]*Job, error) {
	tenantID := tenant.MustGetTenantID(ctx)

	jobs, err := s.repo.List(ctx, tenantID, f)
	if err != nil {
		return nil, apperr.Internal("failed to list service jobs", err)
	}

	return jobs, nil
}

// GetByID returns a single service job (with items) scoped to the tenant in ctx.
func (s *Service) GetByID(ctx context.Context, id string) (*Job, error) {
	tenantID := tenant.MustGetTenantID(ctx)

	j, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return nil, apperr.Internal("failed to get service job", err)
	}

	if j == nil || j.TenantID != tenantID {
		return nil, apperr.NotFound("service job")
	}

	return j, nil
}

// CreateJob opens a new service job for a vehicle.
func (s *Service) CreateJob(ctx context.Context, in CreateJobInput) (*Job, error) {
	tenantID := tenant.MustGetTenantID(ctx)

	jobType, err := validateJobType(in.JobType)
	if err != nil {
		return nil, err
	}

	receivedAt := time.Now().UTC()
	if in.ReceivedAt != nil {
		receivedAt = *in.ReceivedAt
	}

	j, err := s.repo.CreateJob(ctx, CreateJobParams{
		TenantID:      tenantID,
		VehicleID:     in.VehicleID,
		CustomerID:    in.CustomerID,
		MechanicID:    in.MechanicID,
		JobType:       jobType,
		Status:        JobStatusPending,
		ReceivedAt:    receivedAt,
		DueDate:       in.DueDate,
		MileageIn:     in.MileageIn,
		CreatedBy:     in.CreatedBy,
		CustomerNotes: in.CustomerNotes,
		InternalNotes: in.InternalNotes,
	})
	if err != nil {
		return nil, apperr.Internal("failed to create service job", err)
	}

	s.log.Info("service job created",
		"job_id",     j.ID,
		"vehicle_id", in.VehicleID,
		"tenant_id",  tenantID,
	)

	return j, nil
}

// UpdateJob applies a partial update to a service job's details.
// Status transitions must go through UpdateStatus.
func (s *Service) UpdateJob(ctx context.Context, id string, in UpdateJobInput) (*Job, error) {
	tenantID := tenant.MustGetTenantID(ctx)

	existing, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return nil, apperr.Internal("failed to get service job", err)
	}

	if existing == nil || existing.TenantID != tenantID {
		return nil, apperr.NotFound("service job")
	}

	if existing.Status.IsTerminal() {
		return nil, apperr.BadRequest(fmt.Sprintf(
			"cannot update a service job that is %s", existing.Status,
		))
	}

	var jobTypePtr *JobType
	if in.JobType != nil {
		jt, err := validateJobType(*in.JobType)
		if err != nil {
			return nil, err
		}
		jobTypePtr = &jt
	}

	if in.DiscountAmount != nil && *in.DiscountAmount < 0 {
		return nil, apperr.BadRequest("discount_amount must be >= 0")
	}

	// Build pricing update: if discount changed, recalculate final_amount.
	var (
		discountAmount = existing.DiscountAmount
		finalAmount    = existing.FinalAmount
	)

	if in.DiscountAmount != nil {
		discountAmount = *in.DiscountAmount
		fa := roundPrice(max64(0, existing.TotalAmount-discountAmount))
		finalAmount = fa
	}

	j, err := s.repo.UpdateJob(ctx, id, UpdateJobParams{
		CustomerID:    in.CustomerID,
		MechanicID:    in.MechanicID,
		JobType:       jobTypePtr,
		DueDate:       in.DueDate,
		MileageIn:     in.MileageIn,
		DiscountAmount: &discountAmount,
		FinalAmount:   &finalAmount,
		CustomerNotes: in.CustomerNotes,
		InternalNotes: in.InternalNotes,
	})
	if err != nil {
		return nil, apperr.Internal("failed to update service job", err)
	}

	if j == nil {
		return nil, apperr.NotFound("service job")
	}

	// Reload with items.
	return s.GetByID(ctx, id)
}

// UpdateStatus transitions a service job to the next status.
// Completing a job automatically sets the completed_at timestamp.
func (s *Service) UpdateStatus(ctx context.Context, id string, next JobStatus) (*Job, error) {
	tenantID := tenant.MustGetTenantID(ctx)

	existing, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return nil, apperr.Internal("failed to get service job", err)
	}

	if existing == nil || existing.TenantID != tenantID {
		return nil, apperr.NotFound("service job")
	}

	if !existing.Status.CanTransitionTo(next) {
		return nil, apperr.BadRequest(fmt.Sprintf(
			"cannot transition service job from %s to %s", existing.Status, next,
		))
	}

	now := time.Now().UTC()
	p := UpdateJobParams{Status: &next}

	if next == JobStatusCompleted {
		p.CompletedAt = &now
	}

	j, err := s.repo.UpdateJob(ctx, id, p)
	if err != nil {
		return nil, apperr.Internal("failed to update service job status", err)
	}

	if j == nil {
		return nil, apperr.NotFound("service job")
	}

	s.log.Info("service job status updated",
		"job_id",    id,
		"from",      existing.Status,
		"to",        next,
		"tenant_id", tenantID,
	)

	return s.GetByID(ctx, id)
}

// DeleteJob hard-deletes a service job.  Only pending or cancelled jobs may
// be deleted; others must be cancelled first.
func (s *Service) DeleteJob(ctx context.Context, id string) error {
	tenantID := tenant.MustGetTenantID(ctx)

	existing, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return apperr.Internal("failed to get service job", err)
	}

	if existing == nil || existing.TenantID != tenantID {
		return apperr.NotFound("service job")
	}

	if existing.Status != JobStatusPending && existing.Status != JobStatusCancelled {
		return apperr.BadRequest("only pending or cancelled service jobs can be deleted")
	}

	if err := s.repo.DeleteJob(ctx, id); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return apperr.NotFound("service job")
		}
		return apperr.Internal("failed to delete service job", err)
	}

	s.log.Info("service job deleted", "job_id", id, "tenant_id", tenantID)
	return nil
}

// AssignMechanic sets or changes the mechanic assigned to a job.
func (s *Service) AssignMechanic(ctx context.Context, id, mechanicID string) (*Job, error) {
	return s.UpdateJob(ctx, id, UpdateJobInput{MechanicID: &mechanicID})
}

// ── Item service methods ──────────────────────────────────────────────────────

// AddItem adds a labour/part line-item to an existing service job.
func (s *Service) AddItem(ctx context.Context, jobID string, in AddItemInput) (*JobItem, error) {
	tenantID := tenant.MustGetTenantID(ctx)

	j, err := s.repo.FindByID(ctx, jobID)
	if err != nil {
		return nil, apperr.Internal("failed to get service job", err)
	}

	if j == nil || j.TenantID != tenantID {
		return nil, apperr.NotFound("service job")
	}

	if j.Status.IsTerminal() {
		return nil, apperr.BadRequest("cannot add items to a completed or cancelled job")
	}

	itemType, err := validateItemType(in.ItemType)
	if err != nil {
		return nil, err
	}

	if in.Quantity <= 0 {
		return nil, apperr.BadRequest("quantity must be greater than 0")
	}

	if in.UnitPrice < 0 {
		return nil, apperr.BadRequest("unit_price must be >= 0")
	}

	item, err := s.repo.AddItem(ctx, AddItemParams{
		JobID:       jobID,
		TenantID:    tenantID,
		ItemType:    itemType,
		Description: in.Description,
		Quantity:    in.Quantity,
		UnitPrice:   in.UnitPrice,
		PartNumber:  in.PartNumber,
	})
	if err != nil {
		return nil, apperr.Internal("failed to add item to service job", err)
	}

	s.log.Info("service job item added",
		"job_id",  jobID,
		"item_id", item.ID,
	)

	return item, nil
}

// UpdateItem updates an existing job item.
func (s *Service) UpdateItem(ctx context.Context, jobID, itemID string, in UpdateItemInput) (*JobItem, error) {
	tenantID := tenant.MustGetTenantID(ctx)

	// Verify the job exists and belongs to the tenant.
	j, err := s.repo.FindByID(ctx, jobID)
	if err != nil {
		return nil, apperr.Internal("failed to get service job", err)
	}

	if j == nil || j.TenantID != tenantID {
		return nil, apperr.NotFound("service job")
	}

	if j.Status.IsTerminal() {
		return nil, apperr.BadRequest("cannot update items on a completed or cancelled job")
	}

	item, err := s.repo.FindItemByID(ctx, itemID)
	if err != nil {
		return nil, apperr.Internal("failed to get item", err)
	}

	if item == nil || item.JobID != jobID {
		return nil, apperr.NotFound("service job item")
	}

	if in.Quantity != nil && *in.Quantity <= 0 {
		return nil, apperr.BadRequest("quantity must be greater than 0")
	}

	if in.UnitPrice != nil && *in.UnitPrice < 0 {
		return nil, apperr.BadRequest("unit_price must be >= 0")
	}

	var itemTypePtr *ItemType
	if in.ItemType != nil {
		it, err := validateItemType(*in.ItemType)
		if err != nil {
			return nil, err
		}
		itemTypePtr = &it
	}

	updated, err := s.repo.UpdateItem(ctx, itemID, UpdateItemParams{
		ItemType:    itemTypePtr,
		Description: in.Description,
		Quantity:    in.Quantity,
		UnitPrice:   in.UnitPrice,
		PartNumber:  in.PartNumber,
	})
	if err != nil {
		return nil, apperr.Internal("failed to update item", err)
	}

	if updated == nil {
		return nil, apperr.NotFound("service job item")
	}

	return updated, nil
}

// DeleteItem removes a line-item from a service job.
func (s *Service) DeleteItem(ctx context.Context, jobID, itemID string) error {
	tenantID := tenant.MustGetTenantID(ctx)

	j, err := s.repo.FindByID(ctx, jobID)
	if err != nil {
		return apperr.Internal("failed to get service job", err)
	}

	if j == nil || j.TenantID != tenantID {
		return apperr.NotFound("service job")
	}

	if j.Status.IsTerminal() {
		return apperr.BadRequest("cannot remove items from a completed or cancelled job")
	}

	item, err := s.repo.FindItemByID(ctx, itemID)
	if err != nil {
		return apperr.Internal("failed to get item", err)
	}

	if item == nil || item.JobID != jobID {
		return apperr.NotFound("service job item")
	}

	if err := s.repo.DeleteItem(ctx, itemID, nil); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return apperr.NotFound("service job item")
		}
		return apperr.Internal("failed to delete item", err)
	}

	return nil
}

// ── Validators ────────────────────────────────────────────────────────────────

var validJobTypes = map[string]JobType{
	string(JobTypeGeneral):     JobTypeGeneral,
	string(JobTypeRepair):      JobTypeRepair,
	string(JobTypeMaintenance): JobTypeMaintenance,
	string(JobTypeInspection):  JobTypeInspection,
	string(JobTypeBodywork):    JobTypeBodywork,
	string(JobTypeElectrical):  JobTypeElectrical,
	string(JobTypeOther):       JobTypeOther,
}

func validateJobType(t string) (JobType, error) {
	if jt, ok := validJobTypes[t]; ok {
		return jt, nil
	}
	return "", apperr.BadRequest(fmt.Sprintf(
		"invalid job_type %q — must be one of: general, repair, maintenance, inspection, bodywork, electrical, other", t,
	))
}

var validItemTypes = map[string]ItemType{
	string(ItemTypeLabour):     ItemTypeLabour,
	string(ItemTypePart):       ItemTypePart,
	string(ItemTypeConsumable): ItemTypeConsumable,
	string(ItemTypeOther):      ItemTypeOther,
}

func validateItemType(t string) (ItemType, error) {
	if it, ok := validItemTypes[t]; ok {
		return it, nil
	}
	return "", apperr.BadRequest(fmt.Sprintf(
		"invalid item_type %q — must be one of: labour, part, consumable, other", t,
	))
}

func max64(a, b float64) float64 {
	if a > b {
		return a
	}
	return b
}
