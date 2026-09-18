package contact_test

import (
	"context"
	"testing"
	"time"

	"github.com/hekemen/automata/internal/infrastructure/contact"
	"github.com/jackc/pgx/v5/pgxpool"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
)

var _ = Describe("Contact Migration", func() {
	var (
		ctx    context.Context
		pool   *pgxpool.Pool
		cancel context.CancelFunc
	)

	BeforeEach(func() {
		ctx = context.Background()
		var container *postgres.PostgresContainer
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

		cancel = func() {
			_ = container.Terminate(ctx)
			pool.Close()
		}
	})

	AfterEach(func() {
		if cancel != nil {
			cancel()
		}
	})

	Describe("RunMigrations", func() {
		It("creates all tables", func() {
			err := contact.RunMigrations(pool)
			Expect(err).NotTo(HaveOccurred())

			var count int
			err = pool.QueryRow(ctx, `
				SELECT COUNT(*) FROM information_schema.tables
				WHERE table_name IN ('contacts', 'contact_tags', 'contact_tag_memberships', 'contact_field_definitions')
			`).Scan(&count)
			Expect(err).NotTo(HaveOccurred())
			Expect(count).To(Equal(4))
		})

		It("creates all indexes", func() {
			err := contact.RunMigrations(pool)
			Expect(err).NotTo(HaveOccurred())

			var count int
			err = pool.QueryRow(ctx, `
				SELECT COUNT(*) FROM pg_indexes
				WHERE tablename IN ('contacts', 'contact_tags', 'contact_tag_memberships', 'contact_field_definitions')
			`).Scan(&count)
			Expect(err).NotTo(HaveOccurred())
			Expect(count).To(BeNumerically(">", 0))
		})

		It("handles idempotent re-runs", func() {
			err := contact.RunMigrations(pool)
			Expect(err).NotTo(HaveOccurred())

			err = contact.RunMigrations(pool)
			Expect(err).NotTo(HaveOccurred())
		})

		It("creates unique constraint on tenant+email", func() {
			err := contact.RunMigrations(pool)
			Expect(err).NotTo(HaveOccurred())

			// Insert first contact with email
			_, err = pool.Exec(ctx, `
				INSERT INTO contacts (tenant_id, email, first_name)
				VALUES (gen_random_uuid(), 'test@example.com', 'Test')
			`)
			Expect(err).NotTo(HaveOccurred())

			// Try to insert duplicate email — should fail
			_, err = pool.Exec(ctx, `
				INSERT INTO contacts (tenant_id, email, first_name)
				VALUES (gen_random_uuid(), 'test@example.com', 'Duplicate')
			`)
			Expect(err).To(HaveOccurred())
		})
	})
})

func TestContactMigration(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Contact Migration Suite")
}
