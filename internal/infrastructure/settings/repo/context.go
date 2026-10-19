package repo

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/hekemen/automata/internal/domain/settings"
	"github.com/jackc/pgx/v5/pgxpool"
)

// contextRepo implements the settings.ContextRepository interface using PostgreSQL.
type contextRepo struct {
	pool *pgxpool.Pool
}

// NewContext creates a new PostgreSQL per-context settings repository.
func NewContext(pool *pgxpool.Pool) settings.ContextRepository {
	return &contextRepo{pool: pool}
}

func (r *contextRepo) List(contextID string) (map[string]string, error) {
	ctx := context.Background()
	rows, err := r.pool.Query(ctx, "SELECT key, value FROM context_settings WHERE context_id = $1", contextID)
	if err != nil {
		return nil, fmt.Errorf("list context settings: %w", err)
	}
	defer rows.Close()

	m := make(map[string]string)
	for rows.Next() {
		var k, v string
		if err := rows.Scan(&k, &v); err != nil {
			return nil, fmt.Errorf("scan context setting: %w", err)
		}
		m[k] = v
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate context settings: %w", err)
	}
	return m, nil
}

func (r *contextRepo) Get(contextID, key string) (string, error) {
	ctx := context.Background()
	var value string
	err := r.pool.QueryRow(ctx, "SELECT value FROM context_settings WHERE context_id = $1 AND key = $2", contextID, key).Scan(&value)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return "", settings.ErrNotFound
		}
		return "", fmt.Errorf("get context setting: %w", err)
	}
	return value, nil
}

func (r *contextRepo) Upsert(contextID, key, value string) error {
	ctx := context.Background()
	query := `
		INSERT INTO context_settings (context_id, key, value)
		VALUES ($1, $2, $3)
		ON CONFLICT (context_id, key) DO UPDATE SET value = EXCLUDED.value, updated_at = NOW()
	`
	if _, err := r.pool.Exec(ctx, query, contextID, key, value); err != nil {
		return fmt.Errorf("upsert context setting: %w", err)
	}
	return nil
}

func (r *contextRepo) Delete(contextID, key string) error {
	ctx := context.Background()
	result, err := r.pool.Exec(ctx, "DELETE FROM context_settings WHERE context_id = $1 AND key = $2", contextID, key)
	if err != nil {
		return fmt.Errorf("delete context setting: %w", err)
	}
	if result.RowsAffected() == 0 {
		return settings.ErrNotFound
	}
	return nil
}
