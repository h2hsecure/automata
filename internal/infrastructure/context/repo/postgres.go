package repo

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/google/uuid"
	domainctx "github.com/hekemen/automata/internal/domain/context"
	"github.com/jackc/pgx/v5/pgxpool"
)

type postgresRepo struct {
	pool *pgxpool.Pool
}

// NewPostgresRepo creates a new PostgreSQL context repository.
func NewPostgresRepo(pool *pgxpool.Pool) domainctx.Repository {
	return &postgresRepo{pool: pool}
}

func (r *postgresRepo) Create(t *domainctx.Context) error {
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
		INSERT INTO contexts (id, slug, name, domain, is_active, settings, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, NOW(), NOW())
		RETURNING id, created_at, updated_at
	`

	err = r.pool.QueryRow(context.Background(), query,
		t.ID, t.Slug, t.Name, t.Domain, t.IsActive, settingsBytes,
	).Scan(&t.ID, &t.CreatedAt, &t.UpdatedAt)

	if err != nil {
		return fmt.Errorf("create context: %w", err)
	}
	return nil
}

func (r *postgresRepo) GetByID(id string) (*domainctx.Context, error) {
	t := &domainctx.Context{}
	var settingsBytes []byte

	query := `
		SELECT id, slug, name, domain, is_active, settings, created_at, updated_at
		FROM contexts WHERE id = $1
	`

	err := r.pool.QueryRow(context.Background(), query, id).Scan(
		&t.ID, &t.Slug, &t.Name, &t.Domain, &t.IsActive,
		&settingsBytes, &t.CreatedAt, &t.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("get context by ID: %w", err)
	}

	if len(settingsBytes) > 0 {
		_ = json.Unmarshal(settingsBytes, &t.Settings)
	}
	return t, nil
}

func (r *postgresRepo) GetBySlug(slug string) (*domainctx.Context, error) {
	t := &domainctx.Context{}
	var settingsBytes []byte

	query := `
		SELECT id, slug, name, domain, is_active, settings, created_at, updated_at
		FROM contexts WHERE slug = $1
	`

	err := r.pool.QueryRow(context.Background(), query, slug).Scan(
		&t.ID, &t.Slug, &t.Name, &t.Domain, &t.IsActive,
		&settingsBytes, &t.CreatedAt, &t.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("get context by slug: %w (slug=%s, err_detail=%v)", err, slug, err)
	}

	if len(settingsBytes) > 0 {
		_ = json.Unmarshal(settingsBytes, &t.Settings)
	}
	return t, nil
}

func (r *postgresRepo) List(offset, limit int) ([]*domainctx.Context, error) {
	if limit <= 0 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}

	query := `
		SELECT id, slug, name, domain, is_active, settings, created_at, updated_at
		FROM contexts ORDER BY created_at DESC LIMIT $1 OFFSET $2
	`

	rows, err := r.pool.Query(context.Background(), query, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("list contexts: %w", err)
	}
	defer rows.Close()

	var tenants []*domainctx.Context
	for rows.Next() {
		t := &domainctx.Context{}
		var settingsBytes []byte
		err := rows.Scan(
			&t.ID, &t.Slug, &t.Name, &t.Domain, &t.IsActive,
			&settingsBytes, &t.CreatedAt, &t.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("scan context: %w", err)
		}
		if len(settingsBytes) > 0 {
			_ = json.Unmarshal(settingsBytes, &t.Settings)
		}
		tenants = append(tenants, t)
	}

	return tenants, rows.Err()
}

func (r *postgresRepo) Update(t *domainctx.Context) error {
	settingsBytes, err := json.Marshal(t.Settings)
	if err != nil {
		return fmt.Errorf("marshal settings: %w", err)
	}
	if settingsBytes == nil {
		settingsBytes = []byte("{}")
	}

	query := `
		UPDATE contexts SET slug = $1, name = $2, domain = $3, is_active = $4,
			settings = $5, updated_at = NOW()
		WHERE id = $6
	`

	result, err := r.pool.Exec(context.Background(), query,
		t.Slug, t.Name, t.Domain, t.IsActive, settingsBytes, t.ID,
	)
	if err != nil {
		return fmt.Errorf("update context: %w", err)
	}

	if result.RowsAffected() == 0 {
		return fmt.Errorf("context not found: %s", t.ID)
	}

	return nil
}

func (r *postgresRepo) Delete(id string) error {
	query := `DELETE FROM contexts WHERE id = $1`
	result, err := r.pool.Exec(context.Background(), query, id)
	if err != nil {
		return fmt.Errorf("delete context: %w", err)
	}

	if result.RowsAffected() == 0 {
		return fmt.Errorf("context not found: %s", id)
	}

	return nil
}

func (r *postgresRepo) GetByUserEmail(email string) ([]*domainctx.Context, error) {
	query := `
		SELECT DISTINCT c.id, c.slug, c.name, c.domain, c.is_active, c.settings, c.created_at, c.updated_at
		FROM contexts c
		JOIN user_contexts uc ON uc.context_id = c.id
		JOIN users u ON u.id = uc.user_id
		WHERE u.email = $1
		ORDER BY c.created_at
	`
	rows, err := r.pool.Query(context.Background(), query, email)
	if err != nil {
		return nil, fmt.Errorf("get contexts by user email: %w", err)
	}
	defer rows.Close()

	var contexts []*domainctx.Context
	for rows.Next() {
		c := &domainctx.Context{}
		var settingsBytes []byte
		err := rows.Scan(
			&c.ID, &c.Slug, &c.Name, &c.Domain, &c.IsActive,
			&settingsBytes, &c.CreatedAt, &c.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("scan context: %w", err)
		}
		if len(settingsBytes) > 0 {
			_ = json.Unmarshal(settingsBytes, &c.Settings)
		}
		contexts = append(contexts, c)
	}
	return contexts, nil
}

func (r *postgresRepo) GetDefaultContext() (*domainctx.Context, error) {
	t := &domainctx.Context{}
	var settingsBytes []byte

	query := `
		SELECT id, slug, name, domain, is_active, settings, created_at, updated_at
		FROM contexts WHERE is_active = true ORDER BY created_at ASC LIMIT 1
	`

	err := r.pool.QueryRow(context.Background(), query).Scan(
		&t.ID, &t.Slug, &t.Name, &t.Domain, &t.IsActive,
		&settingsBytes, &t.CreatedAt, &t.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("get default context: %w", err)
	}

	if len(settingsBytes) > 0 {
		_ = json.Unmarshal(settingsBytes, &t.Settings)
	}
	return t, nil
}
