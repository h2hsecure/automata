package database

import (
	"context"
	_ "embed"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
)

//go:embed migration_users.sql
var migrationUsersSQL string

// RunUserContextMigrations executes the embedded migration_users.sql.
func RunUserContextMigrations(db *pgxpool.Pool) error {
	if _, err := db.Exec(context.Background(), migrationUsersSQL); err != nil {
		return fmt.Errorf("execute user-context migration: %w", err)
	}
	return nil
}
