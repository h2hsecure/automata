package cicd_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"

	"github.com/gin-gonic/gin"
	"github.com/hekemen/automata/cicd/support"
	bhandler "github.com/hekemen/automata/internal/adapter/api/handler"
	formRepo "github.com/hekemen/automata/internal/infrastructure/form/repo"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func setupFormEngine() *gin.Engine {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	formRepo := formRepo.New(pool)
	formHandler := bhandler.NewFormHandler(formRepo)

	forms := engine.Group("/form")
	{
		forms.GET("/list", formHandler.List)
		forms.GET("/:id", formHandler.Get)
		forms.POST("/submit/:slug", formHandler.SubmitForm)
		forms.GET("/:id/submissions", formHandler.ListSubmissions)
	}

	return engine
}

var _ = Describe("HTTP Integration Tests - Form Endpoints", func() {
	var (
		w      *httptest.ResponseRecorder
		engine *gin.Engine
	)

	BeforeEach(func() {
		engine = setupFormEngine()
		w = httptest.NewRecorder()
	})

	AfterEach(func() {
		_, _ = pool.Exec(ctx, "DELETE FROM form_submissions")
		_, _ = pool.Exec(ctx, "DELETE FROM forms")
	})

	Describe("GET /form/list", func() {
		It("should list forms for a context", func() {
			contextID := support.NewTestContextID()
			formID := support.NewTestUUID()

			fields, _ := json.Marshal([]map[string]interface{}{{"slug": "email", "type": "email", "required": true}})
			settings, _ := json.Marshal(map[string]interface{}{})

			_, err := pool.Exec(ctx, `
				INSERT INTO forms (id, context_id, slug, name, description, fields, settings, created_at, updated_at)
				VALUES ($1, $2, $3, $4, $5, $6, $7, NOW(), NOW())
			`, formID, contextID, "test-form", "Test Form", "A test form", fields, settings)
			Expect(err).NotTo(HaveOccurred())

			req := httptest.NewRequest("GET", "/form/list", nil)
			req.Header.Set("X-Context-ID", contextID)
			engine.ServeHTTP(w, req)

			Expect(w.Code).To(Equal(http.StatusOK))

			var response map[string]interface{}
			err = json.Unmarshal(w.Body.Bytes(), &response)
			Expect(err).NotTo(HaveOccurred())
			Expect(response["forms"]).NotTo(BeNil())
		})

		It("should return 401 when context ID is missing", func() {
			req := httptest.NewRequest("GET", "/form/list", nil)
			engine.ServeHTTP(w, req)

			Expect(w.Code).To(Equal(http.StatusUnauthorized))
		})

		It("should return empty list when no forms exist", func() {
			req := httptest.NewRequest("GET", "/form/list", nil)
			req.Header.Set("X-Context-ID", support.NewTestContextID())
			engine.ServeHTTP(w, req)

			Expect(w.Code).To(Equal(http.StatusOK))
		})
	})

	Describe("GET /form/:id", func() {
		It("should return a form by ID", func() {
			contextID := support.NewTestContextID()
			formID := support.NewTestUUID()

			fields, _ := json.Marshal([]map[string]interface{}{{"slug": "email", "type": "email", "required": true}})
			settings, _ := json.Marshal(map[string]interface{}{})

			_, err := pool.Exec(ctx, `
				INSERT INTO forms (id, context_id, slug, name, description, fields, settings, created_at, updated_at)
				VALUES ($1, $2, $3, $4, $5, $6, $7, NOW(), NOW())
			`, formID, contextID, "test-form", "Test Form", "A test form", fields, settings)
			Expect(err).NotTo(HaveOccurred())

			req := httptest.NewRequest("GET", "/form/"+formID, nil)
			req.Header.Set("X-Context-ID", contextID)
			engine.ServeHTTP(w, req)

			Expect(w.Code).To(Equal(http.StatusOK))

			var response map[string]interface{}
			err = json.Unmarshal(w.Body.Bytes(), &response)
			Expect(err).NotTo(HaveOccurred())
			Expect(response["ID"]).To(Equal(formID))
		})

		It("should return 404 for non-existent form", func() {
			req := httptest.NewRequest("GET", "/form/nonexistent", nil)
			req.Header.Set("X-Context-ID", support.NewTestContextID())
			engine.ServeHTTP(w, req)

			Expect(w.Code).To(Equal(http.StatusNotFound))
		})

		It("should return 401 when context ID is missing", func() {
			req := httptest.NewRequest("GET", "/form/some-id", nil)
			engine.ServeHTTP(w, req)

			Expect(w.Code).To(Equal(http.StatusUnauthorized))
		})
	})

	Describe("POST /form/submit/:slug", func() {
		It("should submit a form with valid data", func() {
			contextID := support.NewTestContextID()
			formID := support.NewTestUUID()

			fields, _ := json.Marshal([]map[string]interface{}{{"slug": "email", "type": "email", "required": true}})
			settings, _ := json.Marshal(map[string]interface{}{})

			_, err := pool.Exec(ctx, `
				INSERT INTO forms (id, context_id, slug, name, description, fields, settings, created_at, updated_at)
				VALUES ($1, $2, $3, $4, $5, $6, $7, NOW(), NOW())
			`, formID, contextID, "test-form", "Test Form", "A test form", fields, settings)
			Expect(err).NotTo(HaveOccurred())

			body := `{"data": {"email": "user@example.com"}}`

			req := httptest.NewRequest("POST", "/form/submit/test-form", bytes.NewBufferString(body))
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("X-Context-ID", contextID)
			engine.ServeHTTP(w, req)

			Expect(w.Code).To(Equal(http.StatusCreated))

			var response map[string]interface{}
			err = json.Unmarshal(w.Body.Bytes(), &response)
			Expect(err).NotTo(HaveOccurred())
			Expect(response["ID"]).NotTo(BeNil())
		})

		It("should return 404 when slug is missing", func() {
			body := `{"data": {"email": "user@example.com"}}`

			req := httptest.NewRequest("POST", "/form/submit/", bytes.NewBufferString(body))
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("X-Context-ID", support.NewTestContextID())
			engine.ServeHTTP(w, req)

			Expect(w.Code).To(Equal(http.StatusNotFound))
		})

		It("should return 401 when context ID is missing", func() {
			body := `{"data": {"email": "user@example.com"}}`

			req := httptest.NewRequest("POST", "/form/submit/test-form", bytes.NewBufferString(body))
			req.Header.Set("Content-Type", "application/json")
			engine.ServeHTTP(w, req)

			Expect(w.Code).To(Equal(http.StatusUnauthorized))
		})

		It("should handle form submission via query params", func() {
			contextID := support.NewTestContextID()
			formID := support.NewTestUUID()

			fields, _ := json.Marshal([]map[string]interface{}{{"slug": "email", "type": "email", "required": true}})
			settings, _ := json.Marshal(map[string]interface{}{})

			_, err := pool.Exec(ctx, `
				INSERT INTO forms (id, context_id, slug, name, description, fields, settings, created_at, updated_at)
				VALUES ($1, $2, $3, $4, $5, $6, $7, NOW(), NOW())
			`, formID, contextID, "test-form", "Test Form", "A test form", fields, settings)
			Expect(err).NotTo(HaveOccurred())

			body := `{"data": {"email": "user@example.com"}}`

			req := httptest.NewRequest("POST", "/form/submit/test-form?slug=test-form", bytes.NewBufferString(body))
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("X-Context-ID", contextID)
			engine.ServeHTTP(w, req)

			Expect(w.Code).To(Equal(http.StatusCreated))
		})
	})

	Describe("GET /form/:id/submissions", func() {
		It("should list submissions for a form", func() {
			contextID := support.NewTestContextID()
			formID := support.NewTestUUID()
			submissionID := support.NewTestUUID()

			fields, _ := json.Marshal([]map[string]interface{}{{"slug": "email", "type": "email", "required": true}})
			settings, _ := json.Marshal(map[string]interface{}{})

			_, err := pool.Exec(ctx, `
				INSERT INTO forms (id, context_id, slug, name, description, fields, settings, created_at, updated_at)
				VALUES ($1, $2, $3, $4, $5, $6, $7, NOW(), NOW())
			`, formID, contextID, "test-form", "Test Form", "A test form", fields, settings)
			Expect(err).NotTo(HaveOccurred())

			data, _ := json.Marshal(map[string]interface{}{"email": "user@example.com"})
			files, _ := json.Marshal([]map[string]interface{}{})

			_, err = pool.Exec(ctx, `
				INSERT INTO form_submissions (id, form_id, context_id, data, files, created_at)
				VALUES ($1, $2, $3, $4, $5, NOW())
			`, submissionID, formID, contextID, data, files)
			Expect(err).NotTo(HaveOccurred())

			req := httptest.NewRequest("GET", "/form/"+formID+"/submissions?page=1&limit=20", nil)
			req.Header.Set("X-Context-ID", contextID)
			engine.ServeHTTP(w, req)

			Expect(w.Code).To(Equal(http.StatusOK))

			var response map[string]interface{}
			err = json.Unmarshal(w.Body.Bytes(), &response)
			Expect(err).NotTo(HaveOccurred())
			Expect(response["submissions"]).NotTo(BeNil())
			Expect(response["total"]).NotTo(BeNil())
			Expect(response["page"]).NotTo(BeNil())
		})

		It("should return 401 when context ID is missing", func() {
			req := httptest.NewRequest("GET", "/form/some-id/submissions", nil)
			engine.ServeHTTP(w, req)

			Expect(w.Code).To(Equal(http.StatusUnauthorized))
		})

		It("should handle pagination with custom limit", func() {
			contextID := support.NewTestContextID()
			formID := support.NewTestUUID()

			fields, _ := json.Marshal([]map[string]interface{}{{"slug": "email", "type": "email", "required": true}})
			settings, _ := json.Marshal(map[string]interface{}{})

			_, err := pool.Exec(ctx, `
				INSERT INTO forms (id, context_id, slug, name, description, fields, settings, created_at, updated_at)
				VALUES ($1, $2, $3, $4, $5, $6, $7, NOW(), NOW())
			`, formID, contextID, "test-form", "Test Form", "A test form", fields, settings)
			Expect(err).NotTo(HaveOccurred())

			req := httptest.NewRequest("GET", "/form/"+formID+"/submissions?page=1&limit=5", nil)
			req.Header.Set("X-Context-ID", contextID)
			engine.ServeHTTP(w, req)

			Expect(w.Code).To(Equal(http.StatusOK))
		})
	})
})
