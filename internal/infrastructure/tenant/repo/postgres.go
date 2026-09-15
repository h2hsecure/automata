package repo

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/google/uuid"
	"github.com/hekemen/automata/internal/domain/tenant"
	"github.com/jackc/pgx/v5/pgxpool"
)

type postgresRepo struct {
	pool *pgxpool.Pool
}

// NewPostgresRepo creates a new PostgreSQL tenant repository.
func NewPostgresRepo(pool *pgxpool.Pool) tenant.Repository {
	return &postgresRepo{pool: pool}
}

func (r *postgresRepo) Create(t *tenant.Tenant) error {
	if t.ID == "" {
		t.ID = uuid.New().String()
	}
	if err := t.Validate(); err != nil {
		return err
	}

	settingsBytes, err := json.Marshal(t.Settings)
	if err != nil {
		return fmt.Errorf("marshal settings: %w", err)
	}
	if settingsBytes == nil {
		settingsBytes = []byte("{}")
	}

	query := `
		INSERT INTO tenants (id, slug, name, domain, is_active, settings, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, NOW(), NOW())
		RETURNING id, created_at, updated_at
	`

	err = r.pool.QueryRow(context.Background(), query,
		t.ID, t.Slug, t.Name, t.Domain, t.IsActive, settingsBytes,
	).Scan(&t.ID, &t.CreatedAt, &t.UpdatedAt)

	if err != nil {
		return fmt.Errorf("create tenant: %w", err)
	}
	return nil
}

func (r *postgresRepo) GetByID(id string) (*tenant.Tenant, error) {
	t := &tenant.Tenant{}
	var settingsBytes []byte

	query := `
		SELECT id, slug, name, domain, is_active, settings, created_at, updated_at
		FROM tenants WHERE id = $1
	`

	err := r.pool.QueryRow(context.Background(), query, id).Scan(
		&t.ID, &t.Slug, &t.Name, &t.Domain, &t.IsActive,
		&settingsBytes, &t.CreatedAt, &t.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("get tenant by ID: %w", err)
	}

	if len(settingsBytes) > 0 {
		_ = json.Unmarshal(settingsBytes, &t.Settings)
	}
	return t, nil
}

func (r *postgresRepo) GetBySlug(slug string) (*tenant.Tenant, error) {
	t := &tenant.Tenant{}
	var settingsBytes []byte

	query := `
		SELECT id, slug, name, domain, is_active, settings, created_at, updated_at
		FROM tenants WHERE slug = $1
	`

	err := r.pool.QueryRow(context.Background(), query, slug).Scan(
		&t.ID, &t.Slug, &t.Name, &t.Domain, &t.IsActive,
		&settingsBytes, &t.CreatedAt, &t.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("get tenant by slug: %w", err)
	}

	if len(settingsBytes) > 0 {
		_ = json.Unmarshal(settingsBytes, &t.Settings)
	}
	return t, nil
}

func (r *postgresRepo) List(offset, limit int) ([]*tenant.Tenant, error) {
	if limit <= 0 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}

	query := `
		SELECT id, slug, name, domain, is_active, settings, created_at, updated_at
		FROM tenants ORDER BY created_at DESC LIMIT $1 OFFSET $2
	`

	rows, err := r.pool.Query(context.Background(), query, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("list tenants: %w", err)
	}
	defer rows.Close()

	var tenants []*tenant.Tenant
	for rows.Next() {
		t := &tenant.Tenant{}
		var settingsBytes []byte
		err := rows.Scan(
			&t.ID, &t.Slug, &t.Name, &t.Domain, &t.IsActive,
			&settingsBytes, &t.CreatedAt, &t.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("scan tenant: %w", err)
		}
		if len(settingsBytes) > 0 {
			_ = json.Unmarshal(settingsBytes, &t.Settings)
		}
		tenants = append(tenants, t)
	}

	return tenants, rows.Err()
}

func (r *postgresRepo) Update(t *tenant.Tenant) error {
	settingsBytes, err := json.Marshal(t.Settings)
	if err != nil {
		return fmt.Errorf("marshal settings: %w", err)
	}
	if settingsBytes == nil {
		settingsBytes = []byte("{}")
	}

	query := `
		UPDATE tenants SET slug = $1, name = $2, domain = $3, is_active = $4,
			settings = $5, updated_at = NOW()
		WHERE id = $6
	`

	result, err := r.pool.Exec(context.Background(), query,
		t.Slug, t.Name, t.Domain, t.IsActive, settingsBytes, t.ID,
	)
	if err != nil {
		return fmt.Errorf("update tenant: %w", err)
	}

	if result.RowsAffected() == 0 {
		return fmt.Errorf("tenant not found: %s", t.ID)
	}

	return nil
}

func (r *postgresRepo) Delete(id string) error {
	query := `DELETE FROM tenants WHERE id = $1`
	result, err := r.pool.Exec(context.Background(), query, id)
	if err != nil {
		return fmt.Errorf("delete tenant: %w", err)
	}

	if result.RowsAffected() == 0 {
		return fmt.Errorf("tenant not found: %s", id)
	}

	return nil
}
