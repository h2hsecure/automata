package cicd_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"

	"github.com/gin-gonic/gin"
	"github.com/hekemen/automata/cicd/support"
	"github.com/hekemen/automata/internal/adapter/api"
	infraauth "github.com/hekemen/automata/internal/infrastructure/auth"
	auth_repo "github.com/hekemen/automata/internal/infrastructure/auth/repo"
	ctxrepo "github.com/hekemen/automata/internal/infrastructure/context/repo"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

// setupCreationServer builds the full API server (real ContextResolver
// middleware, real auth, real repos) the same way cmd/automata does.
func setupCreationServer() *gin.Engine {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	contextRepo := ctxrepo.NewPostgresRepo(pool)
	userRepo := ctxrepo.NewUserPostgresRepo(pool)
	apiKeyRepo := auth_repo.NewApiKeyPostgresRepo(pool)
	authService := infraauth.NewServiceWithAPIKeys(userRepo, apiKeyRepo)
	return api.NewServer(engine, pool, contextRepo, userRepo, authService, apiKeyRepo, "/api")
}

var _ = Describe("Context-scoped creation endpoints (SPA default-context resolution)", func() {
	var (
		engine    *gin.Engine
		token     string
		newCtxID  string
	)

	BeforeEach(func() {
		// Cookie secret must be exactly 32 bytes (AES key) for the real
		// AuthHandler's refresh-token encryption.
		infraauth.InitForTesting("test-secret-key-for-jwt-signing-32bytes", "test-cookie-secret-32bytes-xxxx!")
		engine = setupCreationServer()

		// One active context => the default context for fallback resolution.
		newCtxID = support.NewTestContextID()
		_, err := pool.Exec(ctx, `
			INSERT INTO contexts (id, slug, name, is_active, settings, created_at, updated_at)
			VALUES ($1, $2, $3, $4, $5, NOW(), NOW())
		`, newCtxID, "creation-default", "Creation Default Context", true, `{}`)
		Expect(err).NotTo(HaveOccurred())

		// A user with membership in that context.
		userID := support.NewTestUUID()
		authSvc := infraauth.NewService(ctxrepo.NewUserPostgresRepo(pool))
		passwordHash, err := authSvc.HashPassword("REDACTED")
		Expect(err).NotTo(HaveOccurred())
		_, err = pool.Exec(ctx, `
			INSERT INTO users (id, email, password_hash, is_admin, created_at, updated_at)
			VALUES ($1, $2, $3, $4, NOW(), NOW())
		`, userID, "creator@example.com", passwordHash, true)
		Expect(err).NotTo(HaveOccurred())
		_, err = pool.Exec(ctx, `
			INSERT INTO user_contexts (user_id, context_id, role, created_at, updated_at)
			VALUES ($1, $2, $3, NOW(), NOW())
		`, userID, newCtxID, "owner")
		Expect(err).NotTo(HaveOccurred())

		// Log in through the real HTTP endpoint to obtain a JWT.
		body := `{"email":"creator@example.com","password":"REDACTED"}`
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
		token = loginResp.Token
	})

	AfterEach(func() {
		_, _ = pool.Exec(ctx, "DELETE FROM user_contexts")
		_, _ = pool.Exec(ctx, "DELETE FROM users")
		_, _ = pool.Exec(ctx, "DELETE FROM contacts")
		_, _ = pool.Exec(ctx, "DELETE FROM forms")
		_, _ = pool.Exec(ctx, "DELETE FROM banners")
	})

	doRequest := func(method, path string, payload any) *httptest.ResponseRecorder {
		var buf *bytes.Buffer
		if payload != nil {
			b, err := json.Marshal(payload)
			Expect(err).NotTo(HaveOccurred())
			buf = bytes.NewBuffer(b)
		} else {
			buf = bytes.NewBuffer(nil)
		}
		req := httptest.NewRequest(method, path, buf)
		req.Header.Set("Content-Type", "application/json")
		if token != "" {
			req.Header.Set("Authorization", "Bearer "+token)
		}
		w := httptest.NewRecorder()
		engine.ServeHTTP(w, req)
		return w
	}

	Describe("POST /api/context/contacts without explicit context", func() {
		It("creates the contact in the default context (SPA fallback)", func() {
			w := doRequest("POST", "/api/context/contacts", map[string]any{
				"email":      "created-contact@example.com",
				"first_name": "Created",
				"last_name":  "Contact",
			})
			Expect(w.Code).To(Equal(http.StatusCreated), "body: %s", w.Body.String())

			var created map[string]any
			Expect(json.Unmarshal(w.Body.Bytes(), &created)).To(Succeed())
			Expect(created["ContextID"]).To(Equal(newCtxID))
			Expect(created["Email"]).To(Equal("created-contact@example.com"))
		})

		It("still honors an explicit X-Context-ID header", func() {
			b, _ := json.Marshal(map[string]any{
				"email":      "header-contact@example.com",
				"first_name": "Header",
				"last_name":  "Contact",
			})
			req := httptest.NewRequest("POST", "/api/context/contacts", bytes.NewBuffer(b))
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("Authorization", "Bearer "+token)
			req.Header.Set("X-Context-ID", newCtxID)
			w := httptest.NewRecorder()
			engine.ServeHTTP(w, req)
			Expect(w.Code).To(Equal(http.StatusCreated), "body: %s", w.Body.String())
		})
	})

	Describe("POST /api/context/forms without explicit context", func() {
		It("creates the form in the default context (SPA fallback)", func() {
			w := doRequest("POST", "/api/context/forms", map[string]any{
				"name": "Cicd Creation Form",
				"slug": "cicd-creation-form",
				"fields": []map[string]any{
					{"slug": "email", "type": "email", "label": "Email", "required": true},
				},
			})
			Expect(w.Code).To(Equal(http.StatusCreated), "body: %s", w.Body.String())

			var created map[string]any
			Expect(json.Unmarshal(w.Body.Bytes(), &created)).To(Succeed())
			Expect(created["ContextID"]).To(Equal(newCtxID))
			Expect(created["Slug"]).To(Equal("cicd-creation-form"))
		})
	})

	Describe("POST /api/context/banners without explicit context", func() {
		It("creates the banner in the default context (SPA fallback)", func() {
			w := doRequest("POST", "/api/context/banners", map[string]any{
				"name":       "Cicd Creation Banner",
				"type":       "html",
				"content":    "<p>cicd banner</p>",
				"placements": []string{"hero-top"},
			})
			Expect(w.Code).To(Equal(http.StatusCreated), "body: %s", w.Body.String())

			var created map[string]any
			Expect(json.Unmarshal(w.Body.Bytes(), &created)).To(Succeed())
			Expect(created["ContextID"]).To(Equal(newCtxID))
			Expect(created["Name"]).To(Equal("Cicd Creation Banner"))
		})
	})

	Describe("GET list endpoints without explicit context", func() {
		It("lists contacts, forms, and banners via the default context", func() {
			// Seed one of each.
			w := doRequest("POST", "/api/context/contacts", map[string]any{
				"email":      "list-contact@example.com",
				"first_name": "List",
				"last_name":  "Contact",
			})
			Expect(w.Code).To(Equal(http.StatusCreated), "body: %s", w.Body.String())

			w = doRequest("POST", "/api/context/forms", map[string]any{
				"name": "Cicd List Form",
				"slug": "cicd-list-form",
				"fields": []map[string]any{
					{"slug": "name", "type": "text", "label": "Name", "required": true},
				},
			})
			Expect(w.Code).To(Equal(http.StatusCreated), "body: %s", w.Body.String())

			w = doRequest("POST", "/api/context/banners", map[string]any{
				"name":       "Cicd List Banner",
				"type":       "html",
				"content":    "<p>list banner</p>",
				"placements": []string{"hero-top"},
			})
			Expect(w.Code).To(Equal(http.StatusCreated), "body: %s", w.Body.String())

			// List contacts.
			w = doRequest("GET", "/api/context/contacts", nil)
			Expect(w.Code).To(Equal(http.StatusOK), "body: %s", w.Body.String())
			var contactsResp struct {
				Contacts []map[string]any `json:"contacts"`
			}
			Expect(json.Unmarshal(w.Body.Bytes(), &contactsResp)).To(Succeed())
			Expect(contactsResp.Contacts).To(HaveLen(1))
			Expect(contactsResp.Contacts[0]["Email"]).To(Equal("list-contact@example.com"))

			// List forms.
			w = doRequest("GET", "/api/context/forms", nil)
			Expect(w.Code).To(Equal(http.StatusOK), "body: %s", w.Body.String())
			var formsResp struct {
				Forms []map[string]any `json:"forms"`
			}
			Expect(json.Unmarshal(w.Body.Bytes(), &formsResp)).To(Succeed())
			Expect(formsResp.Forms).To(HaveLen(1))
			Expect(formsResp.Forms[0]["Slug"]).To(Equal("cicd-list-form"))

			// List banners.
			w = doRequest("GET", "/api/context/banners", nil)
			Expect(w.Code).To(Equal(http.StatusOK), "body: %s", w.Body.String())
			var bannersResp struct {
				Banners []map[string]any `json:"banners"`
			}
			Expect(json.Unmarshal(w.Body.Bytes(), &bannersResp)).To(Succeed())
			Expect(bannersResp.Banners).To(HaveLen(1))
			Expect(bannersResp.Banners[0]["Name"]).To(Equal("Cicd List Banner"))
		})
	})
})