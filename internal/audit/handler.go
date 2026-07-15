package audit

import (
	"log/slog"
	"net/http"
	"time"

	apperr "github.com/MauriceOmbewa/garisha-backend/internal/platform/errors"
	"github.com/MauriceOmbewa/garisha-backend/internal/platform/response"
)

// Handler holds the HTTP handlers for the audit domain.
// The audit log is read-only via the API — writes happen internally.
type Handler struct {
	svc *Service
	log *slog.Logger
}

// NewHandler creates a Handler.
func NewHandler(svc *Service, log *slog.Logger) *Handler {
	return &Handler{svc: svc, log: log}
}

// ─── Response DTO ─────────────────────────────────────────────────────────────

type logDTO struct {
	ID       string `json:"id"`
	TenantID string `json:"tenant_id"`

	ActorID    *string `json:"actor_id"`
	ActorEmail *string `json:"actor_email"`
	ActorRole  *string `json:"actor_role"`

	Action       string  `json:"action"`
	ResourceType string  `json:"resource_type"`
	ResourceID   *string `json:"resource_id"`

	Changes map[string]any `json:"changes"`

	IPAddress *string `json:"ip_address"`
	UserAgent *string `json:"user_agent"`
	RequestID *string `json:"request_id"`

	Status       string  `json:"status"`
	ErrorMessage *string `json:"error_message"`

	CreatedAt string `json:"created_at"`
}

// ─── Handlers ─────────────────────────────────────────────────────────────────

// List godoc
// GET /api/v1/audit/logs[?actor_id=&action=&resource_type=&resource_id=&status=&from=&to=&limit=50&offset=0]
func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()

	var f ListFilters

	if v := q.Get("actor_id"); v != "" {
		f.ActorID = &v
	}
	if v := q.Get("action"); v != "" {
		f.Action = &v
	}
	if v := q.Get("resource_type"); v != "" {
		f.ResourceType = &v
	}
	if v := q.Get("resource_id"); v != "" {
		f.ResourceID = &v
	}
	if v := q.Get("status"); v != "" {
		f.Status = &v
	}
	if v := q.Get("from"); v != "" {
		if t, err := time.Parse("2006-01-02", v); err == nil {
			f.FromDate = &t
		}
	}
	if v := q.Get("to"); v != "" {
		if t, err := time.Parse("2006-01-02", v); err == nil {
			f.ToDate = &t
		}
	}

	limit, offset := parsePagination(q.Get("limit"), q.Get("offset"))
	f.Limit = limit
	f.Offset = offset

	entries, err := h.svc.List(r.Context(), f)
	if err != nil {
		apperr.Handle(w, r, err, h.log)
		return
	}

	dtos := make([]logDTO, 0, len(entries))
	for _, e := range entries {
		dtos = append(dtos, toDTO(e))
	}

	response.Success(w, http.StatusOK, "audit logs retrieved", dtos, h.log)
}

// Get godoc
// GET /api/v1/audit/logs/{id}
func (h *Handler) Get(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	entry, err := h.svc.GetByID(r.Context(), id)
	if err != nil {
		apperr.Handle(w, r, err, h.log)
		return
	}

	response.Success(w, http.StatusOK, "audit log entry retrieved", toDTO(entry), h.log)
}

// ─── Helpers ─────────────────────────────────────────────────────────────────

func toDTO(e *Log) logDTO {
	return logDTO{
		ID:           e.ID,
		TenantID:     e.TenantID,
		ActorID:      e.ActorID,
		ActorEmail:   e.ActorEmail,
		ActorRole:    e.ActorRole,
		Action:       e.Action,
		ResourceType: e.ResourceType,
		ResourceID:   e.ResourceID,
		Changes:      e.Changes,
		IPAddress:    e.IPAddress,
		UserAgent:    e.UserAgent,
		RequestID:    e.RequestID,
		Status:       string(e.Status),
		ErrorMessage: e.ErrorMessage,
		CreatedAt:    e.CreatedAt.UTC().Format("2006-01-02T15:04:05Z"),
	}
}

func parsePagination(limitStr, offsetStr string) (limit, offset int) {
	limit = 50
	offset = 0

	if limitStr != "" {
		if n := atoi(limitStr); n > 0 {
			limit = n
		}
	}
	if limit > 200 {
		limit = 200
	}

	if offsetStr != "" {
		if n := atoi(offsetStr); n >= 0 {
			offset = n
		}
	}

	return
}

func atoi(s string) int {
	n := 0
	for _, c := range s {
		if c < '0' || c > '9' {
			return -1
		}
		n = n*10 + int(c-'0')
	}
	return n
}
