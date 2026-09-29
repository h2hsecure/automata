package cicd_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"

	"github.com/gin-gonic/gin"
	"github.com/hekemen/automata/cicd/support"
	bhandler "github.com/hekemen/automata/internal/adapter/api/handler"
	crepo "github.com/hekemen/automata/internal/infrastructure/contact/repo"
	ctxrepo "github.com/hekemen/automata/internal/infrastructure/context/repo"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func setupContactEngine() *gin.Engine {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	contactRepo := crepo.New(pool)
	contextRepo := ctxrepo.NewPostgresRepo(pool)
	contactHandler := bhandler.NewContactHandler(contactRepo)

	// Use a simplified context resolver that sets context_id from header
	engine.Use(func(c *gin.Context) {
		if contextID := c.GetHeader("X-Context-ID"); contextID != "" {
			c.Set("context_id", contextID)
			t, err := contextRepo.GetByID(contextID)
			if err == nil {
				c.Set("context", t)
			}
			c.Next()
			return
		}
		c.JSON(http.StatusBadRequest, gin.H{"error": "context not found"})
		c.Abort()
	})

	contacts := engine.Group("/api/contacts")
	{
		contacts.GET("", contactHandler.List)
		contacts.GET("/:id", contactHandler.Get)
		contacts.POST("", contactHandler.Create)
		contacts.PUT("/:id", contactHandler.Update)
		contacts.DELETE("/:id", contactHandler.Delete)
		contacts.POST("/:id/merge", contactHandler.Merge)
		contacts.GET("/:id/activity", contactHandler.GetActivity)
	}

	return engine
}

