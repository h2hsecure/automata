package repo

import (
	"context"
	_ "embed"
	"fmt"

	"github.com/hekemen/automata/internal/infrastructure/database"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

//go:embed migration.sql
var migrationSQL string

// RunMigrations executes the embedded migration.sql against the provided
// database pool, serialized under the shared migration advisory lock.
func RunMigrations(pool *pgxpool.Pool) error {
	return database.RunUnderMigrationLock(context.Background(), pool, func(conn *pgx.Conn) error {
		if _, err := conn.Exec(context.Background(), migrationSQL); err != nil {
			return fmt.Errorf("execute webhook migration: %w", err)
		}
		return nil
	})
}
