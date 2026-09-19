package repo

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/hekemen/automata/internal/domain/tenant"
	"github.com/jackc/pgx/v5/pgxpool"
)

type userPostgresRepo struct {
	pool *pgxpool.Pool
}

// NewUserPostgresRepo creates a new PostgreSQL user repository.
func NewUserPostgresRepo(pool *pgxpool.Pool) tenant.UserRepository {
	return &userPostgresRepo{pool: pool}
}

func (r *userPostgresRepo) Create(u *tenant.User) error {
	if u.ID == "" {
		u.ID = uuid.New().String()
	}
	if err := u.Validate(); err != nil {
		return err
	}

	query := `
		INSERT INTO tenant_users (id, tenant_id, email, password_hash, is_owner, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, NOW(), NOW())
		RETURNING id, created_at, updated_at
	`

	err := r.pool.QueryRow(context.Background(), query,
		u.ID, u.TenantID, u.Email, u.PasswordHash, u.IsOwner,
	).Scan(&u.ID, &u.CreatedAt, &u.UpdatedAt)

	if err != nil {
		return fmt.Errorf("create user: %w", err)
	}
	return nil
}

func (r *userPostgresRepo) GetByID(id string) (*tenant.User, error) {
	u := &tenant.User{}

	query := `
		SELECT id, tenant_id, email, password_hash, is_owner, created_at, updated_at
		FROM tenant_users WHERE id = $1
	`

	err := r.pool.QueryRow(context.Background(), query, id).Scan(
		&u.ID, &u.TenantID, &u.Email, &u.PasswordHash, &u.IsOwner,
		&u.CreatedAt, &u.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("get user by ID: %w", err)
	}

	return u, nil
}

func (r *userPostgresRepo) GetByEmail(tenantID, email string) (*tenant.User, error) {
	u := &tenant.User{}

	query := `
		SELECT id, tenant_id, email, password_hash, is_owner, created_at, updated_at
		FROM tenant_users WHERE tenant_id = $1 AND email = $2
	`

	err := r.pool.QueryRow(context.Background(), query, tenantID, email).Scan(
		&u.ID, &u.TenantID, &u.Email, &u.PasswordHash, &u.IsOwner,
		&u.CreatedAt, &u.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("get user by email: %w", err)
	}

	return u, nil
}

func (r *userPostgresRepo) GetByEmailGlobal(email string) ([]*tenant.User, error) {
	query := `
		SELECT id, tenant_id, email, password_hash, is_owner, created_at, updated_at
		FROM tenant_users WHERE email = $1
	`

	rows, err := r.pool.Query(context.Background(), query, email)
	if err != nil {
		return nil, fmt.Errorf("get user by email (global): %w", err)
	}
	defer rows.Close()

	var users []*tenant.User
	for rows.Next() {
		u := &tenant.User{}
		err := rows.Scan(
			&u.ID, &u.TenantID, &u.Email, &u.PasswordHash, &u.IsOwner,
			&u.CreatedAt, &u.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("scan user: %w", err)
		}
		users = append(users, u)
	}

	if len(users) == 0 {
		return nil, fmt.Errorf("user not found")
	}

	return users, nil
}

func (r *userPostgresRepo) Update(u *tenant.User) error {
	query := `
		UPDATE tenant_users SET email = $1, password_hash = $2, is_owner = $3, updated_at = NOW()
		WHERE id = $4
	`

	result, err := r.pool.Exec(context.Background(), query,
		u.Email, u.PasswordHash, u.IsOwner, u.ID,
	)
	if err != nil {
		return fmt.Errorf("update user: %w", err)
	}

	if result.RowsAffected() == 0 {
		return fmt.Errorf("user not found: %s", u.ID)
	}

	return nil
}

func (r *userPostgresRepo) Delete(id string) error {
	query := `DELETE FROM tenant_users WHERE id = $1`
	result, err := r.pool.Exec(context.Background(), query, id)
	if err != nil {
		return fmt.Errorf("delete user: %w", err)
	}

	if result.RowsAffected() == 0 {
		return fmt.Errorf("user not found: %s", id)
	}

	return nil
}
