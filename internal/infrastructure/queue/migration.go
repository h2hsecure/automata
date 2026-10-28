package queue

import (
	"context"
	_ "embed"
	"fmt"

	"github.com/h2hsecure/automata/internal/infrastructure/database"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

//go:embed migration.sql
var queueMigrationSQL string

//go:embed webhook_migration.sql
var webhookMigrationSQL string

// RunMigrations executes the queue package migrations (email_jobs and
// webhook_deliveries tables) against the provided database pool, serialized
// under the shared migration advisory lock.
func RunMigrations(pool *pgxpool.Pool) error {
	return database.RunUnderMigrationLock(context.Background(), pool, func(conn *pgx.Conn) error {
		if _, err := conn.Exec(context.Background(), queueMigrationSQL); err != nil {
			return fmt.Errorf("execute queue migration: %w", err)
		}
		if _, err := conn.Exec(context.Background(), webhookMigrationSQL); err != nil {
			return fmt.Errorf("execute webhook queue migration: %w", err)
		}
		return nil
	})
}
