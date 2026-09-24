package cicd_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"

	"github.com/gin-gonic/gin"
	"github.com/hekemen/automata/cicd/support"
	bhandler "github.com/hekemen/automata/internal/adapter/api/handler"
	brepo "github.com/hekemen/automata/internal/infrastructure/banner/repo"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func setupBannerEngine() *gin.Engine {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	bannerRepo := brepo.New(pool)
	bannerHandler := bhandler.NewBannerHandler(bannerRepo)
	placementHandler := bhandler.NewPlacementHandler(bannerRepo)
	campaignHandler := bhandler.NewCampaignHandler(bannerRepo)

	banners := engine.Group("/api/banners")
	{
		banners.POST("", bannerHandler.CreateBanner)
		banners.GET("", bannerHandler.ListBanners)
		banners.GET("/:id", bannerHandler.GetBanner)
		banners.PUT("/:id", bannerHandler.UpdateBanner)
		banners.DELETE("/:id", bannerHandler.DeleteBanner)
	}

	placements := engine.Group("/api/placements")
	{
		placements.POST("", placementHandler.CreatePlacement)
		placements.GET("", placementHandler.ListPlacements)
		placements.GET("/:id", placementHandler.GetPlacement)
		placements.PUT("/:id", placementHandler.UpdatePlacement)
		placements.DELETE("/:id", placementHandler.DeletePlacement)
	}

	campaigns := engine.Group("/api/campaigns")
	{
		campaigns.POST("", campaignHandler.CreateCampaign)
		campaigns.GET("", campaignHandler.ListCampaigns)
		campaigns.GET("/:id", campaignHandler.GetCampaign)
		campaigns.PUT("/:id", campaignHandler.UpdateCampaign)
		campaigns.DELETE("/:id", campaignHandler.DeleteCampaign)
	}

	return engine
}

