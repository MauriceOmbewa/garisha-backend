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

// FindByGoogleSub returns the user matching (tenantID, googleSub).
// Returns (nil, nil) when no row is found so the caller can distinguish
// "not found" from a real database error.
func (r *Repository) FindByGoogleSub(ctx context.Context, tenantID, googleSub string) (*User, error) {
	const q = `
		SELECT id, tenant_id, google_sub, email, name, avatar_url, role,
		       is_active, created_at, updated_at
		FROM   users
		WHERE  tenant_id  = $1
		AND    google_sub = $2
		LIMIT  1`

	user, err := scanUser(r.db.QueryRow(ctx, q, tenantID, googleSub))
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
	const q = `
		SELECT id, tenant_id, google_sub, email, name, avatar_url, role,
		       is_active, created_at, updated_at
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
	const q = `
		INSERT INTO users (tenant_id, google_sub, email, name, avatar_url, role)
		VALUES            ($1, $2, $3, $4, $5, $6)
		RETURNING id, tenant_id, google_sub, email, name, avatar_url, role,
		          is_active, created_at, updated_at`

	user, err := scanUser(r.db.QueryRow(ctx, q,
		params.TenantID,
		params.GoogleSub,
		params.Email,
		params.Name,
		params.AvatarURL,
		params.Role,
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

// scanUser reads one row into a User struct.
func scanUser(row pgx.Row) (*User, error) {
	var u User

	err := row.Scan(
		&u.ID,
		&u.TenantID,
		&u.GoogleSub,
		&u.Email,
		&u.Name,
		&u.AvatarURL,
		&u.Role,
		&u.IsActive,
		&u.CreatedAt,
		&u.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}

	return &u, nil
}
