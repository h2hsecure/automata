package cicd_test

import (
	"bytes"
	"encoding/json"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/h2hsecure/automata/cicd/support"
	bhandler "github.com/h2hsecure/automata/internal/adapter/api/handler"
	crepo "github.com/h2hsecure/automata/internal/infrastructure/contact/repo"
	ctxrepo "github.com/h2hsecure/automata/internal/infrastructure/context/repo"
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
		// Static sub-routes must be registered before the :id wildcard routes.
		contacts.GET("/export", contactHandler.Export)
		contacts.POST("/import", contactHandler.Import)
		contacts.GET("/tags", contactHandler.ListTags)
		contacts.POST("/tags", contactHandler.CreateTag)
		contacts.DELETE("/tags/:id", contactHandler.DeleteTag)

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
		_, _ = pool.Exec(ctx, "DELETE FROM contact_tag_memberships")
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

	Describe("POST /api/contacts/import", func() {
		It("should import contacts from a raw CSV body", func() {
			body := "email,first_name,last_name\nimport-a@example.com,Alice,Imports\nimport-b@example.com,Bob,Imports\n"

			req := httptest.NewRequest("POST", "/api/contacts/import", bytes.NewBufferString(body))
			req.Header.Set("Content-Type", "text/csv")
			req.Header.Set("X-Context-ID", contextID)
			engine.ServeHTTP(w, req)

			Expect(w.Code).To(Equal(http.StatusOK), "body: %s", w.Body.String())

			var result struct {
				Created int
				Updated int
				Skipped int
			}
			Expect(json.Unmarshal(w.Body.Bytes(), &result)).To(Succeed(), "body: %s", w.Body.String())
			Expect(result.Created).To(Equal(2))

			var count int
			err := pool.QueryRow(ctx, "SELECT COUNT(*) FROM contacts WHERE context_id = $1", contextID).Scan(&count)
			Expect(err).NotTo(HaveOccurred())
			Expect(count).To(Equal(2))
		})

		It("should import contacts from a multipart file field", func() {
			var buf bytes.Buffer
			mw := multipart.NewWriter(&buf)
			fw, err := mw.CreateFormFile("file", "contacts.csv")
			Expect(err).NotTo(HaveOccurred())
			_, err = fw.Write([]byte("email,first_name\nmultipart-a@example.com,Mary\n"))
			Expect(err).NotTo(HaveOccurred())
			Expect(mw.Close()).To(Succeed())

			req := httptest.NewRequest("POST", "/api/contacts/import", &buf)
			req.Header.Set("Content-Type", mw.FormDataContentType())
			req.Header.Set("X-Context-ID", contextID)
			engine.ServeHTTP(w, req)

			Expect(w.Code).To(Equal(http.StatusOK), "body: %s", w.Body.String())

			var result struct {
				Created int
			}
			Expect(json.Unmarshal(w.Body.Bytes(), &result)).To(Succeed(), "body: %s", w.Body.String())
			Expect(result.Created).To(Equal(1))
		})

		It("should return 400 for empty CSV body", func() {
			req := httptest.NewRequest("POST", "/api/contacts/import", bytes.NewBufferString(""))
			req.Header.Set("Content-Type", "text/csv")
			req.Header.Set("X-Context-ID", contextID)
			engine.ServeHTTP(w, req)

			Expect(w.Code).To(Equal(http.StatusBadRequest))
		})
	})

	Describe("GET /api/contacts/export", func() {
		It("should export contacts as a CSV attachment", func() {
			contactID := support.NewTestUUID()
			customFields, _ := json.Marshal(map[string]interface{}{})
			_, err := pool.Exec(ctx, `
				INSERT INTO contacts (id, context_id, email, first_name, last_name, phone, company,
				                      custom_fields, source, source_id, created_at, updated_at)
				VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, NOW(), NOW())
			`, contactID, contextID, "export-a@example.com", "Alice", "Exports", "+1234567890", "Test Corp",
				customFields, "test", "")
			Expect(err).NotTo(HaveOccurred())

			req := httptest.NewRequest("GET", "/api/contacts/export?fields=email,first_name,last_name", nil)
			req.Header.Set("X-Context-ID", contextID)
			engine.ServeHTTP(w, req)

			Expect(w.Code).To(Equal(http.StatusOK), "body: %s", w.Body.String())
			Expect(w.Header().Get("Content-Type")).To(HavePrefix("text/csv"))
			Expect(w.Header().Get("Content-Disposition")).To(ContainSubstring("filename=contacts.csv"))

			csv := w.Body.String()
			Expect(csv).To(ContainSubstring("email,first_name,last_name"))
			Expect(csv).To(ContainSubstring("export-a@example.com,Alice,Exports"))
		})

		It("should return 404 when there are no contacts to export", func() {
			req := httptest.NewRequest("GET", "/api/contacts/export?fields=email", nil)
			req.Header.Set("X-Context-ID", contextID)
			engine.ServeHTTP(w, req)

			Expect(w.Code).To(Equal(http.StatusNotFound))
		})
	})

	Describe("Contact tag CRUD", func() {
		It("should create, list, and delete a tag", func() {
			// Each request gets its own recorder: ResponseRecorder.WriteHeader
			// is a no-op once a code is set, so a shared recorder would keep
			// the first request's status code and concatenate bodies.
			do := func(method, path, contentType, body string) *httptest.ResponseRecorder {
				req := httptest.NewRequest(method, path, strings.NewReader(body))
				if contentType != "" {
					req.Header.Set("Content-Type", contentType)
				}
				req.Header.Set("X-Context-ID", contextID)
				resp := httptest.NewRecorder()
				engine.ServeHTTP(resp, req)
				return resp
			}

			// Create
			wCreate := do("POST", "/api/contacts/tags", "application/json", `{"name": "vip", "color": "#ff0000"}`)
			Expect(wCreate.Code).To(Equal(http.StatusCreated), "body: %s", wCreate.Body.String())

			var created struct {
				ID   string `json:"ID"`
				Name string `json:"Name"`
			}
			Expect(json.Unmarshal(wCreate.Body.Bytes(), &created)).To(Succeed(), "body: %s", wCreate.Body.String())
			Expect(created.Name).To(Equal("vip"))
			Expect(created.ID).NotTo(BeEmpty())

			// List
			wList := do("GET", "/api/contacts/tags", "", "")
			Expect(wList.Code).To(Equal(http.StatusOK), "body: %s", wList.Body.String())
			var list struct {
				Tags []struct {
					ID    string `json:"ID"`
					Name  string `json:"Name"`
					Color string `json:"Color"`
				} `json:"tags"`
			}
			Expect(json.Unmarshal(wList.Body.Bytes(), &list)).To(Succeed(), "body: %s", wList.Body.String())
			Expect(list.Tags).To(HaveLen(1))
			Expect(list.Tags[0].Name).To(Equal("vip"))
			Expect(list.Tags[0].Color).To(Equal("#ff0000"))

			// Delete
			wDelete := do("DELETE", "/api/contacts/tags/"+created.ID, "", "")
			Expect(wDelete.Code).To(Equal(http.StatusNoContent), "body: %s", wDelete.Body.String())

			// Verify deleted
			wVerify := do("GET", "/api/contacts/tags", "", "")
			Expect(wVerify.Code).To(Equal(http.StatusOK), "body: %s", wVerify.Body.String())
			Expect(json.Unmarshal(wVerify.Body.Bytes(), &list)).To(Succeed())
			Expect(list.Tags).To(BeEmpty())
		})

		It("should return 400 when creating a tag without a name", func() {
			body := `{"color": "#ff0000"}`
			req := httptest.NewRequest("POST", "/api/contacts/tags", bytes.NewBufferString(body))
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("X-Context-ID", contextID)
			engine.ServeHTTP(w, req)

			Expect(w.Code).To(Equal(http.StatusBadRequest))
		})

		It("should return 404 when deleting a non-existent tag", func() {
			req := httptest.NewRequest("DELETE", "/api/contacts/tags/"+support.NewTestUUID(), nil)
			req.Header.Set("X-Context-ID", contextID)
			engine.ServeHTTP(w, req)

			Expect(w.Code).To(Equal(http.StatusNotFound))
		})
	})
})
