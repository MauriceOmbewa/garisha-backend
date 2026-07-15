package audit

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Repository handles all database operations for the audit domain.
// It only ever inserts — no updates or deletes.
type Repository struct {
	db *pgxpool.Pool
}

// NewRepository creates a Repository backed by the given pool.
func NewRepository(db *pgxpool.Pool) *Repository {
	return &Repository{db: db}
}

// ── Column list ───────────────────────────────────────────────────────────────

const cols = `
	id, tenant_id,
	actor_id, actor_email, actor_role,
	action, resource_type, resource_id,
	changes,
	ip_address, user_agent, request_id,
	status, error_message,
	created_at`

// ── Queries ───────────────────────────────────────────────────────────────────

// List returns audit log entries for a tenant, optionally filtered and paginated.
func (r *Repository) List(ctx context.Context, tenantID string, f ListFilters) ([]*Log, error) {
	args := []any{tenantID}
	conditions := []string{"tenant_id = $1"}

	if f.ActorID != nil {
		args = append(args, *f.ActorID)
		conditions = append(conditions, fmt.Sprintf("actor_id = $%d", len(args)))
	}
	if f.Action != nil {
		args = append(args, *f.Action)
		conditions = append(conditions, fmt.Sprintf("action = $%d", len(args)))
	}
	if f.ResourceType != nil {
		args = append(args, *f.ResourceType)
		conditions = append(conditions, fmt.Sprintf("resource_type = $%d", len(args)))
	}
	if f.ResourceID != nil {
		args = append(args, *f.ResourceID)
		conditions = append(conditions, fmt.Sprintf("resource_id = $%d", len(args)))
	}
	if f.Status != nil {
		args = append(args, *f.Status)
		conditions = append(conditions, fmt.Sprintf("status = $%d", len(args)))
	}
	if f.FromDate != nil {
		args = append(args, *f.FromDate)
		conditions = append(conditions, fmt.Sprintf("created_at >= $%d", len(args)))
	}
	if f.ToDate != nil {
		args = append(args, *f.ToDate)
		conditions = append(conditions, fmt.Sprintf("created_at <= $%d", len(args)))
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
		FROM   audit_logs
		WHERE  %s
		ORDER  BY created_at DESC
		LIMIT  $%d OFFSET $%d`,
		cols,
		strings.Join(conditions, " AND "),
		len(args)-1,
		len(args),
	)

	rows, err := r.db.Query(ctx, q, args...)
	if err != nil {
		return nil, fmt.Errorf("audit: list: %w", err)
	}
	defer rows.Close()

	var result []*Log
	for rows.Next() {
		entry, err := scanLog(rows)
		if err != nil {
			return nil, fmt.Errorf("audit: list scan: %w", err)
		}
		result = append(result, entry)
	}

	return result, rows.Err()
}

// FindByID returns a single audit log entry.  Returns (nil, nil) when not found.
func (r *Repository) FindByID(ctx context.Context, id string) (*Log, error) {
	q := `SELECT ` + cols + ` FROM audit_logs WHERE id = $1 LIMIT 1`

	entry, err := scanLog(r.db.QueryRow(ctx, q, id))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("audit: find by id: %w", err)
	}

	return entry, nil
}

// Append inserts a new audit log entry.  This is the only write operation.
func (r *Repository) Append(ctx context.Context, p AppendParams) (*Log, error) {
	changesJSON, err := json.Marshal(p.Changes)
	if err != nil {
		return nil, fmt.Errorf("audit: marshal changes: %w", err)
	}

	q := `
		INSERT INTO audit_logs (
		    tenant_id,
		    actor_id, actor_email, actor_role,
		    action, resource_type, resource_id,
		    changes,
		    ip_address, user_agent, request_id,
		    status, error_message
		) VALUES (
		    $1,
		    $2, $3, $4,
		    $5, $6, $7,
		    $8,
		    $9, $10, $11,
		    $12, $13
		)
		RETURNING ` + cols

	entry, err := scanLog(r.db.QueryRow(ctx, q,
		p.TenantID,
		p.ActorID, p.ActorEmail, p.ActorRole,
		p.Action, p.ResourceType, p.ResourceID,
		changesJSON,
		p.IPAddress, p.UserAgent, p.RequestID,
		p.Status, p.ErrorMessage,
	))
	if err != nil {
		return nil, fmt.Errorf("audit: append: %w", err)
	}

	return entry, nil
}

// ── Params ────────────────────────────────────────────────────────────────────

// AppendParams holds all fields required to insert an audit entry.
type AppendParams struct {
	TenantID     string
	ActorID      *string
	ActorEmail   *string
	ActorRole    *string
	Action       string
	ResourceType string
	ResourceID   *string
	Changes      map[string]any
	IPAddress    *string
	UserAgent    *string
	RequestID    *string
	Status       LogStatus
	ErrorMessage *string
}

// ── Scanner ───────────────────────────────────────────────────────────────────

type rowScanner interface {
	Scan(dest ...any) error
}

func scanLog(row rowScanner) (*Log, error) {
	var entry Log
	var status string
	var changesRaw []byte
	var ipStr *string

	err := row.Scan(
		&entry.ID, &entry.TenantID,
		&entry.ActorID, &entry.ActorEmail, &entry.ActorRole,
		&entry.Action, &entry.ResourceType, &entry.ResourceID,
		&changesRaw,
		&ipStr, &entry.UserAgent, &entry.RequestID,
		&status, &entry.ErrorMessage,
		&entry.CreatedAt,
	)
	if err != nil {
		return nil, err
	}

	entry.Status = LogStatus(status)

	if len(changesRaw) > 0 {
		if err := json.Unmarshal(changesRaw, &entry.Changes); err != nil {
			entry.Changes = map[string]any{}
		}
	} else {
		entry.Changes = map[string]any{}
	}

	if ipStr != nil {
		entry.IPAddress = ipStr
	}

	return &entry, nil
}
