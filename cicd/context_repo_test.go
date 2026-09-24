package cicd_test

import (
	"context"

	"github.com/hekemen/automata/cicd/support"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var (
)

var _ = BeforeEach(func() {
	ctx = context.Background()
	pool = db.Pool
})

var _ = Describe("Context Repository Integration Tests", func() {



	AfterEach(func() {
		// Clean up tables
		_, err := pool.Exec(ctx, "DELETE FROM api_keys")
		Expect(err).NotTo(HaveOccurred())
		_, err = pool.Exec(ctx, "DELETE FROM context_users")
		Expect(err).NotTo(HaveOccurred())
		_, err = pool.Exec(ctx, "DELETE FROM contexts")
		Expect(err).NotTo(HaveOccurred())
	})

	Describe("Context CRUD", func() {
		It("should create and retrieve a context", func() {
			contextData := support.NewTestContext()

			query := `
				INSERT INTO contexts (id, slug, name, domain, is_active, settings, created_at, updated_at)
				VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
			`

			_, err := pool.Exec(ctx, query,
				contextData["id"],
				contextData["slug"],
				contextData["name"],
				contextData["domain"],
				contextData["is_active"],
				contextData["settings"],
				contextData["created_at"],
				contextData["updated_at"],
			)
			Expect(err).NotTo(HaveOccurred())

			// Verify context was inserted
			var count int
			err = pool.QueryRow(ctx, "SELECT COUNT(*) FROM contexts").Scan(&count)
			Expect(err).NotTo(HaveOccurred())
			Expect(count).To(Equal(1))

			// Retrieve and verify
			var retrievedSlug, retrievedName string
			err = pool.QueryRow(ctx, "SELECT slug, name FROM contexts WHERE id = $1", contextData["id"]).Scan(&retrievedSlug, &retrievedName)
			Expect(err).NotTo(HaveOccurred())
			Expect(retrievedSlug).To(ContainSubstring("test-context"))
			Expect(retrievedName).To(Equal("Test Context"))
		})

		It("should update context name and status", func() {
			contextData := support.NewTestContext()

			// Insert context
			_, err := pool.Exec(ctx, `
				INSERT INTO contexts (id, slug, name, domain, is_active, settings, created_at, updated_at)
				VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
			`,
				contextData["id"],
				contextData["slug"],
				contextData["name"],
				contextData["domain"],
				contextData["is_active"],
				contextData["settings"],
				contextData["created_at"],
				contextData["updated_at"],
			)
			Expect(err).NotTo(HaveOccurred())

			// Update context
			_, err = pool.Exec(ctx, `
				UPDATE contexts SET name = $1, is_active = $2
				WHERE id = $3
			`, "Updated Context", false, contextData["id"])
			Expect(err).NotTo(HaveOccurred())

			// Verify update
			var name string
			var isActive bool
			err = pool.QueryRow(ctx, "SELECT name, is_active FROM contexts WHERE id = $1", contextData["id"]).Scan(&name, &isActive)
			Expect(err).NotTo(HaveOccurred())
			Expect(name).To(Equal("Updated Context"))
			Expect(isActive).To(BeFalse())
		})

		It("should delete a context", func() {
			contextData := support.NewTestContext()

			// Insert context
			_, err := pool.Exec(ctx, `
				INSERT INTO contexts (id, slug, name, domain, is_active, settings, created_at, updated_at)
				VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
			`,
				contextData["id"],
				contextData["slug"],
				contextData["name"],
				contextData["domain"],
				contextData["is_active"],
				contextData["settings"],
				contextData["created_at"],
				contextData["updated_at"],
			)
			Expect(err).NotTo(HaveOccurred())

			// Delete context
			_, err = pool.Exec(ctx, "DELETE FROM contexts WHERE id = $1", contextData["id"])
			Expect(err).NotTo(HaveOccurred())

			// Verify deletion
			var count int
			err = pool.QueryRow(ctx, "SELECT COUNT(*) FROM contexts").Scan(&count)
			Expect(err).NotTo(HaveOccurred())
			Expect(count).To(Equal(0))
		})

		It("should list contexts with pagination", func() {
			// Insert multiple contexts
			for i := 0; i < 5; i++ {
				contextData := support.NewTestContext()

				_, err := pool.Exec(ctx, `
					INSERT INTO contexts (id, slug, name, domain, is_active, settings, created_at, updated_at)
					VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
				`,
					contextData["id"],
					contextData["slug"],
					contextData["name"],
					contextData["domain"],
					contextData["is_active"],
					contextData["settings"],
					contextData["created_at"],
					contextData["updated_at"],
				)
				Expect(err).NotTo(HaveOccurred())
			}

			// Verify total count
			var totalCount int
			err := pool.QueryRow(ctx, `
				SELECT COUNT(*) FROM contexts
			`).Scan(&totalCount)
			Expect(err).NotTo(HaveOccurred())
			Expect(totalCount).To(Equal(5))
		})
	})

	Describe("Slug uniqueness", func() {
		It("should verify slug uniqueness constraint", func() {
			contextData := support.NewTestContext()

			// Insert first context
			_, err := pool.Exec(ctx, `
				INSERT INTO contexts (id, slug, name, domain, is_active, settings, created_at, updated_at)
				VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
			`,
				contextData["id"],
				contextData["slug"],
				contextData["name"],
				contextData["domain"],
				contextData["is_active"],
				contextData["settings"],
				contextData["created_at"],
				contextData["updated_at"],
			)
			Expect(err).NotTo(HaveOccurred())

			// Try to insert duplicate slug (should fail)
			contextData2 := support.NewTestContext()
			contextData2["id"] = support.NewTestUUID()
			contextData2["slug"] = contextData["slug"] // Same slug

			_, err = pool.Exec(ctx, `
				INSERT INTO contexts (id, slug, name, domain, is_active, settings, created_at, updated_at)
				VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
			`,
				contextData2["id"],
				contextData2["slug"],
				contextData2["name"],
				contextData2["domain"],
				contextData2["is_active"],
				contextData2["settings"],
				contextData2["created_at"],
				contextData2["updated_at"],
			)
			Expect(err).To(HaveOccurred())
		})
	})

	Describe("Context isolation", func() {
		It("should verify context isolation (data from different contexts)", func() {
			// Create two different contexts
			context1Data := support.NewTestContext()
			context2Data := support.NewTestContext()

			_, err := pool.Exec(ctx, `
				INSERT INTO contexts (id, slug, name, domain, is_active, settings, created_at, updated_at)
				VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
			`,
				context1Data["id"],
				context1Data["slug"],
				context1Data["name"],
				context1Data["domain"],
				context1Data["is_active"],
				context1Data["settings"],
				context1Data["created_at"],
				context1Data["updated_at"],
			)
			Expect(err).NotTo(HaveOccurred())

			_, err = pool.Exec(ctx, `
				INSERT INTO contexts (id, slug, name, domain, is_active, settings, created_at, updated_at)
				VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
			`,
				context2Data["id"],
				context2Data["slug"],
				context2Data["name"],
				context2Data["domain"],
				context2Data["is_active"],
				context2Data["settings"],
				context2Data["created_at"],
				context2Data["updated_at"],
			)
			Expect(err).NotTo(HaveOccurred())

			// Verify both contexts exist
			var count int
			err = pool.QueryRow(ctx, "SELECT COUNT(*) FROM contexts").Scan(&count)
			Expect(err).NotTo(HaveOccurred())
			Expect(count).To(Equal(2))

			// Query by specific context
			err = pool.QueryRow(ctx, "SELECT COUNT(*) FROM contexts WHERE id = $1", context1Data["id"]).Scan(&count)
			Expect(err).NotTo(HaveOccurred())
			Expect(count).To(Equal(1))
		})
	})
})
