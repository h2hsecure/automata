package database

import (
	"context"
	_ "embed"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

//go:embed migration.sql
var migrationSQL string

// RunMigrations executes the embedded migration.sql against the provided
// database pool. All statements are run in a single Exec call, serialized
// under the shared migration advisory lock so concurrent runs from multiple
// pods cannot race on the DDL.
func RunMigrations(db *pgxpool.Pool) error {
	return RunUnderMigrationLock(context.Background(), db, func(conn *pgx.Conn) error {
		if _, err := conn.Exec(context.Background(), migrationSQL); err != nil {
			return fmt.Errorf("execute migration: %w", err)
		}
		return nil
	})
}
