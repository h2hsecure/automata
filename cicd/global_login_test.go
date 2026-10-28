package cicd_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"

	"github.com/gin-gonic/gin"
	"github.com/h2hsecure/automata/cicd/support"
	domainauth "github.com/h2hsecure/automata/internal/domain/auth"
	infraauth "github.com/h2hsecure/automata/internal/infrastructure/auth"
	ctxrepo "github.com/h2hsecure/automata/internal/infrastructure/context/repo"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Global Login Integration Tests", func() {
	var (
		w        *httptest.ResponseRecorder
		engine   *gin.Engine
		authSvc  domainauth.AuthService
	)

	BeforeEach(func() {
		gin.SetMode(gin.TestMode)
		w = httptest.NewRecorder()
		engine = gin.New()
		engine.Use(gin.Recovery())

		infraauth.InitForTesting("test-secret-key-for-jwt-signing-32bytes", "test-cookie-secret-for-testing-32bytes!!")

		userRepo := ctxrepo.NewUserPostgresRepo(pool)
		authSvc = infraauth.NewService(userRepo)
		handler := &TestAuthHandler{service: authSvc}

		engine.POST("/api/auth/login", handler.Login)
	})

	AfterEach(func() {
		_, err := pool.Exec(ctx, "DELETE FROM context_users")
		Expect(err).NotTo(HaveOccurred())
		_, err = pool.Exec(ctx, "DELETE FROM contexts")
		Expect(err).NotTo(HaveOccurred())
	})

	Describe("POST /api/auth/login", func() {
		It("should login a user globally without context in request", func() {
			contextID := support.NewTestContextID()

			_, err := pool.Exec(ctx, `
				INSERT INTO contexts (id, slug, name, is_active, settings, created_at, updated_at)
				VALUES ($1, $2, $3, $4, $5, NOW(), NOW())
			`, contextID, "default", "Default Context", true, `{}`)
			Expect(err).NotTo(HaveOccurred())

			userID := support.NewTestUUID()
			passwordHash, err := authSvc.HashPassword("REDACTED")
			Expect(err).NotTo(HaveOccurred())

			_, err = pool.Exec(ctx, `
				INSERT INTO context_users (id, context_id, email, password_hash, is_owner, created_at, updated_at)
				VALUES ($1, $2, $3, $4, $5, NOW(), NOW())
			`, userID, contextID, "admin@example.com", passwordHash, true)
			Expect(err).NotTo(HaveOccurred())

			body := `{"email":"admin@example.com","password":"REDACTED"}`

			req := httptest.NewRequest("POST", "/api/auth/login", bytes.NewBufferString(body))
			req.Header.Set("Content-Type", "application/json")
			engine.ServeHTTP(w, req)

			Expect(w.Code).To(Equal(http.StatusOK))

			var response map[string]interface{}
			err = json.Unmarshal(w.Body.Bytes(), &response)
			Expect(err).NotTo(HaveOccurred())
			Expect(response).To(HaveKey("token"))
			Expect(response).To(HaveKey("user_id"))
			Expect(response).To(HaveKey("email"))
			Expect(response).To(HaveKey("contexts"))
		})

		It("should return 401 for invalid password", func() {
			contextID := support.NewTestContextID()

			_, err := pool.Exec(ctx, `
				INSERT INTO contexts (id, slug, name, is_active, settings, created_at, updated_at)
				VALUES ($1, $2, $3, $4, $5, NOW(), NOW())
			`, contextID, "default", "Default Context", true, `{}`)
			Expect(err).NotTo(HaveOccurred())

			userID := support.NewTestUUID()
			passwordHash, err := authSvc.HashPassword("REDACTED")
			Expect(err).NotTo(HaveOccurred())

			_, err = pool.Exec(ctx, `
				INSERT INTO context_users (id, context_id, email, password_hash, is_owner, created_at, updated_at)
				VALUES ($1, $2, $3, $4, $5, NOW(), NOW())
			`, userID, contextID, "admin@example.com", passwordHash, true)
			Expect(err).NotTo(HaveOccurred())

			body := `{"email":"admin@example.com","password":"wrongpassword"}`

			req := httptest.NewRequest("POST", "/api/auth/login", bytes.NewBufferString(body))
			req.Header.Set("Content-Type", "application/json")
			engine.ServeHTTP(w, req)

			Expect(w.Code).To(Equal(http.StatusUnauthorized))
		})

		It("should return 401 for non-existent user", func() {
			body := `{"email":"nonexistent@example.com","password":"REDACTED"}`

			req := httptest.NewRequest("POST", "/api/auth/login", bytes.NewBufferString(body))
			req.Header.Set("Content-Type", "application/json")
			engine.ServeHTTP(w, req)

			Expect(w.Code).To(Equal(http.StatusUnauthorized))
		})

		It("should return contexts list in login response", func() {
			contextID1 := support.NewTestContextID()
			contextID2 := support.NewTestContextID()

			for _, cID := range []string{contextID1, contextID2} {
				_, err := pool.Exec(ctx, `
					INSERT INTO contexts (id, slug, name, is_active, settings, created_at, updated_at)
					VALUES ($1, $2, $3, $4, $5, NOW(), NOW())
				`, cID, "context-"+cID[:8], "Test Context", true, `{}`)
				Expect(err).NotTo(HaveOccurred())

				userID := support.NewTestUUID()
				passwordHash, err := authSvc.HashPassword("REDACTED")
				Expect(err).NotTo(HaveOccurred())

				_, err = pool.Exec(ctx, `
					INSERT INTO context_users (id, context_id, email, password_hash, is_owner, created_at, updated_at)
					VALUES ($1, $2, $3, $4, $5, NOW(), NOW())
				`, userID, cID, "multi@example.com", passwordHash, true)
				Expect(err).NotTo(HaveOccurred())
			}

			body := `{"email":"multi@example.com","password":"REDACTED"}`

			req := httptest.NewRequest("POST", "/api/auth/login", bytes.NewBufferString(body))
			req.Header.Set("Content-Type", "application/json")
			engine.ServeHTTP(w, req)

			Expect(w.Code).To(Equal(http.StatusOK))

			var response map[string]interface{}
			err := json.Unmarshal(w.Body.Bytes(), &response)
			Expect(err).NotTo(HaveOccurred())

			contexts := response["contexts"].([]interface{})
			Expect(contexts).To(HaveLen(2))
		})

		It("should return 400 when request body is invalid", func() {
			body := `{"invalid json`

			req := httptest.NewRequest("POST", "/api/auth/login", bytes.NewBufferString(body))
			req.Header.Set("Content-Type", "application/json")
			engine.ServeHTTP(w, req)

			Expect(w.Code).To(Equal(http.StatusBadRequest))
		})
	})

	Describe("Global login across contexts", func() {
		It("should find user across multiple contexts", func() {
			contextID1 := support.NewTestContextID()
			contextID2 := support.NewTestContextID()

			for _, cID := range []string{contextID1, contextID2} {
				_, err := pool.Exec(ctx, `
					INSERT INTO contexts (id, slug, name, is_active, settings, created_at, updated_at)
					VALUES ($1, $2, $3, $4, $5, NOW(), NOW())
				`, cID, "context-"+cID[:8], "Test Context", true, `{}`)
				Expect(err).NotTo(HaveOccurred())

				userID := support.NewTestUUID()
				passwordHash, err := authSvc.HashPassword("REDACTED")
				Expect(err).NotTo(HaveOccurred())

				_, err = pool.Exec(ctx, `
					INSERT INTO context_users (id, context_id, email, password_hash, is_owner, created_at, updated_at)
					VALUES ($1, $2, $3, $4, $5, NOW(), NOW())
				`, userID, cID, "shared@example.com", passwordHash, true)
				Expect(err).NotTo(HaveOccurred())
			}

			body := `{"email":"shared@example.com","password":"REDACTED"}`

			req := httptest.NewRequest("POST", "/api/auth/login", bytes.NewBufferString(body))
			req.Header.Set("Content-Type", "application/json")
			engine.ServeHTTP(w, req)

			Expect(w.Code).To(Equal(http.StatusOK))

			var response map[string]interface{}
			err := json.Unmarshal(w.Body.Bytes(), &response)
			Expect(err).NotTo(HaveOccurred())
			Expect(response).To(HaveKey("user_id"))
		})
	})
})

// TestAuthHandler handles auth endpoints for testing.
type TestAuthHandler struct {
	service domainauth.AuthService
}

func (h *TestAuthHandler) Login(c *gin.Context) {
	var req struct {
		Email    string `json:"email" binding:"required"`
		Password string `json:"password" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": "invalid request body"})
		return
	}

	token, userID, contexts, err := h.service.Login(req.Email, req.Password)
	if err != nil {
		c.JSON(401, gin.H{"error": "invalid email or password"})
		return
	}

	contextList := make([]map[string]interface{}, 0, len(contexts))
	for _, c := range contexts {
		contextList = append(contextList, map[string]interface{}{
			"id":   c.ID,
			"slug": c.Slug,
			"name": c.Name,
		})
	}

	c.JSON(200, gin.H{
		"token":    token,
		"user_id":  userID,
		"email":    req.Email,
		"contexts": contextList,
	})
}
