package repo_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	domainform "github.com/h2hsecure/automata/internal/domain/form"
	"github.com/h2hsecure/automata/internal/infrastructure/form/repo"
	"github.com/jackc/pgx/v5/pgxpool"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
)

var _ = Describe("Postgres Form Repository", func() {
	var (
		ctx       context.Context
		container *postgres.PostgresContainer
		pool      *pgxpool.Pool
		formRepo  domainform.Repository
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

		// Run the embedded form migrations
		err = repo.RunMigrations(pool)
		Expect(err).NotTo(HaveOccurred())

		formRepo = repo.New(pool)
	})

	AfterEach(func() {
		if container != nil {
			_ = container.Terminate(ctx)
		}
		if pool != nil {
			pool.Close()
		}
	})

	newForm := func(slug string) *domainform.Form {
		return &domainform.Form{
			ID:        uuid.New().String(),
			ContextID: uuid.New().String(),
			Slug:      slug,
			Name:      "Test Form",
			Fields:    []domainform.FormField{{Slug: "name", Type: "text", Label: "Name"}},
			Settings:  map[string]interface{}{},
		}
	}

	Describe("Delete", func() {
		It("deletes a form without submissions", func() {
			f := newForm("delete-plain")
			Expect(formRepo.Create(f)).To(Succeed())

			Expect(formRepo.Delete(f.ID)).To(Succeed())

			_, err := formRepo.GetByID(f.ID)
			Expect(err).To(HaveOccurred())
		})

		It("cascades: deletes a form and its submissions atomically", func() {
			f := newForm("delete-cascade")
			Expect(formRepo.Create(f)).To(Succeed())

			s := &domainform.Submission{
				ID:        uuid.New().String(),
				FormID:    f.ID,
				ContextID: f.ContextID,
				Data:      map[string]interface{}{"name": "Jane"},
				Files:     []domainform.FileUpload{},
			}
			Expect(formRepo.CreateSubmission(s)).To(Succeed())

			submissions, total, err := formRepo.ListSubmissions(f.ID, domainform.ListSubmissionsOptions{})
			Expect(err).NotTo(HaveOccurred())
			Expect(total).To(BeEquivalentTo(1))
			Expect(submissions).To(HaveLen(1))

			// Before the fix this failed with an FK violation (23503).
			Expect(formRepo.Delete(f.ID)).To(Succeed())

			_, err = formRepo.GetByID(f.ID)
			Expect(err).To(HaveOccurred())

			submissions, total, err = formRepo.ListSubmissions(f.ID, domainform.ListSubmissionsOptions{})
			Expect(err).NotTo(HaveOccurred())
			Expect(total).To(BeEquivalentTo(0))
		})

		It("returns error for non-existent form", func() {
			Expect(formRepo.Delete(uuid.New().String())).To(HaveOccurred())
		})
	})
})

func TestFormRepo(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Form Repository Suite")
}
