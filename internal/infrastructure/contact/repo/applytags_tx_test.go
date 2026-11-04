package repo

import (
	"context"
	"testing"
	"time"

	contactinfra "github.com/h2hsecure/automata/internal/infrastructure/contact"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
)

func startApplyTagsTestPostgres(t *testing.T) (*pgxpool.Pool, func()) {
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
	return pool, func() {
		pool.Close()
		_ = container.Terminate(ctx)
	}
}

// TestApplyTags_MidTxFailureRollsBack proves the tag replace is atomic: a
// failure mid-replace must roll back the membership DELETE so the contact
// keeps its prior tags instead of ending up with a partial tag set.
func TestApplyTags_MidTxFailureRollsBack(t *testing.T) {
	pool, cleanup := startApplyTagsTestPostgres(t)
	defer cleanup()

	ctx := context.Background()
	contextID := uuid.New().String()
	contactID := uuid.New().String()
	tagID := uuid.New().String()

	if _, err := pool.Exec(ctx, `INSERT INTO contacts (id, context_id) VALUES ($1, $2)`, contactID, contextID); err != nil {
		t.Fatalf("seed contact: %v", err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO contact_tags (id, context_id, name) VALUES ($1, $2, 'keep-me')`, tagID, contextID); err != nil {
		t.Fatalf("seed tag: %v", err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO contact_tag_memberships (contact_id, tag_id) VALUES ($1, $2)`, contactID, tagID); err != nil {
		t.Fatalf("seed membership: %v", err)
	}

	// Force the replace to fail mid-transaction: any INSERT of a tag named
	// 'boom' raises an exception inside the ApplyTags transaction.
	if _, err := pool.Exec(ctx, `
		CREATE OR REPLACE FUNCTION boom_on_insert() RETURNS trigger AS $$
		BEGIN
			IF NEW.name = 'boom' THEN RAISE EXCEPTION 'boom tag'; END IF;
			RETURN NEW;
		END;
		$$ LANGUAGE plpgsql`); err != nil {
		t.Fatalf("create trigger function: %v", err)
	}
	if _, err := pool.Exec(ctx, `CREATE TRIGGER trg_boom BEFORE INSERT ON contact_tags
		FOR EACH ROW EXECUTE FUNCTION boom_on_insert()`); err != nil {
		t.Fatalf("create trigger: %v", err)
	}

	tagRepo := (&repo{pool: pool}).TagRepository()
	if err := tagRepo.ApplyTags(contextID, contactID, []string{"keep-me", "boom"}, nil); err == nil {
		t.Fatalf("ApplyTags succeeded, want the mid-transaction failure")
	}

	// The prior membership must have survived the rollback.
	var keepCount int
	if err := pool.QueryRow(ctx, `
		SELECT COUNT(*) FROM contact_tag_memberships tm
		JOIN contact_tags t ON t.id = tm.tag_id
		WHERE tm.contact_id = $1 AND t.name = 'keep-me'`, contactID).Scan(&keepCount); err != nil {
		t.Fatalf("check keep-me membership: %v", err)
	}
	if keepCount != 1 {
		t.Errorf("keep-me membership count = %d, want 1 (mid-tx failure must roll back)", keepCount)
	}

	// No partial state: no membership may reference a 'boom' tag.
	var boomCount int
	if err := pool.QueryRow(ctx, `
		SELECT COUNT(*) FROM contact_tag_memberships tm
		JOIN contact_tags t ON t.id = tm.tag_id
		WHERE tm.contact_id = $1 AND t.name = 'boom'`, contactID).Scan(&boomCount); err != nil {
		t.Fatalf("check boom membership: %v", err)
	}
	if boomCount != 0 {
		t.Errorf("boom membership count = %d, want 0", boomCount)
	}
}