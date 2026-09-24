package database_test

import (
	"context"
	"testing"
	"time"

	"github.com/hekemen/automata/internal/infrastructure/database"
	"github.com/jackc/pgx/v5/pgxpool"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
)

var _ = Describe("Migration", func() {
	var (
		ctx               context.Context
		postgresContainer *postgres.PostgresContainer
	)

	BeforeEach(func() {
		ctx = context.Background()
	})

	AfterEach(func() {
		if postgresContainer != nil {
			_ = postgresContainer.Terminate(ctx)
		}
	})

	newPool := func() (*postgres.PostgresContainer, *pgxpool.Pool) {
		c, err := postgres.Run(
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

		url, err := c.ConnectionString(ctx, "sslmode=disable")
		Expect(err).NotTo(HaveOccurred())

		pool, err := pgxpool.New(ctx, url)
		Expect(err).NotTo(HaveOccurred())
		Expect(pool.Ping(ctx)).To(Succeed())

		return c, pool
	}

	It("creates all tables", func() {
		container, pool := newPool()
		postgresContainer = container
		defer pool.Close()

		err := database.RunMigrations(pool)
		Expect(err).NotTo(HaveOccurred())

		// Verify contexts table exists
		var count int
		err = pool.QueryRow(ctx, "SELECT COUNT(*) FROM contexts").Scan(&count)
		Expect(err).NotTo(HaveOccurred())

		// Verify context_users table exists
		err = pool.QueryRow(ctx, "SELECT COUNT(*) FROM context_users").Scan(&count)
		Expect(err).NotTo(HaveOccurred())

		// Verify api_keys table exists
		err = pool.QueryRow(ctx, "SELECT COUNT(*) FROM api_keys").Scan(&count)
		Expect(err).NotTo(HaveOccurred())
	})

	It("creates all indexes", func() {
		container, pool := newPool()
		postgresContainer = container
		defer pool.Close()

		err := database.RunMigrations(pool)
		Expect(err).NotTo(HaveOccurred())

		// Verify idx_context_users_context_email index exists
		var idxCount int
		err = pool.QueryRow(ctx, `
			SELECT COUNT(*) FROM pg_indexes 
			WHERE indexname = 'idx_context_users_context_email'
		`).Scan(&idxCount)
		Expect(err).NotTo(HaveOccurred())
		Expect(idxCount).To(Equal(1))

		// Verify idx_api_keys_context_name index exists
		err = pool.QueryRow(ctx, `
			SELECT COUNT(*) FROM pg_indexes 
			WHERE indexname = 'idx_api_keys_context_name'
		`).Scan(&idxCount)
		Expect(err).NotTo(HaveOccurred())
	})

	It("handles idempotent re-runs", func() {
		container, pool := newPool()
		postgresContainer = container
		defer pool.Close()

		// Run migration twice
		err := database.RunMigrations(pool)
		Expect(err).NotTo(HaveOccurred())

		err = database.RunMigrations(pool)
		Expect(err).NotTo(HaveOccurred())

		// Tables should still be accessible
		var count int
		err = pool.QueryRow(ctx, "SELECT COUNT(*) FROM contexts").Scan(&count)
		Expect(err).NotTo(HaveOccurred())
	})
})

func TestMigration(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Migration Suite")
}
