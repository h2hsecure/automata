package repo

import (
	"context"
	_ "embed"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
)

//go:embed migration.sql
var migrationSQL string

// RunMigrations executes the embedded migration.sql against the provided database pool.
func RunMigrations(pool *pgxpool.Pool) error {
	if _, err := pool.Exec(context.Background(), migrationSQL); err != nil {
		return fmt.Errorf("execute email migration: %w", err)
	}
	return nil
}