var _ = Describe("HTTP Integration Tests - Banner CRUD", func() {
	var (
		w      *httptest.ResponseRecorder
		engine *gin.Engine
	)

	BeforeEach(func() {
		engine = setupBannerEngine()
		w = httptest.NewRecorder()
	})

	AfterEach(func() {
		_, _ = pool.Exec(ctx, "DELETE FROM banner_clicks")
		_, _ = pool.Exec(ctx, "DELETE FROM banner_impressions")
		_, _ = pool.Exec(ctx, "DELETE FROM banner_banners")
		_, _ = pool.Exec(ctx, "DELETE FROM banner_campaigns")
		_, _ = pool.Exec(ctx, "DELETE FROM banner_placements")
	})

	Describe("POST /api/banners", func() {
		It("should create a banner with valid request", func() {
			contextID := support.NewTestContextID()
			campaignID := support.NewTestUUID()
			body := fmt.Sprintf(`{
				"name": "Test Banner",
				"type": "html",
				"content": "<div>Test</div>",
				"link_url": "https://example.com",
				"campaign_id": "%s",
				"placements": ["header"],
				"priority": 1,
				"start_date": "2026-01-01T00:00:00Z",
				"end_date": "2026-12-31T23:59:59Z",
				"ab_test": false
			}`, campaignID)

			req := httptest.NewRequest("POST", "/api/banners", bytes.NewBufferString(body))
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("X-Context-ID", contextID)
			engine.ServeHTTP(w, req)

			Expect(w.Code).To(Equal(http.StatusCreated))

			var response map[string]interface{}
			err := json.Unmarshal(w.Body.Bytes(), &response)
			Expect(err).NotTo(HaveOccurred())
			Expect(response["Name"]).To(Equal("Test Banner"))
		})

		It("should return 400 when context ID is missing", func() {
			body := `{"name": "Test Banner"}`

			req := httptest.NewRequest("POST", "/api/banners", bytes.NewBufferString(body))
			req.Header.Set("Content-Type", "application/json")
			engine.ServeHTTP(w, req)

			Expect(w.Code).To(Equal(http.StatusBadRequest))
		})

		It("should return 400 when name is missing", func() {
			contextID := support.NewTestContextID()
			body := `{"type": "html"}`

			req := httptest.NewRequest("POST", "/api/banners", bytes.NewBufferString(body))
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("X-Context-ID", contextID)
			engine.ServeHTTP(w, req)

			Expect(w.Code).To(Equal(http.StatusBadRequest))
		})
	})

	Describe("GET /api/banners", func() {
		It("should list banners with pagination", func() {
			contextID := support.NewTestContextID()
			bannerID := support.NewTestUUID()

			_, err := pool.Exec(ctx, "INSERT INTO banner_banners (id, context_id, name, type, content, link_url, image_url, alt_text, campaign_id, placements, priority, start_date, end_date, is_active, ab_test, ab_variants, impressions, clicks, created_at, updated_at) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, NULL, NULL, $12, false, '[]', 0, 0, NOW(), NOW())", bannerID, contextID, "Test Banner", "html", "<div>Test</div>", "", "", "", nil, "[]", 1, true)
			Expect(err).NotTo(HaveOccurred())

			req := httptest.NewRequest("GET", "/api/banners?offset=0&limit=10", nil)
			req.Header.Set("X-Context-ID", contextID)
			engine.ServeHTTP(w, req)

			Expect(w.Code).To(Equal(http.StatusOK))

			var response map[string]interface{}
			err = json.Unmarshal(w.Body.Bytes(), &response)
			Expect(err).NotTo(HaveOccurred())
			Expect(response["banners"]).NotTo(BeNil())
			Expect(response["total"]).NotTo(BeNil())
		})

		It("should return 400 when context ID is missing", func() {
			req := httptest.NewRequest("GET", "/api/banners", nil)
			engine.ServeHTTP(w, req)

			Expect(w.Code).To(Equal(http.StatusBadRequest))
		})

		It("should return empty list when no banners exist", func() {
			req := httptest.NewRequest("GET", "/api/banners", nil)
			req.Header.Set("X-Context-ID", support.NewTestContextID())
			engine.ServeHTTP(w, req)

			Expect(w.Code).To(Equal(http.StatusOK))
		})
	})

	Describe("GET /api/banners/:id", func() {
		It("should return a banner by ID", func() {
			contextID := support.NewTestContextID()
			bannerID := support.NewTestUUID()

			_, err := pool.Exec(ctx, "INSERT INTO banner_banners (id, context_id, name, type, content, link_url, image_url, alt_text, campaign_id, placements, priority, start_date, end_date, is_active, ab_test, ab_variants, impressions, clicks, created_at, updated_at) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, NULL, NULL, $12, false, '[]', 0, 0, NOW(), NOW())", bannerID, contextID, "Test Banner", "html", "<div>Test</div>", "", "", "", nil, "[]", 1, true)
			Expect(err).NotTo(HaveOccurred())

			req := httptest.NewRequest("GET", "/api/banners/"+bannerID, nil)
			engine.ServeHTTP(w, req)

			Expect(w.Code).To(Equal(http.StatusOK))

			var response map[string]interface{}
			err = json.Unmarshal(w.Body.Bytes(), &response)
			Expect(err).NotTo(HaveOccurred())
			Expect(response["ID"]).To(Equal(bannerID))
		})

		It("should return 404 for non-existent banner", func() {
			req := httptest.NewRequest("GET", "/api/banners/nonexistent", nil)
			engine.ServeHTTP(w, req)

			Expect(w.Code).To(Equal(http.StatusNotFound))
		})
	})

	Describe("PUT /api/banners/:id", func() {
		It("should update a banner", func() {
			contextID := support.NewTestContextID()
			bannerID := support.NewTestUUID()

			_, err := pool.Exec(ctx, "INSERT INTO banner_banners (id, context_id, name, type, content, link_url, image_url, alt_text, campaign_id, placements, priority, start_date, end_date, is_active, ab_test, ab_variants, impressions, clicks, created_at, updated_at) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, NULL, NULL, $12, false, '[]', 0, 0, NOW(), NOW())", bannerID, contextID, "Old Name", "html", "<div>Test</div>", "", "", "", nil, "[]", 1, true)
			Expect(err).NotTo(HaveOccurred())

			body := `{"name": "Updated Banner", "is_active": true}`

			req := httptest.NewRequest("PUT", "/api/banners/"+bannerID, bytes.NewBufferString(body))
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("X-Context-ID", contextID)
			engine.ServeHTTP(w, req)

			Expect(w.Code).To(Equal(http.StatusOK))

			var response map[string]interface{}
			err = json.Unmarshal(w.Body.Bytes(), &response)
			Expect(err).NotTo(HaveOccurred())
			Expect(response["Name"]).To(Equal("Updated Banner"))
		})

		It("should return 400 when context ID is missing", func() {
			bannerID := support.NewTestUUID()
			body := `{"name": "Updated"}`

			req := httptest.NewRequest("PUT", "/api/banners/"+bannerID, bytes.NewBufferString(body))
			req.Header.Set("Content-Type", "application/json")
			engine.ServeHTTP(w, req)

			Expect(w.Code).To(Equal(http.StatusBadRequest))
		})
	})

	Describe("DELETE /api/banners/:id", func() {
		It("should delete a banner", func() {
			contextID := support.NewTestContextID()
			bannerID := support.NewTestUUID()

			_, err := pool.Exec(ctx, "INSERT INTO banner_banners (id, context_id, name, type, content, link_url, image_url, alt_text, campaign_id, placements, priority, start_date, end_date, is_active, ab_test, ab_variants, impressions, clicks, created_at, updated_at) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, NULL, NULL, $12, false, '[]', 0, 0, NOW(), NOW())", bannerID, contextID, "Test Banner", "html", "<div>Test</div>", "", "", "", nil, "[]", 1, true)
			Expect(err).NotTo(HaveOccurred())

			req := httptest.NewRequest("DELETE", "/api/banners/"+bannerID, nil)
			engine.ServeHTTP(w, req)

			Expect(w.Code).To(Equal(http.StatusOK))

			// Verify banner was deleted
			var count int
			err = pool.QueryRow(ctx, "SELECT COUNT(*) FROM banner_banners WHERE id = $1", bannerID).Scan(&count)
			Expect(err).NotTo(HaveOccurred())
			Expect(count).To(Equal(0))
		})
	})
})

