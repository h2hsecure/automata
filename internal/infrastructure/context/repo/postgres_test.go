package repo_test

import (
	"context"
	"testing"
	"time"

	"github.com/hekemen/automata/internal/domain/context"
	"github.com/hekemen/automata/internal/infrastructure/tenant/repo"
	"github.com/jackc/pgx/v5/pgxpool"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
)

var _ = Describe("Postgres Tenant Repository", func() {
	var (
		ctx               context.Context
		container         *postgres.PostgresContainer
		pool              *pgxpool.Pool
		tenantRepo        context.Repository
		postgresContainer *postgres.PostgresContainer
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

		// Run migrations
		err = runMigrationSQL(pool)
		Expect(err).NotTo(HaveOccurred())

		tenantRepo = repo.NewPostgresRepo(pool)
		postgresContainer = container
	})

	AfterEach(func() {
		if postgresContainer != nil {
			_ = postgresContainer.Terminate(ctx)
		}
		if pool != nil {
			pool.Close()
		}
	})

	Describe("Create", func() {
		It("creates a context and returns an ID", func() {
			t := &context.Context{
				Slug: "test-context",
				Name: "Test Tenant",
			}

			err := tenantRepo.Create(t)
			Expect(err).NotTo(HaveOccurred())
			Expect(t.ID).NotTo(BeEmpty())
			Expect(t.CreatedAt).To(BeTemporally("~", time.Now(), 5*time.Second))
			Expect(t.UpdatedAt).To(BeTemporally("~", time.Now(), 5*time.Second))
		})

		It("validates context before creating", func() {
			t := &context.Context{
				Slug: "",
				Name: "Test",
			}

			err := tenantRepo.Create(t)
			Expect(err).To(HaveOccurred())
		})
	})

	Describe("GetByID", func() {
		It("returns a context by ID", func() {
			t := &context.Context{Slug: "getbyid-test", Name: "Get By ID Test"}
			err := tenantRepo.Create(t)
			Expect(err).NotTo(HaveOccurred())

			found, err := tenantRepo.GetByID(t.ID)
			Expect(err).NotTo(HaveOccurred())
			Expect(found.Slug).To(Equal("getbyid-test"))
			Expect(found.Name).To(Equal("Get By ID Test"))
		})

		It("returns error for non-existent ID", func() {
			_, err := tenantRepo.GetByID("00000000-0000-0000-0000-000000000000")
			Expect(err).To(HaveOccurred())
		})
	})

	Describe("GetBySlug", func() {
		It("returns a context by slug", func() {
			t := &context.Context{Slug: "slug-test", Name: "Slug Test"}
			err := tenantRepo.Create(t)
			Expect(err).NotTo(HaveOccurred())

			found, err := tenantRepo.GetBySlug("slug-test")
			Expect(err).NotTo(HaveOccurred())
			Expect(found.ID).To(Equal(t.ID))
		})

		It("returns error for non-existent slug", func() {
			_, err := tenantRepo.GetBySlug("nonexistent")
			Expect(err).To(HaveOccurred())
		})
	})

	Describe("List", func() {
		It("returns created tenants", func() {
			for i := 0; i < 3; i++ {
				t := &context.Context{Slug: "list-test-" + string(rune('0'+i)), Name: "List Test"}
				err := tenantRepo.Create(t)
				Expect(err).NotTo(HaveOccurred())
			}

			tenants, err := tenantRepo.List(0, 10)
			Expect(err).NotTo(HaveOccurred())
			Expect(tenants).To(HaveLen(3))
		})

		It("respects pagination", func() {
			for i := 0; i < 5; i++ {
				t := &context.Context{Slug: "pag-test-" + string(rune('0'+i)), Name: "Pag Test"}
				err := tenantRepo.Create(t)
				Expect(err).NotTo(HaveOccurred())
			}

			tenants, err := tenantRepo.List(0, 2)
			Expect(err).NotTo(HaveOccurred())
			Expect(tenants).To(HaveLen(2))

			tenants, err = tenantRepo.List(2, 2)
			Expect(err).NotTo(HaveOccurred())
			Expect(tenants).To(HaveLen(2))
		})
	})

	Describe("Update", func() {
		It("updates a context", func() {
			t := &context.Context{Slug: "update-test", Name: "Original"}
			err := tenantRepo.Create(t)
			Expect(err).NotTo(HaveOccurred())

			t.Name = "Updated"
			t.IsActive = false
			err = tenantRepo.Update(t)
			Expect(err).NotTo(HaveOccurred())

			found, err := tenantRepo.GetByID(t.ID)
			Expect(err).NotTo(HaveOccurred())
			Expect(found.Name).To(Equal("Updated"))
			Expect(found.IsActive).To(BeFalse())
		})

		It("returns error for non-existent context", func() {
			t := &context.Context{ID: "00000000-0000-0000-0000-000000000000", Slug: "x", Name: "x"}
			err := tenantRepo.Update(t)
			Expect(err).To(HaveOccurred())
		})
	})

	Describe("Delete", func() {
		It("deletes a context", func() {
			t := &context.Context{Slug: "delete-test", Name: "Delete Me"}
			err := tenantRepo.Create(t)
			Expect(err).NotTo(HaveOccurred())

			err = tenantRepo.Delete(t.ID)
			Expect(err).NotTo(HaveOccurred())

			_, err = tenantRepo.GetByID(t.ID)
			Expect(err).To(HaveOccurred())
		})

		It("returns error for non-existent context", func() {
			err := tenantRepo.Delete("00000000-0000-0000-0000-000000000000")
			Expect(err).To(HaveOccurred())
		})
	})
})

func runMigrationSQL(pool *pgxpool.Pool) error {
	migrationSQL := `
CREATE TABLE IF NOT EXISTS tenants (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    slug            TEXT NOT NULL UNIQUE,
    name            TEXT NOT NULL,
    domain          VARCHAR(256),
    is_active       BOOLEAN DEFAULT true,
    settings        JSONB DEFAULT '{}',
    created_at      TIMESTAMPTZ DEFAULT NOW(),
    updated_at      TIMESTAMPTZ DEFAULT NOW()
);
`
	_, err := pool.Exec(context.Background(), migrationSQL)
	return err
}

func TestTenantRepo(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Tenant Repository Suite")
}
