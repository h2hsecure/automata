package cicd_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"

	"github.com/gin-gonic/gin"
	"github.com/hekemen/automata/cicd/support"
	bhandler "github.com/hekemen/automata/internal/adapter/api/handler"
	bdomain "github.com/hekemen/automata/internal/domain/banner"
	brepo "github.com/hekemen/automata/internal/infrastructure/banner/repo"
	formRepo "github.com/hekemen/automata/internal/infrastructure/form/repo"
	ttracking "github.com/hekemen/automata/internal/adapter/tracking"
	tdomain "github.com/hekemen/automata/internal/domain/tracking"
	trepo "github.com/hekemen/automata/internal/infrastructure/tracking/repo"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("HTTP Integration Tests - Tracking", func() {
	var (
		w                *httptest.ResponseRecorder
		engine           *gin.Engine
		trackingRepo     tdomain.Repository
		bannerRepo       bdomain.Repository
		formHandler      *bhandler.FormHandler
		bannerHandler    *bhandler.BannerHandler
		placementHandler *bhandler.PlacementHandler
		campaignHandler  *bhandler.CampaignHandler
	)

	BeforeEach(func() {
		gin.SetMode(gin.TestMode)
		w = httptest.NewRecorder()
		trackingRepo = trepo.New(pool)
		bannerRepo = brepo.New(pool)
		formHandler = bhandler.NewFormHandler(formRepo.New(pool))
		bannerHandler = bhandler.NewBannerHandler(bannerRepo)
		placementHandler = bhandler.NewPlacementHandler(bannerRepo)
		campaignHandler = bhandler.NewCampaignHandler(bannerRepo)
		engine = ttracking.NewServer(formHandler, bannerHandler, placementHandler, campaignHandler, trackingRepo)
	})

	AfterEach(func() {
		_, _ = pool.Exec(ctx, "DELETE FROM banner_clicks")
		_, _ = pool.Exec(ctx, "DELETE FROM banner_impressions")
		_, _ = pool.Exec(ctx, "DELETE FROM banner_banners")
		_, _ = pool.Exec(ctx, "DELETE FROM banner_campaigns")
		_, _ = pool.Exec(ctx, "DELETE FROM banner_placements")
		_, _ = pool.Exec(ctx, "DELETE FROM tracking_events")
		_, _ = pool.Exec(ctx, "DELETE FROM tracking_visitors")
	})

	Describe("POST /track/batch", func() {
		It("should track multiple events in batch with correct response", func() {
			contextID := support.NewTestContextID()
			body := `{"events":[
				{"type":"pageview","url":"/home","title":"Home"},
				{"type":"click","url":"/about","title":"About"},
				{"type":"scroll","url":"/home","title":"Home"}
			]}`

			req := httptest.NewRequest("POST", "/track/batch", bytes.NewBufferString(body))
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("X-Context-ID", contextID)
			engine.ServeHTTP(w, req)

			Expect(w.Code).To(Equal(http.StatusNoContent))

			var count int
			err := pool.QueryRow(ctx, "SELECT COUNT(*) FROM tracking_events WHERE context_id = $1", contextID).Scan(&count)
			Expect(err).NotTo(HaveOccurred())
			Expect(count).To(Equal(3))
		})

		It("should return 204 for invalid batch payload", func() {
			body := `{"invalid": "payload"}`

			req := httptest.NewRequest("POST", "/track/batch", bytes.NewBufferString(body))
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("X-Context-ID", support.NewTestContextID())
			engine.ServeHTTP(w, req)

			Expect(w.Code).To(Equal(http.StatusNoContent))
		})

		It("should return 204 when X-Context-ID header is missing", func() {
			body := `{"events":[]}`

			req := httptest.NewRequest("POST", "/track/batch", bytes.NewBufferString(body))
			req.Header.Set("Content-Type", "application/json")
			engine.ServeHTTP(w, req)

			Expect(w.Code).To(Equal(http.StatusNoContent))
		})

		It("should handle empty events array", func() {
			body := `{"events":[]}`

			req := httptest.NewRequest("POST", "/track/batch", bytes.NewBufferString(body))
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("X-Context-ID", support.NewTestContextID())
			engine.ServeHTTP(w, req)

			Expect(w.Code).To(Equal(http.StatusNoContent))
		})
	})

	Describe("GET /track/banner/list", func() {
		It("should list banners for a context", func() {
			contextID := support.NewTestContextID()
			placementID := support.NewTestUUID()
			bannerID := support.NewTestUUID()

			_, err := pool.Exec(ctx, `
				INSERT INTO banner_placements (id, context_id, name, location, css_selector, max_banners, priority, is_active, created_at, updated_at)
				VALUES ($1, $2, $3, $4, $5, $6, $7, $8, NOW(), NOW())
			`, placementID, contextID, "Test Placement", "header", "#header", 1, 1, true)
			Expect(err).NotTo(HaveOccurred())

			_, err = pool.Exec(ctx, `
				INSERT INTO banner_banners (id, context_id, name, type, content, link_url, image_url, alt_text, campaign_id, placements, priority, is_active, created_at, updated_at)
				VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, NOW(), NOW())
			`, bannerID, contextID, "Test Banner", "html", "<div>Test</div>", "https://example.com", "", "", nil, `["`+placementID+`"]`, 1, true)
			Expect(err).NotTo(HaveOccurred())

			req := httptest.NewRequest("GET", "/track/banner/list", nil)
			req.Header.Set("X-Context-ID", contextID)
			engine.ServeHTTP(w, req)

			Expect(w.Code).To(Equal(http.StatusOK))

			var response map[string]interface{}
			err = json.Unmarshal(w.Body.Bytes(), &response)
			Expect(err).NotTo(HaveOccurred())
			Expect(response["banners"]).NotTo(BeNil())
		})

		It("should return empty list when no banners exist", func() {
			req := httptest.NewRequest("GET", "/track/banner/list", nil)
			req.Header.Set("X-Context-ID", support.NewTestContextID())
			engine.ServeHTTP(w, req)

			Expect(w.Code).To(Equal(http.StatusOK))

			var response map[string]interface{}
			err := json.Unmarshal(w.Body.Bytes(), &response)
			Expect(err).NotTo(HaveOccurred())
			Expect(response["banners"]).To(BeNil())
		})

		It("should filter banners by campaign_id", func() {
			contextID := support.NewTestContextID()
			campaignID := support.NewTestUUID()
			bannerID := support.NewTestUUID()

			_, err := pool.Exec(ctx, `
				INSERT INTO banner_campaigns (id, context_id, name, description, start_date, end_date, is_active, target_url, tracking_code, created_at, updated_at)
				VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, NOW(), NOW())
			`, campaignID, contextID, "Test Campaign", "desc", "2026-01-01", "2026-12-31", true, "https://example.com", "code")
			Expect(err).NotTo(HaveOccurred())

			_, err = pool.Exec(ctx, `
				INSERT INTO banner_banners (id, context_id, name, type, content, link_url, image_url, alt_text, campaign_id, placements, priority, is_active, created_at, updated_at)
				VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, NOW(), NOW())
			`, bannerID, contextID, "Test Banner", "html", "<div>Test</div>", "", "", "", campaignID, "[]", 1, true)
			Expect(err).NotTo(HaveOccurred())

			req := httptest.NewRequest("GET", "/track/banner/list?campaign_id="+campaignID, nil)
			req.Header.Set("X-Context-ID", contextID)
			engine.ServeHTTP(w, req)

			Expect(w.Code).To(Equal(http.StatusOK))
		})

		It("should filter banners by is_active", func() {
			contextID := support.NewTestContextID()
			bannerID := support.NewTestUUID()

			_, err := pool.Exec(ctx, `
				INSERT INTO banner_banners (id, context_id, name, type, content, link_url, image_url, alt_text, campaign_id, placements, priority, is_active, created_at, updated_at)
				VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, NOW(), NOW())
			`, bannerID, contextID, "Test Banner", "html", "<div>Test</div>", "", "", "", nil, "[]", 1, false)
			Expect(err).NotTo(HaveOccurred())

			req := httptest.NewRequest("GET", "/track/banner/list?is_active=false", nil)
			req.Header.Set("X-Context-ID", contextID)
			engine.ServeHTTP(w, req)

			Expect(w.Code).To(Equal(http.StatusOK))
		})
	})

	Describe("GET /track/banner/:id", func() {
		It("should return a banner by ID", func() {
			contextID := support.NewTestContextID()
			bannerID := support.NewTestUUID()

			_, err := pool.Exec(ctx, `
				INSERT INTO banner_banners (id, context_id, name, type, content, link_url, image_url, alt_text, campaign_id, placements, priority, is_active, created_at, updated_at)
				VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, NOW(), NOW())
			`, bannerID, contextID, "Test Banner", "html", "<div>Test</div>", "", "", "", nil, "[]", 1, true)
			Expect(err).NotTo(HaveOccurred())

			req := httptest.NewRequest("GET", "/track/banner/"+bannerID, nil)
			engine.ServeHTTP(w, req)

			Expect(w.Code).To(Equal(http.StatusOK))

			var response map[string]interface{}
			err = json.Unmarshal(w.Body.Bytes(), &response)
			Expect(err).NotTo(HaveOccurred())
			Expect(response["ID"]).To(Equal(bannerID))
		})

		It("should return 404 for non-existent banner", func() {
			req := httptest.NewRequest("GET", "/track/banner/nonexistent", nil)
			engine.ServeHTTP(w, req)

			Expect(w.Code).To(Equal(http.StatusNotFound))
		})
	})

	Describe("GET /track/placement/:id", func() {
		It("should return a placement by ID", func() {
			contextID := support.NewTestContextID()
			placementID := support.NewTestUUID()

			_, err := pool.Exec(ctx, `
				INSERT INTO banner_placements (id, context_id, name, location, css_selector, max_banners, priority, is_active, created_at, updated_at)
				VALUES ($1, $2, $3, $4, $5, $6, $7, $8, NOW(), NOW())
			`, placementID, contextID, "Test Placement", "header", "#header", 1, 1, true)
			Expect(err).NotTo(HaveOccurred())

			req := httptest.NewRequest("GET", "/track/placement/"+placementID, nil)
			engine.ServeHTTP(w, req)

			Expect(w.Code).To(Equal(http.StatusOK))

			var response map[string]interface{}
			err = json.Unmarshal(w.Body.Bytes(), &response)
			Expect(err).NotTo(HaveOccurred())
			Expect(response["ID"]).To(Equal(placementID))
		})

		It("should return 404 for non-existent placement", func() {
			req := httptest.NewRequest("GET", "/track/placement/nonexistent", nil)
			engine.ServeHTTP(w, req)

			Expect(w.Code).To(Equal(http.StatusNotFound))
		})
	})

	Describe("GET /track/campaign/:id", func() {
		It("should return a campaign by ID", func() {
			contextID := support.NewTestContextID()
			campaignID := support.NewTestUUID()

			_, err := pool.Exec(ctx, `
				INSERT INTO banner_campaigns (id, context_id, name, description, start_date, end_date, is_active, target_url, tracking_code, created_at, updated_at)
				VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, NOW(), NOW())
			`, campaignID, contextID, "Test Campaign", "A test", "2026-01-01", "2026-12-31", true, "https://example.com", "test_code")
			Expect(err).NotTo(HaveOccurred())

			req := httptest.NewRequest("GET", "/track/campaign/"+campaignID, nil)
			engine.ServeHTTP(w, req)

			Expect(w.Code).To(Equal(http.StatusOK))

			var response map[string]interface{}
			err = json.Unmarshal(w.Body.Bytes(), &response)
			Expect(err).NotTo(HaveOccurred())
			Expect(response["ID"]).To(Equal(campaignID))
		})

		It("should return 404 for non-existent campaign", func() {
			req := httptest.NewRequest("GET", "/track/campaign/nonexistent", nil)
			engine.ServeHTTP(w, req)

			Expect(w.Code).To(Equal(http.StatusNotFound))
		})
	})

	Describe("GET /snippet/:id.js", func() {
		It("should serve a snippet for any context ID", func() {
			req := httptest.NewRequest("GET", "/snippet/nonexistent.js", nil)
			engine.ServeHTTP(w, req)

			Expect(w.Code).To(Equal(http.StatusOK))
			Expect(w.Body.String()).To(ContainSubstring("nonexistent"))
		})

		It("should serve a snippet with valid context ID", func() {
			contextID := support.NewTestContextID()
			req := httptest.NewRequest("GET", "/snippet/"+contextID+".js", nil)
			engine.ServeHTTP(w, req)

			Expect(w.Code).To(Equal(http.StatusOK))
			Expect(w.Body.String()).To(ContainSubstring(contextID))
		})
	})
})
