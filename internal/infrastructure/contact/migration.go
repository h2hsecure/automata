package contact

import (
	"context"
	"embed"
	"fmt"

	"github.com/hekemen/automata/internal/infrastructure/database"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

//go:embed migration.sql
var migrationFS embed.FS

// RunMigrations executes the embedded migration SQL against the database,
// serialized under the shared migration advisory lock.
func RunMigrations(pool *pgxpool.Pool) error {
	migrationSQL, err := migrationFS.ReadFile("migration.sql")
	if err != nil {
		return fmt.Errorf("read migration.sql: %w", err)
	}

	return database.RunUnderMigrationLock(context.Background(), pool, func(conn *pgx.Conn) error {
		if _, err := conn.Exec(context.Background(), string(migrationSQL)); err != nil {
			return fmt.Errorf("execute migration: %w", err)
		}
		return nil
	})
}
