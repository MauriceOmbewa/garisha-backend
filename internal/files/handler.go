package files

import (
	"log/slog"
	"net/http"

	apperr "github.com/MauriceOmbewa/garisha-backend/internal/platform/errors"
	"github.com/MauriceOmbewa/garisha-backend/internal/platform/response"
	"github.com/MauriceOmbewa/garisha-backend/internal/platform/validation"
)

// Handler holds the HTTP handlers for the files domain.
type Handler struct {
	svc *Service
	log *slog.Logger
}

// NewHandler creates a Handler.
func NewHandler(svc *Service, log *slog.Logger) *Handler {
	return &Handler{svc: svc, log: log}
}

// ─── Request DTOs ─────────────────────────────────────────────────────────────

type presignRequest struct {
	OriginalName string  `json:"original_name"  validate:"required,min=1,max=500"`
	MimeType     string  `json:"mime_type"      validate:"required,max=100"`
	SizeBytes    int64   `json:"size_bytes"     validate:"required,gt=0"`
	ResourceType *string `json:"resource_type"  validate:"omitempty,max=50"`
	ResourceID   *string `json:"resource_id"    validate:"omitempty,uuid4"`
}

type confirmRequest struct {
	StorageKey   string  `json:"storage_key"    validate:"required,min=1,max=1000"`
	OriginalName string  `json:"original_name"  validate:"required,min=1,max=500"`
	MimeType     string  `json:"mime_type"      validate:"required,max=100"`
	SizeBytes    int64   `json:"size_bytes"     validate:"required,gt=0"`
	ResourceType *string `json:"resource_type"  validate:"omitempty,max=50"`
	ResourceID   *string `json:"resource_id"    validate:"omitempty,uuid4"`
}

// ─── Response DTO ─────────────────────────────────────────────────────────────

type uploadDTO struct {
	ID           string  `json:"id"`
	TenantID     string  `json:"tenant_id"`
	UploadedBy   *string `json:"uploaded_by"`
	StorageKey   string  `json:"storage_key"`
	OriginalName string  `json:"original_name"`
	MimeType     string  `json:"mime_type"`
	SizeBytes    int64   `json:"size_bytes"`
	ResourceType *string `json:"resource_type"`
	ResourceID   *string `json:"resource_id"`
	IsActive     bool    `json:"is_active"`
	CreatedAt    string  `json:"created_at"`
}

// ─── Handlers ─────────────────────────────────────────────────────────────────

// List godoc
// GET /api/v1/files[?resource_type=vehicle&resource_id=...&is_active=true]
func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()

	var f ListFilters
	if v := q.Get("resource_type"); v != "" {
		f.ResourceType = &v
	}
	if v := q.Get("resource_id"); v != "" {
		f.ResourceID = &v
	}
	if v := q.Get("is_active"); v != "" {
		b := v == "true"
		f.IsActive = &b
	}

	uploads, err := h.svc.List(r.Context(), f)
	if err != nil {
		apperr.Handle(w, r, err, h.log)
		return
	}

	dtos := make([]uploadDTO, 0, len(uploads))
	for _, u := range uploads {
		dtos = append(dtos, toDTO(u))
	}

	response.Success(w, http.StatusOK, "files retrieved", dtos, h.log)
}

// Get godoc
// GET /api/v1/files/{id}
func (h *Handler) Get(w http.ResponseWriter, r *http.Request) {
	u, err := h.svc.GetByID(r.Context(), r.PathValue("id"))
	if err != nil {
		apperr.Handle(w, r, err, h.log)
		return
	}

	response.Success(w, http.StatusOK, "file retrieved", toDTO(u), h.log)
}

// Presign godoc
// POST /api/v1/files/presign
// Returns a presigned PUT URL for direct client-to-S3 upload.
func (h *Handler) Presign(w http.ResponseWriter, r *http.Request) {
	var req presignRequest
	if err := validation.DecodeJSON(r, &req); err != nil {
		apperr.Handle(w, r, err, h.log)
		return
	}

	result, err := h.svc.Presign(r.Context(), PresignInput{
		OriginalName: req.OriginalName,
		MimeType:     req.MimeType,
		SizeBytes:    req.SizeBytes,
		ResourceType: req.ResourceType,
		ResourceID:   req.ResourceID,
	})
	if err != nil {
		apperr.Handle(w, r, err, h.log)
		return
	}

	response.Success(w, http.StatusOK, "presigned upload URL generated", result, h.log)
}

// Confirm godoc
// POST /api/v1/files/confirm
// Registers a completed S3 upload in the database.
func (h *Handler) Confirm(w http.ResponseWriter, r *http.Request) {
	var req confirmRequest
	if err := validation.DecodeJSON(r, &req); err != nil {
		apperr.Handle(w, r, err, h.log)
		return
	}

	u, err := h.svc.Confirm(r.Context(), ConfirmInput{
		StorageKey:   req.StorageKey,
		OriginalName: req.OriginalName,
		MimeType:     req.MimeType,
		SizeBytes:    req.SizeBytes,
		ResourceType: req.ResourceType,
		ResourceID:   req.ResourceID,
	})
	if err != nil {
		apperr.Handle(w, r, err, h.log)
		return
	}

	response.Success(w, http.StatusCreated, "file upload confirmed", toDTO(u), h.log)
}

// GetDownloadURL godoc
// GET /api/v1/files/{id}/url
// Returns a presigned (or public CDN) download URL for the file.
func (h *Handler) GetDownloadURL(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	result, err := h.svc.GetDownloadURL(r.Context(), id)
	if err != nil {
		apperr.Handle(w, r, err, h.log)
		return
	}

	response.Success(w, http.StatusOK, "download URL generated", result, h.log)
}

// Delete godoc
// DELETE /api/v1/files/{id}
// Removes the object from storage and deletes the DB record.
func (h *Handler) Delete(w http.ResponseWriter, r *http.Request) {
	if err := h.svc.Delete(r.Context(), r.PathValue("id")); err != nil {
		apperr.Handle(w, r, err, h.log)
		return
	}

	response.NoContent(w)
}

// ─── Helper ───────────────────────────────────────────────────────────────────

func toDTO(u *Upload) uploadDTO {
	return uploadDTO{
		ID:           u.ID,
		TenantID:     u.TenantID,
		UploadedBy:   u.UploadedBy,
		StorageKey:   u.StorageKey,
		OriginalName: u.OriginalName,
		MimeType:     u.MimeType,
		SizeBytes:    u.SizeBytes,
		ResourceType: u.ResourceType,
		ResourceID:   u.ResourceID,
		IsActive:     u.IsActive,
		CreatedAt:    u.CreatedAt.UTC().Format("2006-01-02T15:04:05Z"),
	}
}
