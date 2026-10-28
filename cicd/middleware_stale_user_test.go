package cicd_test

import (
	"net/http"
	"net/http/httptest"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/h2hsecure/automata/internal/adapter/api/middleware"
	domainauth "github.com/h2hsecure/automata/internal/domain/auth"
	domainctx "github.com/h2hsecure/automata/internal/domain/context"
	infraauth "github.com/h2hsecure/automata/internal/infrastructure/auth"
	ctxrepo "github.com/h2hsecure/automata/internal/infrastructure/context/repo"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Auth middleware stale user", func() {
	var (
		w        *httptest.ResponseRecorder
		engine   *gin.Engine
		authSvc  domainauth.AuthService
		userRepo domainctx.UserRepository
		userID   string
	)

	BeforeEach(func() {
		gin.SetMode(gin.TestMode)
		w = httptest.NewRecorder()

		infraauth.InitForTesting("test-secret-key-for-jwt-signing-32bytes", "test-cookie-secret-for-testing-32bytes!!")

		userRepo = ctxrepo.NewUserPostgresRepo(pool)
		authSvc = infraauth.NewService(userRepo)

		engine = gin.New()
		engine.Use(middleware.AuthMiddleware(authSvc))
		engine.GET("/api/stub", func(c *gin.Context) {
			c.JSON(http.StatusOK, gin.H{"user_id": c.GetString("user_id")})
		})
	})

	AfterEach(func() {
		if userID != "" && pool != nil {
			_, _ = pool.Exec(ctx, "DELETE FROM users WHERE id = $1", userID)
		}
		userID = ""
	})

	It("accepts a valid JWT whose user exists", func() {
		email := "stale-user-" + uuid.New().String() + "@example.com"
		userID = uuid.New().String()
		hash, err := authSvc.HashPassword("test-password-123")
		Expect(err).NotTo(HaveOccurred())
		Expect(userRepo.Create(&domainctx.User{
			ID:           userID,
			Email:        email,
			PasswordHash: hash,
			IsAdmin:      true,
		})).To(Succeed())

		token, _, _, err := authSvc.Login(email, "test-password-123")
		Expect(err).NotTo(HaveOccurred())

		req := httptest.NewRequest(http.MethodGet, "/api/stub", nil)
		req.Header.Set("Authorization", "Bearer "+token)
		engine.ServeHTTP(w, req)

		Expect(w.Code).To(Equal(http.StatusOK))
	})

	It("rejects a valid JWT when the user no longer exists", func() {
		email := "stale-user-" + uuid.New().String() + "@example.com"
		userID = uuid.New().String()
		hash, err := authSvc.HashPassword("test-password-123")
		Expect(err).NotTo(HaveOccurred())
		Expect(userRepo.Create(&domainctx.User{
			ID:           userID,
			Email:        email,
			PasswordHash: hash,
			IsAdmin:      true,
		})).To(Succeed())

		// Login issues a cryptographically valid JWT for this user.
		token, _, _, err := authSvc.Login(email, "test-password-123")
		Expect(err).NotTo(HaveOccurred())

		// Simulate data loss: the user row disappears (e.g. DB wipe).
		Expect(userRepo.Delete(userID)).To(Succeed())

		req := httptest.NewRequest(http.MethodGet, "/api/stub", nil)
		req.Header.Set("Authorization", "Bearer "+token)
		engine.ServeHTTP(w, req)

		// The request must be rejected, not processed with a dangling user_id.
		Expect(w.Code).To(Equal(http.StatusUnauthorized))
	})
})