package repo

import (
	"context"
	"database/sql"
	_ "embed"
	"fmt"

	"github.com/hekemen/automata/internal/domain/staticfile"
	"github.com/hekemen/automata/internal/infrastructure/database"
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
			return fmt.Errorf("execute static_files migration: %w", err)
		}
		return nil
	})
}

// repo implements the staticfile.Repository interface using PostgreSQL.
type repo struct {
	pool *pgxpool.Pool
}

// New creates a new PostgreSQL static file repository.
func New(pool *pgxpool.Pool) staticfile.Repository {
	return &repo{pool: pool}
}

func (r *repo) Create(f *staticfile.StaticFile) error {
	ctx := context.Background()
	query := `
		INSERT INTO static_files (context_id, name, token, mime, size)
		VALUES ($1, $2, $3, $4, $5)
	`
	_, err := r.pool.Exec(ctx, query, f.ContextID, f.Name, f.Token, f.Mime, f.Size)
	if err != nil {
		return fmt.Errorf("create static file: %w", err)
	}
	return nil
}

func (r *repo) GetByName(contextID, name string) (*staticfile.StaticFile, error) {
	ctx := context.Background()
	query := `
		SELECT context_id, name, token, mime, size, created_at
		FROM static_files
		WHERE context_id = $1 AND name = $2
	`
	f := &staticfile.StaticFile{}
	err := r.pool.QueryRow(ctx, query, contextID, name).Scan(
		&f.ContextID, &f.Name, &f.Token, &f.Mime, &f.Size, &f.CreatedAt,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("static file not found: %w", err)
		}
		return nil, fmt.Errorf("get static file: %w", err)
	}
	return f, nil
}

func (r *repo) List(contextID string) ([]*staticfile.StaticFile, error) {
	ctx := context.Background()
	query := `
		SELECT context_id, name, token, mime, size, created_at
		FROM static_files
		WHERE context_id = $1
		ORDER BY created_at DESC
	`
	rows, err := r.pool.Query(ctx, query, contextID)
	if err != nil {
		return nil, fmt.Errorf("list static files: %w", err)
	}
	defer rows.Close()

	var files []*staticfile.StaticFile
	for rows.Next() {
		f := &staticfile.StaticFile{}
		err := rows.Scan(&f.ContextID, &f.Name, &f.Token, &f.Mime, &f.Size, &f.CreatedAt)
		if err != nil {
			return nil, fmt.Errorf("scan static file: %w", err)
		}
		files = append(files, f)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate static files: %w", err)
	}
	return files, nil
}

func (r *repo) Delete(contextID, name string) error {
	ctx := context.Background()
	query := `
		DELETE FROM static_files
		WHERE context_id = $1 AND name = $2
	`
	tag, err := r.pool.Exec(ctx, query, contextID, name)
	if err != nil {
		return fmt.Errorf("delete static file: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("static file not found")
	}
	return nil
}