package repo_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/h2hsecure/automata/internal/infrastructure/auth/repo"
	"github.com/jackc/pgx/v5/pgxpool"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
)

// refreshMigrationSQL mirrors internal/infrastructure/database/migration_user_contexts.sql
// for the tables the refresh token store depends on.
const refreshMigrationSQL = `
CREATE TABLE IF NOT EXISTS users (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    email           VARCHAR(254) NOT NULL UNIQUE,
    password_hash   TEXT,
    sso_provider    VARCHAR(64),
    sso_id          VARCHAR(512),
    is_admin        BOOLEAN DEFAULT false,
    created_at      TIMESTAMPTZ DEFAULT NOW(),
    updated_at      TIMESTAMPTZ DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS refresh_tokens (
    jti        UUID PRIMARY KEY,
    user_id    UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    expires_at TIMESTAMPTZ NOT NULL,
    revoked_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_refresh_tokens_user ON refresh_tokens(user_id);
`

var _ = Describe("Postgres Refresh Token Store", func() {
	var (
		ctx       context.Context
		container *postgres.PostgresContainer
		pool      *pgxpool.Pool
		store     *repo.RefreshTokenPostgresStore
		userID    string
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

		_, err = pool.Exec(ctx, refreshMigrationSQL)
		Expect(err).NotTo(HaveOccurred())

		store = repo.NewRefreshTokenPostgresStore(pool)

		userID = uuid.New().String()
		_, err = pool.Exec(ctx, `
			INSERT INTO users (id, email) VALUES ($1, $2)
		`, userID, "refresh-test-"+userID[:8]+"@example.com")
		Expect(err).NotTo(HaveOccurred())
	})

	AfterEach(func() {
		pool.Close()
		if container != nil {
			_ = container.Terminate(context.Background())
		}
	})

	Context("Insert and RevokeIfActive", func() {
		It("revokes an active token, then reports reuse, then missing for unknown", func() {
			jti := uuid.New().String()
			Expect(store.Insert(ctx, jti, userID, time.Now().Add(7*24*time.Hour))).To(Succeed())

			outcome, err := store.RevokeIfActive(ctx, jti)
			Expect(err).NotTo(HaveOccurred())
			Expect(outcome).To(Equal("ok"))

			outcome, err = store.RevokeIfActive(ctx, jti)
			Expect(err).NotTo(HaveOccurred())
			Expect(outcome).To(Equal("already_revoked"))

			outcome, err = store.RevokeIfActive(ctx, uuid.New().String())
			Expect(err).NotTo(HaveOccurred())
			Expect(outcome).To(Equal("missing"))
		})

		It("tracks tokens independently per jti", func() {
			jtiA, jtiB := uuid.New().String(), uuid.New().String()
			Expect(store.Insert(ctx, jtiA, userID, time.Now().Add(7*24*time.Hour))).To(Succeed())
			Expect(store.Insert(ctx, jtiB, userID, time.Now().Add(7*24*time.Hour))).To(Succeed())

			outcome, err := store.RevokeIfActive(ctx, jtiA)
			Expect(err).NotTo(HaveOccurred())
			Expect(outcome).To(Equal("ok"))

			// jtiB is unaffected by jtiA's revocation.
			outcome, err = store.RevokeIfActive(ctx, jtiB)
			Expect(err).NotTo(HaveOccurred())
			Expect(outcome).To(Equal("ok"))
		})
	})

	Context("Prune", func() {
		It("removes long-revoked and expired rows, keeps active ones", func() {
			jtiStale, jtiExpired, jtiActive := uuid.New().String(), uuid.New().String(), uuid.New().String()
			Expect(store.Insert(ctx, jtiStale, userID, time.Now().Add(7*24*time.Hour))).To(Succeed())
			Expect(store.Insert(ctx, jtiExpired, userID, time.Now().Add(7*24*time.Hour))).To(Succeed())
			Expect(store.Insert(ctx, jtiActive, userID, time.Now().Add(7*24*time.Hour))).To(Succeed())

			// Revoke the stale one and backdate it 60 days.
			_, err := store.RevokeIfActive(ctx, jtiStale)
			Expect(err).NotTo(HaveOccurred())
			_, err = pool.Exec(ctx, `UPDATE refresh_tokens SET revoked_at = NOW() - INTERVAL '60 days' WHERE jti = $1`, jtiStale)
			Expect(err).NotTo(HaveOccurred())

			// Expire the other one well past the prune cutoff.
			_, err = pool.Exec(ctx, `UPDATE refresh_tokens SET expires_at = NOW() - INTERVAL '60 days' WHERE jti = $1`, jtiExpired)
			Expect(err).NotTo(HaveOccurred())

			n, err := store.Prune(ctx, time.Now().Add(-30*24*time.Hour))
			Expect(err).NotTo(HaveOccurred())
			Expect(n).To(Equal(int64(2)))

			// The active token survives.
			outcome, err := store.RevokeIfActive(ctx, jtiActive)
			Expect(err).NotTo(HaveOccurred())
			Expect(outcome).To(Equal("ok"))
		})
	})
})

func TestRefreshTokenStore(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Refresh Token Store Suite")
}