package cicd_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"

	"github.com/gin-gonic/gin"
	"github.com/hekemen/automata/cicd/support"
	bhandler "github.com/hekemen/automata/internal/adapter/api/handler"
	ctxrepo "github.com/hekemen/automata/internal/infrastructure/context/repo"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func setupContextEngine() *gin.Engine {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	contextRepo := ctxrepo.NewPostgresRepo(pool)
	contextHandler := bhandler.NewContextHandler(contextRepo)

	contexts := engine.Group("/api/admin/contexts")
	{
		contexts.GET("", contextHandler.List)
		contexts.POST("", contextHandler.Create)
		contexts.GET("/:id", contextHandler.Get)
		contexts.PUT("/:id", contextHandler.Update)
		contexts.DELETE("/:id", contextHandler.Delete)
	}

	return engine
}

var _ = Describe("HTTP Integration Tests - Context CRUD", func() {
	var (
		w      *httptest.ResponseRecorder
		engine *gin.Engine
	)

	BeforeEach(func() {
		engine = setupContextEngine()
		w = httptest.NewRecorder()
	})

	AfterEach(func() {
		_, _ = pool.Exec(ctx, "DELETE FROM contexts")
	})

	Describe("POST /api/admin/contexts", func() {
		It("should create a context with valid request", func() {
			body := `{
				"slug": "test-context",
				"name": "Test Context",
				"domain": "test.example.com",
				"settings": {}
			}`

			req := httptest.NewRequest("POST", "/api/admin/contexts", bytes.NewBufferString(body))
			req.Header.Set("Content-Type", "application/json")
			engine.ServeHTTP(w, req)

			Expect(w.Code).To(Equal(http.StatusCreated))

			var response map[string]interface{}
			err := json.Unmarshal(w.Body.Bytes(), &response)
			Expect(err).NotTo(HaveOccurred())
			Expect(response["Slug"]).To(Equal("test-context"))
			Expect(response["Name"]).To(Equal("Test Context"))
		})

		It("should return 400 when slug is missing", func() {
			body := `{"name": "Test Context"}`

			req := httptest.NewRequest("POST", "/api/admin/contexts", bytes.NewBufferString(body))
			req.Header.Set("Content-Type", "application/json")
			engine.ServeHTTP(w, req)

			Expect(w.Code).To(Equal(http.StatusBadRequest))
		})

		It("should return 400 when name is missing", func() {
			body := `{"slug": "test-context"}`

			req := httptest.NewRequest("POST", "/api/admin/contexts", bytes.NewBufferString(body))
			req.Header.Set("Content-Type", "application/json")
			engine.ServeHTTP(w, req)

			Expect(w.Code).To(Equal(http.StatusBadRequest))
		})

		It("should return 400 for invalid JSON", func() {
			body := `{invalid json}`

			req := httptest.NewRequest("POST", "/api/admin/contexts", bytes.NewBufferString(body))
			req.Header.Set("Content-Type", "application/json")
			engine.ServeHTTP(w, req)

			Expect(w.Code).To(Equal(http.StatusBadRequest))
		})

		It("should lowercase the slug automatically", func() {
			body := `{
				"slug": "UPPERCASE-Context",
				"name": "Test Context"
			}`

			req := httptest.NewRequest("POST", "/api/admin/contexts", bytes.NewBufferString(body))
			req.Header.Set("Content-Type", "application/json")
			engine.ServeHTTP(w, req)

			Expect(w.Code).To(Equal(http.StatusCreated))

			var response map[string]interface{}
			err := json.Unmarshal(w.Body.Bytes(), &response)
			Expect(err).NotTo(HaveOccurred())
			Expect(response["Slug"]).To(Equal("uppercase-context"))
		})
	})

	Describe("GET /api/admin/contexts", func() {
		It("should list contexts with pagination", func() {
			contextID := support.NewTestContextID()

			_, err := pool.Exec(ctx, `
				INSERT INTO contexts (id, slug, name, domain, is_active, settings, created_at, updated_at)
				VALUES ($1, $2, $3, $4, $5, $6, NOW(), NOW())
			`, contextID, "list-context", "List Context", "list.example.com", true, "{}")
			Expect(err).NotTo(HaveOccurred())

			req := httptest.NewRequest("GET", "/api/admin/contexts?offset=0&limit=20", nil)
			engine.ServeHTTP(w, req)

			Expect(w.Code).To(Equal(http.StatusOK))

			var response map[string]interface{}
			err = json.Unmarshal(w.Body.Bytes(), &response)
			Expect(err).NotTo(HaveOccurred())
			Expect(response["contexts"]).NotTo(BeNil())
			Expect(response["count"]).NotTo(BeNil())
		})

		It("should return empty list when no contexts exist", func() {
			req := httptest.NewRequest("GET", "/api/admin/contexts", nil)
			engine.ServeHTTP(w, req)

			Expect(w.Code).To(Equal(http.StatusOK))
		})

		It("should handle pagination parameters", func() {
			req := httptest.NewRequest("GET", "/api/admin/contexts?offset=0&limit=5", nil)
			engine.ServeHTTP(w, req)

			Expect(w.Code).To(Equal(http.StatusOK))
		})
	})

	Describe("GET /api/admin/contexts/:id", func() {
		It("should return a context by ID", func() {
			contextID := support.NewTestContextID()

			_, err := pool.Exec(ctx, `
				INSERT INTO contexts (id, slug, name, domain, is_active, settings, created_at, updated_at)
				VALUES ($1, $2, $3, $4, $5, $6, NOW(), NOW())
			`, contextID, "get-context", "Get Context", "get.example.com", true, "{}")
			Expect(err).NotTo(HaveOccurred())

			req := httptest.NewRequest("GET", "/api/admin/contexts/"+contextID, nil)
			engine.ServeHTTP(w, req)

			Expect(w.Code).To(Equal(http.StatusOK))

			var response map[string]interface{}
			err = json.Unmarshal(w.Body.Bytes(), &response)
			Expect(err).NotTo(HaveOccurred())
			Expect(response["ID"]).To(Equal(contextID))
			Expect(response["Slug"]).To(Equal("get-context"))
		})

		It("should return 404 for non-existent context", func() {
			req := httptest.NewRequest("GET", "/api/admin/contexts/nonexistent", nil)
			engine.ServeHTTP(w, req)

			Expect(w.Code).To(Equal(http.StatusNotFound))
		})
	})

	Describe("PUT /api/admin/contexts/:id", func() {
		It("should update a context", func() {
			contextID := support.NewTestContextID()

			_, err := pool.Exec(ctx, `
				INSERT INTO contexts (id, slug, name, domain, is_active, settings, created_at, updated_at)
				VALUES ($1, $2, $3, $4, $5, $6, NOW(), NOW())
			`, contextID, "old-slug", "Old Name", "old.example.com", true, "{}")
			Expect(err).NotTo(HaveOccurred())

			body := `{
				"name": "Updated Name",
				"domain": "updated.example.com",
				"is_active": false
			}`

			req := httptest.NewRequest("PUT", "/api/admin/contexts/"+contextID, bytes.NewBufferString(body))
			req.Header.Set("Content-Type", "application/json")
			engine.ServeHTTP(w, req)

			Expect(w.Code).To(Equal(http.StatusOK))

			var response map[string]interface{}
			err = json.Unmarshal(w.Body.Bytes(), &response)
			Expect(err).NotTo(HaveOccurred())
			Expect(response["Name"]).To(Equal("Updated Name"))
		})

		It("should return 400 for invalid JSON", func() {
			contextID := support.NewTestContextID()

			_, err := pool.Exec(ctx, `
				INSERT INTO contexts (id, slug, name, domain, is_active, settings, created_at, updated_at)
				VALUES ($1, $2, $3, $4, $5, $6, NOW(), NOW())
			`, contextID, "test-slug", "Test Name", "test.example.com", true, "{}")
			Expect(err).NotTo(HaveOccurred())

			body := `{invalid json}`

			req := httptest.NewRequest("PUT", "/api/admin/contexts/"+contextID, bytes.NewBufferString(body))
			req.Header.Set("Content-Type", "application/json")
			engine.ServeHTTP(w, req)

			Expect(w.Code).To(Equal(http.StatusBadRequest))
		})

		It("should return 404 for non-existent context", func() {
			body := `{"name": "Updated"}`

			req := httptest.NewRequest("PUT", "/api/admin/contexts/nonexistent", bytes.NewBufferString(body))
			req.Header.Set("Content-Type", "application/json")
			engine.ServeHTTP(w, req)

			Expect(w.Code).To(Equal(http.StatusNotFound))
		})
	})

	Describe("DELETE /api/admin/contexts/:id", func() {
		It("should delete a context", func() {
			contextID := support.NewTestContextID()

			_, err := pool.Exec(ctx, `
				INSERT INTO contexts (id, slug, name, domain, is_active, settings, created_at, updated_at)
				VALUES ($1, $2, $3, $4, $5, $6, NOW(), NOW())
			`, contextID, "delete-context", "Delete Context", "delete.example.com", true, "{}")
			Expect(err).NotTo(HaveOccurred())

			req := httptest.NewRequest("DELETE", "/api/admin/contexts/"+contextID, nil)
			engine.ServeHTTP(w, req)

			Expect(w.Code).To(Equal(http.StatusOK))

			// Verify context was deleted
			var count int
			err = pool.QueryRow(ctx, "SELECT COUNT(*) FROM contexts WHERE id = $1", contextID).Scan(&count)
			Expect(err).NotTo(HaveOccurred())
			Expect(count).To(Equal(0))
		})

		It("should return 404 for non-existent context", func() {
			req := httptest.NewRequest("DELETE", "/api/admin/contexts/nonexistent", nil)
			engine.ServeHTTP(w, req)

			Expect(w.Code).To(Equal(http.StatusNotFound))
		})
	})
})