var _ = Describe("HTTP Integration Tests - Placement CRUD", func() {
	var (
		w      *httptest.ResponseRecorder
		engine *gin.Engine
	)

	BeforeEach(func() {
		engine = setupBannerEngine()
		w = httptest.NewRecorder()
	})

	AfterEach(func() {
		_, _ = pool.Exec(ctx, "DELETE FROM banner_placements")
	})

	Describe("POST /api/placements", func() {
		It("should create a placement with valid request", func() {
			contextID := support.NewTestContextID()
			body := `{
				"name": "Test Placement",
				"location": "header",
				"css_selector": "#header",
				"max_banners": 3,
				"priority": 1
			}`

			req := httptest.NewRequest("POST", "/api/placements", bytes.NewBufferString(body))
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("X-Context-ID", contextID)
			engine.ServeHTTP(w, req)

			Expect(w.Code).To(Equal(http.StatusCreated))

			var response map[string]interface{}
			err := json.Unmarshal(w.Body.Bytes(), &response)
			Expect(err).NotTo(HaveOccurred())
			Expect(response["Name"]).To(Equal("Test Placement"))
		})

		It("should return 400 when context ID is missing", func() {
			body := `{"name": "Test"}`

			req := httptest.NewRequest("POST", "/api/placements", bytes.NewBufferString(body))
			req.Header.Set("Content-Type", "application/json")
			engine.ServeHTTP(w, req)

			Expect(w.Code).To(Equal(http.StatusBadRequest))
		})

		It("should return 400 when name is missing", func() {
			contextID := support.NewTestContextID()
			body := `{"css_selector": "#header"}`

			req := httptest.NewRequest("POST", "/api/placements", bytes.NewBufferString(body))
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("X-Context-ID", contextID)
			engine.ServeHTTP(w, req)

			Expect(w.Code).To(Equal(http.StatusBadRequest))
		})
	})

	Describe("GET /api/placements", func() {
		It("should list placements with filters", func() {
			contextID := support.NewTestContextID()
			placementID := support.NewTestUUID()

			_, err := pool.Exec(ctx, "INSERT INTO banner_placements (id, context_id, name, location, css_selector, max_banners, priority, is_active, created_at, updated_at) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, NOW(), NOW())", placementID, contextID, "Test Placement", "header", "#header", 1, 1, true)
			Expect(err).NotTo(HaveOccurred())

			req := httptest.NewRequest("GET", "/api/placements?is_active=true", nil)
			req.Header.Set("X-Context-ID", contextID)
			engine.ServeHTTP(w, req)

			Expect(w.Code).To(Equal(http.StatusOK))

			var response map[string]interface{}
			err = json.Unmarshal(w.Body.Bytes(), &response)
			Expect(err).NotTo(HaveOccurred())
			Expect(response["placements"]).NotTo(BeNil())
		})

		It("should return 400 when context ID is missing", func() {
			req := httptest.NewRequest("GET", "/api/placements", nil)
			engine.ServeHTTP(w, req)

			Expect(w.Code).To(Equal(http.StatusBadRequest))
		})
	})

	Describe("GET /api/placements/:id", func() {
		It("should return a placement by ID", func() {
			contextID := support.NewTestContextID()
			placementID := support.NewTestUUID()

			_, err := pool.Exec(ctx, "INSERT INTO banner_placements (id, context_id, name, location, css_selector, max_banners, priority, is_active, created_at, updated_at) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, NOW(), NOW())", placementID, contextID, "Test Placement", "header", "#header", 1, 1, true)
			Expect(err).NotTo(HaveOccurred())

			req := httptest.NewRequest("GET", "/api/placements/"+placementID, nil)
			engine.ServeHTTP(w, req)

			Expect(w.Code).To(Equal(http.StatusOK))

			var response map[string]interface{}
			err = json.Unmarshal(w.Body.Bytes(), &response)
			Expect(err).NotTo(HaveOccurred())
			Expect(response["ID"]).To(Equal(placementID))
		})

		It("should return 404 for non-existent placement", func() {
			req := httptest.NewRequest("GET", "/api/placements/nonexistent", nil)
			engine.ServeHTTP(w, req)

			Expect(w.Code).To(Equal(http.StatusNotFound))
		})
	})

	Describe("PUT /api/placements/:id", func() {
		It("should update a placement", func() {
			contextID := support.NewTestContextID()
			placementID := support.NewTestUUID()

			_, err := pool.Exec(ctx, "INSERT INTO banner_placements (id, context_id, name, location, css_selector, max_banners, priority, is_active, created_at, updated_at) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, NOW(), NOW())", placementID, contextID, "Old Name", "header", "#header", 1, 1, true)
			Expect(err).NotTo(HaveOccurred())

			body := `{"name": "Updated Placement", "is_active": false}`

			req := httptest.NewRequest("PUT", "/api/placements/"+placementID, bytes.NewBufferString(body))
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("X-Context-ID", contextID)
			engine.ServeHTTP(w, req)

			Expect(w.Code).To(Equal(http.StatusOK))

			var response map[string]interface{}
			err = json.Unmarshal(w.Body.Bytes(), &response)
			Expect(err).NotTo(HaveOccurred())
			Expect(response["Name"]).To(Equal("Updated Placement"))
		})

		It("should return 400 when context ID is missing", func() {
			placementID := support.NewTestUUID()
			body := `{"name": "Updated"}`

			req := httptest.NewRequest("PUT", "/api/placements/"+placementID, bytes.NewBufferString(body))
			req.Header.Set("Content-Type", "application/json")
			engine.ServeHTTP(w, req)

			Expect(w.Code).To(Equal(http.StatusBadRequest))
		})
	})

	Describe("DELETE /api/placements/:id", func() {
		It("should delete a placement", func() {
			contextID := support.NewTestContextID()
			placementID := support.NewTestUUID()

			_, err := pool.Exec(ctx, "INSERT INTO banner_placements (id, context_id, name, location, css_selector, max_banners, priority, is_active, created_at, updated_at) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, NOW(), NOW())", placementID, contextID, "Test Placement", "header", "#header", 1, 1, true)
			Expect(err).NotTo(HaveOccurred())

			req := httptest.NewRequest("DELETE", "/api/placements/"+placementID, nil)
			engine.ServeHTTP(w, req)

			Expect(w.Code).To(Equal(http.StatusOK))

			// Verify placement was deleted
			var count int
			err = pool.QueryRow(ctx, "SELECT COUNT(*) FROM banner_placements WHERE id = $1", placementID).Scan(&count)
			Expect(err).NotTo(HaveOccurred())
			Expect(count).To(Equal(0))
		})
	})
})

