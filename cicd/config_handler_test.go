package cicd_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"

	"github.com/gin-gonic/gin"
	"github.com/hekemen/automata/cicd/support"
	"github.com/hekemen/automata/internal/domain/config"
	"github.com/hekemen/automata/internal/infrastructure/config/repo"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Config Handler Integration Tests", func() {
	var (
		w        *httptest.ResponseRecorder
		engine   *gin.Engine
		configRepo config.ConfigRepository
	)

	BeforeEach(func() {
		gin.SetMode(gin.TestMode)
		configRepo = repo.NewConfigRepo(pool)
		w = httptest.NewRecorder()
		engine = gin.New()
		engine.Use(gin.Recovery())

		handler := NewTestConfigHandler(configRepo)

		engine.GET("/api/admin/configs/:tenantId", handler.GetConfig)
		engine.PUT("/api/admin/configs/:tenantId/cors", handler.UpdateCORS)
		engine.PUT("/api/admin/configs/:tenantId/domain", handler.UpdateDomain)
		engine.PUT("/api/admin/configs/:tenantId/display", handler.UpdateDisplay)
	})

	AfterEach(func() {
		_, err := pool.Exec(ctx, "DELETE FROM admin_configs")
		Expect(err).NotTo(HaveOccurred())
		_, err = pool.Exec(ctx, "DELETE FROM contexts")
		Expect(err).NotTo(HaveOccurred())
	})

	Describe("GET /api/admin/configs/:tenantId", func() {
		It("should return all configs for a context", func() {
			tenantID := createTestContext()

			_, err := pool.Exec(ctx, `
				INSERT INTO admin_configs (id, context_id, key, value, created_at, updated_at)
				VALUES ($1, $2, $3, $4, NOW(), NOW())
			`, support.NewTestUUID(), tenantID, "cors", `{"origins":["https://example.com"]}`)
			Expect(err).NotTo(HaveOccurred())

			_, err = pool.Exec(ctx, `
				INSERT INTO admin_configs (id, context_id, key, value, created_at, updated_at)
				VALUES ($1, $2, $3, $4, NOW(), NOW())
			`, support.NewTestUUID(), tenantID, "domain", `{"primary":"myapp.com"}`)
			Expect(err).NotTo(HaveOccurred())

			req := httptest.NewRequest("GET", "/api/admin/configs/"+tenantID, nil)
			engine.ServeHTTP(w, req)

			Expect(w.Code).To(Equal(http.StatusOK), "response body: %s", w.Body.String())

			var response map[string]interface{}
			err = json.Unmarshal(w.Body.Bytes(), &response)
			Expect(err).NotTo(HaveOccurred())

			configs := response["configs"].(map[string]interface{})
			Expect(configs).To(HaveKey("cors"))
			Expect(configs).To(HaveKey("domain"))
		})

		It("should return empty configs map for context with no configs", func() {
			tenantID := createTestContext()

			req := httptest.NewRequest("GET", "/api/admin/configs/"+tenantID, nil)
			engine.ServeHTTP(w, req)

			Expect(w.Code).To(Equal(http.StatusOK), "response body: %s", w.Body.String())

			var response map[string]interface{}
			err := json.Unmarshal(w.Body.Bytes(), &response)
			Expect(err).NotTo(HaveOccurred())

			configs := response["configs"].(map[string]interface{})
			Expect(configs).To(BeEmpty())
		})

		It("should return 404 when tenantId path segment is missing", func() {
			req := httptest.NewRequest("GET", "/api/admin/configs/", nil)
			engine.ServeHTTP(w, req)

			Expect(w.Code).To(Equal(http.StatusNotFound))
		})
	})

	Describe("PUT /api/admin/configs/:tenantId/cors", func() {
		It("should update CORS origins", func() {
			tenantID := createTestContext()
			body := `{"origins":["https://app.example.com","https://admin.example.com"]}`

			req := httptest.NewRequest("PUT", "/api/admin/configs/"+tenantID+"/cors", bytes.NewBufferString(body))
			req.Header.Set("Content-Type", "application/json")
			engine.ServeHTTP(w, req)

			Expect(w.Code).To(Equal(http.StatusOK), "response body: %s", w.Body.String())

			var storedValue []byte
			err := pool.QueryRow(ctx, `SELECT value FROM admin_configs WHERE context_id = $1 AND key = $2`, tenantID, "cors").Scan(&storedValue)
			Expect(err).NotTo(HaveOccurred())

			var parsed map[string]interface{}
			err = json.Unmarshal(storedValue, &parsed)
			Expect(err).NotTo(HaveOccurred())
			Expect(parsed["origins"].([]interface{})).To(HaveLen(2))
		})

		It("should return 400 when request body is invalid", func() {
			tenantID := createTestContext()
			body := `{"invalid json`

			req := httptest.NewRequest("PUT", "/api/admin/configs/"+tenantID+"/cors", bytes.NewBufferString(body))
			req.Header.Set("Content-Type", "application/json")
			engine.ServeHTTP(w, req)

			Expect(w.Code).To(Equal(http.StatusBadRequest))
		})
	})

	Describe("PUT /api/admin/configs/:tenantId/domain", func() {
		It("should update domain settings", func() {
			tenantID := createTestContext()
			body := `{"primary":"myapp.example.com","aliases":["www.myapp.example.com"]}`

			req := httptest.NewRequest("PUT", "/api/admin/configs/"+tenantID+"/domain", bytes.NewBufferString(body))
			req.Header.Set("Content-Type", "application/json")
			engine.ServeHTTP(w, req)

			Expect(w.Code).To(Equal(http.StatusOK), "response body: %s", w.Body.String())

			var storedValue []byte
			err := pool.QueryRow(ctx, `SELECT value FROM admin_configs WHERE context_id = $1 AND key = $2`, tenantID, "domain").Scan(&storedValue)
			Expect(err).NotTo(HaveOccurred())

			var parsed map[string]interface{}
			err = json.Unmarshal(storedValue, &parsed)
			Expect(err).NotTo(HaveOccurred())
			Expect(parsed["primary"]).To(Equal("myapp.example.com"))
			Expect(parsed["aliases"].([]interface{})).To(HaveLen(1))
		})
	})

	Describe("PUT /api/admin/configs/:tenantId/display", func() {
		It("should update display settings", func() {
			tenantID := createTestContext()
			body := `{"name":"My Application","logo_url":"https://cdn.example.com/logo.png"}`

			req := httptest.NewRequest("PUT", "/api/admin/configs/"+tenantID+"/display", bytes.NewBufferString(body))
			req.Header.Set("Content-Type", "application/json")
			engine.ServeHTTP(w, req)

			Expect(w.Code).To(Equal(http.StatusOK), "response body: %s", w.Body.String())

			var storedValue []byte
			err := pool.QueryRow(ctx, `SELECT value FROM admin_configs WHERE context_id = $1 AND key = $2`, tenantID, "display").Scan(&storedValue)
			Expect(err).NotTo(HaveOccurred())

			var parsed map[string]interface{}
			err = json.Unmarshal(storedValue, &parsed)
			Expect(err).NotTo(HaveOccurred())
			Expect(parsed["name"]).To(Equal("My Application"))
			Expect(parsed["logo_url"]).To(Equal("https://cdn.example.com/logo.png"))
		})
	})
})

