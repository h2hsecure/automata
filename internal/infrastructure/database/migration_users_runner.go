package database

import (
	"context"
	_ "embed"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
)

//go:embed migration_users.sql
var migrationUsersSQL string

//go:embed migration_users_columns.sql
var migrationUsersColumnsSQL string

//go:embed migration_user_contexts.sql
var migrationUserContextsSQL string

//go:embed migration_api_keys_users_fk.sql
var migrationAPIKeysUsersFKSQL string

// RunUserContextMigrations executes the embedded migration_users.sql,
// migration_users_columns.sql, and migration_user_contexts.sql, then
// repoints the api_keys.user_id FK onto the users table.
func RunUserContextMigrations(db *pgxpool.Pool) error {
	if _, err := db.Exec(context.Background(), migrationUsersSQL); err != nil {
		return fmt.Errorf("execute user-context migration: %w", err)
	}
	if _, err := db.Exec(context.Background(), migrationUsersColumnsSQL); err != nil {
		return fmt.Errorf("execute user-columns migration: %w", err)
	}
	if _, err := db.Exec(context.Background(), migrationUserContextsSQL); err != nil {
		return fmt.Errorf("execute user-contexts migration: %w", err)
	}
	if _, err := db.Exec(context.Background(), migrationAPIKeysUsersFKSQL); err != nil {
		return fmt.Errorf("execute api-keys users FK migration: %w", err)
	}
	return nil
}
