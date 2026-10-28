package cicd_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"

	"github.com/h2hsecure/automata/cicd/support"
	infraauth "github.com/h2hsecure/automata/internal/infrastructure/auth"
	ctxrepo "github.com/h2hsecure/automata/internal/infrastructure/context/repo"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

// Swagger docs are served from the full production engine (setupCreationServer
// from creation_endpoints_test.go) so the real route registration, auth
// middleware, and embedded YAML are all exercised.

var _ = Describe("Swagger endpoints (GET /api/swagger.yaml, GET /api/swagger-public.yaml)", func() {
	var (
		engine    interface{ ServeHTTP(w http.ResponseWriter, r *http.Request) }
		adminJWT  string
		contextID string
	)

	BeforeEach(func() {
		infraauth.InitForTesting("test-secret-key-for-jwt-signing-32bytes", "test-cookie-secret-32bytes-xxxx!")
		engine = setupCreationServer()
		adminJWT = loginAsAdmin(engine)

		// API keys are context-scoped; create a context to bind new keys to.
		contextID = support.NewTestUUID()
		_, err := pool.Exec(ctx, `
			INSERT INTO contexts (id, slug, name, created_at, updated_at)
			VALUES ($1, $2, $3, NOW(), NOW())
		`, contextID, "swagger-ctx-"+contextID, "Swagger Test Context")
		Expect(err).NotTo(HaveOccurred())
	})

	AfterEach(func() {
		_, _ = pool.Exec(ctx, "DELETE FROM api_keys")
		_, _ = pool.Exec(ctx, `DELETE FROM contexts WHERE id = $1`, contextID)
		_, _ = pool.Exec(ctx, `DELETE FROM users WHERE email LIKE 'swagger-admin-%@example.com'`)
	})

	doRequest := func(authHeader, path string) *httptest.ResponseRecorder {
		req := httptest.NewRequest("GET", path, bytes.NewBuffer(nil))
		if authHeader != "" {
			req.Header.Set("Authorization", authHeader)
		}
		w := httptest.NewRecorder()
		engine.ServeHTTP(w, req)
		return w
	}

	// createAPIKey creates a real API key through the admin endpoint and
// returns the plaintext key.
createAPIKey := func() string {
		body := `{"name":"` + "swagger-test-key-" + support.NewTestUUID() + `"}`
		req := httptest.NewRequest("POST", "/api/admin/api-keys", bytes.NewBufferString(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+adminJWT)
		req.Header.Set("X-Context-ID", contextID)
		w := httptest.NewRecorder()
		engine.ServeHTTP(w, req)
		Expect(w.Code).To(Equal(http.StatusCreated), "create api key: %s", w.Body.String())
		var resp struct {
			Key string `json:"key"`
		}
		Expect(json.Unmarshal(w.Body.Bytes(), &resp)).To(Succeed())
		Expect(resp.Key).NotTo(BeEmpty())
		return resp.Key
	}

	Describe("GET /api/swagger.yaml (main API doc)", func() {
		It("returns 401 without authentication", func() {
			w := doRequest("", "/api/swagger.yaml")
			Expect(w.Code).To(Equal(http.StatusUnauthorized))
		})

		It("serves the YAML doc with a valid admin JWT", func() {
			w := doRequest("Bearer "+adminJWT, "/api/swagger.yaml")
			Expect(w.Code).To(Equal(http.StatusOK))
			Expect(w.Header().Get("Content-Type")).To(HavePrefix("application/yaml"))

			body := w.Body.String()
			Expect(body).To(ContainSubstring("openapi: 3.1"))
			Expect(body).To(ContainSubstring("/context/contacts"))
			Expect(body).To(ContainSubstring("x-automata-api-keys"))
			Expect(body).To(ContainSubstring("bearerAuth"))
		})

		It("serves the YAML doc with a valid API key", func() {
			key := createAPIKey()
			w := doRequest("Bearer "+key, "/api/swagger.yaml")
			Expect(w.Code).To(Equal(http.StatusOK))
			Expect(w.Header().Get("Content-Type")).To(HavePrefix("application/yaml"))
		})
	})

	Describe("GET /api/swagger-public.yaml (public tracking doc)", func() {
		It("returns 401 without authentication", func() {
			w := doRequest("", "/api/swagger-public.yaml")
			Expect(w.Code).To(Equal(http.StatusUnauthorized))
		})

		It("serves the YAML doc with a valid admin JWT", func() {
			w := doRequest("Bearer "+adminJWT, "/api/swagger-public.yaml")
			Expect(w.Code).To(Equal(http.StatusOK))
			Expect(w.Header().Get("Content-Type")).To(HavePrefix("application/yaml"))

			body := w.Body.String()
			Expect(body).To(ContainSubstring("openapi: 3.1"))
			Expect(body).To(ContainSubstring("/track/batch"))
			Expect(body).To(ContainSubstring("security: []"))
		})

		It("serves the YAML doc with a valid API key", func() {
			key := createAPIKey()
			w := doRequest("Bearer "+key, "/api/swagger-public.yaml")
			Expect(w.Code).To(Equal(http.StatusOK))
			Expect(w.Header().Get("Content-Type")).To(HavePrefix("application/yaml"))
		})
	})
})

// loginAsAdmin seeds a temp admin user and logs in through the real login
// endpoint to obtain a JWT (same flow as creation_endpoints_test.go).
// The email is unique per call so successive Its do not collide on
// users_email_key.
func loginAsAdmin(engine interface{ ServeHTTP(w http.ResponseWriter, r *http.Request) }) string {
	userID := support.NewTestUUID()
	email := "swagger-admin-" + userID + "@example.com"
	authSvc := infraauth.NewService(ctxrepo.NewUserPostgresRepo(pool))
	passwordHash, err := authSvc.HashPassword("REDACTED")
	Expect(err).NotTo(HaveOccurred())
	_, err = pool.Exec(ctx, `
		INSERT INTO users (id, email, password_hash, is_admin, created_at, updated_at)
		VALUES ($1, $2, $3, $4, NOW(), NOW())
	`, userID, email, passwordHash, true)
	Expect(err).NotTo(HaveOccurred())

	body := `{"email":"` + email + `","password":"REDACTED"}`
	req := httptest.NewRequest("POST", "/api/auth/login", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	engine.ServeHTTP(w, req)
	Expect(w.Code).To(Equal(http.StatusOK), "login failed: %s", w.Body.String())

	var loginResp struct {
		Token string `json:"token"`
	}
	Expect(json.Unmarshal(w.Body.Bytes(), &loginResp)).To(Succeed())
	Expect(loginResp.Token).NotTo(BeEmpty())
	return loginResp.Token
}