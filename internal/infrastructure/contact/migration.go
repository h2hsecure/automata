package contact

import (
	"context"
	"embed"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
)

//go:embed migration.sql
var migrationFS embed.FS

// RunMigrations executes the embedded migration SQL against the database.
func RunMigrations(pool *pgxpool.Pool) error {
	migrationSQL, err := migrationFS.ReadFile("migration.sql")
	if err != nil {
		return fmt.Errorf("read migration.sql: %w", err)
	}

	_, err = pool.Exec(context.Background(), string(migrationSQL))
	if err != nil {
		return fmt.Errorf("execute migration: %w", err)
	}

	return nil
}