var _ = Describe("HTTP Integration Tests - Campaign CRUD", func() {
	var (
		w      *httptest.ResponseRecorder
		engine *gin.Engine
	)

	BeforeEach(func() {
		engine = setupBannerEngine()
		w = httptest.NewRecorder()
	})

	AfterEach(func() {
		_, _ = pool.Exec(ctx, "DELETE FROM banner_campaigns")
	})

	Describe("POST /api/campaigns", func() {
		It("should create a campaign with valid request", func() {
			contextID := support.NewTestContextID()
			body := `{
				"name": "Test Campaign",
				"description": "A test campaign",
				"start_date": "2026-01-01T00:00:00Z",
				"end_date": "2026-12-31T23:59:59Z",
				"target_url": "https://example.com",
				"tracking_code": "test_campaign"
			}`

			req := httptest.NewRequest("POST", "/api/campaigns", bytes.NewBufferString(body))
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("X-Context-ID", contextID)
			engine.ServeHTTP(w, req)

			Expect(w.Code).To(Equal(http.StatusCreated))

			var response map[string]interface{}
			err := json.Unmarshal(w.Body.Bytes(), &response)
			Expect(err).NotTo(HaveOccurred())
			Expect(response["Name"]).To(Equal("Test Campaign"))
		})

		It("should return 400 when context ID is missing", func() {
			body := `{"name": "Test"}`

			req := httptest.NewRequest("POST", "/api/campaigns", bytes.NewBufferString(body))
			req.Header.Set("Content-Type", "application/json")
			engine.ServeHTTP(w, req)

			Expect(w.Code).To(Equal(http.StatusBadRequest))
		})

		It("should return 400 when name is missing", func() {
			contextID := support.NewTestContextID()
			body := `{"description": "desc"}`

			req := httptest.NewRequest("POST", "/api/campaigns", bytes.NewBufferString(body))
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("X-Context-ID", contextID)
			engine.ServeHTTP(w, req)

			Expect(w.Code).To(Equal(http.StatusBadRequest))
		})
	})

	Describe("GET /api/campaigns", func() {
		It("should list campaigns with filters", func() {
			contextID := support.NewTestContextID()
			campaignID := support.NewTestUUID()

			_, err := pool.Exec(ctx, "INSERT INTO banner_campaigns (id, context_id, name, description, start_date, end_date, is_active, target_url, tracking_code, impressions, clicks, conversion_rate, created_at, updated_at) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, 0, 0, 0.0, NOW(), NOW())", campaignID, contextID, "Test Campaign", "desc", nil, nil, true, "https://example.com", "code")
			Expect(err).NotTo(HaveOccurred())

			req := httptest.NewRequest("GET", "/api/campaigns?is_active=true", nil)
			req.Header.Set("X-Context-ID", contextID)
			engine.ServeHTTP(w, req)

			Expect(w.Code).To(Equal(http.StatusOK))

			var response map[string]interface{}
			err = json.Unmarshal(w.Body.Bytes(), &response)
			Expect(err).NotTo(HaveOccurred())
			Expect(response["campaigns"]).NotTo(BeNil())
		})

		It("should return 400 when context ID is missing", func() {
			req := httptest.NewRequest("GET", "/api/campaigns", nil)
			engine.ServeHTTP(w, req)

			Expect(w.Code).To(Equal(http.StatusBadRequest))
		})
	})

	Describe("GET /api/campaigns/:id", func() {
		It("should return a campaign by ID", func() {
			contextID := support.NewTestContextID()
			campaignID := support.NewTestUUID()

			_, err := pool.Exec(ctx, "INSERT INTO banner_campaigns (id, context_id, name, description, start_date, end_date, is_active, target_url, tracking_code, impressions, clicks, conversion_rate, created_at, updated_at) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, 0, 0, 0.0, NOW(), NOW())", campaignID, contextID, "Test Campaign", "desc", nil, nil, true, "https://example.com", "code")
			Expect(err).NotTo(HaveOccurred())

			req := httptest.NewRequest("GET", "/api/campaigns/"+campaignID, nil)
			engine.ServeHTTP(w, req)

			Expect(w.Code).To(Equal(http.StatusOK))

			var response map[string]interface{}
			err = json.Unmarshal(w.Body.Bytes(), &response)
			Expect(err).NotTo(HaveOccurred())
			Expect(response["ID"]).To(Equal(campaignID))
		})

		It("should return 404 for non-existent campaign", func() {
			req := httptest.NewRequest("GET", "/api/campaigns/nonexistent", nil)
			engine.ServeHTTP(w, req)

			Expect(w.Code).To(Equal(http.StatusNotFound))
		})
	})

	Describe("PUT /api/campaigns/:id", func() {
		It("should update a campaign", func() {
			contextID := support.NewTestContextID()
			campaignID := support.NewTestUUID()

			_, err := pool.Exec(ctx, "INSERT INTO banner_campaigns (id, context_id, name, description, start_date, end_date, is_active, target_url, tracking_code, impressions, clicks, conversion_rate, created_at, updated_at) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, 0, 0, 0.0, NOW(), NOW())", campaignID, contextID, "Old Campaign", "desc", nil, nil, true, "https://example.com", "code")
			Expect(err).NotTo(HaveOccurred())

			body := `{"name": "Updated Campaign", "is_active": false}`

			req := httptest.NewRequest("PUT", "/api/campaigns/"+campaignID, bytes.NewBufferString(body))
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("X-Context-ID", contextID)
			engine.ServeHTTP(w, req)

			Expect(w.Code).To(Equal(http.StatusOK))

			var response map[string]interface{}
			err = json.Unmarshal(w.Body.Bytes(), &response)
			Expect(err).NotTo(HaveOccurred())
			Expect(response["Name"]).To(Equal("Updated Campaign"))
		})

		It("should return 400 when context ID is missing", func() {
			campaignID := support.NewTestUUID()
			body := `{"name": "Updated"}`

			req := httptest.NewRequest("PUT", "/api/campaigns/"+campaignID, bytes.NewBufferString(body))
			req.Header.Set("Content-Type", "application/json")
			engine.ServeHTTP(w, req)

			Expect(w.Code).To(Equal(http.StatusBadRequest))
		})
	})

	Describe("DELETE /api/campaigns/:id", func() {
		It("should delete a campaign", func() {
			contextID := support.NewTestContextID()
			campaignID := support.NewTestUUID()

			_, err := pool.Exec(ctx, "INSERT INTO banner_campaigns (id, context_id, name, description, start_date, end_date, is_active, target_url, tracking_code, impressions, clicks, conversion_rate, created_at, updated_at) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, 0, 0, 0.0, NOW(), NOW())", campaignID, contextID, "Test Campaign", "desc", nil, nil, true, "https://example.com", "code")
			Expect(err).NotTo(HaveOccurred())

			req := httptest.NewRequest("DELETE", "/api/campaigns/"+campaignID, nil)
			engine.ServeHTTP(w, req)

			Expect(w.Code).To(Equal(http.StatusOK))

			// Verify campaign was deleted
			var count int
			err = pool.QueryRow(ctx, "SELECT COUNT(*) FROM banner_campaigns WHERE id = $1", campaignID).Scan(&count)
			Expect(err).NotTo(HaveOccurred())
			Expect(count).To(Equal(0))
		})
	})
})
