package cicd_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"

	"github.com/gin-gonic/gin"
	"github.com/h2hsecure/automata/cicd/support"
	"github.com/h2hsecure/automata/internal/adapter/api/middleware"
	domainauth "github.com/h2hsecure/automata/internal/domain/auth"
	infraauth "github.com/h2hsecure/automata/internal/infrastructure/auth"
	ctxrepo "github.com/h2hsecure/automata/internal/infrastructure/context/repo"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Session Revocation Integration Tests", func() {
	var (
		engine  *gin.Engine
		authSvc domainauth.AuthService
		userID  string
	)

	BeforeEach(func() {
		gin.SetMode(gin.TestMode)
		engine = gin.New()
		engine.Use(gin.Recovery())

		infraauth.InitForTesting("test-secret-key-for-jwt-signing-32bytes", "test-cookie-secret-for-testing-32bytes!!")

		userRepo := ctxrepo.NewUserPostgresRepo(pool)
		authSvc = infraauth.NewService(userRepo)
		handler := &TestAuthHandler{service: authSvc}

		engine.POST("/api/auth/login", handler.Login)
		engine.GET("/api/whoami", middleware.AuthMiddleware(authSvc), func(c *gin.Context) {
			c.JSON(http.StatusOK, gin.H{"user_id": c.GetString("user_id")})
		})
	})

	AfterEach(func() {
		_, _ = pool.Exec(ctx, "DELETE FROM api_keys WHERE user_id = $1", userID)
		_, _ = pool.Exec(ctx, "DELETE FROM user_contexts WHERE user_id = $1", userID)
		_, _ = pool.Exec(ctx, "DELETE FROM users WHERE id = $1", userID)
		_, _ = pool.Exec(ctx, "DELETE FROM context_users")
		_, _ = pool.Exec(ctx, "DELETE FROM contexts")
	})

	// do issues a request against the engine with a fresh recorder, so that
	// each response code and body is captured independently (a shared
	// ResponseRecorder would keep the first status code).
	do := func(method, path, token, body string) *httptest.ResponseRecorder {
		var req *http.Request
		if body != "" {
			req = httptest.NewRequest(method, path, bytes.NewBufferString(body))
			req.Header.Set("Content-Type", "application/json")
		} else {
			req = httptest.NewRequest(method, path, nil)
		}
		if token != "" {
			req.Header.Set("Authorization", "Bearer "+token)
		}
		w := httptest.NewRecorder()
		engine.ServeHTTP(w, req)
		return w
	}

	It("rejects tokens whose iat predates password_changed_at (session revoked)", func() {
		userID = support.NewTestUUID()
		passwordHash, err := authSvc.HashPassword("REDACTED")
		Expect(err).NotTo(HaveOccurred())

		_, err = pool.Exec(ctx, `
			INSERT INTO users (id, email, password_hash, is_admin, password_changed_at, created_at, updated_at)
			VALUES ($1, $2, $3, false, NOW(), NOW(), NOW())
		`, userID, "revoked@example.com", passwordHash)
		Expect(err).NotTo(HaveOccurred())

		// Login issues a fresh access token (iat = now, password_changed_at = earlier)
		w := do("POST", "/api/auth/login", "", `{"email":"revoked@example.com","password":"REDACTED"}`)
		Expect(w.Code).To(Equal(http.StatusOK), "body: %s", w.Body.String())

		var response map[string]interface{}
		Expect(json.Unmarshal(w.Body.Bytes(), &response)).To(Succeed())
		token, ok := response["token"].(string)
		Expect(ok).To(BeTrue())
		Expect(token).NotTo(BeEmpty())

		// Token is valid before revocation
		w = do("GET", "/api/whoami", token, "")
		Expect(w.Code).To(Equal(http.StatusOK), "body: %s", w.Body.String())

		// Simulate a password change / logout-all: move password_changed_at past the token's iat
		_, err = pool.Exec(ctx, `
			UPDATE users SET password_changed_at = NOW() + interval '1 second' WHERE id = $1
		`, userID)
		Expect(err).NotTo(HaveOccurred())

		// The same token must now be rejected as a revoked session
		w = do("GET", "/api/whoami", token, "")
		Expect(w.Code).To(Equal(http.StatusUnauthorized), "body: %s", w.Body.String())

		var errResp map[string]interface{}
		Expect(json.Unmarshal(w.Body.Bytes(), &errResp)).To(Succeed())
		Expect(errResp["error"]).To(Equal("session revoked"))
	})
})