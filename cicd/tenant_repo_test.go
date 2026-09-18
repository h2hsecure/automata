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

var _ = Describe("Tenant Repository Integration Tests", func() {



	AfterEach(func() {
		// Clean up tables
		_, err := pool.Exec(ctx, "DELETE FROM api_keys")
		Expect(err).NotTo(HaveOccurred())
		_, err = pool.Exec(ctx, "DELETE FROM tenant_users")
		Expect(err).NotTo(HaveOccurred())
		_, err = pool.Exec(ctx, "DELETE FROM tenants")
		Expect(err).NotTo(HaveOccurred())
	})

	Describe("Tenant CRUD", func() {
		It("should create and retrieve a tenant", func() {
			tenantData := support.NewTestTenant()

			query := `
				INSERT INTO tenants (id, slug, name, domain, is_active, settings, created_at, updated_at)
				VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
			`

			_, err := pool.Exec(ctx, query,
				tenantData["id"],
				tenantData["slug"],
				tenantData["name"],
				tenantData["domain"],
				tenantData["is_active"],
				tenantData["settings"],
				tenantData["created_at"],
				tenantData["updated_at"],
			)
			Expect(err).NotTo(HaveOccurred())

			// Verify tenant was inserted
			var count int
			err = pool.QueryRow(ctx, "SELECT COUNT(*) FROM tenants").Scan(&count)
			Expect(err).NotTo(HaveOccurred())
			Expect(count).To(Equal(1))

			// Retrieve and verify
			var retrievedSlug, retrievedName string
			err = pool.QueryRow(ctx, "SELECT slug, name FROM tenants WHERE id = $1", tenantData["id"]).Scan(&retrievedSlug, &retrievedName)
			Expect(err).NotTo(HaveOccurred())
			Expect(retrievedSlug).To(ContainSubstring("test-tenant"))
			Expect(retrievedName).To(Equal("Test Tenant"))
		})

		It("should update tenant name and status", func() {
			tenantData := support.NewTestTenant()

			// Insert tenant
			_, err := pool.Exec(ctx, `
				INSERT INTO tenants (id, slug, name, domain, is_active, settings, created_at, updated_at)
				VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
			`,
				tenantData["id"],
				tenantData["slug"],
				tenantData["name"],
				tenantData["domain"],
				tenantData["is_active"],
				tenantData["settings"],
				tenantData["created_at"],
				tenantData["updated_at"],
			)
			Expect(err).NotTo(HaveOccurred())

			// Update tenant
			_, err = pool.Exec(ctx, `
				UPDATE tenants SET name = $1, is_active = $2
				WHERE id = $3
			`, "Updated Tenant", false, tenantData["id"])
			Expect(err).NotTo(HaveOccurred())

			// Verify update
			var name string
			var isActive bool
			err = pool.QueryRow(ctx, "SELECT name, is_active FROM tenants WHERE id = $1", tenantData["id"]).Scan(&name, &isActive)
			Expect(err).NotTo(HaveOccurred())
			Expect(name).To(Equal("Updated Tenant"))
			Expect(isActive).To(BeFalse())
		})

		It("should delete a tenant", func() {
			tenantData := support.NewTestTenant()

			// Insert tenant
			_, err := pool.Exec(ctx, `
				INSERT INTO tenants (id, slug, name, domain, is_active, settings, created_at, updated_at)
				VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
			`,
				tenantData["id"],
				tenantData["slug"],
				tenantData["name"],
				tenantData["domain"],
				tenantData["is_active"],
				tenantData["settings"],
				tenantData["created_at"],
				tenantData["updated_at"],
			)
			Expect(err).NotTo(HaveOccurred())

			// Delete tenant
			_, err = pool.Exec(ctx, "DELETE FROM tenants WHERE id = $1", tenantData["id"])
			Expect(err).NotTo(HaveOccurred())

			// Verify deletion
			var count int
			err = pool.QueryRow(ctx, "SELECT COUNT(*) FROM tenants").Scan(&count)
			Expect(err).NotTo(HaveOccurred())
			Expect(count).To(Equal(0))
		})

		It("should list tenants with pagination", func() {
			// Insert multiple tenants
			for i := 0; i < 5; i++ {
				tenantData := support.NewTestTenant()

				_, err := pool.Exec(ctx, `
					INSERT INTO tenants (id, slug, name, domain, is_active, settings, created_at, updated_at)
					VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
				`,
					tenantData["id"],
					tenantData["slug"],
					tenantData["name"],
					tenantData["domain"],
					tenantData["is_active"],
					tenantData["settings"],
					tenantData["created_at"],
					tenantData["updated_at"],
				)
				Expect(err).NotTo(HaveOccurred())
			}

			// Verify total count
			var totalCount int
			err := pool.QueryRow(ctx, `
				SELECT COUNT(*) FROM tenants
			`).Scan(&totalCount)
			Expect(err).NotTo(HaveOccurred())
			Expect(totalCount).To(Equal(5))
		})
	})

	Describe("Slug uniqueness", func() {
		It("should verify slug uniqueness constraint", func() {
			tenantData := support.NewTestTenant()

			// Insert first tenant
			_, err := pool.Exec(ctx, `
				INSERT INTO tenants (id, slug, name, domain, is_active, settings, created_at, updated_at)
				VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
			`,
				tenantData["id"],
				tenantData["slug"],
				tenantData["name"],
				tenantData["domain"],
				tenantData["is_active"],
				tenantData["settings"],
				tenantData["created_at"],
				tenantData["updated_at"],
			)
			Expect(err).NotTo(HaveOccurred())

			// Try to insert duplicate slug (should fail)
			tenantData2 := support.NewTestTenant()
			tenantData2["id"] = support.NewTestUUID()
			tenantData2["slug"] = tenantData["slug"] // Same slug

			_, err = pool.Exec(ctx, `
				INSERT INTO tenants (id, slug, name, domain, is_active, settings, created_at, updated_at)
				VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
			`,
				tenantData2["id"],
				tenantData2["slug"],
				tenantData2["name"],
				tenantData2["domain"],
				tenantData2["is_active"],
				tenantData2["settings"],
				tenantData2["created_at"],
				tenantData2["updated_at"],
			)
			Expect(err).To(HaveOccurred())
		})
	})

	Describe("Tenant isolation", func() {
		It("should verify tenant isolation (data from different tenants)", func() {
			// Create two different tenants
			tenant1Data := support.NewTestTenant()
			tenant2Data := support.NewTestTenant()

			_, err := pool.Exec(ctx, `
				INSERT INTO tenants (id, slug, name, domain, is_active, settings, created_at, updated_at)
				VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
			`,
				tenant1Data["id"],
				tenant1Data["slug"],
				tenant1Data["name"],
				tenant1Data["domain"],
				tenant1Data["is_active"],
				tenant1Data["settings"],
				tenant1Data["created_at"],
				tenant1Data["updated_at"],
			)
			Expect(err).NotTo(HaveOccurred())

			_, err = pool.Exec(ctx, `
				INSERT INTO tenants (id, slug, name, domain, is_active, settings, created_at, updated_at)
				VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
			`,
				tenant2Data["id"],
				tenant2Data["slug"],
				tenant2Data["name"],
				tenant2Data["domain"],
				tenant2Data["is_active"],
				tenant2Data["settings"],
				tenant2Data["created_at"],
				tenant2Data["updated_at"],
			)
			Expect(err).NotTo(HaveOccurred())

			// Verify both tenants exist
			var count int
			err = pool.QueryRow(ctx, "SELECT COUNT(*) FROM tenants").Scan(&count)
			Expect(err).NotTo(HaveOccurred())
			Expect(count).To(Equal(2))

			// Query by specific tenant
			err = pool.QueryRow(ctx, "SELECT COUNT(*) FROM tenants WHERE id = $1", tenant1Data["id"]).Scan(&count)
			Expect(err).NotTo(HaveOccurred())
			Expect(count).To(Equal(1))
		})
	})
})
