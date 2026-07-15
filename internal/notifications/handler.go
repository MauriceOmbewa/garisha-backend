package notifications

import (
	"log/slog"
	"net/http"

	apperr "github.com/MauriceOmbewa/garisha-backend/internal/platform/errors"
	"github.com/MauriceOmbewa/garisha-backend/internal/platform/response"
)

// Handler holds the HTTP handlers for the notifications domain.
type Handler struct {
	svc *Service
	log *slog.Logger
}

// NewHandler creates a Handler.
func NewHandler(svc *Service, log *slog.Logger) *Handler {
	return &Handler{svc: svc, log: log}
}

// ─── Response DTO ─────────────────────────────────────────────────────────────

type notificationDTO struct {
	ID           string  `json:"id"`
	TenantID     string  `json:"tenant_id"`
	UserID       string  `json:"user_id"`
	Type         string  `json:"type"`
	Title        string  `json:"title"`
	Body         string  `json:"body"`
	ResourceType *string `json:"resource_type"`
	ResourceID   *string `json:"resource_id"`
	IsRead       bool    `json:"is_read"`
	ReadAt       *string `json:"read_at"`
	CreatedAt    string  `json:"created_at"`
}

type markAllReadResponse struct {
	Updated int64 `json:"updated"`
}

type deleteReadResponse struct {
	Deleted int64 `json:"deleted"`
}

// ─── Handlers ─────────────────────────────────────────────────────────────────

// List godoc
// GET /api/v1/notifications[?is_read=false&type=reorder_alert&limit=50&offset=0]
func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()

	var f ListFilters

	if v := q.Get("is_read"); v != "" {
		b := v == "true"
		f.IsRead = &b
	}
	if v := q.Get("type"); v != "" {
		f.Type = &v
	}
	if v := q.Get("limit"); v != "" {
		var n int
		if _, err := parseIntParam(v, &n); err == nil && n > 0 {
			f.Limit = n
		}
	}
	if v := q.Get("offset"); v != "" {
		var n int
		if _, err := parseIntParam(v, &n); err == nil && n >= 0 {
			f.Offset = n
		}
	}

	ns, err := h.svc.List(r.Context(), f)
	if err != nil {
		apperr.Handle(w, r, err, h.log)
		return
	}

	dtos := make([]notificationDTO, 0, len(ns))
	for _, n := range ns {
		dtos = append(dtos, toDTO(n))
	}

	response.Success(w, http.StatusOK, "notifications retrieved", dtos, h.log)
}

// UnreadCount godoc
// GET /api/v1/notifications/unread-count
func (h *Handler) UnreadCount(w http.ResponseWriter, r *http.Request) {
	count, err := h.svc.GetUnreadCount(r.Context())
	if err != nil {
		apperr.Handle(w, r, err, h.log)
		return
	}

	response.Success(w, http.StatusOK, "unread count retrieved", count, h.log)
}

// MarkRead godoc
// PATCH /api/v1/notifications/{id}/read
func (h *Handler) MarkRead(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	n, err := h.svc.MarkRead(r.Context(), id)
	if err != nil {
		apperr.Handle(w, r, err, h.log)
		return
	}

	response.Success(w, http.StatusOK, "notification marked as read", toDTO(n), h.log)
}

// MarkAllRead godoc
// PATCH /api/v1/notifications/read-all
func (h *Handler) MarkAllRead(w http.ResponseWriter, r *http.Request) {
	count, err := h.svc.MarkAllRead(r.Context())
	if err != nil {
		apperr.Handle(w, r, err, h.log)
		return
	}

	response.Success(w, http.StatusOK, "all notifications marked as read",
		markAllReadResponse{Updated: count}, h.log)
}

// Delete godoc
// DELETE /api/v1/notifications/{id}
func (h *Handler) Delete(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	if err := h.svc.Delete(r.Context(), id); err != nil {
		apperr.Handle(w, r, err, h.log)
		return
	}

	response.NoContent(w)
}

// DeleteRead godoc
// DELETE /api/v1/notifications/read
// Housekeeping — removes all read notifications for the calling user.
func (h *Handler) DeleteRead(w http.ResponseWriter, r *http.Request) {
	count, err := h.svc.DeleteRead(r.Context())
	if err != nil {
		apperr.Handle(w, r, err, h.log)
		return
	}

	response.Success(w, http.StatusOK, "read notifications deleted",
		deleteReadResponse{Deleted: count}, h.log)
}

// ─── Helpers ─────────────────────────────────────────────────────────────────

func toDTO(n *Notification) notificationDTO {
	dto := notificationDTO{
		ID:           n.ID,
		TenantID:     n.TenantID,
		UserID:       n.UserID,
		Type:         n.Type,
		Title:        n.Title,
		Body:         n.Body,
		ResourceType: n.ResourceType,
		ResourceID:   n.ResourceID,
		IsRead:       n.IsRead,
		CreatedAt:    n.CreatedAt.UTC().Format("2006-01-02T15:04:05Z"),
	}

	if n.ReadAt != nil {
		s := n.ReadAt.UTC().Format("2006-01-02T15:04:05Z")
		dto.ReadAt = &s
	}

	return dto
}

func parseIntParam(s string, out *int) (int, error) {
	n := 0
	for _, c := range s {
		if c < '0' || c > '9' {
			return 0, apperr.BadRequest("invalid integer parameter")
		}
		n = n*10 + int(c-'0')
	}
	*out = n
	return n, nil
}
