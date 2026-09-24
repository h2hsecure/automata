package proxy

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/hekemen/automata/internal/domain/context"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Proxy Adapter", func() {
	var (
		engine   *gin.Engine
		proxyObj *Proxy
	)

	BeforeEach(func() {
		gin.SetMode(gin.TestMode)
		engine = gin.New()
		proxyObj = NewProxy(&mockTenantRepo{}, engine)
	})

	Describe("Form route", func() {
		It("returns 400 when context is not in context", func() {
			req := httptest.NewRequest("GET", "/form/test-form", nil)
			w := httptest.NewRecorder()
			engine.ServeHTTP(w, req)

			Expect(w.Code).To(Equal(http.StatusBadRequest))
			Expect(w.Body.String()).To(ContainSubstring("context not found"))
		})

		It("returns HTML with context name when context is in context", func() {
			// Register a middleware that sets the context before the handler
			engine.Use(func(c *gin.Context) {
				t := &context.Context{ID: "550e8400-e29b-41d4-a716-446655440000", Name: "My App", Slug: "myapp"}
				c.Set("context", t)
				c.Next()
			})

			// Create a fresh engine with the middleware to avoid the 400 test polluting
			freshEngine := gin.New()
			freshEngine.Use(func(c *gin.Context) {
				t := &context.Context{ID: "550e8400-e29b-41d4-a716-446655440000", Name: "My App", Slug: "myapp"}
				c.Set("context", t)
				c.Next()
			})
			proxyObj = NewProxy(&mockTenantRepo{}, freshEngine)

			req := httptest.NewRequest("GET", "/form/my-form", nil)
			w := httptest.NewRecorder()
			freshEngine.ServeHTTP(w, req)

			Expect(w.Code).To(Equal(http.StatusOK))
			Expect(w.Header().Get("Content-Type")).To(ContainSubstring("text/html"))
			Expect(w.Body.String()).To(ContainSubstring("My App"))
		})
	})

	Describe("Snippet route", func() {
		It("returns JavaScript for a context ID", func() {
			req := httptest.NewRequest("GET", "/snippet/abc123.js", nil)
			w := httptest.NewRecorder()
			engine.ServeHTTP(w, req)

			Expect(w.Code).To(Equal(http.StatusOK))
			Expect(w.Header().Get("Content-Type")).To(ContainSubstring("application/javascript"))
			Expect(w.Body.String()).To(ContainSubstring("abc123"))
		})

		It("strips .js extension from context ID in snippet", func() {
			req := httptest.NewRequest("GET", "/snippet/context-123.js", nil)
			w := httptest.NewRecorder()
			engine.ServeHTTP(w, req)

			Expect(w.Code).To(Equal(http.StatusOK))
			Expect(w.Body.String()).To(ContainSubstring("context-123"))
			// Should not contain .js in the tracking ID
			Expect(w.Body.String()).NotTo(ContainSubstring("'context-123.js'"))
		})
	})

	Describe("Static assets route", func() {
		It("returns 404 for non-existent static files", func() {
			req := httptest.NewRequest("GET", "/static/nonexistent.css", nil)
			w := httptest.NewRecorder()
			engine.ServeHTTP(w, req)

			// Should return 404 for non-existent files, not panic
			Expect(w.Code).To(Equal(http.StatusNotFound))
		})
	})

	Describe("generateSnippet", func() {
		It("generates a valid tracking snippet", func() {
			snippet := generateSnippet("test-context-id")
			Expect(snippet).To(ContainSubstring("test-context-id"))
			Expect(snippet).To(ContainSubstring("automata.example.com"))
			Expect(snippet).To(ContainSubstring("track.js"))
			Expect(snippet).To(ContainSubstring("dataLayer"))
		})
	})

	_ = proxyObj // avoid unused variable warning
})

// mockTenantRepo implements context.Repository for testing
type mockTenantRepo struct{}

func (m *mockTenantRepo) Create(t *context.Context) error {
	t.ID = "550e8400-e29b-41d4-a716-446655440000"
	return nil
}
func (m *mockTenantRepo) GetByID(id string) (*context.Context, error) {
	return nil, nil
}
func (m *mockTenantRepo) GetBySlug(slug string) (*context.Context, error) {
	return nil, nil
}
func (m *mockTenantRepo) List(offset, limit int) ([]*context.Context, error) {
	return nil, nil
}
func (m *mockTenantRepo) Update(t *context.Context) error {
	return nil
}
func (m *mockTenantRepo) Delete(id string) error {
	return nil
}

func TestProxy(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Proxy Adapter Suite")
}
