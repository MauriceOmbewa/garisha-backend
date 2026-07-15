package dashboard

import (
	"log/slog"
	"net/http"

	apperr "github.com/MauriceOmbewa/garisha-backend/internal/platform/errors"
	"github.com/MauriceOmbewa/garisha-backend/internal/platform/response"
)

// Handler holds the HTTP handlers for the dashboard domain.
type Handler struct {
	svc *Service
	log *slog.Logger
}

// NewHandler creates a Handler.
func NewHandler(svc *Service, log *slog.Logger) *Handler {
	return &Handler{svc: svc, log: log}
}

// Summary godoc
// GET /api/v1/dashboard/summary
// Returns all KPI card values for the current calendar month.
func (h *Handler) Summary(w http.ResponseWriter, r *http.Request) {
	summary, err := h.svc.GetSummary(r.Context())
	if err != nil {
		apperr.Handle(w, r, err, h.log)
		return
	}

	response.Success(w, http.StatusOK, "dashboard summary retrieved", summary, h.log)
}

// Charts godoc
// GET /api/v1/dashboard/charts
// Returns time-series and distribution datasets for dashboard charts.
func (h *Handler) Charts(w http.ResponseWriter, r *http.Request) {
	charts, err := h.svc.GetCharts(r.Context())
	if err != nil {
		apperr.Handle(w, r, err, h.log)
		return
	}

	response.Success(w, http.StatusOK, "dashboard charts retrieved", charts, h.log)
}

// Activity godoc
// GET /api/v1/dashboard/activity
// Returns the 20 most recent transactions across all modules.
func (h *Handler) Activity(w http.ResponseWriter, r *http.Request) {
	activity, err := h.svc.GetActivity(r.Context())
	if err != nil {
		apperr.Handle(w, r, err, h.log)
		return
	}

	response.Success(w, http.StatusOK, "recent activity retrieved", activity, h.log)
}
