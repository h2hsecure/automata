package api

import (
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/hekemen/automata/internal/domain/context"
	. 	"github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func strPtr(s string) *string { return &s }

// mockContextRepo is an in-memory implementation of context.Repository for testing.
type mockContextRepo struct {
	contexts map[string]*context.Context
}

func newMockContextRepo() *mockContextRepo {
	return &mockContextRepo{contexts: make(map[string]*context.Context)}
}

func (m *mockContextRepo) Create(t *context.Context) error {
	if t.ID == "" {
		t.ID = "550e8400-e29b-41d4-a716-446655440000"
	}
	m.contexts[t.Slug] = t
	return nil
}

func (m *mockContextRepo) GetByID(id string) (*context.Context, error) {
	for _, t := range m.contexts {
		if t.ID == id {
			return t, nil
		}
	}
	return nil, errors.New("context not found")
}

func (m *mockContextRepo) GetBySlug(slug string) (*context.Context, error) {
	t, ok := m.contexts[slug]
	if !ok {
		return nil, errors.New("context not found")
	}
	return t, nil
}

func (m *mockContextRepo) List(offset, limit int) ([]*context.Context, error) {
	var result []*context.Context
	for _, t := range m.contexts {
		result = append(result, t)
	}
	return result, nil
}

func (m *mockContextRepo) Update(t *context.Context) error {
	m.contexts[t.Slug] = t
	return nil
}

func (m *mockContextRepo) Delete(id string) error {
	for slug, t := range m.contexts {
		if t.ID == id {
			delete(m.contexts, slug)
			return nil
		}
	}
	return errors.New("context not found")
}

var _ context.Repository = (*mockContextRepo)(nil)

func TestContextMiddleware(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Context Middleware Suite")
}

// requestWithHost creates an HTTP request and sets the Host header.
func requestWithHost(method, path, host string) *http.Request {
	req := httptest.NewRequest(method, path, nil)
	req.Host = host
	return req
}

// performRequest executes a request against the Gin engine and returns the ResponseRecorder.
func performRequest(engine *gin.Engine, req *http.Request) *httptest.ResponseRecorder {
	w := httptest.NewRecorder()
	engine.ServeHTTP(w, req)
	return w
}

// tenantCapture holds the captured context from a middleware execution.
type tenantCapture struct {
	Context *context.Context
	Found   bool
}

// runWithCapture creates a fresh engine with the middleware and a capture handler,
// then executes the request to capture the context from context.
func runWithCapture(middleware gin.HandlerFunc, repo *mockContextRepo, req *http.Request) (*httptest.ResponseRecorder, *tenantCapture) {
	captured := &tenantCapture{}
	engine := gin.New()
	engine.Use(middleware)
	engine.GET(req.URL.Path, func(c *gin.Context) {
		val, exists := c.Get(string(ContextKey))
		captured.Found = exists
		if exists {
			if ctx, ok := val.(context.Context); ok {
				captured.Context = &ctx
			}
		}
		c.String(http.StatusOK, "ok")
	})
	w := performRequest(engine, req)
	return w, captured
}

var _ = Describe("ContextResolver middleware", func() {
	var (
		mockRepo   *mockContextRepo
		middleware gin.HandlerFunc
	)

	BeforeEach(func() {
		gin.SetMode(gin.TestMode)
		mockRepo = newMockContextRepo()

		// Seed test contexts
		mockRepo.contexts["context1"] = &context.Context{
			ID:        "550e8400-e29b-41d4-a716-446655440001",
			Slug:      "context1",
			Name:      "Context One",
			Domain:    strPtr("context1.example.com"),
			IsActive:  true,
			Settings:  map[string]interface{}{},
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		}
		mockRepo.contexts["myapp"] = &context.Context{
			ID:        "550e8400-e29b-41d4-a716-446655440002",
			Slug:      "myapp",
			Name:      "My App",
			Domain:    strPtr("myapp.example.com"),
			IsActive:  true,
			Settings:  map[string]interface{}{},
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		}
		mockRepo.contexts["acme"] = &context.Context{
			ID:        "550e8400-e29b-41d4-a716-446655440003",
			Slug:      "acme",
			Name:      "Acme Corp",
			Domain:    strPtr("acme.example.com"),
			IsActive:  true,
			Settings:  map[string]interface{}{},
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		}

		middleware = ContextResolver(mockRepo)
	})

	Describe("subdomain resolution", func() {
		It("resolves context from subdomain", func() {
			req := requestWithHost("GET", "/test", "context1.example.com")
			w, captured := runWithCapture(middleware, mockRepo, req)
			Expect(w.Code).To(Equal(http.StatusOK))
			Expect(captured.Found).To(BeTrue())
			Expect(captured.Context.Slug).To(Equal("context1"))
		})

		It("resolves context from subdomain with port", func() {
			req := requestWithHost("GET", "/test", "myapp.example.com:8080")
			w, captured := runWithCapture(middleware, mockRepo, req)
			Expect(w.Code).To(Equal(http.StatusOK))
			Expect(captured.Found).To(BeTrue())
			Expect(captured.Context.Slug).To(Equal("myapp"))
		})

		It("skips www subdomain", func() {
			req := requestWithHost("GET", "/test", "www.example.com")
			w, _ := runWithCapture(middleware, mockRepo, req)
			Expect(w.Code).To(Equal(http.StatusBadRequest))
			body, _ := io.ReadAll(w.Body)
			Expect(string(body)).To(ContainSubstring("no context found"))
		})

		It("skips api subdomain", func() {
			req := requestWithHost("GET", "/test", "api.example.com")
			w, _ := runWithCapture(middleware, mockRepo, req)
			Expect(w.Code).To(Equal(http.StatusBadRequest))
			body, _ := io.ReadAll(w.Body)
			Expect(string(body)).To(ContainSubstring("no context found"))
		})

		It("skips mail subdomain", func() {
			req := requestWithHost("GET", "/test", "mail.example.com")
			w, _ := runWithCapture(middleware, mockRepo, req)
			Expect(w.Code).To(Equal(http.StatusBadRequest))
			body, _ := io.ReadAll(w.Body)
			Expect(string(body)).To(ContainSubstring("no context found"))
		})

		It("returns error for non-existent subdomain", func() {
			req := requestWithHost("GET", "/test", "unknown.example.com")
			w, _ := runWithCapture(middleware, mockRepo, req)
			Expect(w.Code).To(Equal(http.StatusBadRequest))
			body, _ := io.ReadAll(w.Body)
			Expect(string(body)).To(ContainSubstring("no context found"))
		})

		It("returns error for no subdomain (single part host)", func() {
			req := requestWithHost("GET", "/test", "localhost")
			w, _ := runWithCapture(middleware, mockRepo, req)
			Expect(w.Code).To(Equal(http.StatusBadRequest))
			body, _ := io.ReadAll(w.Body)
			Expect(string(body)).To(ContainSubstring("no context found"))
		})
	})

	Describe("path-based resolution", func() {
		It("resolves context from /context/<slug>/... path", func() {
			req := requestWithHost("GET", "/context/acme/forms", "localhost")
			w, captured := runWithCapture(middleware, mockRepo, req)
			Expect(w.Code).To(Equal(http.StatusOK))
			Expect(captured.Found).To(BeTrue())
			Expect(captured.Context.Slug).To(Equal("acme"))
		})

		It("resolves context from /t/<slug>/... path", func() {
			req := requestWithHost("GET", "/t/acme/forms", "localhost")
			w, captured := runWithCapture(middleware, mockRepo, req)
			Expect(w.Code).To(Equal(http.StatusOK))
			Expect(captured.Found).To(BeTrue())
			Expect(captured.Context.Slug).To(Equal("acme"))
		})

		It("returns error for /context/ with empty slug", func() {
			req := requestWithHost("GET", "/context/", "localhost")
			w, _ := runWithCapture(middleware, mockRepo, req)
			Expect(w.Code).To(Equal(http.StatusBadRequest))
			body, _ := io.ReadAll(w.Body)
			Expect(string(body)).To(ContainSubstring("no context found"))
		})

		It("returns error for /t/ with empty slug", func() {
			req := requestWithHost("GET", "/t/", "localhost")
			w, _ := runWithCapture(middleware, mockRepo, req)
			Expect(w.Code).To(Equal(http.StatusBadRequest))
			body, _ := io.ReadAll(w.Body)
			Expect(string(body)).To(ContainSubstring("no context found"))
		})

		It("returns error for non-existent context in path", func() {
			req := requestWithHost("GET", "/context/nonexistent/forms", "localhost")
			w, _ := runWithCapture(middleware, mockRepo, req)
			Expect(w.Code).To(Equal(http.StatusBadRequest))
			body, _ := io.ReadAll(w.Body)
			Expect(string(body)).To(ContainSubstring("no context found"))
		})
	})

	Describe("skipped routes", func() {
		It("skips /admin routes", func() {
			req := requestWithHost("GET", "/admin/settings", "localhost")
			w, captured := runWithCapture(middleware, mockRepo, req)
			Expect(w.Code).To(Equal(http.StatusOK))
			// No context should be set for skipped routes
			Expect(captured.Found).To(BeFalse())
		})

		It("skips /auth routes", func() {
			req := requestWithHost("GET", "/auth/login", "localhost")
			w, captured := runWithCapture(middleware, mockRepo, req)
			Expect(w.Code).To(Equal(http.StatusOK))
			Expect(captured.Found).To(BeFalse())
		})

		It("skips /health endpoint", func() {
			req := requestWithHost("GET", "/health", "localhost")
			w, captured := runWithCapture(middleware, mockRepo, req)
			Expect(w.Code).To(Equal(http.StatusOK))
			Expect(captured.Found).To(BeFalse())
		})
	})

	Describe("subdomain takes priority over path", func() {
		It("prefers subdomain resolution when both are valid", func() {
			// context1.example.com has context "context1", but path /context/myapp would resolve "myapp"
			req := requestWithHost("GET", "/context/myapp/forms", "context1.example.com")
			w, captured := runWithCapture(middleware, mockRepo, req)
			Expect(w.Code).To(Equal(http.StatusOK))
			Expect(captured.Found).To(BeTrue())
			// Subdomain resolution should win
			Expect(captured.Context.Slug).To(Equal("context1"))
		})
	})
})
