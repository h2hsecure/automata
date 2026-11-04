package repo

import (
	"context"
	"database/sql"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/h2hsecure/automata/internal/domain/webhook"
	"github.com/h2hsecure/automata/internal/infrastructure/auth"
	"github.com/h2hsecure/automata/internal/infrastructure/database"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
)

func startWebhookTestPostgres(t *testing.T) (*pgxpool.Pool, func()) {
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

// seedContext inserts a context row (webhook_endpoints.context_id is an FK).
func seedContext(t *testing.T, pool *pgxpool.Pool) string {
	t.Helper()
	id := uuid.New().String()
	if _, err := pool.Exec(context.Background(),
		`INSERT INTO contexts (id, name, slug) VALUES ($1, 'ctx', $2)`,
		id, "ctx-"+id); err != nil {
		t.Fatalf("seed context: %v", err)
	}
	return id
}

// TestSecretMigration_PrePostState is the regression test for the plaintext
// webhook signing secret finding: rows created with a plaintext
// signing_secret must be encrypted into signing_secret_enc by the migration,
// and the plaintext column must be dropped.
func TestSecretMigration_PrePostState(t *testing.T) {
	auth.InitForTesting("test-jwt-secret-0123456789", "test-cookie-secret-0123456")

	pool, cleanup := startWebhookTestPostgres(t)
	defer cleanup()
	ctx := context.Background()

	if err := database.RunMigrations(pool); err != nil {
		t.Fatalf("core migrations: %v", err)
	}
	contextID := seedContext(t, pool)

	// Create the old schema (plaintext column) directly, before the full
	// migration sequence, then insert a pre-existing row with a plaintext
	// secret — simulating the pre-fix production state.
	if _, err := pool.Exec(ctx, migrationSQL); err != nil {
		t.Fatalf("base migration: %v", err)
	}
	secret := "pre-existing-plaintext-secret-123"
	endpointID := uuid.New().String()
	if _, err := pool.Exec(ctx,
		`INSERT INTO webhook_endpoints (id, context_id, name, url, events, signing_secret, active)
		 VALUES ($1, $2, 'hook', 'https://example.com/x', 'test.ping', $3, true)`,
		endpointID, contextID, secret); err != nil {
		t.Fatalf("insert plaintext secret row: %v", err)
	}

	// Run the full migration sequence: backfill must encrypt + clear, then
	// drop the plaintext column.
	if err := RunMigrations(pool); err != nil {
		t.Fatalf("webhook migrations: %v", err)
	}

	// 1. The plaintext column is gone.
	if _, err := pool.Exec(ctx, "SELECT signing_secret FROM webhook_endpoints WHERE id = $1", endpointID); err == nil {
		t.Fatal("signing_secret column still exists after migration")
	}

	// 2. The encrypted column decrypts back to the original secret.
	var enc string
	if err := pool.QueryRow(ctx,
		"SELECT signing_secret_enc::text FROM webhook_endpoints WHERE id = $1", endpointID,
	).Scan(&enc); err != nil {
		t.Fatalf("read signing_secret_enc: %v", err)
	}
	if enc == "" {
		t.Fatal("signing_secret_enc is empty after migration")
	}
	decrypted, err := decryptSecret(enc)
	if err != nil {
		t.Fatalf("decrypt backfilled secret: %v", err)
	}
	if decrypted != secret {
		t.Fatalf("backfilled secret = %q, want %q", decrypted, secret)
	}

	// 3. The repo returns the plaintext secret via the domain field.
	r := New(pool)
	e, err := r.GetByID(endpointID)
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if e.SigningSecret == nil || *e.SigningSecret != secret {
		t.Fatalf("repo SigningSecret = %v, want %q", e.SigningSecret, secret)
	}
}

// TestSecret_RoundTrip verifies that secrets created through the repo are
// encrypted at rest (the stored value is not the plaintext) and read back
// correctly.
func TestSecret_RoundTrip(t *testing.T) {
	auth.InitForTesting("test-jwt-secret-0123456789", "test-cookie-secret-0123456")

	pool, cleanup := startWebhookTestPostgres(t)
	defer cleanup()
	ctx := context.Background()

	if err := database.RunMigrations(pool); err != nil {
		t.Fatalf("core migrations: %v", err)
	}
	if err := RunMigrations(pool); err != nil {
		t.Fatalf("webhook migrations: %v", err)
	}
	contextID := seedContext(t, pool)

	secret := "round-trip-secret-abcdef"
	r := New(pool)
	e := &webhook.WebhookEndpoint{
		ContextID:     contextID,
		Name:          "hook",
		URL:           "https://example.com/x",
		Events:        []string{"test.ping"},
		Active:        true,
		SigningSecret: &secret,
	}
	hash := "hash"
	e.SecretHash = &hash
	if err := r.Create(e); err != nil {
		t.Fatalf("Create: %v", err)
	}

	// The column holds ciphertext, never the plaintext.
	var enc string
	if err := pool.QueryRow(ctx,
		"SELECT signing_secret_enc::text FROM webhook_endpoints WHERE id = $1", e.ID,
	).Scan(&enc); err != nil {
		t.Fatalf("read signing_secret_enc: %v", err)
	}
	if enc == "" {
		t.Fatal("signing_secret_enc is NULL for an endpoint created with a secret")
	}
	if strings.Contains(enc, secret) {
		t.Fatal("stored signing_secret_enc contains the plaintext secret")
	}
	var plainExists bool
	if err := pool.QueryRow(ctx,
		"SELECT EXISTS (SELECT 1 FROM webhook_endpoints WHERE id = $1 AND signing_secret IS NOT NULL)", e.ID,
	).Scan(&plainExists); err != nil && err != sql.ErrNoRows {
		// The column is dropped by the migration; a SQL error proves it is
		// gone, which is what we want.
		if !strings.Contains(err.Error(), "signing_secret") {
			t.Fatalf("unexpected error probing signing_secret column: %v", err)
		}
	} else if plainExists {
		t.Fatal("plaintext signing_secret column still holds data")
	}

	// Read back through the repo: plaintext must match.
	got, err := r.GetByID(e.ID)
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if got.SigningSecret == nil || *got.SigningSecret != secret {
		t.Fatalf("round-trip SigningSecret = %v, want %q", got.SigningSecret, secret)
	}
}

// TestSecret_UpdateRoundTrip verifies that updating the secret re-encrypts at
// rest and the new value reads back.
func TestSecret_UpdateRoundTrip(t *testing.T) {
	auth.InitForTesting("test-jwt-secret-0123456789", "test-cookie-secret-0123456")

	pool, cleanup := startWebhookTestPostgres(t)
	defer cleanup()

	if err := database.RunMigrations(pool); err != nil {
		t.Fatalf("core migrations: %v", err)
	}
	if err := RunMigrations(pool); err != nil {
		t.Fatalf("webhook migrations: %v", err)
	}
	contextID := seedContext(t, pool)

	r := New(pool)
	first := "first-secret-value-123456"
	e := &webhook.WebhookEndpoint{
		ContextID:     contextID,
		Name:          "hook",
		URL:           "https://example.com/x",
		Events:        []string{"test.ping"},
		Active:        true,
		SigningSecret: &first,
	}
	if err := r.Create(e); err != nil {
		t.Fatalf("Create: %v", err)
	}

	second := "second-secret-value-654321"
	e.SigningSecret = &second
	if err := r.Update(e); err != nil {
		t.Fatalf("Update: %v", err)
	}

	got, err := r.GetByID(e.ID)
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if got.SigningSecret == nil || *got.SigningSecret != second {
		t.Fatalf("updated SigningSecret = %v, want %q", got.SigningSecret, second)
	}
}