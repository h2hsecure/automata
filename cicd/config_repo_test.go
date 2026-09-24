package cicd_test

import (
	"encoding/json"

	"github.com/hekemen/automata/cicd/support"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

// createTestContext inserts a context into the DB and returns its ID.
func createTestContext() string {
	contextID := support.NewTestContextID()
	_, err := pool.Exec(ctx, `
		INSERT INTO contexts (id, slug, name, is_active, settings, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, NOW(), NOW())
	`, contextID, "test-"+contextID[:8], "Test Context", true, `{}`)
	Expect(err).NotTo(HaveOccurred())
	return contextID
}

var _ = Describe("Admin Config Repository Integration Tests", func() {

	AfterEach(func() {
		_, err := pool.Exec(ctx, "DELETE FROM admin_configs")
		Expect(err).NotTo(HaveOccurred())
	})

	Describe("Upsert config", func() {
		It("should insert a new CORS config", func() {
			contextID := createTestContext()
			value := map[string]interface{}{"origins": []string{"https://example.com", "https://app.example.com"}}

			_, err := pool.Exec(ctx, `
				INSERT INTO admin_configs (id, context_id, key, value, created_at, updated_at)
				VALUES ($1, $2, $3, $4, NOW(), NOW())
			`, support.NewTestUUID(), contextID, "cors", value)
			Expect(err).NotTo(HaveOccurred())

			var storedValue []byte
			err = pool.QueryRow(ctx, `SELECT value FROM admin_configs WHERE context_id = $1 AND key = $2`, contextID, "cors").Scan(&storedValue)
			Expect(err).NotTo(HaveOccurred())

			var parsed map[string]interface{}
			err = json.Unmarshal(storedValue, &parsed)
			Expect(err).NotTo(HaveOccurred())
			Expect(parsed["origins"]).To(HaveLen(2))
			Expect(parsed["origins"].([]interface{})[0]).To(Equal("https://example.com"))
		})

		It("should upsert and overwrite an existing config", func() {
			contextID := createTestContext()
			configID := support.NewTestUUID()

			_, err := pool.Exec(ctx, `
				INSERT INTO admin_configs (id, context_id, key, value, created_at, updated_at)
				VALUES ($1, $2, $3, $4, NOW(), NOW())
			`, configID, contextID, "cors", `{"origins":["https://old.com"]}`)
			Expect(err).NotTo(HaveOccurred())

			_, err = pool.Exec(ctx, `
				INSERT INTO admin_configs (id, context_id, key, value, created_at, updated_at)
				VALUES ($1, $2, $3, $4, NOW(), NOW())
				ON CONFLICT (context_id, key) DO UPDATE SET value = $4, updated_at = NOW()
			`, support.NewTestUUID(), contextID, "cors", `{"origins":["https://new.com"]}`)
			Expect(err).NotTo(HaveOccurred())

			var storedValue []byte
			err = pool.QueryRow(ctx, `SELECT value FROM admin_configs WHERE context_id = $1 AND key = $2`, contextID, "cors").Scan(&storedValue)
			Expect(err).NotTo(HaveOccurred())

			var parsed map[string]interface{}
			err = json.Unmarshal(storedValue, &parsed)
			Expect(err).NotTo(HaveOccurred())
			Expect(parsed["origins"].([]interface{})[0]).To(Equal("https://new.com"))
		})

		It("should store domain config", func() {
			contextID := createTestContext()
			value := map[string]interface{}{"primary": "myapp.example.com", "aliases": []string{"www.myapp.example.com"}}

			_, err := pool.Exec(ctx, `
				INSERT INTO admin_configs (id, context_id, key, value, created_at, updated_at)
				VALUES ($1, $2, $3, $4, NOW(), NOW())
			`, support.NewTestUUID(), contextID, "domain", value)
			Expect(err).NotTo(HaveOccurred())

			var storedValue []byte
			err = pool.QueryRow(ctx, `SELECT value FROM admin_configs WHERE context_id = $1 AND key = $2`, contextID, "domain").Scan(&storedValue)
			Expect(err).NotTo(HaveOccurred())

			var parsed map[string]interface{}
			err = json.Unmarshal(storedValue, &parsed)
			Expect(err).NotTo(HaveOccurred())
			Expect(parsed["primary"]).To(Equal("myapp.example.com"))
			Expect(parsed["aliases"].([]interface{})).To(HaveLen(1))
		})

		It("should store display config", func() {
			contextID := createTestContext()
			value := map[string]interface{}{"name": "My App", "logo_url": "https://cdn.example.com/logo.png"}

			_, err := pool.Exec(ctx, `
				INSERT INTO admin_configs (id, context_id, key, value, created_at, updated_at)
				VALUES ($1, $2, $3, $4, NOW(), NOW())
			`, support.NewTestUUID(), contextID, "display", value)
			Expect(err).NotTo(HaveOccurred())

			var storedValue []byte
			err = pool.QueryRow(ctx, `SELECT value FROM admin_configs WHERE context_id = $1 AND key = $2`, contextID, "display").Scan(&storedValue)
			Expect(err).NotTo(HaveOccurred())

			var parsed map[string]interface{}
			err = json.Unmarshal(storedValue, &parsed)
			Expect(err).NotTo(HaveOccurred())
			Expect(parsed["name"]).To(Equal("My App"))
			Expect(parsed["logo_url"]).To(Equal("https://cdn.example.com/logo.png"))
		})
	})

	Describe("GetByContext", func() {
		It("should return all configs for a context", func() {
			contextID := createTestContext()

			_, err := pool.Exec(ctx, `
				INSERT INTO admin_configs (id, context_id, key, value, created_at, updated_at)
				VALUES ($1, $2, $3, $4, NOW(), NOW())
			`, support.NewTestUUID(), contextID, "cors", `{"origins":["https://a.com"]}`)
			Expect(err).NotTo(HaveOccurred())

			_, err = pool.Exec(ctx, `
				INSERT INTO admin_configs (id, context_id, key, value, created_at, updated_at)
				VALUES ($1, $2, $3, $4, NOW(), NOW())
			`, support.NewTestUUID(), contextID, "domain", `{"primary":"a.com"}`)
			Expect(err).NotTo(HaveOccurred())

			var count int
			err = pool.QueryRow(ctx, `SELECT COUNT(*) FROM admin_configs WHERE context_id = $1`, contextID).Scan(&count)
			Expect(err).NotTo(HaveOccurred())
			Expect(count).To(Equal(2))
		})

		It("should return empty result for context with no configs", func() {
			contextID := createTestContext()

			var count int
			err := pool.QueryRow(ctx, `SELECT COUNT(*) FROM admin_configs WHERE context_id = $1`, contextID).Scan(&count)
			Expect(err).NotTo(HaveOccurred())
			Expect(count).To(Equal(0))
		})
	})

	Describe("Delete config", func() {
		It("should delete a config by context and key", func() {
			contextID := createTestContext()
			configID := support.NewTestUUID()

			_, err := pool.Exec(ctx, `
				INSERT INTO admin_configs (id, context_id, key, value, created_at, updated_at)
				VALUES ($1, $2, $3, $4, NOW(), NOW())
			`, configID, contextID, "cors", `{"origins":["https://example.com"]}`)
			Expect(err).NotTo(HaveOccurred())

			_, err = pool.Exec(ctx, `DELETE FROM admin_configs WHERE context_id = $1 AND key = $2`, contextID, "cors")
			Expect(err).NotTo(HaveOccurred())

			var count int
			err = pool.QueryRow(ctx, `SELECT COUNT(*) FROM admin_configs WHERE context_id = $1 AND key = $2`, contextID, "cors").Scan(&count)
			Expect(err).NotTo(HaveOccurred())
			Expect(count).To(Equal(0))
		})

		It("should return 0 rows affected when deleting non-existent config", func() {
			contextID := createTestContext()

			result, err := pool.Exec(ctx, `DELETE FROM admin_configs WHERE context_id = $1 AND key = $2`, contextID, "cors")
			Expect(err).NotTo(HaveOccurred())
			Expect(result.RowsAffected()).To(Equal(int64(0)))
		})
	})

	Describe("Unique constraint", func() {
		It("should enforce unique constraint on context_id + key", func() {
			contextID := createTestContext()
			configID := support.NewTestUUID()

			_, err := pool.Exec(ctx, `
				INSERT INTO admin_configs (id, context_id, key, value, created_at, updated_at)
				VALUES ($1, $2, $3, $4, NOW(), NOW())
			`, configID, contextID, "cors", `{"origins":["https://first.com"]}`)
			Expect(err).NotTo(HaveOccurred())

			_, err = pool.Exec(ctx, `
				INSERT INTO admin_configs (id, context_id, key, value, created_at, updated_at)
				VALUES ($1, $2, $3, $4, NOW(), NOW())
			`, support.NewTestUUID(), contextID, "cors", `{"origins":["https://second.com"]}`)
			Expect(err).To(HaveOccurred())
		})
	})

	Describe("Multiple tenants isolation", func() {
		It("should allow same config key for different tenants", func() {
			tenant1 := createTestContext()
			tenant2 := createTestContext()

			_, err := pool.Exec(ctx, `
				INSERT INTO admin_configs (id, context_id, key, value, created_at, updated_at)
				VALUES ($1, $2, $3, $4, NOW(), NOW())
			`, support.NewTestUUID(), tenant1, "cors", `{"origins":["https://tenant1.com"]}`)
			Expect(err).NotTo(HaveOccurred())

			_, err = pool.Exec(ctx, `
				INSERT INTO admin_configs (id, context_id, key, value, created_at, updated_at)
				VALUES ($1, $2, $3, $4, NOW(), NOW())
			`, support.NewTestUUID(), tenant2, "cors", `{"origins":["https://tenant2.com"]}`)
			Expect(err).NotTo(HaveOccurred())

			var count int
			err = pool.QueryRow(ctx, `SELECT COUNT(*) FROM admin_configs WHERE key = 'cors'`).Scan(&count)
			Expect(err).NotTo(HaveOccurred())
			Expect(count).To(Equal(2))
		})
	})
})