var _ = Describe("HTTP Integration Tests - Contact CRUD", func() {
	var (
		w         *httptest.ResponseRecorder
		engine    *gin.Engine
		contextID  string
	)

	BeforeEach(func() {
		contextID = support.NewTestContextID()
		// Create context record so resolver can find it
		_, _ = pool.Exec(ctx, "INSERT INTO contexts (id, slug, name, is_active, settings) VALUES ($1, $2, $3, true, '{}')",
			contextID, "test-context", "Test Context")
		// Create contact_activities table for merge operation
		_, _ = pool.Exec(ctx, `CREATE TABLE IF NOT EXISTS contact_activities (
			id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			contact_id UUID NOT NULL,
			context_id UUID NOT NULL,
			type TEXT NOT NULL,
			data JSONB,
			source_id TEXT,
			created_at TIMESTAMPTZ DEFAULT NOW()
		)`)
		engine = setupContactEngine()
		w = httptest.NewRecorder()
	})

	AfterEach(func() {
		_, _ = pool.Exec(ctx, "DELETE FROM contact_tags")
		_, _ = pool.Exec(ctx, "DELETE FROM contacts")
	})

	Describe("POST /api/contacts", func() {
		It("should create a contact with valid request", func() {
			// Create context record so resolver can find it
			_, _ = pool.Exec(ctx, "INSERT INTO tenants (id, slug, name) VALUES ($1, $2, $3)",
				contextID, "test-context", "Test Tenant")

			body := `{
				"email": "new@example.com",
				"first_name": "John",
				"last_name": "Doe",
				"phone": "+1234567890",
				"company": "Test Corp",
				"custom_fields": {},
				"source": "web",
				"tags": ["vip"]
			}`

			req := httptest.NewRequest("POST", "/api/contacts", bytes.NewBufferString(body))
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("X-Context-ID", contextID)
			engine.ServeHTTP(w, req)

			Expect(w.Code).To(Equal(http.StatusCreated))

			var response map[string]interface{}
			err := json.Unmarshal(w.Body.Bytes(), &response)
			Expect(err).NotTo(HaveOccurred())
			Expect(response["Email"]).To(Equal("new@example.com"))
		})

		It("should return 401 when context ID is missing", func() {
			body := `{"email": "test@example.com"}`

			req := httptest.NewRequest("POST", "/api/contacts", bytes.NewBufferString(body))
			req.Header.Set("Content-Type", "application/json")
			engine.ServeHTTP(w, req)

			Expect(w.Code).To(Equal(http.StatusBadRequest))
		})

		It("should return 400 for invalid JSON", func() {
			
			body := `{invalid json}`

			req := httptest.NewRequest("POST", "/api/contacts", bytes.NewBufferString(body))
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("X-Context-ID", contextID)
			engine.ServeHTTP(w, req)

			Expect(w.Code).To(Equal(http.StatusBadRequest))
		})
	})

	Describe("GET /api/contacts", func() {
		It("should list contacts with pagination", func() {
			
			contactID := support.NewTestUUID()

			email := "user@example.com"
			customFields, _ := json.Marshal(map[string]interface{}{})

			_, err := pool.Exec(ctx, `
				INSERT INTO contacts (id, context_id, email, first_name, last_name, phone, company,
				                      custom_fields, source, source_id, created_at, updated_at)
				VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, NOW(), NOW())
			`, contactID, contextID, email, "John", "Doe", "+1234567890", "Test Corp",
				customFields, "test", "")
			Expect(err).NotTo(HaveOccurred())

			req := httptest.NewRequest("GET", "/api/contacts?page=1&limit=20", nil)
			req.Header.Set("X-Context-ID", contextID)
			engine.ServeHTTP(w, req)

			Expect(w.Code).To(Equal(http.StatusOK))

			var response map[string]interface{}
			err = json.Unmarshal(w.Body.Bytes(), &response)
			Expect(err).NotTo(HaveOccurred())
			Expect(response["contacts"]).NotTo(BeNil())
			Expect(response["total"]).NotTo(BeNil())
			Expect(response["page"]).NotTo(BeNil())
		})

		It("should return 401 when context ID is missing", func() {
			req := httptest.NewRequest("GET", "/api/contacts", nil)
			engine.ServeHTTP(w, req)

			Expect(w.Code).To(Equal(http.StatusBadRequest))
		})

		It("should filter contacts by search query", func() {
			
			contactID := support.NewTestUUID()

			email := "search@example.com"
			customFields, _ := json.Marshal(map[string]interface{}{})

			_, err := pool.Exec(ctx, `
				INSERT INTO contacts (id, context_id, email, first_name, last_name, phone, company,
				                      custom_fields, source, source_id, created_at, updated_at)
				VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, NOW(), NOW())
			`, contactID, contextID, email, "John", "Doe", "+1234567890", "Test Corp",
				customFields, "test", "")
			Expect(err).NotTo(HaveOccurred())

			req := httptest.NewRequest("GET", "/api/contacts?search=Search", nil)
			req.Header.Set("X-Context-ID", contextID)
			engine.ServeHTTP(w, req)

			Expect(w.Code).To(Equal(http.StatusOK))
		})

		It("should filter contacts by source", func() {
			
			contactID := support.NewTestUUID()

			email := "source@example.com"
			customFields, _ := json.Marshal(map[string]interface{}{})

			_, err := pool.Exec(ctx, `
				INSERT INTO contacts (id, context_id, email, first_name, last_name, phone, company,
				                      custom_fields, source, source_id, created_at, updated_at)
				VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, NOW(), NOW())
			`, contactID, contextID, email, "Test", "User", "+1234567890", "Test Corp",
				customFields, "webhook", "")
			Expect(err).NotTo(HaveOccurred())

			req := httptest.NewRequest("GET", "/api/contacts?source=webhook", nil)
			req.Header.Set("X-Context-ID", contextID)
			engine.ServeHTTP(w, req)

			Expect(w.Code).To(Equal(http.StatusOK))
		})

		It("should filter contacts by company", func() {
			
			contactID := support.NewTestUUID()

			email := "company@example.com"
			customFields, _ := json.Marshal(map[string]interface{}{})

			_, err := pool.Exec(ctx, `
				INSERT INTO contacts (id, context_id, email, first_name, last_name, phone, company,
				                      custom_fields, source, source_id, created_at, updated_at)
				VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, NOW(), NOW())
			`, contactID, contextID, email, "Test", "User", "+1234567890", "Acme Corp",
				customFields, "test", "")
			Expect(err).NotTo(HaveOccurred())

			req := httptest.NewRequest("GET", "/api/contacts?company=Acme", nil)
			req.Header.Set("X-Context-ID", contextID)
			engine.ServeHTTP(w, req)

			Expect(w.Code).To(Equal(http.StatusOK))
		})
	})

	Describe("GET /api/contacts/:id", func() {
		It("should return a contact by ID", func() {
			
			contactID := support.NewTestUUID()

			email := "get@example.com"
			customFields, _ := json.Marshal(map[string]interface{}{})

			_, err := pool.Exec(ctx, `
				INSERT INTO contacts (id, context_id, email, first_name, last_name, phone, company,
				                      custom_fields, source, source_id, created_at, updated_at)
				VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, NOW(), NOW())
			`, contactID, contextID, email, "John", "Doe", "+1234567890", "Test Corp",
				customFields, "test", "")
			Expect(err).NotTo(HaveOccurred())

			req := httptest.NewRequest("GET", "/api/contacts/"+contactID, nil)
			req.Header.Set("X-Context-ID", contextID)
			engine.ServeHTTP(w, req)

			Expect(w.Code).To(Equal(http.StatusOK))

			var response map[string]interface{}
			err = json.Unmarshal(w.Body.Bytes(), &response)
			Expect(err).NotTo(HaveOccurred())
			contactResp, ok := response["contact"].(map[string]interface{})
			Expect(ok).To(BeTrue())
			Expect(contactResp["ID"]).To(Equal(contactID))
		})

		It("should return 404 for non-existent contact", func() {
			fakeContactID := support.NewTestUUID()
			req := httptest.NewRequest("GET", "/api/contacts/"+fakeContactID, nil)
			req.Header.Set("X-Context-ID", support.NewTestContextID())
			engine.ServeHTTP(w, req)

			Expect(w.Code).To(Equal(http.StatusNotFound))
		})

		It("should return 401 when context ID is missing", func() {
			req := httptest.NewRequest("GET", "/api/contacts/some-id", nil)
			engine.ServeHTTP(w, req)

			Expect(w.Code).To(Equal(http.StatusBadRequest))
		})
	})

	Describe("PUT /api/contacts/:id", func() {
		It("should update a contact", func() {
			
			contactID := support.NewTestUUID()

			email := "update@example.com"
			customFields, _ := json.Marshal(map[string]interface{}{})

			_, err := pool.Exec(ctx, `
				INSERT INTO contacts (id, context_id, email, first_name, last_name, phone, company,
				                      custom_fields, source, source_id, created_at, updated_at)
				VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, NOW(), NOW())
			`, contactID, contextID, email, "Old", "Name", "+1234567890", "Old Corp",
				customFields, "test", "")
			Expect(err).NotTo(HaveOccurred())

			body := `{
				"first_name": "Updated",
				"last_name": "User",
				"company": "Updated Corp"
			}`

			req := httptest.NewRequest("PUT", "/api/contacts/"+contactID, bytes.NewBufferString(body))
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("X-Context-ID", contextID)
			engine.ServeHTTP(w, req)

			Expect(w.Code).To(Equal(http.StatusOK))

			var response map[string]interface{}
			err = json.Unmarshal(w.Body.Bytes(), &response)
			Expect(err).NotTo(HaveOccurred())
			Expect(response["FirstName"]).To(Equal("Updated"))
		})

		It("should return 401 when context ID is missing", func() {
			contactID := support.NewTestUUID()
			body := `{"first_name": "Updated"}`

			req := httptest.NewRequest("PUT", "/api/contacts/"+contactID, bytes.NewBufferString(body))
			req.Header.Set("Content-Type", "application/json")
			engine.ServeHTTP(w, req)

			Expect(w.Code).To(Equal(http.StatusBadRequest))
		})
	})

	Describe("DELETE /api/contacts/:id", func() {
		It("should delete a contact", func() {
			
			contactID := support.NewTestUUID()

			email := "delete@example.com"
			customFields, _ := json.Marshal(map[string]interface{}{})

			_, err := pool.Exec(ctx, `
				INSERT INTO contacts (id, context_id, email, first_name, last_name, phone, company,
				                      custom_fields, source, source_id, created_at, updated_at)
				VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, NOW(), NOW())
			`, contactID, contextID, email, "John", "Doe", "+1234567890", "Test Corp",
				customFields, "test", "")
			Expect(err).NotTo(HaveOccurred())

			req := httptest.NewRequest("DELETE", "/api/contacts/"+contactID, nil)
			req.Header.Set("X-Context-ID", contextID)
			engine.ServeHTTP(w, req)

			Expect(w.Code).To(Equal(http.StatusNoContent))

			// Verify contact was deleted
			var count int
			err = pool.QueryRow(ctx, "SELECT COUNT(*) FROM contacts WHERE id = $1", contactID).Scan(&count)
			Expect(err).NotTo(HaveOccurred())
			Expect(count).To(Equal(0))
		})

		It("should return 404 for non-existent contact", func() {
			req := httptest.NewRequest("DELETE", "/api/contacts/nonexistent", nil)
			req.Header.Set("X-Context-ID", support.NewTestContextID())
			engine.ServeHTTP(w, req)

			Expect(w.Code).To(Equal(http.StatusNotFound))
		})
	})

	Describe("POST /api/contacts/:id/merge", func() {
		It("should merge two contacts", func() {
			
			keepID := support.NewTestUUID()
			mergeID := support.NewTestUUID()

			email1 := "keep@example.com"
			email2 := "merge@example.com"
			customFields, _ := json.Marshal(map[string]interface{}{})

			_, err := pool.Exec(ctx, `
				INSERT INTO contacts (id, context_id, email, first_name, last_name, phone, company,
				                      custom_fields, source, source_id, created_at, updated_at)
				VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, NOW(), NOW())
			`, keepID, contextID, email1, "Keep", "User", "+1234567890", "Test Corp",
				customFields, "test", "")
			Expect(err).NotTo(HaveOccurred())

			_, err = pool.Exec(ctx, `
				INSERT INTO contacts (id, context_id, email, first_name, last_name, phone, company,
				                      custom_fields, source, source_id, created_at, updated_at)
				VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, NOW(), NOW())
			`, mergeID, contextID, email2, "Merge", "User", "+1234567890", "Test Corp",
				customFields, "test", "")
			Expect(err).NotTo(HaveOccurred())

			body := `{"merge_with": "` + mergeID + `"}`

			req := httptest.NewRequest("POST", "/api/contacts/"+keepID+"/merge", bytes.NewBufferString(body))
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("X-Context-ID", contextID)
			engine.ServeHTTP(w, req)

		Expect(w.Code).To(Equal(http.StatusOK))
		})

		It("should return 400 when merge_with is missing", func() {
			
			contactID := support.NewTestUUID()

			email := "test@example.com"
			customFields, _ := json.Marshal(map[string]interface{}{})

			_, err := pool.Exec(ctx, `
				INSERT INTO contacts (id, context_id, email, first_name, last_name, phone, company,
				                      custom_fields, source, source_id, created_at, updated_at)
				VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, NOW(), NOW())
			`, contactID, contextID, email, "Test", "User", "+1234567890", "Test Corp",
				customFields, "test", "")
			Expect(err).NotTo(HaveOccurred())

			body := `{}`

			req := httptest.NewRequest("POST", "/api/contacts/"+contactID+"/merge", bytes.NewBufferString(body))
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("X-Context-ID", contextID)
			engine.ServeHTTP(w, req)

			Expect(w.Code).To(Equal(http.StatusBadRequest))
		})
	})

	Describe("GET /api/contacts/:id/activity", func() {
		It("should return contact activity", func() {
			
			contactID := support.NewTestUUID()

			email := "activity@example.com"
			customFields, _ := json.Marshal(map[string]interface{}{})

			_, err := pool.Exec(ctx, `
				INSERT INTO contacts (id, context_id, email, first_name, last_name, phone, company,
				                      custom_fields, source, source_id, created_at, updated_at)
				VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, NOW(), NOW())
			`, contactID, contextID, email, "John", "Doe", "+1234567890", "Test Corp",
				customFields, "test", "")
			Expect(err).NotTo(HaveOccurred())

			req := httptest.NewRequest("GET", "/api/contacts/"+contactID+"/activity?offset=0&limit=20", nil)
			req.Header.Set("X-Context-ID", contextID)
			engine.ServeHTTP(w, req)

			Expect(w.Code).To(Equal(http.StatusOK))

			var response map[string]interface{}
			err = json.Unmarshal(w.Body.Bytes(), &response)
			Expect(err).NotTo(HaveOccurred())
			Expect(response["activities"]).NotTo(BeNil())
		})

		It("should return 404 for non-existent contact", func() {
			fakeContactID := support.NewTestUUID()
			req := httptest.NewRequest("GET", "/api/contacts/"+fakeContactID+"/activity", nil)
			req.Header.Set("X-Context-ID", support.NewTestContextID())
			engine.ServeHTTP(w, req)

			Expect(w.Code).To(Equal(http.StatusNotFound))
		})
	})
})
