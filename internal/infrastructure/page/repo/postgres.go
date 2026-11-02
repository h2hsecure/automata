package repo

import (
	"context"
	"database/sql"
	_ "embed"
	"fmt"

	"github.com/h2hsecure/automata/internal/domain/page"
	"github.com/h2hsecure/automata/internal/infrastructure/database"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

//go:embed migration.sql
var migrationSQL string

// RunMigrations executes the embedded migration.sql against the provided
// database pool, serialized under the shared migration advisory lock.
func RunMigrations(db *pgxpool.Pool) error {
	return database.RunUnderMigrationLock(context.Background(), db, func(conn *pgx.Conn) error {
		if _, err := conn.Exec(context.Background(), migrationSQL); err != nil {
			return fmt.Errorf("execute page migration: %w", err)
		}
		return nil
	})
}

// repo implements the page.Repository interface using PostgreSQL.
type repo struct {
	pool *pgxpool.Pool
}

// New creates a new PostgreSQL page repository.
func New(pool *pgxpool.Pool) page.Repository {
	return &repo{pool: pool}
}

func (r *repo) Create(p *page.Page) error {
	ctx := context.Background()
	query := `
		INSERT INTO page_pages (id, context_id, slug, title, content, is_active, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, NOW(), NOW())
	`
	_, err := r.pool.Exec(ctx, query, p.ID, p.ContextID, p.Slug, p.Title, p.Content, p.IsActive)
	if err != nil {
		return fmt.Errorf("create page: %w", err)
	}
	return nil
}

func (r *repo) GetByID(id string) (*page.Page, error) {
	ctx := context.Background()
	query := `
		SELECT id, context_id, slug, title, content, is_active, created_at, updated_at
		FROM page_pages
		WHERE id = $1
	`
	return r.scanPage(r.pool.QueryRow(ctx, query, id))
}

func (r *repo) GetBySlug(contextID, slug string) (*page.Page, error) {
	ctx := context.Background()
	query := `
		SELECT id, context_id, slug, title, content, is_active, created_at, updated_at
		FROM page_pages
		WHERE context_id = $1 AND slug = $2
	`
	return r.scanPage(r.pool.QueryRow(ctx, query, contextID, slug))
}

func (r *repo) List(contextID string) ([]*page.Page, error) {
	ctx := context.Background()
	query := `
		SELECT id, context_id, slug, title, content, is_active, created_at, updated_at
		FROM page_pages
		WHERE context_id = $1
		ORDER BY created_at DESC
	`
	rows, err := r.pool.Query(ctx, query, contextID)
	if err != nil {
		return nil, fmt.Errorf("list pages: %w", err)
	}
	defer rows.Close()

	var pages []*page.Page
	for rows.Next() {
		p := &page.Page{}
		err := rows.Scan(
			&p.ID, &p.ContextID, &p.Slug, &p.Title, &p.Content,
			&p.IsActive, &p.CreatedAt, &p.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("scan page: %w", err)
		}
		pages = append(pages, p)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate pages: %w", err)
	}
	return pages, nil
}

func (r *repo) Update(p *page.Page) error {
	ctx := context.Background()
	query := `
		UPDATE page_pages
		SET slug = $2, title = $3, content = $4, is_active = $5, updated_at = NOW()
		WHERE id = $1
	`
	result, err := r.pool.Exec(ctx, query, p.ID, p.Slug, p.Title, p.Content, p.IsActive)
	if err != nil {
		return fmt.Errorf("update page: %w", err)
	}
	if result.RowsAffected() == 0 {
		return fmt.Errorf("page not found: %w", sql.ErrNoRows)
	}
	return nil
}

func (r *repo) Delete(id string) error {
	ctx := context.Background()
	query := `DELETE FROM page_pages WHERE id = $1`
	result, err := r.pool.Exec(ctx, query, id)
	if err != nil {
		return fmt.Errorf("delete page: %w", err)
	}
	if result.RowsAffected() == 0 {
		return fmt.Errorf("page not found: %w", sql.ErrNoRows)
	}
	return nil
}

func (r *repo) scanPage(row interface {
	Scan(dest ...interface{}) error
}) (*page.Page, error) {
	p := &page.Page{}
	err := row.Scan(
		&p.ID, &p.ContextID, &p.Slug, &p.Title, &p.Content,
		&p.IsActive, &p.CreatedAt, &p.UpdatedAt,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("page not found: %w", err)
		}
		return nil, fmt.Errorf("scan page: %w", err)
	}
	return p, nil
}
