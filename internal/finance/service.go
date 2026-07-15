package finance

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

// Service implements business logic for the tenant financial ledger.
type Service struct {
	repo *Repository
	log  *slog.Logger
}

// NewService creates a Service.
func NewService(repo *Repository, log *slog.Logger) *Service {
	return &Service{repo: repo, log: log}
}

// ── Category inputs ───────────────────────────────────────────────────────────

type CreateCategoryInput struct {
	Name        string
	Type        string // "income" | "expense"
	Description *string
}

type UpdateCategoryInput struct {
	Name        *string
	Description *string
	IsActive    *bool
}

// ── Record inputs ─────────────────────────────────────────────────────────────

type CreateRecordInput struct {
	CategoryID      string
	Type            string // must match category type
	Amount          float64
	HireBookingID   *string
	SaleID          *string
	ServiceJobID    *string
	Description     string
	TransactionDate *time.Time // defaults to today
	PaymentMethod   *string
	Reference       *string
	CreatedBy       *string
	Notes           *string
}

type UpdateRecordInput struct {
	CategoryID      *string
	Amount          *float64
	HireBookingID   *string
	SaleID          *string
	ServiceJobID    *string
	Description     *string
	TransactionDate *time.Time
	PaymentMethod   *string
	Reference       *string
	Notes           *string
}

// ── Category methods ──────────────────────────────────────────────────────────

// ListCategories returns all categories for the tenant, optionally by type.
func (s *Service) ListCategories(ctx context.Context, entryType *string) ([]*Category, error) {
	tenantID := tenant.MustGetTenantID(ctx)

	if entryType != nil {
		if err := validateEntryType(*entryType); err != nil {
			return nil, err
		}
	}

	cats, err := s.repo.ListCategories(ctx, tenantID, entryType)
	if err != nil {
		return nil, apperr.Internal("failed to list categories", err)
	}

	return cats, nil
}

// GetCategory returns a single category scoped to the tenant in ctx.
func (s *Service) GetCategory(ctx context.Context, id string) (*Category, error) {
	tenantID := tenant.MustGetTenantID(ctx)

	c, err := s.repo.FindCategoryByID(ctx, id)
	if err != nil {
		return nil, apperr.Internal("failed to get category", err)
	}

	if c == nil || c.TenantID != tenantID {
		return nil, apperr.NotFound("finance category")
	}

	return c, nil
}

// CreateCategory adds a new finance category for the tenant.
func (s *Service) CreateCategory(ctx context.Context, in CreateCategoryInput) (*Category, error) {
	tenantID := tenant.MustGetTenantID(ctx)

	if err := validateEntryType(in.Type); err != nil {
		return nil, err
	}

	// Guard duplicate name+type within the tenant.
	dup, err := s.repo.FindCategoryByName(ctx, tenantID, in.Name, in.Type)
	if err != nil {
		return nil, apperr.Internal("failed to check for duplicate category", err)
	}

	if dup != nil {
		return nil, apperr.Conflict(fmt.Sprintf(
			"a %s category named %q already exists", in.Type, in.Name,
		))
	}

	c, err := s.repo.CreateCategory(ctx, CreateCategoryParams{
		TenantID:    tenantID,
		Name:        in.Name,
		Type:        EntryType(in.Type),
		Description: in.Description,
	})
	if err != nil {
		return nil, apperr.Internal("failed to create category", err)
	}

	s.log.Info("finance category created", "category_id", c.ID, "tenant_id", tenantID)
	return c, nil
}

// UpdateCategory applies a partial update to a category.
func (s *Service) UpdateCategory(ctx context.Context, id string, in UpdateCategoryInput) (*Category, error) {
	tenantID := tenant.MustGetTenantID(ctx)

	existing, err := s.repo.FindCategoryByID(ctx, id)
	if err != nil {
		return nil, apperr.Internal("failed to get category", err)
	}

	if existing == nil || existing.TenantID != tenantID {
		return nil, apperr.NotFound("finance category")
	}

	// Guard rename collision.
	if in.Name != nil && *in.Name != existing.Name {
		dup, err := s.repo.FindCategoryByName(ctx, tenantID, *in.Name, string(existing.Type))
		if err != nil {
			return nil, apperr.Internal("failed to check for duplicate category", err)
		}

		if dup != nil && dup.ID != id {
			return nil, apperr.Conflict(fmt.Sprintf(
				"a %s category named %q already exists", existing.Type, *in.Name,
			))
		}
	}

	c, err := s.repo.UpdateCategory(ctx, id, UpdateCategoryParams{
		Name:        in.Name,
		Description: in.Description,
		IsActive:    in.IsActive,
	})
	if err != nil {
		return nil, apperr.Internal("failed to update category", err)
	}

	if c == nil {
		return nil, apperr.NotFound("finance category")
	}

	return c, nil
}

