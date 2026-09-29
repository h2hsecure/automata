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

func setupUserEngine() *gin.Engine {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	userRepo := ctxrepo.NewUserPostgresRepo(pool)
	userHandler := bhandler.NewUserHandler(userRepo, nil)

	users := engine.Group("/admin/users")
	{
		users.GET("", userHandler.List)
		users.POST("", userHandler.Create)
		users.GET("/:id", userHandler.Get)
		users.PUT("/:id", userHandler.Update)
		users.DELETE("/:id", userHandler.Delete)
	}

	return engine
}

var _ = Describe("User CRUD", func() {
	var (
		w      *httptest.ResponseRecorder
		engine *gin.Engine
	)

	BeforeEach(func() {
		engine = setupUserEngine()
		w = httptest.NewRecorder()
	})

	AfterEach(func() {
		_, _ = pool.Exec(ctx, "DELETE FROM users")
		_, _ = pool.Exec(ctx, "DELETE FROM user_contexts")
	})

	Describe("GET /admin/users", func() {
		It("should list all users", func() {
			user1ID := support.NewTestUUID()
			user2ID := support.NewTestUUID()

			hash1 := "$2a$10$testhash1testhash1testhash1t"
			hash2 := "$2a$10$testhash2testhash2testhash2t"

			_, err := pool.Exec(ctx, `
				INSERT INTO users (id, email, password_hash, is_admin, created_at, updated_at)
				VALUES ($1, $2, $3, $4, NOW(), NOW())
			`, user1ID, "user1@example.com", hash1, false)
			Expect(err).NotTo(HaveOccurred())

			_, err = pool.Exec(ctx, `
				INSERT INTO users (id, email, password_hash, is_admin, created_at, updated_at)
				VALUES ($1, $2, $3, $4, NOW(), NOW())
			`, user2ID, "user2@example.com", hash2, true)
			Expect(err).NotTo(HaveOccurred())

			req := httptest.NewRequest("GET", "/admin/users", nil)
			engine.ServeHTTP(w, req)

			Expect(w.Code).To(Equal(http.StatusOK))

			var response map[string]interface{}
			err = json.Unmarshal(w.Body.Bytes(), &response)
			Expect(err).NotTo(HaveOccurred())
			Expect(response["users"]).NotTo(BeNil())

			usersArr, ok := response["users"].([]interface{})
			Expect(ok).To(BeTrue())
			Expect(len(usersArr)).To(Equal(2))
		})
	})

	Describe("POST /admin/users", func() {
		It("should create a user", func() {
			body := `{
				"email": "newuser@example.com",
				"password": "TestPassword123!",
				"is_admin": false
			}`

			req := httptest.NewRequest("POST", "/admin/users", bytes.NewBufferString(body))
			req.Header.Set("Content-Type", "application/json")
			engine.ServeHTTP(w, req)

			Expect(w.Code).To(Equal(http.StatusCreated))

			var response map[string]interface{}
			err := json.Unmarshal(w.Body.Bytes(), &response)
			Expect(err).NotTo(HaveOccurred())
			Expect(response["id"]).NotTo(BeNil())
			Expect(response["email"]).To(Equal("newuser@example.com"))
			Expect(response["is_admin"]).To(Equal(false))
		})

		It("should return 409 for duplicate email on create", func() {
			body := `{
				"email": "duplicate@example.com",
				"password": "TestPassword123!"
			}`

			req := httptest.NewRequest("POST", "/admin/users", bytes.NewBufferString(body))
			req.Header.Set("Content-Type", "application/json")
			engine.ServeHTTP(w, req)
			Expect(w.Code).To(Equal(http.StatusCreated))

			// Try creating the same email again
			req2 := httptest.NewRequest("POST", "/admin/users", bytes.NewBufferString(body))
			req2.Header.Set("Content-Type", "application/json")
			w2 := httptest.NewRecorder()
			engine.ServeHTTP(w2, req2)

			Expect(w2.Code).To(Equal(http.StatusConflict))

			var response map[string]interface{}
			err := json.Unmarshal(w2.Body.Bytes(), &response)
			Expect(err).NotTo(HaveOccurred())
			Expect(response["error"]).To(Equal("email already exists"))
		})
	})

	Describe("GET /admin/users/:id", func() {
		It("should get a user by ID", func() {
			userID := support.NewTestUUID()
			hash := "$2a$10$testhash1testhash1testhash1t"

			_, err := pool.Exec(ctx, `
				INSERT INTO users (id, email, password_hash, is_admin, created_at, updated_at)
				VALUES ($1, $2, $3, $4, NOW(), NOW())
			`, userID, "getuser@example.com", hash, false)
			Expect(err).NotTo(HaveOccurred())

			req := httptest.NewRequest("GET", "/admin/users/"+userID, nil)
			engine.ServeHTTP(w, req)

			Expect(w.Code).To(Equal(http.StatusOK))

			var response map[string]interface{}
			err = json.Unmarshal(w.Body.Bytes(), &response)
			Expect(err).NotTo(HaveOccurred())
			Expect(response["id"]).To(Equal(userID))
			Expect(response["email"]).To(Equal("getuser@example.com"))
			Expect(response["is_admin"]).To(Equal(false))
		})

		It("should return 404 for non-existent user", func() {
			req := httptest.NewRequest("GET", "/admin/users/"+support.NewTestUUID(), nil)
			engine.ServeHTTP(w, req)

			Expect(w.Code).To(Equal(http.StatusNotFound))
		})

		It("should return 400 for invalid user ID", func() {
			req := httptest.NewRequest("GET", "/admin/users/not-a-uuid", nil)
			engine.ServeHTTP(w, req)

			Expect(w.Code).To(Equal(http.StatusBadRequest))
		})
	})

	Describe("PUT /admin/users/:id", func() {
		It("should update a user", func() {
			userID := support.NewTestUUID()
			hash := "$2a$10$testhash1testhash1testhash1t"

			_, err := pool.Exec(ctx, `
				INSERT INTO users (id, email, password_hash, is_admin, created_at, updated_at)
				VALUES ($1, $2, $3, $4, NOW(), NOW())
			`, userID, "old@example.com", hash, false)
			Expect(err).NotTo(HaveOccurred())

			newEmail := "updated@example.com"
			body := `{
				"email": "` + newEmail + `"
			}`

			req := httptest.NewRequest("PUT", "/admin/users/"+userID, bytes.NewBufferString(body))
			req.Header.Set("Content-Type", "application/json")
			engine.ServeHTTP(w, req)

			Expect(w.Code).To(Equal(http.StatusOK))

			var response map[string]interface{}
			err = json.Unmarshal(w.Body.Bytes(), &response)
			Expect(err).NotTo(HaveOccurred())
			Expect(response["email"]).To(Equal(newEmail))
		})
	})

	Describe("DELETE /admin/users/:id", func() {
		It("should delete a user", func() {
			userID := support.NewTestUUID()
			hash := "$2a$10$testhash1testhash1testhash1t"

			_, err := pool.Exec(ctx, `
				INSERT INTO users (id, email, password_hash, is_admin, created_at, updated_at)
				VALUES ($1, $2, $3, $4, NOW(), NOW())
			`, userID, "delete@example.com", hash, false)
			Expect(err).NotTo(HaveOccurred())

			req := httptest.NewRequest("DELETE", "/admin/users/"+userID, nil)
			engine.ServeHTTP(w, req)

			Expect(w.Code).To(Equal(http.StatusNoContent))

			// Verify user was deleted
			var count int
			err = pool.QueryRow(ctx, "SELECT COUNT(*) FROM users WHERE id = $1", userID).Scan(&count)
			Expect(err).NotTo(HaveOccurred())
			Expect(count).To(Equal(0))
		})
	})
})
