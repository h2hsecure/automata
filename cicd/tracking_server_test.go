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

var _ = Describe("Tracking Server Integration Tests", func() {
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
		_, err := pool.Exec(ctx, "DELETE FROM banner_clicks")
		Expect(err).NotTo(HaveOccurred())
		_, err = pool.Exec(ctx, "DELETE FROM banner_impressions")
		Expect(err).NotTo(HaveOccurred())
		_, err = pool.Exec(ctx, "DELETE FROM banner_banners")
		Expect(err).NotTo(HaveOccurred())
		_, err = pool.Exec(ctx, "DELETE FROM banner_campaigns")
		Expect(err).NotTo(HaveOccurred())
		_, err = pool.Exec(ctx, "DELETE FROM banner_placements")
		Expect(err).NotTo(HaveOccurred())
		_, err = pool.Exec(ctx, "DELETE FROM tracking_events")
		Expect(err).NotTo(HaveOccurred())
		_, err = pool.Exec(ctx, "DELETE FROM tracking_visitors")
		Expect(err).NotTo(HaveOccurred())
	})

	Describe("GET /health", func() {
		It("should return 200 with health status", func() {
			req := httptest.NewRequest("GET", "/health", nil)
			engine.ServeHTTP(w, req)

			Expect(w.Code).To(Equal(http.StatusOK))

			var response map[string]interface{}
			err := json.Unmarshal(w.Body.Bytes(), &response)
			Expect(err).NotTo(HaveOccurred())
			Expect(response["status"]).To(Equal("ok"))
		})
	})

		Describe("POST /track", func() {
			It("should track an event with X-Context-ID header", func() {
				tenantID := support.NewTestContextID()
				body := `{"type":"pageview","url":"/home","title":"Home Page"}`

				req := httptest.NewRequest("POST", "/track", bytes.NewBufferString(body))
				req.Header.Set("Content-Type", "application/json")
				req.Header.Set("X-Context-ID", tenantID)
				engine.ServeHTTP(w, req)

				Expect(w.Code).To(Equal(http.StatusNoContent))
		})

		It("should return 400 when X-Context-ID header is missing", func() {
			body := `{"type":"pageview","url":"/home"}`

			req := httptest.NewRequest("POST", "/track", bytes.NewBufferString(body))
			req.Header.Set("Content-Type", "application/json")
			req.Host = ""
			engine.ServeHTTP(w, req)

			Expect(w.Code).To(Equal(http.StatusBadRequest))
		})

		It("should track an event with subdomain resolution", func() {
			tenantID := support.NewTestContextID()
			body := `{"type":"pageview","url":"/home","title":"Home Page"}`

			req := httptest.NewRequest("POST", "/track", bytes.NewBufferString(body))
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("Host", tenantID+".example.com")
			engine.ServeHTTP(w, req)

			Expect(w.Code).To(Equal(http.StatusNoContent))
		})

		It("should reject www subdomain as context", func() {
			body := `{"type":"pageview","url":"/home"}`

			req := httptest.NewRequest("POST", "/track", bytes.NewBufferString(body))
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("Host", "www.example.com")
			engine.ServeHTTP(w, req)

			Expect(w.Code).To(Equal(http.StatusBadRequest))
		})
	})

	Describe("POST /track/batch", func() {
		It("should track multiple events in batch", func() {
			tenantID := support.NewTestContextID()
			body := `{"events":[
				{"type":"pageview","url":"/home","title":"Home"},
				{"type":"click","url":"/about","title":"About"}
			]}`

			req := httptest.NewRequest("POST", "/track/batch", bytes.NewBufferString(body))
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("X-Context-ID", tenantID)
			engine.ServeHTTP(w, req)

			Expect(w.Code).To(Equal(http.StatusNoContent))
		})
	})

	Describe("GET /track/banner/list", func() {
		It("should list banners for a context", func() {
			tenantID := support.NewTestContextID()
			placementID := support.NewTestUUID()
			bannerID := support.NewTestUUID()

			_, err := pool.Exec(ctx, `
				INSERT INTO banner_placements (id, context_id, name, location, css_selector, max_banners, priority, is_active, created_at, updated_at)
				VALUES ($1, $2, $3, $4, $5, $6, $7, $8, NOW(), NOW())
			`, placementID, tenantID, "Test Placement", "header", "#header", 1, 1, true)
			Expect(err).NotTo(HaveOccurred())

			_, err = pool.Exec(ctx, `
				INSERT INTO banner_banners (id, context_id, name, type, content, link_url, image_url, alt_text, campaign_id, placements, priority, is_active, created_at, updated_at)
				VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, NOW(), NOW())
			`, bannerID, tenantID, "Test Banner", "html", "<div>Test</div>", "https://example.com", "", "", nil, `["`+placementID+`"]`, 1, true)
			Expect(err).NotTo(HaveOccurred())

			req := httptest.NewRequest("GET", "/track/banner/list", nil)
			req.Header.Set("X-Context-ID", tenantID)
			engine.ServeHTTP(w, req)

			Expect(w.Code).To(Equal(http.StatusOK))
		})
	})

	Describe("GET /track/placement/:id", func() {
		It("should return a placement by ID", func() {
			tenantID := support.NewTestContextID()
			placementID := support.NewTestUUID()

			_, err := pool.Exec(ctx, `
				INSERT INTO banner_placements (id, context_id, name, location, css_selector, max_banners, priority, is_active, created_at, updated_at)
				VALUES ($1, $2, $3, $4, $5, $6, $7, $8, NOW(), NOW())
			`, placementID, tenantID, "Test Placement", "header", "#header", 1, 1, true)
			Expect(err).NotTo(HaveOccurred())

			req := httptest.NewRequest("GET", "/track/placement/"+placementID, nil)
			engine.ServeHTTP(w, req)

			Expect(w.Code).To(Equal(http.StatusOK))
		})
	})

	Describe("GET /track/campaign/:id", func() {
		It("should return a campaign by ID", func() {
			tenantID := support.NewTestContextID()
			campaignID := support.NewTestUUID()

			_, err := pool.Exec(ctx, `
				INSERT INTO banner_campaigns (id, context_id, name, description, start_date, end_date, is_active, target_url, tracking_code, created_at, updated_at)
				VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, NOW(), NOW())
			`, campaignID, tenantID, "Test Campaign", "A test", "2026-01-01", "2026-12-31", true, "https://example.com", "test_code")
			Expect(err).NotTo(HaveOccurred())

			req := httptest.NewRequest("GET", "/track/campaign/"+campaignID, nil)
			engine.ServeHTTP(w, req)

			Expect(w.Code).To(Equal(http.StatusOK))
		})
	})

	Describe("GET /snippet/:id.js", func() {
		It("should serve a snippet for any context ID", func() {
			req := httptest.NewRequest("GET", "/snippet/nonexistent.js", nil)
			engine.ServeHTTP(w, req)

			Expect(w.Code).To(Equal(http.StatusOK))
			Expect(w.Body.String()).To(ContainSubstring("nonexistent"))
		})
	})
})