// DeleteCategory removes a category.  Fails if it has associated records.
func (s *Service) DeleteCategory(ctx context.Context, id string) error {
	tenantID := tenant.MustGetTenantID(ctx)

	existing, err := s.repo.FindCategoryByID(ctx, id)
	if err != nil {
		return apperr.Internal("failed to get category", err)
	}

	if existing == nil || existing.TenantID != tenantID {
		return apperr.NotFound("finance category")
	}

	if err := s.repo.DeleteCategory(ctx, id); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return apperr.NotFound("finance category")
		}
		// FK violation — records exist for this category.
		if isFKViolation(err) {
			return apperr.Conflict("cannot delete a category that has associated records")
		}
		return apperr.Internal("failed to delete category", err)
	}

	s.log.Info("finance category deleted", "category_id", id, "tenant_id", tenantID)
	return nil
}

// ── Record methods ────────────────────────────────────────────────────────────

// ListRecords returns finance records for the tenant, optionally filtered.
func (s *Service) ListRecords(ctx context.Context, f RecordFilters) ([]*Record, error) {
	tenantID := tenant.MustGetTenantID(ctx)

	if f.Type != nil {
		if err := validateEntryType(*f.Type); err != nil {
			return nil, err
		}
	}

	records, err := s.repo.ListRecords(ctx, tenantID, f)
	if err != nil {
		return nil, apperr.Internal("failed to list finance records", err)
	}

	return records, nil
}

// GetRecord returns a single record scoped to the tenant in ctx.
func (s *Service) GetRecord(ctx context.Context, id string) (*Record, error) {
	tenantID := tenant.MustGetTenantID(ctx)

	rec, err := s.repo.FindRecordByID(ctx, id)
	if err != nil {
		return nil, apperr.Internal("failed to get finance record", err)
	}

	if rec == nil || rec.TenantID != tenantID {
		return nil, apperr.NotFound("finance record")
	}

	return rec, nil
}

// GetSummary returns aggregated income/expense totals with optional date range.
func (s *Service) GetSummary(ctx context.Context, from, to *time.Time) (LedgerSummary, error) {
	tenantID := tenant.MustGetTenantID(ctx)

	summary, err := s.repo.GetSummary(ctx, tenantID, from, to)
	if err != nil {
		return LedgerSummary{}, apperr.Internal("failed to get ledger summary", err)
	}

	return summary, nil
}

// CreateRecord adds a new income or expense entry to the ledger.
func (s *Service) CreateRecord(ctx context.Context, in CreateRecordInput) (*Record, error) {
	tenantID := tenant.MustGetTenantID(ctx)

	if err := validateEntryType(in.Type); err != nil {
		return nil, err
	}

	if in.Amount <= 0 {
		return nil, apperr.BadRequest("amount must be greater than 0")
	}

	if in.PaymentMethod != nil {
		if err := validatePaymentMethod(*in.PaymentMethod); err != nil {
			return nil, err
		}
	}

	// Verify category belongs to this tenant and type matches.
	cat, err := s.repo.FindCategoryByID(ctx, in.CategoryID)
	if err != nil {
		return nil, apperr.Internal("failed to get category", err)
	}

	if cat == nil || cat.TenantID != tenantID {
		return nil, apperr.NotFound("finance category")
	}

	if string(cat.Type) != in.Type {
		return nil, apperr.BadRequest(fmt.Sprintf(
			"category is of type %q but record type is %q", cat.Type, in.Type,
		))
	}

	txDate := time.Now().UTC().Truncate(24 * time.Hour)
	if in.TransactionDate != nil {
		txDate = in.TransactionDate.UTC().Truncate(24 * time.Hour)
	}

	rec, err := s.repo.CreateRecord(ctx, CreateRecordParams{
		TenantID:        tenantID,
		CategoryID:      in.CategoryID,
		Type:            EntryType(in.Type),
		Amount:          in.Amount,
		HireBookingID:   in.HireBookingID,
		SaleID:          in.SaleID,
		ServiceJobID:    in.ServiceJobID,
		Description:     in.Description,
		TransactionDate: txDate,
		PaymentMethod:   in.PaymentMethod,
		Reference:       in.Reference,
		CreatedBy:       in.CreatedBy,
		Notes:           in.Notes,
	})
	if err != nil {
		return nil, apperr.Internal("failed to create finance record", err)
	}

	s.log.Info("finance record created",
		"record_id", rec.ID,
		"type",      in.Type,
		"amount",    in.Amount,
		"tenant_id", tenantID,
	)

	return rec, nil
}

