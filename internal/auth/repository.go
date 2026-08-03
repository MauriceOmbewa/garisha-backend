package auth

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Repository handles all database operations for the auth domain.
type Repository struct {
	db *pgxpool.Pool
}

// NewRepository creates a Repository backed by the given pool.
func NewRepository(db *pgxpool.Pool) *Repository {
	return &Repository{db: db}
}

const userCols = `id, tenant_id, branch_id, google_sub, email, name, avatar_url, role,
                  permissions, is_active, created_at, updated_at`

// FindByGoogleSub returns the user matching googleSub.
// tenant_id is optional — pass empty string to find a user regardless of tenant.
func (r *Repository) FindByGoogleSub(ctx context.Context, tenantID, googleSub string) (*User, error) {
	var q string
	var args []any

	if tenantID != "" {
		q = `SELECT ` + userCols + `
		     FROM   users
		     WHERE  tenant_id = $1 AND google_sub = $2
		     LIMIT  1`
		args = []any{tenantID, googleSub}
	} else {
		q = `SELECT ` + userCols + `
		     FROM   users
		     WHERE  google_sub = $1
		     LIMIT  1`
		args = []any{googleSub}
	}

	user, err := scanUser(r.db.QueryRow(ctx, q, args...))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("auth: find by google sub: %w", err)
	}
	return user, nil
}

// FindByID returns a user by primary key or (nil, nil) if not found.
func (r *Repository) FindByID(ctx context.Context, id string) (*User, error) {
	q := `SELECT ` + userCols + `
	      FROM   users
	      WHERE  id = $1
	      LIMIT  1`

	user, err := scanUser(r.db.QueryRow(ctx, q, id))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("auth: find by id: %w", err)
	}
	return user, nil
}

// Create inserts a new user and returns the persisted record.
func (r *Repository) Create(ctx context.Context, params CreateUserParams) (*User, error) {
	q := `INSERT INTO users (tenant_id, google_sub, email, name, avatar_url, role)
	      VALUES            ($1, $2, $3, $4, $5, $6)
	      RETURNING ` + userCols

	user, err := scanUser(r.db.QueryRow(ctx, q,
		params.TenantID, params.GoogleSub, params.Email,
		params.Name, params.AvatarURL, params.Role,
	))
	if err != nil {
		return nil, fmt.Errorf("auth: create user: %w", err)
	}
	return user, nil
}

// CreateUserParams holds the fields required to insert a new user.
type CreateUserParams struct {
	TenantID  *string
	GoogleSub string
	Email     string
	Name      string
	AvatarURL *string
	Role      string
}

// AssignTenant updates a user's tenant_id and role atomically.
func (r *Repository) AssignTenant(ctx context.Context, userID, tenantID, role string) error {
	const q = `UPDATE users SET tenant_id = $2, role = $3, updated_at = NOW() WHERE id = $1`
	ct, err := r.db.Exec(ctx, q, userID, tenantID, role)
	if err != nil {
		return fmt.Errorf("auth: assign tenant: %w", err)
	}
	if ct.RowsAffected() == 0 {
		return fmt.Errorf("auth: assign tenant: user not found")
	}
	return nil
}

// scanUser reads one row (must include branch_id) into a User struct.
func scanUser(row pgx.Row) (*User, error) {
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
