package cicd_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"

	"github.com/gin-gonic/gin"
	"github.com/h2hsecure/automata/cicd/support"
	bhandler "github.com/h2hsecure/automata/internal/adapter/api/handler"
	"github.com/h2hsecure/automata/internal/adapter/tracking"
	bdomain "github.com/h2hsecure/automata/internal/domain/banner"
	pagedomain "github.com/h2hsecure/automata/internal/domain/page"
	brepo "github.com/h2hsecure/automata/internal/infrastructure/banner/repo"
	formrepo "github.com/h2hsecure/automata/internal/infrastructure/form/repo"
	pagerepo "github.com/h2hsecure/automata/internal/infrastructure/page/repo"
	trepo "github.com/h2hsecure/automata/internal/infrastructure/tracking/repo"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Banner + pages end-to-end", func() {
	var (
		w          *httptest.ResponseRecorder
		engine     *gin.Engine
		bannerRepo bdomain.Repository
		pageRepo   pagedomain.Repository
		contextID  string
		bannerID   string
	)

	BeforeEach(func() {
		gin.SetMode(gin.TestMode)
		w = httptest.NewRecorder()
		bannerRepo = brepo.New(pool)
		pageRepo = pagerepo.New(pool)

		contextID = support.NewTestContextID()

		// Page: cookie policy.
		page := &pagedomain.Page{
			ID:        support.NewTestUUID(),
			ContextID: contextID,
			Slug:      "cookie-policy",
			Title:     "Cookie Policy",
			Content:   "<p>Cookie text</p>",
			IsActive:  true,
		}
		Expect(pageRepo.Create(page)).To(Succeed())

		// Fullpage banner referencing the page from a button.
		b := &bdomain.Banner{
			ContextID: contextID,
			Name:      "Cookie banner",
			Type:      bdomain.BannerTypeHTML,
			Content:   "We use cookies",
			Position:  "fullpage",
			IsActive:  true,
			Buttons: []bdomain.Button{
				{ID: "b1", Label: "Accept", Action: "accept", Dismisses: true},
				{ID: "b2", Label: "Policy", LinkURL: "/cookie-policy"},
			},
		}
		Expect(bannerRepo.CreateBanner(b)).To(Succeed())
		bannerID = b.ID

		// Tracking server plus the page and banner public routes.
		engine = tracking.NewServer(
			bhandler.NewFormHandler(formrepo.New(pool)),
			bhandler.NewBannerHandler(bannerRepo),
			bhandler.NewPlacementHandler(bannerRepo),
			bhandler.NewCampaignHandler(bannerRepo),
			trepo.New(pool),
			nil,
		)
		tracking.RegisterPageRoutes(engine, tracking.NewPageServeHandler(pageRepo))
		tracking.RegisterBannerPublicRoutes(engine, tracking.NewBannerPublicHandler(bannerRepo, pageRepo))
	})

	AfterEach(func() {
		_, _ = pool.Exec(ctx, "DELETE FROM banner_decisions")
		_, _ = pool.Exec(ctx, "DELETE FROM banner_impressions")
		_, _ = pool.Exec(ctx, "DELETE FROM banner_banners")
		_, _ = pool.Exec(ctx, "DELETE FROM page_pages")
	})

	It("serves the page, banner config with inlined page, and records decisions", func() {
		// 1. Page is served as a minimal HTML document.
		req := httptest.NewRequest(http.MethodGet, "/page/"+contextID+"/cookie-policy", nil)
		w = httptest.NewRecorder()
		engine.ServeHTTP(w, req)
		Expect(w.Code).To(Equal(http.StatusOK))
		Expect(w.Body.String()).To(ContainSubstring("Cookie Policy"))
		Expect(w.Body.String()).To(ContainSubstring("Cookie text"))

		// 2. Banner config inlines the referenced page and records an impression.
		req = httptest.NewRequest(http.MethodGet, "/banner/config/"+contextID+"?visitor_id=v1", nil)
		w = httptest.NewRecorder()
		engine.ServeHTTP(w, req)
		Expect(w.Code).To(Equal(http.StatusOK))

		var resp struct {
			Banners []struct {
				ID       string  `json:"id"`
				Position string  `json:"position"`
				Content  string  `json:"content"`
				Page     *string `json:"page"`
			} `json:"banners"`
		}
		Expect(json.Unmarshal(w.Body.Bytes(), &resp)).To(Succeed())
		Expect(resp.Banners).To(HaveLen(1))
		Expect(resp.Banners[0].ID).To(Equal(bannerID))
		Expect(resp.Banners[0].Position).To(Equal("fullpage"))
		Expect(resp.Banners[0].Page).NotTo(BeNil())
		Expect(*resp.Banners[0].Page).To(ContainSubstring("Cookie text"))

		var impCount int
		Expect(pool.QueryRow(ctx, "SELECT COUNT(*) FROM banner_impressions WHERE banner_id = $1 AND visitor_id = $2",
			bannerID, "v1").Scan(&impCount)).To(Succeed())
		Expect(impCount).To(Equal(1))

		// 3. Decision is recorded (text/plain body, as the snippet posts it).
		req = httptest.NewRequest(http.MethodPost, "/banner/decision/"+contextID,
			bytes.NewBufferString(`{"banner_id":"`+bannerID+`","action":"accept","visitor_id":"v1"}`))
		req.Header.Set("Content-Type", "text/plain")
		w = httptest.NewRecorder()
		engine.ServeHTTP(w, req)
		Expect(w.Code).To(Equal(http.StatusNoContent))

		var action, ipHash string
		Expect(pool.QueryRow(ctx, "SELECT action, ip_hash FROM banner_decisions WHERE banner_id = $1", bannerID).
			Scan(&action, &ipHash)).To(Succeed())
		Expect(action).To(Equal("accept"))
		Expect(len(ipHash)).To(Equal(64))

		// 4. Config still lists the banner after a decision (snippet-side
		//    cookies do the skipping, not the server).
		req = httptest.NewRequest(http.MethodGet, "/banner/config/"+contextID, nil)
		w = httptest.NewRecorder()
		engine.ServeHTTP(w, req)
		Expect(w.Code).To(Equal(http.StatusOK))
		Expect(w.Body.String()).To(ContainSubstring(bannerID))
	})
})