// UpdateRecord applies a partial update to a finance record.
func (s *Service) UpdateRecord(ctx context.Context, id string, in UpdateRecordInput) (*Record, error) {
	tenantID := tenant.MustGetTenantID(ctx)

	existing, err := s.repo.FindRecordByID(ctx, id)
	if err != nil {
		return nil, apperr.Internal("failed to get finance record", err)
	}

	if existing == nil || existing.TenantID != tenantID {
		return nil, apperr.NotFound("finance record")
	}

	if in.Amount != nil && *in.Amount <= 0 {
		return nil, apperr.BadRequest("amount must be greater than 0")
	}

	if in.PaymentMethod != nil {
		if err := validatePaymentMethod(*in.PaymentMethod); err != nil {
			return nil, err
		}
	}

	// If category is being changed, verify it belongs to this tenant and type is consistent.
	if in.CategoryID != nil {
		cat, err := s.repo.FindCategoryByID(ctx, *in.CategoryID)
		if err != nil {
			return nil, apperr.Internal("failed to get category", err)
		}

		if cat == nil || cat.TenantID != tenantID {
			return nil, apperr.NotFound("finance category")
		}

		if cat.Type != existing.Type {
			return nil, apperr.BadRequest(fmt.Sprintf(
				"category type %q does not match record type %q", cat.Type, existing.Type,
			))
		}
	}

	var txDate *time.Time
	if in.TransactionDate != nil {
		t := in.TransactionDate.UTC().Truncate(24 * time.Hour)
		txDate = &t
	}

	rec, err := s.repo.UpdateRecord(ctx, id, UpdateRecordParams{
		CategoryID:      in.CategoryID,
		Amount:          in.Amount,
		HireBookingID:   in.HireBookingID,
		SaleID:          in.SaleID,
		ServiceJobID:    in.ServiceJobID,
		Description:     in.Description,
		TransactionDate: txDate,
		PaymentMethod:   in.PaymentMethod,
		Reference:       in.Reference,
		Notes:           in.Notes,
	})
	if err != nil {
		return nil, apperr.Internal("failed to update finance record", err)
	}

	if rec == nil {
		return nil, apperr.NotFound("finance record")
	}

	s.log.Info("finance record updated", "record_id", id, "tenant_id", tenantID)
	return rec, nil
}

// DeleteRecord removes a finance record permanently.
func (s *Service) DeleteRecord(ctx context.Context, id string) error {
	tenantID := tenant.MustGetTenantID(ctx)

	existing, err := s.repo.FindRecordByID(ctx, id)
	if err != nil {
		return apperr.Internal("failed to get finance record", err)
	}

	if existing == nil || existing.TenantID != tenantID {
		return apperr.NotFound("finance record")
	}

	if err := s.repo.DeleteRecord(ctx, id); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return apperr.NotFound("finance record")
		}
		return apperr.Internal("failed to delete finance record", err)
	}

	s.log.Info("finance record deleted", "record_id", id, "tenant_id", tenantID)
	return nil
}

// ── Validators ────────────────────────────────────────────────────────────────

func validateEntryType(t string) error {
	if t != string(EntryTypeIncome) && t != string(EntryTypeExpense) {
		return apperr.BadRequest(fmt.Sprintf(
			"invalid type %q — must be one of: income, expense", t,
		))
	}
	return nil
}

var validPaymentMethods = map[string]struct{}{
	string(PaymentMethodCash):         {},
	string(PaymentMethodMpesa):        {},
	string(PaymentMethodBankTransfer): {},
	string(PaymentMethodCard):         {},
	string(PaymentMethodOther):        {},
}

func validatePaymentMethod(m string) error {
	if _, ok := validPaymentMethods[m]; !ok {
		return apperr.BadRequest(fmt.Sprintf(
			"invalid payment_method %q — must be one of: cash, mpesa, bank_transfer, card, other", m,
		))
	}
	return nil
}

// isFKViolation detects PostgreSQL FK constraint violations (code 23503).
func isFKViolation(err error) bool {
	return err != nil && fmt.Sprintf("%v", err) != "" &&
		contains(fmt.Sprintf("%v", err), "23503")
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr ||
		len(s) > 0 && len(substr) > 0 &&
			func() bool {
				for i := 0; i <= len(s)-len(substr); i++ {
					if s[i:i+len(substr)] == substr {
						return true
					}
				}
				return false
			}())
}
