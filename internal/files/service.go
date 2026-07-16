package files

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"path"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"

	apperr "github.com/MauriceOmbewa/garisha-backend/internal/platform/errors"
	"github.com/MauriceOmbewa/garisha-backend/internal/platform/middleware"
	"github.com/MauriceOmbewa/garisha-backend/internal/platform/storage"
	"github.com/MauriceOmbewa/garisha-backend/internal/platform/tenant"
)

const (
	presignUploadTTL   = 15 * time.Minute
	presignDownloadTTL = 60 * time.Minute
)

// Service implements business logic for file upload management.
type Service struct {
	repo    *Repository
	storage *storage.Client
	log     *slog.Logger
}

// NewService creates a Service.
func NewService(repo *Repository, storage *storage.Client, log *slog.Logger) *Service {
	return &Service{repo: repo, storage: storage, log: log}
}

// ── Input types ───────────────────────────────────────────────────────────────

// PresignInput holds the metadata needed to generate a presigned upload URL.
type PresignInput struct {
	OriginalName string
	MimeType     string
	SizeBytes    int64
	ResourceType *string
	ResourceID   *string
}

// ConfirmInput completes a presigned upload by registering it in the DB.
type ConfirmInput struct {
	StorageKey   string
	OriginalName string
	MimeType     string
	SizeBytes    int64
	ResourceType *string
	ResourceID   *string
}

// ── Service methods ───────────────────────────────────────────────────────────

// List returns file upload records for the tenant in ctx, optionally filtered.
func (s *Service) List(ctx context.Context, f ListFilters) ([]*Upload, error) {
	tenantID := tenant.MustGetTenantID(ctx)

	uploads, err := s.repo.List(ctx, tenantID, f)
	if err != nil {
		return nil, apperr.Internal("failed to list files", err)
	}

	return uploads, nil
}

// GetByID returns a single file upload scoped to the tenant in ctx.
func (s *Service) GetByID(ctx context.Context, id string) (*Upload, error) {
	tenantID := tenant.MustGetTenantID(ctx)

	u, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return nil, apperr.Internal("failed to get file", err)
	}

	if u == nil || u.TenantID != tenantID {
		return nil, apperr.NotFound("file")
	}

	return u, nil
}

// Presign generates a presigned PUT URL for direct client-to-S3 upload.
// It does NOT create a DB record yet — that happens in Confirm.
func (s *Service) Presign(ctx context.Context, in PresignInput) (PresignResponse, error) {
	tenantID := tenant.MustGetTenantID(ctx)

	if !s.storage.Enabled() {
		return PresignResponse{}, apperr.BadRequest("file storage is not configured")
	}

	// Validate MIME type.
	if !AllowedMimeTypes[in.MimeType] {
		return PresignResponse{}, apperr.BadRequest(fmt.Sprintf(
			"unsupported file type %q", in.MimeType,
		))
	}

	// Validate size.
	if in.SizeBytes <= 0 {
		return PresignResponse{}, apperr.BadRequest("size_bytes must be greater than 0")
	}

	if in.SizeBytes > MaxFileSizeBytes {
		return PresignResponse{}, apperr.BadRequest(fmt.Sprintf(
			"file exceeds maximum allowed size of %d MB", MaxFileSizeBytes/1024/1024,
		))
	}

	// Build a storage key:  tenants/{tenantID}/{resourceType}/{resourceID}/{uuid}/{filename}
	key := buildStorageKey(tenantID, in.ResourceType, in.ResourceID, in.OriginalName)

	uploadURL, err := s.storage.PresignUpload(ctx, key, in.MimeType, presignUploadTTL)
	if err != nil {
		return PresignResponse{}, apperr.Internal("failed to generate upload URL", err)
	}

	return PresignResponse{
		UploadURL:  uploadURL,
		StorageKey: key,
		ExpiresIn:  int(presignUploadTTL.Seconds()),
	}, nil
}

