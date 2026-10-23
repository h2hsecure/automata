package repo

import (
	"context"
	"database/sql"
	_ "embed"
	"errors"
	"fmt"

	"github.com/hekemen/automata/internal/domain/settings"
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
			return fmt.Errorf("execute settings migration: %w", err)
		}
		return nil
	})
}

// repo implements the settings.Repository interface using PostgreSQL.
type repo struct {
	pool *pgxpool.Pool
}

// New creates a new PostgreSQL settings repository.
func New(pool *pgxpool.Pool) settings.Repository {
	return &repo{pool: pool}
}

func (r *repo) List() (map[string]string, error) {
	ctx := context.Background()
	rows, err := r.pool.Query(ctx, "SELECT key, value FROM global_settings")
	if err != nil {
		return nil, fmt.Errorf("list settings: %w", err)
	}
	defer rows.Close()

	m := make(map[string]string)
	for rows.Next() {
		var k, v string
		if err := rows.Scan(&k, &v); err != nil {
			return nil, fmt.Errorf("scan setting: %w", err)
		}
		m[k] = v
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate settings: %w", err)
	}
	return m, nil
}

func (r *repo) Get(key string) (string, error) {
	ctx := context.Background()
	var value string
	err := r.pool.QueryRow(ctx, "SELECT value FROM global_settings WHERE key = $1", key).Scan(&value)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return "", nil
		}
		return "", fmt.Errorf("get setting: %w", err)
	}
	return value, nil
}

func (r *repo) Upsert(key, value string) error {
	ctx := context.Background()
	query := `
		INSERT INTO global_settings (key, value)
		VALUES ($1, $2)
		ON CONFLICT (key) DO UPDATE SET value = EXCLUDED.value, updated_at = NOW()
	`
	if _, err := r.pool.Exec(ctx, query, key, value); err != nil {
		return fmt.Errorf("upsert setting: %w", err)
	}
	return nil
}

func (r *repo) Delete(key string) error {
	ctx := context.Background()
	result, err := r.pool.Exec(ctx, "DELETE FROM global_settings WHERE key = $1", key)
	if err != nil {
		return fmt.Errorf("delete setting: %w", err)
	}
	if result.RowsAffected() == 0 {
		return settings.ErrNotFound
	}
	return nil
}
