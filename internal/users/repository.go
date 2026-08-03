package users

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Repository handles all database operations for the users domain.
type Repository struct {
	db *pgxpool.Pool
}

// NewRepository creates a Repository backed by the given pool.
func NewRepository(db *pgxpool.Pool) *Repository {
	return &Repository{db: db}
}

const userCols = `id, tenant_id, branch_id, google_sub, email, name, avatar_url,
                  role, permissions, is_active, created_at, updated_at`

// List returns all users belonging to the given tenant, ordered by creation date.
func (r *Repository) List(ctx context.Context, tenantID string) ([]*User, error) {
	q := `SELECT ` + userCols + `
	      FROM   users
	      WHERE  tenant_id = $1
	      ORDER  BY created_at DESC`

	rows, err := r.db.Query(ctx, q, tenantID)
	if err != nil {
		return nil, fmt.Errorf("users: list: %w", err)
	}
	defer rows.Close()

	var result []*User
	for rows.Next() {
		u, err := scanUser(rows)
		if err != nil {
			return nil, fmt.Errorf("users: list scan: %w", err)
		}
		result = append(result, u)
	}

	return result, rows.Err()
}

// FindByID returns a user by UUID. Returns (nil, nil) when not found.
func (r *Repository) FindByID(ctx context.Context, id string) (*User, error) {
	q := `SELECT ` + userCols + ` FROM users WHERE id = $1 LIMIT 1`

	u, err := scanUser(r.db.QueryRow(ctx, q, id))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("users: find by id: %w", err)
	}

	return u, nil
}

// UpdateRole changes the role of a user.
func (r *Repository) UpdateRole(ctx context.Context, id, role string) (*User, error) {
	q := `UPDATE users SET role = $2, updated_at = NOW()
	      WHERE id = $1 RETURNING ` + userCols

	u, err := scanUser(r.db.QueryRow(ctx, q, id, role))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("users: update role: %w", err)
	}

	return u, nil
}

// AssignBranch assigns (or removes) a branch from a user.
// Pass nil branchID to remove the branch assignment (grant cross-branch access).
func (r *Repository) AssignBranch(ctx context.Context, id string, branchID *string) (*User, error) {
	q := `UPDATE users SET branch_id = $2, updated_at = NOW()
	      WHERE id = $1 RETURNING ` + userCols

	u, err := scanUser(r.db.QueryRow(ctx, q, id, branchID))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("users: assign branch: %w", err)
	}

	return u, nil
}

// SetActive toggles the is_active flag on a user.
func (r *Repository) SetActive(ctx context.Context, id string, active bool) (*User, error) {
	q := `UPDATE users SET is_active = $2, updated_at = NOW()
	      WHERE id = $1 RETURNING ` + userCols

	u, err := scanUser(r.db.QueryRow(ctx, q, id, active))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("users: set active: %w", err)
	}

	return u, nil
}

// UpdatePermissions replaces the per-user permission overrides.
func (r *Repository) UpdatePermissions(ctx context.Context, id string, perms []string) (*User, error) {
	q := `UPDATE users SET permissions = $2, updated_at = NOW()
	      WHERE id = $1 RETURNING ` + userCols

	u, err := scanUser(r.db.QueryRow(ctx, q, id, perms))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("users: update permissions: %w", err)
	}

	return u, nil
}

// Delete hard-deletes a user by ID.
func (r *Repository) Delete(ctx context.Context, id string) error {
	ct, err := r.db.Exec(ctx, `DELETE FROM users WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("users: delete: %w", err)
	}
	if ct.RowsAffected() == 0 {
		return pgx.ErrNoRows
	}
	return nil
}

// ── Scanner ───────────────────────────────────────────────────────────────────

type scanner interface{ Scan(dest ...any) error }

func scanUser(row scanner) (*User, error) {
	var u User
	err := row.Scan(
		&u.ID,
		&u.TenantID,
		&u.BranchID,
		&u.GoogleSub,
		&u.Email,
		&u.Name,
		&u.AvatarURL,
		&u.Role,
		&u.Permissions,
		&u.IsActive,
		&u.CreatedAt,
		&u.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	return &u, nil
}
