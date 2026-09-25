package database

import (
	"context"
	_ "embed"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
)

//go:embed migration_profile.sql
var profileMigrationSQL string

// RunProfileMigrations executes the embedded migration_profile.sql against the provided
// database pool. All statements are run in a single Exec call.
func RunProfileMigrations(db *pgxpool.Pool) error {
	if _, err := db.Exec(context.Background(), profileMigrationSQL); err != nil {
		return fmt.Errorf("execute profile migration: %w", err)
	}
	return nil
}
