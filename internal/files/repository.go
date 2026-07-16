package files

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Repository handles all database operations for the files domain.
type Repository struct {
	db *pgxpool.Pool
}

// NewRepository creates a Repository backed by the given pool.
func NewRepository(db *pgxpool.Pool) *Repository {
	return &Repository{db: db}
}

// ── Column list ───────────────────────────────────────────────────────────────

const cols = `
	id, tenant_id, uploaded_by,
	storage_key, bucket,
	original_name, mime_type, size_bytes,
	resource_type, resource_id,
	is_active, created_at`

// ── Filters ───────────────────────────────────────────────────────────────────

// ListFilters narrows the file uploads query.
type ListFilters struct {
	ResourceType *string
	ResourceID   *string
	IsActive     *bool
}

// ── Queries ───────────────────────────────────────────────────────────────────

// List returns file upload records for a tenant, optionally filtered.
func (r *Repository) List(ctx context.Context, tenantID string, f ListFilters) ([]*Upload, error) {
	args := []any{tenantID}
	conditions := []string{"tenant_id = $1"}

	if f.ResourceType != nil {
		args = append(args, *f.ResourceType)
		conditions = append(conditions, fmt.Sprintf("resource_type = $%d", len(args)))
	}
	if f.ResourceID != nil {
		args = append(args, *f.ResourceID)
		conditions = append(conditions, fmt.Sprintf("resource_id = $%d", len(args)))
	}
	if f.IsActive != nil {
		args = append(args, *f.IsActive)
		conditions = append(conditions, fmt.Sprintf("is_active = $%d", len(args)))
	}

	q := `SELECT ` + cols + `
	      FROM   file_uploads
	      WHERE  ` + strings.Join(conditions, " AND ") + `
	      ORDER  BY created_at DESC`

	rows, err := r.db.Query(ctx, q, args...)
	if err != nil {
		return nil, fmt.Errorf("files: list: %w", err)
	}
	defer rows.Close()

	var result []*Upload
	for rows.Next() {
		u, err := scanUpload(rows)
		if err != nil {
			return nil, fmt.Errorf("files: list scan: %w", err)
		}
		result = append(result, u)
	}

	return result, rows.Err()
}

// FindByID returns a file upload record by UUID. Returns (nil, nil) when not found.
func (r *Repository) FindByID(ctx context.Context, id string) (*Upload, error) {
	q := `SELECT ` + cols + ` FROM file_uploads WHERE id = $1 LIMIT 1`

	u, err := scanUpload(r.db.QueryRow(ctx, q, id))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("files: find by id: %w", err)
	}

	return u, nil
}

// FindByStorageKey returns the record with the given bucket + key combination.
// Returns (nil, nil) when not found.
func (r *Repository) FindByStorageKey(ctx context.Context, bucket, key string) (*Upload, error) {
	q := `SELECT ` + cols + `
	      FROM   file_uploads
	      WHERE  bucket = $1 AND storage_key = $2
	      LIMIT  1`

	u, err := scanUpload(r.db.QueryRow(ctx, q, bucket, key))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("files: find by storage key: %w", err)
	}

	return u, nil
}

// Create inserts a new file upload record.
func (r *Repository) Create(ctx context.Context, p CreateParams) (*Upload, error) {
	q := `
		INSERT INTO file_uploads (
		    tenant_id, uploaded_by,
		    storage_key, bucket,
		    original_name, mime_type, size_bytes,
		    resource_type, resource_id
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		RETURNING ` + cols

	u, err := scanUpload(r.db.QueryRow(ctx, q,
		p.TenantID, p.UploadedBy,
		p.StorageKey, p.Bucket,
		p.OriginalName, p.MimeType, p.SizeBytes,
		p.ResourceType, p.ResourceID,
	))
	if err != nil {
		return nil, fmt.Errorf("files: create: %w", err)
	}

	return u, nil
}

// Deactivate soft-deletes a file upload record.
func (r *Repository) Deactivate(ctx context.Context, id string) error {
	ct, err := r.db.Exec(ctx,
		`UPDATE file_uploads SET is_active = FALSE WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("files: deactivate: %w", err)
	}

	if ct.RowsAffected() == 0 {
		return pgx.ErrNoRows
	}

	return nil
}

// HardDelete removes the record from the database.
// Call this only after the object has been deleted from storage.
func (r *Repository) HardDelete(ctx context.Context, id string) error {
	ct, err := r.db.Exec(ctx, `DELETE FROM file_uploads WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("files: hard delete: %w", err)
	}

	if ct.RowsAffected() == 0 {
		return pgx.ErrNoRows
	}

	return nil
}

// ── Params ────────────────────────────────────────────────────────────────────

type CreateParams struct {
	TenantID     string
	UploadedBy   *string
	StorageKey   string
	Bucket       string
	OriginalName string
	MimeType     string
	SizeBytes    int64
	ResourceType *string
	ResourceID   *string
}

// ── Scanner ───────────────────────────────────────────────────────────────────

type rowScanner interface {
	Scan(dest ...any) error
}

func scanUpload(row rowScanner) (*Upload, error) {
	var u Upload

	err := row.Scan(
		&u.ID, &u.TenantID, &u.UploadedBy,
		&u.StorageKey, &u.Bucket,
		&u.OriginalName, &u.MimeType, &u.SizeBytes,
		&u.ResourceType, &u.ResourceID,
		&u.IsActive, &u.CreatedAt,
	)
	if err != nil {
		return nil, err
	}

	return &u, nil
}
