package payments

import (
	"context"
	"fmt"
	"log/slog"
	"math"
	"time"

	"github.com/MauriceOmbewa/garisha-backend/internal/platform/mpesa"
	apperr "github.com/MauriceOmbewa/garisha-backend/internal/platform/errors"
	"github.com/MauriceOmbewa/garisha-backend/internal/platform/tenant"
)

// Service implements business logic for payment management.
type Service struct {
	repo        *Repository
	mpesaClient *mpesa.Client
	log         *slog.Logger
}

// NewService creates a Service.
func NewService(repo *Repository, mpesaClient *mpesa.Client, log *slog.Logger) *Service {
	return &Service{repo: repo, mpesaClient: mpesaClient, log: log}
}

// ── Input types ───────────────────────────────────────────────────────────────

// CreateManualInput records a manual (cash/card/bank) payment.
type CreateManualInput struct {
	HireBookingID *string
	SaleID        *string
	ServiceJobID  *string
	CustomerID    *string
	Method        string
	Amount        float64
	Currency      string
	Reference     *string
	CreatedBy     *string
	Notes         *string
}

// InitiateMpesaInput triggers an M-PESA STK Push for a payment.
type InitiateMpesaInput struct {
	HireBookingID    *string
	SaleID           *string
	ServiceJobID     *string
	CustomerID       *string
	PhoneNumber      string
	Amount           float64
	AccountReference string
	Description      string
	CreatedBy        *string
	Notes            *string
}

// ── Service methods ───────────────────────────────────────────────────────────

// List returns payments for the tenant in ctx, optionally filtered.
func (s *Service) List(ctx context.Context, f ListFilters) ([]*Payment, error) {
	tenantID := tenant.MustGetTenantID(ctx)

	payments, err := s.repo.List(ctx, tenantID, f)
	if err != nil {
		return nil, apperr.Internal("failed to list payments", err)
	}

	return payments, nil
}

// GetByID returns a single payment scoped to the tenant in ctx.
func (s *Service) GetByID(ctx context.Context, id string) (*Payment, error) {
	tenantID := tenant.MustGetTenantID(ctx)

	p, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return nil, apperr.Internal("failed to get payment", err)
	}

	if p == nil || p.TenantID != tenantID {
		return nil, apperr.NotFound("payment")
	}

	return p, nil
}

// CreateManual records a cash, card, or bank transfer payment directly as
// completed (manual confirmation, no async callback needed).
func (s *Service) CreateManual(ctx context.Context, in CreateManualInput) (*Payment, error) {
	tenantID := tenant.MustGetTenantID(ctx)

	if err := validateMethod(in.Method); err != nil {
		return nil, err
	}

	if in.Method == string(MethodMpesa) {
		return nil, apperr.BadRequest("use the M-PESA endpoint to initiate M-PESA payments")
	}

	if in.Amount <= 0 {
		return nil, apperr.BadRequest("amount must be greater than 0")
	}

	currency := in.Currency
	if currency == "" {
		currency = "KES"
	}

	now := time.Now().UTC()

	p, err := s.repo.Create(ctx, CreateParams{
		TenantID:      tenantID,
		HireBookingID: in.HireBookingID,
		SaleID:        in.SaleID,
		ServiceJobID:  in.ServiceJobID,
		CustomerID:    in.CustomerID,
		Method:        PaymentMethod(in.Method),
		Amount:        roundAmount(in.Amount),
		Currency:      currency,
		Status:        StatusCompleted,
		Reference:     in.Reference,
		CreatedBy:     in.CreatedBy,
		Notes:         in.Notes,
	})
	if err != nil {
		return nil, apperr.Internal("failed to record payment", err)
	}

	// Mark as paid immediately for manual payments.
	completed := StatusCompleted
	p, err = s.repo.Update(ctx, p.ID, UpdateParams{
		Status: &completed,
		PaidAt: &now,
	})
	if err != nil {
		return nil, apperr.Internal("failed to complete payment", err)
	}

	s.log.Info("manual payment recorded",
		"payment_id", p.ID,
		"method",     in.Method,
		"amount",     in.Amount,
		"tenant_id",  tenantID,
	)

	return p, nil
}