// Confirm registers a completed upload in the database.
// Called by the client after a successful PUT to the presigned URL.
func (s *Service) Confirm(ctx context.Context, in ConfirmInput) (*Upload, error) {
	tenantID := tenant.MustGetTenantID(ctx)

	if !s.storage.Enabled() {
		return nil, apperr.BadRequest("file storage is not configured")
	}

	// Validate MIME type again — guard against client tampering.
	if !AllowedMimeTypes[in.MimeType] {
		return nil, apperr.BadRequest(fmt.Sprintf("unsupported file type %q", in.MimeType))
	}

	if in.SizeBytes <= 0 || in.SizeBytes > MaxFileSizeBytes {
		return nil, apperr.BadRequest("invalid size_bytes")
	}

	// Prevent duplicate confirmations for the same key.
	existing, err := s.repo.FindByStorageKey(ctx, s.storage.Bucket(), in.StorageKey)
	if err != nil {
		return nil, apperr.Internal("failed to check for duplicate file", err)
	}

	if existing != nil {
		return existing, nil // idempotent — already confirmed
	}

	// Extract actor from JWT.
	var uploaderID *string
	if claims := middleware.GetClaims(ctx); claims != nil && claims.UserID != "" {
		id := claims.UserID
		uploaderID = &id
	}

	u, err := s.repo.Create(ctx, CreateParams{
		TenantID:     tenantID,
		UploadedBy:   uploaderID,
		StorageKey:   in.StorageKey,
		Bucket:       s.storage.Bucket(),
		OriginalName: in.OriginalName,
		MimeType:     in.MimeType,
		SizeBytes:    in.SizeBytes,
		ResourceType: in.ResourceType,
		ResourceID:   in.ResourceID,
	})
	if err != nil {
		return nil, apperr.Internal("failed to register file upload", err)
	}

	s.log.Info("file upload confirmed",
		"file_id",     u.ID,
		"storage_key", in.StorageKey,
		"tenant_id",   tenantID,
	)

	return u, nil
}

// GetDownloadURL generates a presigned GET URL for a file.
// If the storage client has a public CDN URL configured, it returns that
// instead (no expiry, no presign overhead).
func (s *Service) GetDownloadURL(ctx context.Context, id string) (DownloadURLResponse, error) {
	u, err := s.GetByID(ctx, id)
	if err != nil {
		return DownloadURLResponse{}, err
	}

	if !s.storage.Enabled() {
		return DownloadURLResponse{}, apperr.BadRequest("file storage is not configured")
	}

	// Prefer public CDN URL when available.
	if pub := s.storage.PublicURL(u.StorageKey); pub != "" {
		return DownloadURLResponse{URL: pub, ExpiresIn: 0}, nil
	}

	url, err := s.storage.PresignDownload(ctx, u.StorageKey, presignDownloadTTL)
	if err != nil {
		return DownloadURLResponse{}, apperr.Internal("failed to generate download URL", err)
	}

	return DownloadURLResponse{
		URL:       url,
		ExpiresIn: int(presignDownloadTTL.Seconds()),
	}, nil
}

// Delete deactivates the DB record and removes the object from storage.
func (s *Service) Delete(ctx context.Context, id string) error {
	tenantID := tenant.MustGetTenantID(ctx)

	u, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return apperr.Internal("failed to get file", err)
	}

	if u == nil || u.TenantID != tenantID {
		return apperr.NotFound("file")
	}

	// Delete from object storage first.
	if s.storage.Enabled() {
		if err := s.storage.Delete(ctx, u.StorageKey); err != nil {
			// Log but continue — orphaned storage objects are recoverable;
			// an inconsistent DB state is worse.
			s.log.Warn("failed to delete file from storage",
				"file_id",     id,
				"storage_key", u.StorageKey,
				"error",       err,
			)
		}
	}

	// Hard-delete the DB record.
	if err := s.repo.HardDelete(ctx, id); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return apperr.NotFound("file")
		}
		return apperr.Internal("failed to delete file record", err)
	}

	s.log.Info("file deleted", "file_id", id, "tenant_id", tenantID)
	return nil
}

// ── Helpers ───────────────────────────────────────────────────────────────────

// buildStorageKey produces a structured object key:
//
//	tenants/{tenantID}/{resourceType}/{resourceID}/{timestamp}_{filename}
//	tenants/{tenantID}/general/{timestamp}_{filename}  (no resource)
func buildStorageKey(tenantID string, resourceType, resourceID *string, filename string) string {
	ts := fmt.Sprintf("%d", time.Now().UnixMilli())

	// Sanitise filename: keep only the base name, replace spaces.
	base := path.Base(filename)
	base = strings.ReplaceAll(base, " ", "_")

	resType := "general"
	if resourceType != nil && *resourceType != "" {
		resType = *resourceType
	}

	if resourceID != nil && *resourceID != "" {
		return fmt.Sprintf("tenants/%s/%s/%s/%s_%s", tenantID, resType, *resourceID, ts, base)
	}

	return fmt.Sprintf("tenants/%s/%s/%s_%s", tenantID, resType, ts, base)
}
