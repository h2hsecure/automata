package repo_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	domainauth "github.com/h2hsecure/automata/internal/domain/auth"
	"github.com/h2hsecure/automata/internal/infrastructure/auth/repo"
	"github.com/jackc/pgx/v5/pgxpool"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
)

// migrationSQL mirrors internal/infrastructure/database/migration.sql for the
// tables the API key repository depends on.
const migrationSQL = `
CREATE TABLE IF NOT EXISTS contexts (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    slug            TEXT NOT NULL UNIQUE,
    name            TEXT NOT NULL,
    domain          VARCHAR(256),
    is_active       BOOLEAN DEFAULT true,
    settings        JSONB DEFAULT '{}',
    created_at      TIMESTAMPTZ DEFAULT NOW(),
    updated_at      TIMESTAMPTZ DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS context_users (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    context_id   UUID NOT NULL REFERENCES contexts(id) ON DELETE CASCADE,
    email       VARCHAR(254) NOT NULL,
    password_hash TEXT NOT NULL,
    is_owner    BOOLEAN DEFAULT false,
    created_at  TIMESTAMPTZ DEFAULT NOW(),
    updated_at  TIMESTAMPTZ DEFAULT NOW()
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_context_users_context_email ON context_users(context_id, email);

CREATE TABLE IF NOT EXISTS api_keys (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    context_id   UUID NOT NULL REFERENCES contexts(id) ON DELETE CASCADE,
    user_id     UUID NOT NULL REFERENCES context_users(id) ON DELETE CASCADE,
    key_hash    VARCHAR(64) NOT NULL,
    name        TEXT NOT NULL,
    expires_at  TIMESTAMPTZ,
    created_at  TIMESTAMPTZ DEFAULT NOW()
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_api_keys_context_name ON api_keys(context_id, name);
`

var _ = Describe("Postgres API Key Repository", func() {
	var (
		ctx        context.Context
		container  *postgres.PostgresContainer
		pool       *pgxpool.Pool
		keyRepo    repo.ApiKeyRepository
		contextID  string
		userID     string
	)

	BeforeEach(func() {
		ctx = context.Background()
		var err error
		container, err = postgres.Run(
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
		Expect(err).NotTo(HaveOccurred())

		url, err := container.ConnectionString(ctx, "sslmode=disable")
		Expect(err).NotTo(HaveOccurred())

		pool, err = pgxpool.New(ctx, url)
		Expect(err).NotTo(HaveOccurred())
		Expect(pool.Ping(ctx)).To(Succeed())

		_, err = pool.Exec(ctx, migrationSQL)
		Expect(err).NotTo(HaveOccurred())

		keyRepo = repo.NewApiKeyPostgresRepo(pool)

		contextID = uuid.New().String()
		userID = uuid.New().String()
		_, err = pool.Exec(ctx, `
			INSERT INTO contexts (id, slug, name) VALUES ($1, $2, 'Test Context')
		`, contextID, "api-key-test-"+contextID[:8])
		Expect(err).NotTo(HaveOccurred())
		_, err = pool.Exec(ctx, `
			INSERT INTO context_users (id, context_id, email, password_hash) VALUES ($1, $2, $3, 'hash')
		`, userID, contextID, "owner@example.com")
		Expect(err).NotTo(HaveOccurred())
	})

	AfterEach(func() {
		if container != nil {
			_ = container.Terminate(ctx)
		}
		if pool != nil {
			pool.Close()
		}
	})

	Describe("Create", func() {
		It("returns ErrDuplicateKey when the name already exists in the context", func() {
			first := &domainauth.APIKey{
				ContextID: contextID,
				UserID:    userID,
				KeyHash:   "hash-1",
				Name:      "Production API Key",
			}
			Expect(keyRepo.Create(first)).To(Succeed())

			second := &domainauth.APIKey{
				ContextID: contextID,
				UserID:    userID,
				KeyHash:   "hash-2",
				Name:      "Production API Key",
			}
			err := keyRepo.Create(second)
			Expect(err).To(MatchError(domainauth.ErrDuplicateKey))
		})

		It("allows the same name in a different context", func() {
			first := &domainauth.APIKey{
				ContextID: contextID,
				UserID:    userID,
				KeyHash:   "hash-1",
				Name:      "Production API Key",
			}
			Expect(keyRepo.Create(first)).To(Succeed())

			otherContextID := uuid.New().String()
			otherUserID := uuid.New().String()
			_, err := pool.Exec(ctx, `
				INSERT INTO contexts (id, slug, name) VALUES ($1, $2, 'Other Context')
			`, otherContextID, "api-key-other-"+otherContextID[:8])
			Expect(err).NotTo(HaveOccurred())
			_, err = pool.Exec(ctx, `
				INSERT INTO context_users (id, context_id, email, password_hash) VALUES ($1, $2, $3, 'hash')
			`, otherUserID, otherContextID, "owner@example.com")
			Expect(err).NotTo(HaveOccurred())

			second := &domainauth.APIKey{
				ContextID: otherContextID,
				UserID:    otherUserID,
				KeyHash:   "hash-2",
				Name:      "Production API Key",
			}
			Expect(keyRepo.Create(second)).To(Succeed())
		})
	})
})

func TestApiKeyRepo(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "API Key Repository Suite")
}