// InitiateMpesa triggers an M-PESA STK Push and stores a pending payment record.
// The payment status is updated to completed/failed when Safaricom fires the
// callback — handled by HandleMpesaCallback.
func (s *Service) InitiateMpesa(ctx context.Context, in InitiateMpesaInput) (*Payment, error) {
	tenantID := tenant.MustGetTenantID(ctx)

	if !s.mpesaClient.Enabled() {
		return nil, apperr.BadRequest("M-PESA is not configured for this instance")
	}

	if in.Amount <= 0 {
		return nil, apperr.BadRequest("amount must be greater than 0")
	}

	if in.PhoneNumber == "" {
		return nil, apperr.BadRequest("phone_number is required for M-PESA payments")
	}

	currency := "KES"
	amountKES := int64(math.Round(in.Amount))

	// Create the pending payment record first.
	phone := in.PhoneNumber
	p, err := s.repo.Create(ctx, CreateParams{
		TenantID:      tenantID,
		HireBookingID: in.HireBookingID,
		SaleID:        in.SaleID,
		ServiceJobID:  in.ServiceJobID,
		CustomerID:    in.CustomerID,
		Method:        MethodMpesa,
		Amount:        float64(amountKES),
		Currency:      currency,
		Status:        StatusPending,
		MpesaPhone:    &phone,
		CreatedBy:     in.CreatedBy,
		Notes:         in.Notes,
	})
	if err != nil {
		return nil, apperr.Internal("failed to create pending payment", err)
	}

	// Trigger the STK push.
	stkResp, err := s.mpesaClient.STKPush(ctx, mpesa.STKPushRequest{
		PhoneNumber:      in.PhoneNumber,
		Amount:           amountKES,
		AccountReference: in.AccountReference,
		TransactionDesc:  in.Description,
	})
	if err != nil {
		// Mark payment as failed — STK push not delivered.
		failed := StatusFailed
		reason := err.Error()
		_, _ = s.repo.Update(ctx, p.ID, UpdateParams{
			Status:        &failed,
			FailureReason: &reason,
		})
		return nil, apperr.Internal("M-PESA STK push failed", err)
	}

	// Store the CheckoutRequestID so the callback can match this record.
	p, err = s.updateCheckoutReqID(ctx, p.ID, stkResp.CheckoutRequestID)
	if err != nil {
		s.log.Warn("failed to store checkout request id",
			"payment_id",  p.ID,
			"checkout_id", stkResp.CheckoutRequestID,
		)
	}

	s.log.Info("M-PESA STK push initiated",
		"payment_id",     p.ID,
		"checkout_req_id", stkResp.CheckoutRequestID,
		"phone",          in.PhoneNumber,
		"tenant_id",      tenantID,
	)

	return p, nil
}

// HandleMpesaCallback processes a Daraja STK Push callback body.
// It matches the CheckoutRequestID to a pending payment and updates its status.
// This method does NOT require tenant context — callbacks are unauthenticated
// public webhooks verified by CheckoutRequestID lookup.
func (s *Service) HandleMpesaCallback(ctx context.Context, rawBody []byte) error {
	result, err := mpesa.ParseCallback(rawBody)
	if err != nil {
		return apperr.BadRequest(fmt.Sprintf("invalid callback body: %v", err))
	}

	p, err := s.repo.FindByCheckoutRequestID(ctx, result.CheckoutRequestID)
	if err != nil {
		return apperr.Internal("failed to find payment for callback", err)
	}

	if p == nil {
		// Unknown checkout ID — could be a replay or misconfigured callback.
		s.log.Warn("mpesa callback: no payment found for checkout request id",
			"checkout_req_id", result.CheckoutRequestID,
		)
		return nil
	}

	if p.Status.IsTerminal() {
		// Already processed — idempotent, ignore.
		return nil
	}

	now := time.Now().UTC()
	up := UpdateParams{
		MpesaResultCode: &result.ResultCode,
		MpesaResultDesc: &result.ResultDesc,
	}

	if result.ResultCode == 0 {
		completed := StatusCompleted
		up.Status = &completed
		up.PaidAt = &now
		up.MpesaReceiptNumber = &result.MpesaReceiptNumber
	} else {
		failed := StatusFailed
		up.Status = &failed
		reason := result.ResultDesc
		up.FailureReason = &reason
	}

	if _, err := s.repo.Update(ctx, p.ID, up); err != nil {
		return apperr.Internal("failed to update payment from callback", err)
	}

	s.log.Info("mpesa callback processed",
		"payment_id",    p.ID,
		"result_code",   result.ResultCode,
		"result_desc",   result.ResultDesc,
	)

	return nil
}

// Cancel marks a pending payment as cancelled.
func (s *Service) Cancel(ctx context.Context, id string) (*Payment, error) {
	tenantID := tenant.MustGetTenantID(ctx)

	existing, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return nil, apperr.Internal("failed to get payment", err)
	}

	if existing == nil || existing.TenantID != tenantID {
		return nil, apperr.NotFound("payment")
	}

	if existing.Status.IsTerminal() {
		return nil, apperr.BadRequest(fmt.Sprintf(
			"cannot cancel a payment that is already %s", existing.Status,
		))
	}

	cancelled := StatusCancelled
	p, err := s.repo.Update(ctx, id, UpdateParams{Status: &cancelled})
	if err != nil {
		return nil, apperr.Internal("failed to cancel payment", err)
	}

	s.log.Info("payment cancelled", "payment_id", id, "tenant_id", tenantID)
	return p, nil
}

// ── Internal helpers ──────────────────────────────────────────────────────────

// updateCheckoutReqID persists the STK CheckoutRequestID via the repository.
func (s *Service) updateCheckoutReqID(ctx context.Context, id, checkoutReqID string) (*Payment, error) {
	if err := s.repo.SetCheckoutRequestID(ctx, id, checkoutReqID); err != nil {
		return nil, err
	}

	return s.repo.FindByID(ctx, id)
}

func roundAmount(v float64) float64 {
	return math.Round(v*100) / 100
}

var validMethods = map[string]struct{}{
	string(MethodMpesa):        {},
	string(MethodCash):         {},
	string(MethodBankTransfer): {},
	string(MethodCard):         {},
	string(MethodOther):        {},
}

func validateMethod(m string) error {
	if _, ok := validMethods[m]; !ok {
		return apperr.BadRequest(fmt.Sprintf(
			"invalid payment_method %q — must be one of: mpesa, cash, bank_transfer, card, other", m,
		))
	}
	return nil
}
