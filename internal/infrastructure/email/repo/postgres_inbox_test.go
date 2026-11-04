package repo_test

import (
	"context"
	"database/sql"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/h2hsecure/automata/internal/infrastructure/database"
	"github.com/h2hsecure/automata/internal/infrastructure/email/repo"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
)

func startInboxTestPostgres(t *testing.T) (*pgxpool.Pool, func()) {
	t.Helper()
	ctx := context.Background()
	container, err := postgres.Run(
		ctx,
		"postgres:16-alpine",
		postgres.WithDatabase("testdb"),
		postgres.WithUsername("testuser"),
		postgres.WithPassword("testpass"),
		testcontainers.WithWaitStrategy(
			wait.ForLog("database system is ready to accept connections").
				WithOccurrence(2).
				WithStartupTimeout(30*time.Second),
		),
	)
	if err != nil {
		t.Fatalf("start postgres container: %v", err)
	}
	url, err := container.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		_ = container.Terminate(ctx)
		t.Fatalf("connection string: %v", err)
	}
	pool, err := pgxpool.New(ctx, url)
	if err != nil {
		_ = container.Terminate(ctx)
		t.Fatalf("pool: %v", err)
	}
	cleanup := func() {
		pool.Close()
		_ = container.Terminate(context.Background())
	}
	return pool, cleanup
}

// seedMessage inserts an unread inbox message and returns its id.
func seedMessage(t *testing.T, pool *pgxpool.Pool, contextID string, uid int64) string {
	t.Helper()
	id := uuid.New().String()
	_, err := pool.Exec(context.Background(),
		`INSERT INTO email_inbox (id, context_id, uid, sender, recipient, subject, body_text, message_date)
		 VALUES ($1, $2, $3, 'a@b.c', 'd@e.f', 'hi', 'body', now())`,
		id, contextID, uid,
	)
	if err != nil {
		t.Fatalf("seed message: %v", err)
	}
	return id
}

// TestMarkInboxRead_CrossTenantIgnored is the regression test for the
// cross-tenant inbox mutation finding: marking a message read with a
// different context ID must affect zero rows.
func TestMarkInboxRead_CrossTenantIgnored(t *testing.T) {
	pool, cleanup := startInboxTestPostgres(t)
	defer cleanup()

	// email_inbox has an FK to contexts, so run the core platform
	// migrations first.
	if err := database.RunMigrations(pool); err != nil {
		t.Fatalf("core migrations: %v", err)
	}
	if err := repo.RunMigrations(pool); err != nil {
		t.Fatalf("migrations: %v", err)
	}

	// Two contexts (email_inbox.context_id is a UUID FK).
	ctxA := uuid.New()
	ctxB := uuid.New()
	if _, err := pool.Exec(context.Background(),
		`INSERT INTO contexts (id, slug, name) VALUES ($1, 'a', 'A'), ($2, 'b', 'B')`,
		ctxA, ctxB,
	); err != nil {
		t.Fatalf("seed contexts: %v", err)
	}

	msgID := seedMessage(t, pool, ctxA.String(), 1)
	inbox := repo.NewInboxRepo(pool)

	// Same context: works.
	if err := inbox.MarkRead(msgID, ctxA.String()); err != nil {
		t.Fatalf("same-context MarkRead failed: %v", err)
	}

	// Cross-context: must be a no-op (ErrNoRows), not a mutation.
	msgID2 := seedMessage(t, pool, ctxA.String(), 2)
	err := inbox.MarkRead(msgID2, ctxB.String())
	if !errorsIsNoRows(err) {
		t.Fatalf("cross-context MarkRead should return ErrNoRows, got %v", err)
	}

	var isRead bool
	if err := pool.QueryRow(context.Background(),
		`SELECT is_read FROM email_inbox WHERE id = $1`, msgID2,
	).Scan(&isRead); err != nil {
		t.Fatalf("read back: %v", err)
	}
	if isRead {
		t.Fatalf("message was mutated across tenant boundary")
	}
}

func errorsIsNoRows(err error) bool {
	return errors.Is(err, sql.ErrNoRows)
}

