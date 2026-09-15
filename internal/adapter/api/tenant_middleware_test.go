package api

import (
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/hekemen/automata/internal/domain/tenant"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

// mockTenantRepo is an in-memory implementation of tenant.Repository for testing.
type mockTenantRepo struct {
	tenants map[string]*tenant.Tenant
}

func newMockTenantRepo() *mockTenantRepo {
	return &mockTenantRepo{tenants: make(map[string]*tenant.Tenant)}
}

func (m *mockTenantRepo) Create(t *tenant.Tenant) error {
	if t.ID == "" {
		t.ID = "550e8400-e29b-41d4-a716-446655440000"
	}
	m.tenants[t.Slug] = t
	return nil
}

func (m *mockTenantRepo) GetByID(id string) (*tenant.Tenant, error) {
	for _, t := range m.tenants {
		if t.ID == id {
			return t, nil
		}
	}
	return nil, errors.New("tenant not found")
}

func (m *mockTenantRepo) GetBySlug(slug string) (*tenant.Tenant, error) {
	t, ok := m.tenants[slug]
	if !ok {
		return nil, errors.New("tenant not found")
	}
	return t, nil
}

func (m *mockTenantRepo) List(offset, limit int) ([]*tenant.Tenant, error) {
	var result []*tenant.Tenant
	for _, t := range m.tenants {
		result = append(result, t)
	}
	return result, nil
}

func (m *mockTenantRepo) Update(t *tenant.Tenant) error {
	m.tenants[t.Slug] = t
	return nil
}

func (m *mockTenantRepo) Delete(id string) error {
	for slug, t := range m.tenants {
		if t.ID == id {
			delete(m.tenants, slug)
			return nil
		}
	}
	return errors.New("tenant not found")
}

var _ tenant.Repository = (*mockTenantRepo)(nil)

func TestTenantMiddleware(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Tenant Middleware Suite")
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

// tenantCapture holds the captured tenant from a middleware execution.
type tenantCapture struct {
	Tenant *tenant.Tenant
	Found  bool
}

// runWithCapture creates a fresh engine with the middleware and a capture handler,
// then executes the request to capture the tenant from context.
func runWithCapture(middleware gin.HandlerFunc, repo *mockTenantRepo, req *http.Request) (*httptest.ResponseRecorder, *tenantCapture) {
	captured := &tenantCapture{}
	engine := gin.New()
	engine.Use(middleware)
	engine.GET(req.URL.Path, func(c *gin.Context) {
		val, exists := c.Get(string(TenantContextKey))
		captured.Found = exists
		if exists {
			captured.Tenant = val.(*tenant.Tenant)
		}
		c.String(http.StatusOK, "ok")
	})
	w := performRequest(engine, req)
	return w, captured
}

var _ = Describe("TenantResolver middleware", func() {
	var (
		mockRepo   *mockTenantRepo
		middleware gin.HandlerFunc
	)

	BeforeEach(func() {
		gin.SetMode(gin.TestMode)
		mockRepo = newMockTenantRepo()

		// Seed test tenants
		mockRepo.tenants["tenant1"] = &tenant.Tenant{
			ID:        "550e8400-e29b-41d4-a716-446655440001",
			Slug:      "tenant1",
			Name:      "Tenant One",
			Domain:    "tenant1.example.com",
			IsActive:  true,
			Settings:  map[string]interface{}{},
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		}
		mockRepo.tenants["myapp"] = &tenant.Tenant{
			ID:        "550e8400-e29b-41d4-a716-446655440002",
			Slug:      "myapp",
			Name:      "My App",
			Domain:    "myapp.example.com",
			IsActive:  true,
			Settings:  map[string]interface{}{},
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		}
		mockRepo.tenants["acme"] = &tenant.Tenant{
			ID:        "550e8400-e29b-41d4-a716-446655440003",
			Slug:      "acme",
			Name:      "Acme Corp",
			Domain:    "acme.example.com",
			IsActive:  true,
			Settings:  map[string]interface{}{},
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		}

		middleware = TenantResolver(mockRepo)
	})

	Describe("subdomain resolution", func() {
		It("resolves tenant from subdomain", func() {
			req := requestWithHost("GET", "/test", "tenant1.example.com")
			w, captured := runWithCapture(middleware, mockRepo, req)
			Expect(w.Code).To(Equal(http.StatusOK))
			Expect(captured.Found).To(BeTrue())
			Expect(captured.Tenant.Slug).To(Equal("tenant1"))
		})

		It("resolves tenant from subdomain with port", func() {
			req := requestWithHost("GET", "/test", "myapp.example.com:8080")
			w, captured := runWithCapture(middleware, mockRepo, req)
			Expect(w.Code).To(Equal(http.StatusOK))
			Expect(captured.Found).To(BeTrue())
			Expect(captured.Tenant.Slug).To(Equal("myapp"))
		})

		It("skips www subdomain", func() {
			req := requestWithHost("GET", "/test", "www.example.com")
			w, _ := runWithCapture(middleware, mockRepo, req)
			Expect(w.Code).To(Equal(http.StatusBadRequest))
			body, _ := io.ReadAll(w.Body)
			Expect(string(body)).To(ContainSubstring("no tenant found"))
		})

		It("skips api subdomain", func() {
			req := requestWithHost("GET", "/test", "api.example.com")
			w, _ := runWithCapture(middleware, mockRepo, req)
			Expect(w.Code).To(Equal(http.StatusBadRequest))
			body, _ := io.ReadAll(w.Body)
			Expect(string(body)).To(ContainSubstring("no tenant found"))
		})

		It("skips mail subdomain", func() {
			req := requestWithHost("GET", "/test", "mail.example.com")
			w, _ := runWithCapture(middleware, mockRepo, req)
			Expect(w.Code).To(Equal(http.StatusBadRequest))
			body, _ := io.ReadAll(w.Body)
			Expect(string(body)).To(ContainSubstring("no tenant found"))
		})

		It("returns error for non-existent subdomain", func() {
			req := requestWithHost("GET", "/test", "unknown.example.com")
			w, _ := runWithCapture(middleware, mockRepo, req)
			Expect(w.Code).To(Equal(http.StatusBadRequest))
			body, _ := io.ReadAll(w.Body)
			Expect(string(body)).To(ContainSubstring("no tenant found"))
		})

		It("returns error for no subdomain (single part host)", func() {
			req := requestWithHost("GET", "/test", "localhost")
			w, _ := runWithCapture(middleware, mockRepo, req)
			Expect(w.Code).To(Equal(http.StatusBadRequest))
			body, _ := io.ReadAll(w.Body)
			Expect(string(body)).To(ContainSubstring("no tenant found"))
		})
	})

	Describe("path-based resolution", func() {
		It("resolves tenant from /tenant/<slug>/... path", func() {
			req := requestWithHost("GET", "/tenant/acme/forms", "localhost")
			w, captured := runWithCapture(middleware, mockRepo, req)
			Expect(w.Code).To(Equal(http.StatusOK))
			Expect(captured.Found).To(BeTrue())
			Expect(captured.Tenant.Slug).To(Equal("acme"))
		})

		It("resolves tenant from /t/<slug>/... path", func() {
			req := requestWithHost("GET", "/t/acme/forms", "localhost")
			w, captured := runWithCapture(middleware, mockRepo, req)
			Expect(w.Code).To(Equal(http.StatusOK))
			Expect(captured.Found).To(BeTrue())
			Expect(captured.Tenant.Slug).To(Equal("acme"))
		})

		It("returns error for /tenant/ with empty slug", func() {
			req := requestWithHost("GET", "/tenant/", "localhost")
			w, _ := runWithCapture(middleware, mockRepo, req)
			Expect(w.Code).To(Equal(http.StatusBadRequest))
			body, _ := io.ReadAll(w.Body)
			Expect(string(body)).To(ContainSubstring("no tenant found"))
		})

		It("returns error for /t/ with empty slug", func() {
			req := requestWithHost("GET", "/t/", "localhost")
			w, _ := runWithCapture(middleware, mockRepo, req)
			Expect(w.Code).To(Equal(http.StatusBadRequest))
			body, _ := io.ReadAll(w.Body)
			Expect(string(body)).To(ContainSubstring("no tenant found"))
		})

		It("returns error for non-existent tenant in path", func() {
			req := requestWithHost("GET", "/tenant/nonexistent/forms", "localhost")
			w, _ := runWithCapture(middleware, mockRepo, req)
			Expect(w.Code).To(Equal(http.StatusBadRequest))
			body, _ := io.ReadAll(w.Body)
			Expect(string(body)).To(ContainSubstring("no tenant found"))
		})
	})

	Describe("skipped routes", func() {
		It("skips /admin routes", func() {
			req := requestWithHost("GET", "/admin/settings", "localhost")
			w, captured := runWithCapture(middleware, mockRepo, req)
			Expect(w.Code).To(Equal(http.StatusOK))
			// No tenant should be set for skipped routes
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
			// tenant1.example.com has tenant "tenant1", but path /tenant/myapp would resolve "myapp"
			req := requestWithHost("GET", "/tenant/myapp/forms", "tenant1.example.com")
			w, captured := runWithCapture(middleware, mockRepo, req)
			Expect(w.Code).To(Equal(http.StatusOK))
			Expect(captured.Found).To(BeTrue())
			// Subdomain resolution should win
			Expect(captured.Tenant.Slug).To(Equal("tenant1"))
		})
	})
})
