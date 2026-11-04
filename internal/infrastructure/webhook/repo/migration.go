package repo

import (
	"context"
	_ "embed"
	"fmt"

	"github.com/h2hsecure/automata/internal/infrastructure/database"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

//go:embed migration.sql
var migrationSQL string

//go:embed migration_webhook_secret.sql
var migrationWebhookSecretSQL string

// RunMigrations executes the embedded webhook migrations against the provided
// database pool, serialized under the shared migration advisory lock.
//
// The sequence is:
//  1. the base schema (webhook_endpoints, old plaintext signing_secret column);
//  2. the signing_secret_enc column;
//  3. a Go backfill that encrypts every plaintext secret into
//     signing_secret_enc and clears signing_secret;
//  4. dropping the plaintext signing_secret column.
//
// All steps are idempotent, so re-running on an already-migrated database is
// a no-op.
func RunMigrations(pool *pgxpool.Pool) error {
	return database.RunUnderMigrationLock(context.Background(), pool, func(conn *pgx.Conn) error {
		if _, err := conn.Exec(context.Background(), migrationSQL); err != nil {
			return fmt.Errorf("execute webhook migration: %w", err)
		}
		if _, err := conn.Exec(context.Background(), migrationWebhookSecretSQL); err != nil {
			return fmt.Errorf("execute webhook secret migration: %w", err)
		}

		// The backfill and column drop are idempotent: once the plaintext
		// column is gone, this is a no-op on subsequent runs.
		var hasPlaintextColumn bool
		rows, err := conn.Query(context.Background(), `
			SELECT EXISTS (
				SELECT 1 FROM information_schema.columns
				WHERE table_name = 'webhook_endpoints' AND column_name = 'signing_secret'
			)`)
		if err != nil {
			return fmt.Errorf("check signing_secret column: %w", err)
		}
		if !rows.Next() {
			rows.Close()
			if err := rows.Err(); err != nil {
				return fmt.Errorf("check signing_secret column: %w", err)
			}
			return fmt.Errorf("check signing_secret column: no row returned")
		}
		if err := rows.Scan(&hasPlaintextColumn); err != nil {
			rows.Close()
			return fmt.Errorf("check signing_secret column: %w", err)
		}
		rows.Close()
		if hasPlaintextColumn {
			if err := backfillSecrets(conn); err != nil {
				return fmt.Errorf("backfill webhook secrets: %w", err)
			}
			if _, err := conn.Exec(context.Background(),
				"ALTER TABLE webhook_endpoints DROP COLUMN IF EXISTS signing_secret;"); err != nil {
				return fmt.Errorf("drop plaintext signing_secret column: %w", err)
			}
		}
		return nil
	})
}

// backfillSecrets encrypts any plaintext signing_secret values into
// signing_secret_enc and clears the plaintext column. It is a no-op once the
// plaintext column is gone.
func backfillSecrets(conn *pgx.Conn) error {
	ctx := context.Background()

	rows, err := conn.Query(ctx,
		"SELECT id, signing_secret FROM webhook_endpoints WHERE signing_secret IS NOT NULL")
	if err != nil {
		return fmt.Errorf("query plaintext secrets: %w", err)
	}
	type pending struct {
		id     string
		secret string
	}
	var pendingSecrets []pending
	for rows.Next() {
		var p pending
		if err := rows.Scan(&p.id, &p.secret); err != nil {
			rows.Close()
			return fmt.Errorf("scan plaintext secret: %w", err)
		}
		pendingSecrets = append(pendingSecrets, p)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return fmt.Errorf("iterate plaintext secrets: %w", err)
	}
	rows.Close()

	for _, p := range pendingSecrets {
		enc, err := encryptSecret(p.secret)
		if err != nil {
			return fmt.Errorf("encrypt secret for %s: %w", p.id, err)
		}
		if _, err := conn.Exec(ctx,
			"UPDATE webhook_endpoints SET signing_secret_enc = $2, signing_secret = NULL WHERE id = $1",
			p.id, enc); err != nil {
			return fmt.Errorf("store encrypted secret for %s: %w", p.id, err)
		}
	}
	return nil
}