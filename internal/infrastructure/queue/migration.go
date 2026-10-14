package queue

import (
	"context"
	_ "embed"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
)

//go:embed migration.sql
var queueMigrationSQL string

//go:embed webhook_migration.sql
var webhookMigrationSQL string

// RunMigrations executes the queue package migrations (email_jobs and
// webhook_deliveries tables) against the provided database pool.
func RunMigrations(pool *pgxpool.Pool) error {
	if _, err := pool.Exec(context.Background(), queueMigrationSQL); err != nil {
		return fmt.Errorf("execute queue migration: %w", err)
	}
	if _, err := pool.Exec(context.Background(), webhookMigrationSQL); err != nil {
		return fmt.Errorf("execute webhook queue migration: %w", err)
	}
	return nil
}