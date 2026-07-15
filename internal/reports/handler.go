package reports

import (
	"log/slog"
	"net/http"
	"time"

	apperr "github.com/MauriceOmbewa/garisha-backend/internal/platform/errors"
	"github.com/MauriceOmbewa/garisha-backend/internal/platform/response"
)

// Handler holds the HTTP handlers for the reports domain.
type Handler struct {
	svc *Service
	log *slog.Logger
}

// NewHandler creates a Handler.
func NewHandler(svc *Service, log *slog.Logger) *Handler {
	return &Handler{svc: svc, log: log}
}

// ─── Handlers ─────────────────────────────────────────────────────────────────

// HireReport godoc
// GET /api/v1/reports/hire[?from=YYYY-MM-DD&to=YYYY-MM-DD]
func (h *Handler) HireReport(w http.ResponseWriter, r *http.Request) {
	dr, err := parseDateRange(r)
	if err != nil {
		apperr.Handle(w, r, err, h.log)
		return
	}

	summary, err := h.svc.HireSummary(r.Context(), dr)
	if err != nil {
		apperr.Handle(w, r, err, h.log)
		return
	}

	response.Success(w, http.StatusOK, "hire report generated", summary, h.log)
}

// SalesReport godoc
// GET /api/v1/reports/sales[?from=YYYY-MM-DD&to=YYYY-MM-DD]
func (h *Handler) SalesReport(w http.ResponseWriter, r *http.Request) {
	dr, err := parseDateRange(r)
	if err != nil {
		apperr.Handle(w, r, err, h.log)
		return
	}

	summary, err := h.svc.SalesSummary(r.Context(), dr)
	if err != nil {
		apperr.Handle(w, r, err, h.log)
		return
	}

	response.Success(w, http.StatusOK, "sales report generated", summary, h.log)
}

// ServiceReport godoc
// GET /api/v1/reports/service[?from=YYYY-MM-DD&to=YYYY-MM-DD]
func (h *Handler) ServiceReport(w http.ResponseWriter, r *http.Request) {
	dr, err := parseDateRange(r)
	if err != nil {
		apperr.Handle(w, r, err, h.log)
		return
	}

	summary, err := h.svc.ServiceSummary(r.Context(), dr)
	if err != nil {
		apperr.Handle(w, r, err, h.log)
		return
	}

	response.Success(w, http.StatusOK, "service report generated", summary, h.log)
}

// FinanceReport godoc
// GET /api/v1/reports/finance[?from=YYYY-MM-DD&to=YYYY-MM-DD]
func (h *Handler) FinanceReport(w http.ResponseWriter, r *http.Request) {
	dr, err := parseDateRange(r)
	if err != nil {
		apperr.Handle(w, r, err, h.log)
		return
	}

	summary, err := h.svc.FinanceSummary(r.Context(), dr)
	if err != nil {
		apperr.Handle(w, r, err, h.log)
		return
	}

	response.Success(w, http.StatusOK, "finance report generated", summary, h.log)
}

// PaymentsReport godoc
// GET /api/v1/reports/payments[?from=YYYY-MM-DD&to=YYYY-MM-DD]
func (h *Handler) PaymentsReport(w http.ResponseWriter, r *http.Request) {
	dr, err := parseDateRange(r)
	if err != nil {
		apperr.Handle(w, r, err, h.log)
		return
	}

	summary, err := h.svc.PaymentSummary(r.Context(), dr)
	if err != nil {
		apperr.Handle(w, r, err, h.log)
		return
	}

	response.Success(w, http.StatusOK, "payments report generated", summary, h.log)
}

// ─── Helper ───────────────────────────────────────────────────────────────────

func parseDateRange(r *http.Request) (DateRange, error) {
	q := r.URL.Query()
	var dr DateRange

	if v := q.Get("from"); v != "" {
		t, err := time.Parse("2006-01-02", v)
		if err != nil {
			return dr, apperr.BadRequest("invalid from — expected YYYY-MM-DD")
		}
		dr.From = &t
	}

	if v := q.Get("to"); v != "" {
		t, err := time.Parse("2006-01-02", v)
		if err != nil {
			return dr, apperr.BadRequest("invalid to — expected YYYY-MM-DD")
		}
		dr.To = &t
	}

	if dr.From != nil && dr.To != nil && dr.To.Before(*dr.From) {
		return dr, apperr.BadRequest("to must be on or after from")
	}

	return dr, nil
}
