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

func setupContextUserEngine() *gin.Engine {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	userRepo := ctxrepo.NewUserPostgresRepo(pool)
	contextRepo := ctxrepo.NewPostgresRepo(pool)
	contextUserHandler := bhandler.NewContextUserHandler(userRepo, contextRepo)

	contexts := engine.Group("/admin/contexts/:id/users")
	{
		contexts.GET("", contextUserHandler.List)
		contexts.POST("", contextUserHandler.Create)
		contexts.DELETE("/:userId", contextUserHandler.Delete)
	}

	return engine
}

var _ = Describe("Context User Management", func() {
	var (
		w      *httptest.ResponseRecorder
		engine *gin.Engine
	)

	BeforeEach(func() {
		engine = setupContextUserEngine()
		w = httptest.NewRecorder()
	})

	AfterEach(func() {
		_, _ = pool.Exec(ctx, "DELETE FROM user_contexts")
		_, _ = pool.Exec(ctx, "DELETE FROM contexts")
		_, _ = pool.Exec(ctx, "DELETE FROM users")
	})

	Describe("GET /admin/contexts/:id/users", func() {
		It("should list members of a context", func() {
			contextID := support.NewTestContextID()
			user1ID := support.NewTestUUID()
			user2ID := support.NewTestUUID()

			// Create context record
			_, err := pool.Exec(ctx, `
				INSERT INTO contexts (id, slug, name, is_active, settings)
				VALUES ($1, $2, $3, $4, $5)
			`, contextID, "test-context", "Test Context", true, "{}")
			Expect(err).NotTo(HaveOccurred())

			// Create user records
			hash1 := "$2a$10$testhash1testhash1testhash1t"
			hash2 := "$2a$10$testhash2testhash2testhash2t"
			_, err = pool.Exec(ctx, `
				INSERT INTO users (id, email, password_hash, is_admin, created_at, updated_at)
				VALUES ($1, $2, $3, $4, NOW(), NOW())
			`, user1ID, "member1@example.com", hash1, false)
			Expect(err).NotTo(HaveOccurred())

			_, err = pool.Exec(ctx, `
				INSERT INTO users (id, email, password_hash, is_admin, created_at, updated_at)
				VALUES ($1, $2, $3, $4, NOW(), NOW())
			`, user2ID, "member2@example.com", hash2, false)
			Expect(err).NotTo(HaveOccurred())

			// Create memberships
			_, err = pool.Exec(ctx, `
				INSERT INTO user_contexts (id, user_id, context_id, role, created_at, updated_at)
				VALUES (gen_random_uuid(), $1, $2, $3, NOW(), NOW())
			`, user1ID, contextID, "member")
			Expect(err).NotTo(HaveOccurred())

			_, err = pool.Exec(ctx, `
				INSERT INTO user_contexts (id, user_id, context_id, role, created_at, updated_at)
				VALUES (gen_random_uuid(), $1, $2, $3, NOW(), NOW())
			`, user2ID, contextID, "admin")
			Expect(err).NotTo(HaveOccurred())

			req := httptest.NewRequest("GET", "/admin/contexts/"+contextID+"/users", nil)
			engine.ServeHTTP(w, req)

			Expect(w.Code).To(Equal(http.StatusOK))

			var response map[string]interface{}
			err = json.Unmarshal(w.Body.Bytes(), &response)
			Expect(err).NotTo(HaveOccurred())
			Expect(response["members"]).NotTo(BeNil())

			membersArr, ok := response["members"].([]interface{})
			Expect(ok).To(BeTrue())
			Expect(len(membersArr)).To(Equal(2))
		})

		It("should return 400 for invalid context ID", func() {
			req := httptest.NewRequest("GET", "/admin/contexts/not-a-uuid/users", nil)
			engine.ServeHTTP(w, req)

			Expect(w.Code).To(Equal(http.StatusBadRequest))
		})
	})

	Describe("POST /admin/contexts/:id/users", func() {
		It("should add a user to a context", func() {
			contextID := support.NewTestContextID()
			userID := support.NewTestUUID()

			// Create context
			_, err := pool.Exec(ctx, `
				INSERT INTO contexts (id, slug, name, is_active, settings)
				VALUES ($1, $2, $3, $4, $5)
			`, contextID, "test-context", "Test Context", true, "{}")
			Expect(err).NotTo(HaveOccurred())

			// Create user
			hash := "$2a$10$testhash1testhash1testhash1t"
			_, err = pool.Exec(ctx, `
				INSERT INTO users (id, email, password_hash, is_admin, created_at, updated_at)
				VALUES ($1, $2, $3, $4, NOW(), NOW())
			`, userID, "contextuser@example.com", hash, false)
			Expect(err).NotTo(HaveOccurred())

			body := `{
				"user_id": "` + userID + `",
				"role": "member"
			}`

			req := httptest.NewRequest("POST", "/admin/contexts/"+contextID+"/users", bytes.NewBufferString(body))
			req.Header.Set("Content-Type", "application/json")
			engine.ServeHTTP(w, req)

			Expect(w.Code).To(Equal(http.StatusCreated))

			var response map[string]interface{}
			err = json.Unmarshal(w.Body.Bytes(), &response)
			Expect(err).NotTo(HaveOccurred())
			Expect(response["user_id"]).To(Equal(userID))
			Expect(response["context_id"]).To(Equal(contextID))
			Expect(response["role"]).To(Equal("member"))
		})

		It("should return 400 for invalid role", func() {
			contextID := support.NewTestContextID()

			// Create context
			_, err := pool.Exec(ctx, `
				INSERT INTO contexts (id, slug, name, is_active, settings)
				VALUES ($1, $2, $3, $4, $5)
			`, contextID, "test-context", "Test Context", true, "{}")
			Expect(err).NotTo(HaveOccurred())

			userID := support.NewTestUUID()
			_, err = pool.Exec(ctx, `
				INSERT INTO users (id, email, password_hash, is_admin, created_at, updated_at)
				VALUES ($1, $2, $3, $4, NOW(), NOW())
			`, userID, "contextuser@example.com", "$2a$10$testhash1testhash1testhash1t", false)
			Expect(err).NotTo(HaveOccurred())

			body := `{
				"user_id": "` + userID + `",
				"role": "superuser"
			}`

			req := httptest.NewRequest("POST", "/admin/contexts/"+contextID+"/users", bytes.NewBufferString(body))
			req.Header.Set("Content-Type", "application/json")
			engine.ServeHTTP(w, req)

			Expect(w.Code).To(Equal(http.StatusBadRequest))
		})
	})

	Describe("DELETE /admin/contexts/:id/users/:userId", func() {
		It("should delete a membership", func() {
			contextID := support.NewTestContextID()
			userID := support.NewTestUUID()

			// Create context
			_, err := pool.Exec(ctx, `
				INSERT INTO contexts (id, slug, name, is_active, settings)
				VALUES ($1, $2, $3, $4, $5)
			`, contextID, "test-context", "Test Context", true, "{}")
			Expect(err).NotTo(HaveOccurred())

			// Create user
			hash := "$2a$10$testhash1testhash1testhash1t"
			_, err = pool.Exec(ctx, `
				INSERT INTO users (id, email, password_hash, is_admin, created_at, updated_at)
				VALUES ($1, $2, $3, $4, NOW(), NOW())
			`, userID, "member@example.com", hash, false)
			Expect(err).NotTo(HaveOccurred())

			// Create membership
			membershipID := support.NewTestUUID()
			_, err = pool.Exec(ctx, `
				INSERT INTO user_contexts (id, user_id, context_id, role, created_at, updated_at)
				VALUES ($1, $2, $3, $4, NOW(), NOW())
			`, membershipID, userID, contextID, "member")
			Expect(err).NotTo(HaveOccurred())

			req := httptest.NewRequest("DELETE", "/admin/contexts/"+contextID+"/users/"+userID, nil)
			engine.ServeHTTP(w, req)

			Expect(w.Code).To(Equal(http.StatusNoContent))

			// Verify membership was deleted
			var count int
			err = pool.QueryRow(ctx, "SELECT COUNT(*) FROM user_contexts WHERE user_id = $1 AND context_id = $2", userID, contextID).Scan(&count)
			Expect(err).NotTo(HaveOccurred())
			Expect(count).To(Equal(0))
		})

		It("should return 404 for non-existent membership", func() {
			contextID := support.NewTestContextID()
			userID := support.NewTestUUID()

			// Create context
			_, err := pool.Exec(ctx, `
				INSERT INTO contexts (id, slug, name, is_active, settings)
				VALUES ($1, $2, $3, $4, $5)
			`, contextID, "test-context", "Test Context", true, "{}")
			Expect(err).NotTo(HaveOccurred())

			// Don't create membership - try to delete it

			req := httptest.NewRequest("DELETE", "/admin/contexts/"+contextID+"/users/"+userID, nil)
			engine.ServeHTTP(w, req)

			Expect(w.Code).To(Equal(http.StatusNotFound))
		})

		It("should return 400 for invalid context ID", func() {
			userID := support.NewTestUUID()
			req := httptest.NewRequest("DELETE", "/admin/contexts/not-a-uuid/users/"+userID, nil)
			engine.ServeHTTP(w, req)

			Expect(w.Code).To(Equal(http.StatusBadRequest))
		})

		It("should return 400 for invalid user ID", func() {
			contextID := support.NewTestContextID()
			req := httptest.NewRequest("DELETE", "/admin/contexts/"+contextID+"/users/not-a-uuid", nil)
			engine.ServeHTTP(w, req)

			Expect(w.Code).To(Equal(http.StatusBadRequest))
		})
	})
})
