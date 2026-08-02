package sales

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"math"
	"time"

	"github.com/jackc/pgx/v5"

	apperr "github.com/MauriceOmbewa/garisha-backend/internal/platform/errors"
	"github.com/MauriceOmbewa/garisha-backend/internal/platform/tenant"
)

// Service implements business logic for vehicle-sale management.
type Service struct {
	repo *Repository
	log  *slog.Logger
}

// NewService creates a Service.
func NewService(repo *Repository, log *slog.Logger) *Service {
	return &Service{repo: repo, log: log}
}

// ── Input types ───────────────────────────────────────────────────────────────

// CreateInput carries the fields a caller provides when recording a sale.
type CreateInput struct {
	VehicleID  string
	CustomerID string

	AskingPrice    float64
	AgreedPrice    float64
	DepositAmount  float64
	DiscountAmount float64

	PaymentMethod *string
	PaymentTerms  *string

	SaleDate time.Time // defaults to today if zero

	InvoiceNumber *string
	ContractRef   *string

	CreatedBy *string
	Notes     *string
}

// UpdateInput carries mutable fields for a partial sale update.
// nil pointer = leave existing value unchanged.
type UpdateInput struct {
	CustomerID *string

	AskingPrice    *float64
	AgreedPrice    *float64
	DepositAmount  *float64
	DiscountAmount *float64

	PaymentMethod *string
	PaymentTerms  *string

	SaleDate   *time.Time
	HandoverAt *time.Time

	InvoiceNumber *string
	ContractRef   *string

	Notes *string
}

// ── Service methods ───────────────────────────────────────────────────────────

// ListEnriched returns sales joined with customer and vehicle data.
func (s *Service) ListEnriched(ctx context.Context, f ListFilters) ([]*SaleEnriched, error) {
	tenantID := tenant.MustGetTenantID(ctx)
	sales, err := s.repo.ListEnriched(ctx, tenantID, f)
	if err != nil {
		return nil, apperr.Internal("failed to list sales", err)
	}
	return sales, nil
}

// List returns sales for the tenant in ctx, optionally filtered.
func (s *Service) List(ctx context.Context, f ListFilters) ([]*Sale, error) {
	tenantID := tenant.MustGetTenantID(ctx)

	sales, err := s.repo.List(ctx, tenantID, f)
	if err != nil {
		return nil, apperr.Internal("failed to list sales", err)
	}

	return sales, nil
}

// GetByID returns a single sale scoped to the tenant in ctx.
func (s *Service) GetByID(ctx context.Context, id string) (*Sale, error) {
	tenantID := tenant.MustGetTenantID(ctx)

	sale, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return nil, apperr.Internal("failed to get sale", err)
	}

	if sale == nil || sale.TenantID != tenantID {
		return nil, apperr.NotFound("sale")
	}

	return sale, nil
}

// Create records a new vehicle sale.  It validates pricing, checks that the
// vehicle is not already under an active sale, calculates final_amount, and
// persists with status=pending.
func (s *Service) Create(ctx context.Context, in CreateInput) (*Sale, error) {
	tenantID := tenant.MustGetTenantID(ctx)

	// ── Validation ────────────────────────────────────────────────────────────
	if in.AskingPrice < 0 {
		return nil, apperr.BadRequest("asking_price must be >= 0")
	}

	if in.AgreedPrice <= 0 {
		return nil, apperr.BadRequest("agreed_price must be greater than 0")
	}

	if in.DepositAmount < 0 {
		return nil, apperr.BadRequest("deposit_amount must be >= 0")
	}

	if in.DiscountAmount < 0 {
		return nil, apperr.BadRequest("discount_amount must be >= 0")
	}

	if in.PaymentMethod != nil {
		if err := validatePaymentMethod(*in.PaymentMethod); err != nil {
			return nil, err
		}
	}

	// Default sale date to today.
	saleDate := in.SaleDate
	if saleDate.IsZero() {
		saleDate = time.Now().UTC().Truncate(24 * time.Hour)
	}

	// ── Duplicate sale guard ──────────────────────────────────────────────────
	active, err := s.repo.HasActiveSale(ctx, in.VehicleID, nil)
	if err != nil {
		return nil, apperr.Internal("failed to check vehicle sale status", err)
	}

	if active {
		return nil, apperr.Conflict("vehicle already has an active sale record")
	}

	// ── Pricing ───────────────────────────────────────────────────────────────
	finalAmount := roundAmount(math.Max(0, in.AgreedPrice-in.DiscountAmount))

	// ── Persist ───────────────────────────────────────────────────────────────
	sale, err := s.repo.Create(ctx, CreateParams{
		TenantID:       tenantID,
		VehicleID:      in.VehicleID,
		CustomerID:     in.CustomerID,
		AskingPrice:    in.AskingPrice,
		AgreedPrice:    in.AgreedPrice,
		DepositAmount:  in.DepositAmount,
		DiscountAmount: in.DiscountAmount,
		FinalAmount:    finalAmount,
		PaymentMethod:  in.PaymentMethod,
		PaymentTerms:   in.PaymentTerms,
		SaleDate:       saleDate,
		Status:         SaleStatusPending,
		InvoiceNumber:  in.InvoiceNumber,
		ContractRef:    in.ContractRef,
		CreatedBy:      in.CreatedBy,
		Notes:          in.Notes,
	})
	if err != nil {
		return nil, apperr.Internal("failed to create sale", err)
	}

	s.log.Info("vehicle sale created",
		"sale_id",    sale.ID,
		"vehicle_id", in.VehicleID,
		"tenant_id",  tenantID,
	)

	return sale, nil
}

