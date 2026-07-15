package notifications

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Repository handles all database operations for the notifications domain.
type Repository struct {
	db *pgxpool.Pool
}

// NewRepository creates a Repository backed by the given pool.
func NewRepository(db *pgxpool.Pool) *Repository {
	return &Repository{db: db}
}

// ── Column list ───────────────────────────────────────────────────────────────

const cols = `
	id, tenant_id, user_id,
	type, title, body,
	resource_type, resource_id,
	is_read, read_at,
	created_at`

// ── Filters ───────────────────────────────────────────────────────────────────

// ListFilters narrows the notifications query.
type ListFilters struct {
	IsRead *bool   // nil = all, true = read only, false = unread only
	Type   *string // filter by notification type
	Limit  int     // 0 = default (50)
	Offset int
}

// ── Queries ───────────────────────────────────────────────────────────────────

// List returns notifications for a specific user within a tenant.
func (r *Repository) List(ctx context.Context, tenantID, userID string, f ListFilters) ([]*Notification, error) {
	args := []any{tenantID, userID}
	where := "tenant_id = $1 AND user_id = $2"

	if f.IsRead != nil {
		args = append(args, *f.IsRead)
		where += fmt.Sprintf(" AND is_read = $%d", len(args))
	}
	if f.Type != nil {
		args = append(args, *f.Type)
		where += fmt.Sprintf(" AND type = $%d", len(args))
	}

	limit := f.Limit
	if limit <= 0 {
		limit = 50
	}
	if limit > 200 {
		limit = 200
	}

	args = append(args, limit, f.Offset)
	q := fmt.Sprintf(`
		SELECT %s
		FROM   notifications
		WHERE  %s
		ORDER  BY created_at DESC
		LIMIT  $%d OFFSET $%d`,
		cols, where, len(args)-1, len(args),
	)

	rows, err := r.db.Query(ctx, q, args...)
	if err != nil {
		return nil, fmt.Errorf("notifications: list: %w", err)
	}
	defer rows.Close()

	var result []*Notification
	for rows.Next() {
		n, err := scanNotification(rows)
		if err != nil {
			return nil, fmt.Errorf("notifications: list scan: %w", err)
		}
		result = append(result, n)
	}

	return result, rows.Err()
}

// FindByID returns a notification by UUID.  Returns (nil, nil) when not found.
func (r *Repository) FindByID(ctx context.Context, id string) (*Notification, error) {
	q := `SELECT ` + cols + ` FROM notifications WHERE id = $1 LIMIT 1`

	n, err := scanNotification(r.db.QueryRow(ctx, q, id))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("notifications: find by id: %w", err)
	}

	return n, nil
}

// CountUnread returns the number of unread notifications for a user.
func (r *Repository) CountUnread(ctx context.Context, tenantID, userID string) (int, error) {
	var count int
	err := r.db.QueryRow(ctx,
		`SELECT COUNT(*) FROM notifications WHERE tenant_id = $1 AND user_id = $2 AND is_read = FALSE`,
		tenantID, userID,
	).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("notifications: count unread: %w", err)
	}

	return count, nil
}

// Create inserts a new notification.
func (r *Repository) Create(ctx context.Context, p CreateParams) (*Notification, error) {
	q := `
		INSERT INTO notifications (tenant_id, user_id, type, title, body, resource_type, resource_id)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		RETURNING ` + cols

	n, err := scanNotification(r.db.QueryRow(ctx, q,
		p.TenantID, p.UserID, p.Type, p.Title, p.Body, p.ResourceType, p.ResourceID,
	))
	if err != nil {
		return nil, fmt.Errorf("notifications: create: %w", err)
	}

	return n, nil
}

// MarkRead marks a single notification as read and sets read_at.
func (r *Repository) MarkRead(ctx context.Context, id string, readAt time.Time) (*Notification, error) {
	q := `
		UPDATE notifications
		SET    is_read = TRUE, read_at = $2
		WHERE  id = $1
		RETURNING ` + cols

	n, err := scanNotification(r.db.QueryRow(ctx, q, id, readAt))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("notifications: mark read: %w", err)
	}

	return n, nil
}

// MarkAllRead marks every unread notification for a user as read.
// Returns the number of rows updated.
func (r *Repository) MarkAllRead(ctx context.Context, tenantID, userID string, readAt time.Time) (int64, error) {
	ct, err := r.db.Exec(ctx,
		`UPDATE notifications
		 SET    is_read = TRUE, read_at = $3
		 WHERE  tenant_id = $1 AND user_id = $2 AND is_read = FALSE`,
		tenantID, userID, readAt,
	)
	if err != nil {
		return 0, fmt.Errorf("notifications: mark all read: %w", err)
	}

	return ct.RowsAffected(), nil
}

// Delete removes a single notification.
func (r *Repository) Delete(ctx context.Context, id string) error {
	ct, err := r.db.Exec(ctx, `DELETE FROM notifications WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("notifications: delete: %w", err)
	}

	if ct.RowsAffected() == 0 {
		return pgx.ErrNoRows
	}

	return nil
}

// DeleteAllRead removes all read notifications for a user (housekeeping).
func (r *Repository) DeleteAllRead(ctx context.Context, tenantID, userID string) (int64, error) {
	ct, err := r.db.Exec(ctx,
		`DELETE FROM notifications WHERE tenant_id = $1 AND user_id = $2 AND is_read = TRUE`,
		tenantID, userID,
	)
	if err != nil {
		return 0, fmt.Errorf("notifications: delete all read: %w", err)
	}

	return ct.RowsAffected(), nil
}

// ── Params ────────────────────────────────────────────────────────────────────

// CreateParams holds all fields required to insert a new notification.
type CreateParams struct {
	TenantID     string
	UserID       string
	Type         string
	Title        string
	Body         string
	ResourceType *string
	ResourceID   *string
}

// ── Scanner ───────────────────────────────────────────────────────────────────

type rowScanner interface {
	Scan(dest ...any) error
}

func scanNotification(row rowScanner) (*Notification, error) {
	var n Notification

	err := row.Scan(
		&n.ID, &n.TenantID, &n.UserID,
		&n.Type, &n.Title, &n.Body,
		&n.ResourceType, &n.ResourceID,
		&n.IsRead, &n.ReadAt,
		&n.CreatedAt,
	)
	if err != nil {
		return nil, err
	}

	return &n, nil
}
