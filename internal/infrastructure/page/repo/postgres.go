package repo

import (
	"context"
	"database/sql"
	_ "embed"
	"encoding/json"
	"fmt"

	"github.com/google/uuid"
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
		INSERT INTO page_pages (id, context_id, slug, title, content, is_active, position, buttons, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, NOW(), NOW())
	`
	_, err := r.pool.Exec(ctx, query, p.ID, p.ContextID, p.Slug, p.Title, p.Content, p.IsActive, p.Position, encodeButtons(p.Buttons))
	if err != nil {
		return fmt.Errorf("create page: %w", err)
	}
	return nil
}

func (r *repo) GetByID(id string) (*page.Page, error) {
	ctx := context.Background()
	query := `
		SELECT id, context_id, slug, title, content, is_active, position, buttons, created_at, updated_at
		FROM page_pages
		WHERE id = $1
	`
	return r.scanPage(r.pool.QueryRow(ctx, query, id))
}

func (r *repo) GetBySlug(contextID, slug string) (*page.Page, error) {
	ctx := context.Background()
	query := `
		SELECT id, context_id, slug, title, content, is_active, position, buttons, created_at, updated_at
		FROM page_pages
		WHERE context_id = $1 AND slug = $2
	`
	return r.scanPage(r.pool.QueryRow(ctx, query, contextID, slug))
}

func (r *repo) List(contextID string) ([]*page.Page, error) {
	ctx := context.Background()
	query := `
		SELECT id, context_id, slug, title, content, is_active, position, buttons, created_at, updated_at
		FROM page_pages
		WHERE context_id = $1
		ORDER BY created_at DESC
	`
	return r.queryPages(ctx, query, "list pages", contextID)
}

func (r *repo) ListActive(contextID string) ([]*page.Page, error) {
	ctx := context.Background()
	query := `
		SELECT id, context_id, slug, title, content, is_active, position, buttons, created_at, updated_at
		FROM page_pages
		WHERE context_id = $1 AND is_active = TRUE
		ORDER BY created_at ASC
	`
	return r.queryPages(ctx, query, "list active pages", contextID)
}

func (r *repo) Update(p *page.Page) error {
	ctx := context.Background()
	query := `
		UPDATE page_pages
		SET slug = $2, title = $3, content = $4, is_active = $5, position = $6, buttons = $7, updated_at = NOW()
		WHERE id = $1
	`
	result, err := r.pool.Exec(ctx, query, p.ID, p.Slug, p.Title, p.Content, p.IsActive, p.Position, encodeButtons(p.Buttons))
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

// RecordImpression records that a positioned page was shown to a visitor.
func (r *repo) RecordImpression(contextID, pageID, visitorID string) error {
	ctx := context.Background()
	query := `
		INSERT INTO page_impressions (id, context_id, page_id, visitor_id, created_at)
		VALUES ($1, $2, $3, $4, NOW())
	`
	_, err := r.pool.Exec(ctx, query, uuid.New().String(), contextID, pageID, visitorID)
	if err != nil {
		return fmt.Errorf("record impression: %w", err)
	}
	return nil
}

// RecordDecision records a visitor's choice (accept/decline) for a page.
func (r *repo) RecordDecision(d *page.Decision) error {
	ctx := context.Background()
	if d.ID == "" {
		d.ID = uuid.New().String()
	}
	query := `
		INSERT INTO page_decisions (id, context_id, page_id, visitor_id, action, ip_hash, user_agent, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, NOW())
	`
	_, err := r.pool.Exec(ctx, query, d.ID, d.ContextID, d.PageID, d.VisitorID, d.Action, d.IPHash, d.UserAgent)
	if err != nil {
		return fmt.Errorf("record decision: %w", err)
	}
	return nil
}

// ListDecisions returns the decision rows for a page (newest first) plus
// the total count of decisions for that page.
func (r *repo) ListDecisions(pageID string, offset, limit int) ([]*page.Decision, int64, error) {
	ctx := context.Background()
	if limit <= 0 || limit > 1000 {
		limit = 50
	}
	if offset < 0 {
		offset = 0
	}

	countQuery := `SELECT COUNT(*) FROM page_decisions WHERE page_id = $1`
	var total int64
	if err := r.pool.QueryRow(ctx, countQuery, pageID).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count decisions: %w", err)
	}

	query := `
		SELECT id, context_id, page_id, visitor_id, action, ip_hash, user_agent, created_at
		FROM page_decisions
		WHERE page_id = $1
		ORDER BY created_at DESC
		LIMIT $2 OFFSET $3
	`
	rows, err := r.pool.Query(ctx, query, pageID, limit, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("list decisions: %w", err)
	}
	defer rows.Close()

	var decisions []*page.Decision
	for rows.Next() {
		d := &page.Decision{}
		if err := rows.Scan(&d.ID, &d.ContextID, &d.PageID, &d.VisitorID, &d.Action, &d.IPHash, &d.UserAgent, &d.CreatedAt); err != nil {
			return nil, 0, fmt.Errorf("scan decision: %w", err)
		}
		decisions = append(decisions, d)
	}
	return decisions, total, nil
}

// DecisionCounts returns the number of decisions per action for a page.
func (r *repo) DecisionCounts(pageID string) (map[string]int64, error) {
	ctx := context.Background()
	query := `SELECT action, COUNT(*) FROM page_decisions WHERE page_id = $1 GROUP BY action`
	rows, err := r.pool.Query(ctx, query, pageID)
	if err != nil {
		return nil, fmt.Errorf("decision counts: %w", err)
	}
	defer rows.Close()

	counts := make(map[string]int64)
	for rows.Next() {
		var action string
		var count int64
		if err := rows.Scan(&action, &count); err != nil {
			return nil, fmt.Errorf("scan decision count: %w", err)
		}
		counts[action] = count
	}
	return counts, nil
}

// encodeButtons marshals the buttons for storage; an empty/nil slice is
// stored as an empty JSON array.
func encodeButtons(buttons []page.Button) []byte {
	if buttons == nil {
		return []byte("[]")
	}
	b, err := json.Marshal(buttons)
	if err != nil {
		return []byte("[]")
	}
	return b
}

func (r *repo) queryPages(ctx context.Context, query, op string, args ...interface{}) ([]*page.Page, error) {
	rows, err := r.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", op, err)
	}
	defer rows.Close()

	var pages []*page.Page
	for rows.Next() {
		p := &page.Page{}
		var position string
		var buttons []byte
		err := rows.Scan(
			&p.ID, &p.ContextID, &p.Slug, &p.Title, &p.Content,
			&p.IsActive, &position, &buttons,
			&p.CreatedAt, &p.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("scan page: %w", err)
		}
		p.Position = page.Position(position)
		p.Buttons = decodeButtons(buttons)
		pages = append(pages, p)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("%s: %w", op, err)
	}
	return pages, nil
}

// decodeButtons unmarshels stored buttons; NULL or empty values yield an
// empty (non-nil) slice.
func decodeButtons(data []byte) []page.Button {
	if len(data) == 0 {
		return []page.Button{}
	}
	var buttons []page.Button
	if err := json.Unmarshal(data, &buttons); err != nil {
		return []page.Button{}
	}
	return buttons
}

func (r *repo) scanPage(row interface {
	Scan(dest ...interface{}) error
}) (*page.Page, error) {
	p := &page.Page{}
	var position string
	var buttons []byte
	err := row.Scan(
		&p.ID, &p.ContextID, &p.Slug, &p.Title, &p.Content,
		&p.IsActive, &position, &buttons,
		&p.CreatedAt, &p.UpdatedAt,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("page not found: %w", err)
		}
		return nil, fmt.Errorf("scan page: %w", err)
	}
	p.Position = page.Position(position)
	p.Buttons = decodeButtons(buttons)
	return p, nil
}