// Update applies a partial update to a sale.  Reprices final_amount when
// agreed_price or discount_amount change.
func (s *Service) Update(ctx context.Context, id string, in UpdateInput) (*Sale, error) {
	tenantID := tenant.MustGetTenantID(ctx)

	existing, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return nil, apperr.Internal("failed to get sale", err)
	}

	if existing == nil || existing.TenantID != tenantID {
		return nil, apperr.NotFound("sale")
	}

	if existing.Status.IsTerminal() {
		return nil, apperr.BadRequest(fmt.Sprintf(
			"cannot update a sale that is %s", existing.Status,
		))
	}

	if in.AgreedPrice != nil && *in.AgreedPrice <= 0 {
		return nil, apperr.BadRequest("agreed_price must be greater than 0")
	}

	if in.DepositAmount != nil && *in.DepositAmount < 0 {
		return nil, apperr.BadRequest("deposit_amount must be >= 0")
	}

	if in.DiscountAmount != nil && *in.DiscountAmount < 0 {
		return nil, apperr.BadRequest("discount_amount must be >= 0")
	}

	if in.PaymentMethod != nil {
		if err := validatePaymentMethod(*in.PaymentMethod); err != nil {
			return nil, err
		}
	}

	// Recalculate final_amount if pricing changed.
	agreedPrice := existing.AgreedPrice
	discountAmount := existing.DiscountAmount

	if in.AgreedPrice != nil {
		agreedPrice = *in.AgreedPrice
	}
	if in.DiscountAmount != nil {
		discountAmount = *in.DiscountAmount
	}

	finalAmount := roundAmount(math.Max(0, agreedPrice-discountAmount))

	sale, err := s.repo.Update(ctx, id, UpdateParams{
		CustomerID:     in.CustomerID,
		AskingPrice:    in.AskingPrice,
		AgreedPrice:    &agreedPrice,
		DepositAmount:  in.DepositAmount,
		DiscountAmount: &discountAmount,
		FinalAmount:    &finalAmount,
		PaymentMethod:  in.PaymentMethod,
		PaymentTerms:   in.PaymentTerms,
		SaleDate:       in.SaleDate,
		HandoverAt:     in.HandoverAt,
		InvoiceNumber:  in.InvoiceNumber,
		ContractRef:    in.ContractRef,
		Notes:          in.Notes,
	})
	if err != nil {
		return nil, apperr.Internal("failed to update sale", err)
	}

	if sale == nil {
		return nil, apperr.NotFound("sale")
	}

	s.log.Info("vehicle sale updated", "sale_id", id, "tenant_id", tenantID)
	return sale, nil
}

// UpdateStatus transitions a sale to a new status, enforcing lifecycle rules.
// When completing a sale the handover timestamp is set automatically.
func (s *Service) UpdateStatus(ctx context.Context, id string, next SaleStatus) (*Sale, error) {
	tenantID := tenant.MustGetTenantID(ctx)

	existing, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return nil, apperr.Internal("failed to get sale", err)
	}

	if existing == nil || existing.TenantID != tenantID {
		return nil, apperr.NotFound("sale")
	}

	if !existing.Status.CanTransitionTo(next) {
		return nil, apperr.BadRequest(fmt.Sprintf(
			"cannot transition sale from %s to %s", existing.Status, next,
		))
	}

	now := time.Now().UTC()
	p := UpdateParams{Status: &next}

	// Auto-set handover timestamp when the deal is completed.
	if next == SaleStatusCompleted {
		p.HandoverAt = &now
	}

	sale, err := s.repo.Update(ctx, id, p)
	if err != nil {
		return nil, apperr.Internal("failed to update sale status", err)
	}

	if sale == nil {
		return nil, apperr.NotFound("sale")
	}

	s.log.Info("vehicle sale status updated",
		"sale_id",   id,
		"from",      existing.Status,
		"to",        next,
		"tenant_id", tenantID,
	)

	return sale, nil
}

// Delete hard-deletes a sale record.  Only pending or cancelled records
// may be deleted; others must be cancelled first.
func (s *Service) Delete(ctx context.Context, id string) error {
	tenantID := tenant.MustGetTenantID(ctx)

	existing, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return apperr.Internal("failed to get sale", err)
	}

	if existing == nil || existing.TenantID != tenantID {
		return apperr.NotFound("sale")
	}

	if existing.Status != SaleStatusPending && existing.Status != SaleStatusCancelled {
		return apperr.BadRequest("only pending or cancelled sales can be deleted")
	}

	if err := s.repo.Delete(ctx, id); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return apperr.NotFound("sale")
		}
		return apperr.Internal("failed to delete sale", err)
	}

	s.log.Info("vehicle sale deleted", "sale_id", id, "tenant_id", tenantID)
	return nil
}

// ── Helpers ───────────────────────────────────────────────────────────────────

func roundAmount(v float64) float64 {
	return math.Round(v*100) / 100
}

var validPaymentMethods = map[string]struct{}{
	string(PaymentMethodCash):         {},
	string(PaymentMethodMpesa):        {},
	string(PaymentMethodBankTransfer): {},
	string(PaymentMethodFinance):      {},
	string(PaymentMethodOther):        {},
}

func validatePaymentMethod(m string) error {
	if _, ok := validPaymentMethods[m]; !ok {
		return apperr.BadRequest(fmt.Sprintf(
			"invalid payment_method %q — must be one of: cash, mpesa, bank_transfer, finance, other", m,
		))
	}
	return nil
}
