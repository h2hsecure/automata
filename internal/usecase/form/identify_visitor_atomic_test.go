package form

import (
	"context"
	"strings"
	"testing"
	"time"

	contactinfra "github.com/h2hsecure/automata/internal/infrastructure/contact"
	contactrepo "github.com/h2hsecure/automata/internal/infrastructure/contact/repo"
	trackingrepo "github.com/h2hsecure/automata/internal/infrastructure/tracking/repo"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
)

func startAtomicTestPostgres(t *testing.T) (*pgxpool.Pool, func()) {
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
		t.Fatalf("create pool: %v", err)
	}
	if err := pool.Ping(ctx); err != nil {
		_ = container.Terminate(ctx)
		t.Fatalf("ping: %v", err)
	}
	if err := contactinfra.RunMigrations(pool); err != nil {
		_ = container.Terminate(ctx)
		t.Fatalf("contact migrations: %v", err)
	}
	if err := trackingrepo.RunMigrations(pool); err != nil {
		_ = container.Terminate(ctx)
		t.Fatalf("tracking migrations: %v", err)
	}
	return pool, func() {
		pool.Close()
		_ = container.Terminate(ctx)
	}
}

// TestIdentifyVisitor_MergeAtomic proves the contact merge and the visitor
// reassign run in one transaction: forcing the reassign step to fail must
// roll back the contact merge (hidden contact intact, nothing moved).
func TestIdentifyVisitor_MergeAtomic(t *testing.T) {
	pool, cleanup := startAtomicTestPostgres(t)
	defer cleanup()

	cr := contactrepo.New(pool)
	tr := trackingrepo.New(pool)
	ctx := context.Background()
	contextID := uuid.New().String()

	hiddenID := uuid.New().String()
	existingID := uuid.New().String()
	tagID := uuid.New().String()
	email := "ann@example.com"

	if _, err := pool.Exec(ctx, `
		INSERT INTO contacts (id, context_id, email, source, source_id, hidden, custom_fields)
		VALUES ($1, $2, NULL, 'visitor', 'vid-1', TRUE, '{"hidden_field":"h"}')`,
		hiddenID, contextID); err != nil {
		t.Fatalf("seed hidden contact: %v", err)
	}
	if _, err := pool.Exec(ctx, `
		INSERT INTO contacts (id, context_id, email, custom_fields)
		VALUES ($1, $2, $3, '{"keep_field":"k"}')`,
		existingID, contextID, email); err != nil {
		t.Fatalf("seed existing contact: %v", err)
	}
	if _, err := pool.Exec(ctx, `
		INSERT INTO contact_tags (id, context_id, name) VALUES ($1, $2, 'vip')`,
		tagID, contextID); err != nil {
		t.Fatalf("seed tag: %v", err)
	}
	if _, err := pool.Exec(ctx, `
		INSERT INTO contact_tag_memberships (contact_id, tag_id) VALUES ($1, $2)`,
		hiddenID, tagID); err != nil {
		t.Fatalf("seed tag membership: %v", err)
	}
	if _, err := pool.Exec(ctx, `
		INSERT INTO tracking_visitors (id, context_id, cookie_value, contact_id)
		VALUES ('vid-1', $1, 'vid-1', $2)`, contextID, hiddenID); err != nil {
		t.Fatalf("seed visitor: %v", err)
	}

	// Force the reassign step to fail: its UPDATE hits a missing relation
	// inside the transaction.
	if _, err := pool.Exec(ctx, `DROP TABLE tracking_visitors`); err != nil {
		t.Fatalf("drop tracking_visitors: %v", err)
	}

	_, err := IdentifyVisitor(cr, tr, contextID, "signup", "vid-1", map[string]interface{}{
		"email": email,
	})
	if err == nil {
		t.Fatalf("IdentifyVisitor succeeded, want the reassign step to fail")
	}

	var hiddenExists bool
	if err := pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM contacts WHERE id = $1)`, hiddenID).Scan(&hiddenExists); err != nil {
		t.Fatalf("check hidden contact: %v", err)
	}
	if !hiddenExists {
		t.Errorf("hidden contact was deleted despite the reassign failure (no rollback)")
	}

	var fields string
	if err := pool.QueryRow(ctx, `SELECT custom_fields::text FROM contacts WHERE id = $1`, existingID).Scan(&fields); err != nil {
		t.Fatalf("scan existing fields: %v", err)
	}
	if strings.Contains(fields, "hidden_field") {
		t.Errorf("existing contact fields = %s, want no merged hidden fields", fields)
	}

	var tagMoved bool
	if err := pool.QueryRow(ctx, `
		SELECT EXISTS(SELECT 1 FROM contact_tag_memberships m
			JOIN contact_tags tg ON tg.id = m.tag_id
			WHERE m.contact_id = $1 AND tg.name = 'vip')`, existingID).Scan(&tagMoved); err != nil {
		t.Fatalf("check tag: %v", err)
	}
	if tagMoved {
		t.Errorf("tag moved to the existing contact despite the reassign failure (no rollback)")
	}
}