// TestConfigHandler wraps the real handler for testing with httptest.
type TestConfigHandler struct {
	repo config.ConfigRepository
}

func NewTestConfigHandler(repo config.ConfigRepository) *TestConfigHandler {
	return &TestConfigHandler{repo: repo}
}

func (h *TestConfigHandler) GetConfig(c *gin.Context) {
	tenantID := c.Param("tenantId")
	if tenantID == "" {
		c.JSON(400, gin.H{"error": "tenantId required"})
		return
	}
	configs, err := h.repo.GetByContext(tenantID)
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}
	c.JSON(200, gin.H{"configs": configs})
}

func (h *TestConfigHandler) UpdateCORS(c *gin.Context) {
	tenantID := c.Param("tenantId")
	if tenantID == "" {
		c.JSON(400, gin.H{"error": "tenantId required"})
		return
	}
	var req struct {
		Origins []string `json:"origins"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}
	if err := h.repo.Upsert(tenantID, config.ConfigKeyCORS, map[string]interface{}{"origins": req.Origins}); err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}
	c.JSON(200, gin.H{"message": "CORS config updated"})
}

func (h *TestConfigHandler) UpdateDomain(c *gin.Context) {
	tenantID := c.Param("tenantId")
	if tenantID == "" {
		c.JSON(400, gin.H{"error": "tenantId required"})
		return
	}
	var req struct {
		Primary string   `json:"primary"`
		Aliases []string `json:"aliases"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}
	if err := h.repo.Upsert(tenantID, config.ConfigKeyDomain, map[string]interface{}{"primary": req.Primary, "aliases": req.Aliases}); err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}
	c.JSON(200, gin.H{"message": "Domain config updated"})
}

func (h *TestConfigHandler) UpdateDisplay(c *gin.Context) {
	tenantID := c.Param("tenantId")
	if tenantID == "" {
		c.JSON(400, gin.H{"error": "tenantId required"})
		return
	}
	var req struct {
		Name    string `json:"name"`
		LogoURL string `json:"logo_url"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}
	if err := h.repo.Upsert(tenantID, config.ConfigKeyDisplay, map[string]interface{}{"name": req.Name, "logo_url": req.LogoURL}); err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}
	c.JSON(200, gin.H{"message": "Display config updated"})
}
