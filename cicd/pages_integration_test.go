package cicd_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"

	"github.com/gin-gonic/gin"
	"github.com/h2hsecure/automata/cicd/support"
	"github.com/h2hsecure/automata/internal/adapter/api/handler"
	"github.com/h2hsecure/automata/internal/adapter/tracking"
	pagedomain "github.com/h2hsecure/automata/internal/domain/page"
	formrepo "github.com/h2hsecure/automata/internal/infrastructure/form/repo"
	pagerepo "github.com/h2hsecure/automata/internal/infrastructure/page/repo"
	trepo "github.com/h2hsecure/automata/internal/infrastructure/tracking/repo"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Pages end-to-end", func() {
	var (
		w        *httptest.ResponseRecorder
		engine   *gin.Engine
		pageRepo pagedomain.Repository
		contextID string
		bannerPageID string
	)

	BeforeEach(func() {
		gin.SetMode(gin.TestMode)
		w = httptest.NewRecorder()
		pageRepo = pagerepo.New(pool)

		contextID = support.NewTestContextID()

		// Plain legal page referenced by a button href.
		policy := &pagedomain.Page{
			ID:        support.NewTestUUID(),
			ContextID: contextID,
			Slug:      "cookie-policy",
			Title:     "Cookie Policy",
			Content:   "<p>Cookie policy text</p>",
			IsActive:  true,
		}
		Expect(pageRepo.Create(policy)).To(Succeed())

		// Positioned page (a.k.a. banner) referencing the plain page.
		banner := &pagedomain.Page{
			ID:        support.NewTestUUID(),
			ContextID: contextID,
			Slug:      "cookie-banner",
			Title:     "Cookie banner",
			Content:   "<p>We use cookies</p>",
			IsActive:  true,
			Position:  "bottom",
			Buttons: []pagedomain.Button{
				{Label: "Accept", Action: "dismiss_accept", Variant: "primary"},
				{Label: "Policy", Href: "/cookie-policy", Action: "link"},
			},
		}
		Expect(pageRepo.Create(banner)).To(Succeed())
		bannerPageID = banner.ID

		// Tracking server with page serve + page public routes (no banner
		// handlers).
		engine = tracking.NewServer(
			handler.NewFormHandler(formrepo.New(pool)),
			trepo.New(pool),
			nil,
		)
		tracking.RegisterPageRoutes(engine, tracking.NewPageServeHandler(pageRepo))
		tracking.RegisterPagePublicRoutes(engine, tracking.NewPagePublicHandler(pageRepo))
	})

	It("serves config, decision, snippet, and the plain page", func() {
		// 1. Config lists the active pages with the referenced page inlined
		//    and records an impression for the visitor.
		req := httptest.NewRequest(http.MethodGet, "/page/config/"+contextID+"?visitor_id=v1", nil)
		w = httptest.NewRecorder()
		engine.ServeHTTP(w, req)
		Expect(w.Code).To(Equal(http.StatusOK))

		var resp struct {
			Pages []struct {
				ID       string             `json:"id"`
				Slug     string             `json:"slug"`
				Position string             `json:"position"`
				Content  string             `json:"content"`
				Buttons  []pagedomain.Button `json:"buttons"`
				Page     *string            `json:"page"`
			} `json:"pages"`
		}
		Expect(json.Unmarshal(w.Body.Bytes(), &resp)).To(Succeed())
		Expect(resp.Pages).To(HaveLen(2))

		var found *struct {
			ID       string             `json:"id"`
			Slug     string             `json:"slug"`
			Position string             `json:"position"`
			Content  string             `json:"content"`
			Buttons  []pagedomain.Button `json:"buttons"`
			Page     *string            `json:"page"`
		}
		for i := range resp.Pages {
			if resp.Pages[i].Slug == "cookie-banner" {
				found = &resp.Pages[i]
			}
		}
		Expect(found).NotTo(BeNil())
		Expect(found.ID).To(Equal(bannerPageID))
		Expect(found.Position).To(Equal("bottom"))
		Expect(found.Buttons).To(HaveLen(2))
		Expect(found.Page).NotTo(BeNil())
		Expect(*found.Page).To(ContainSubstring("Cookie policy text"))

		var impCount int
		Expect(pool.QueryRow(ctx, "SELECT COUNT(*) FROM page_impressions WHERE page_id = $1 AND visitor_id = $2",
			bannerPageID, "v1").Scan(&impCount)).To(Succeed())
		Expect(impCount).To(Equal(1))

		// 2. Decision is recorded (text/plain body, as the snippet posts it).
		req = httptest.NewRequest(http.MethodPost, "/page/decision/"+contextID,
			bytes.NewBufferString(`{"page_id":"`+bannerPageID+`","decision":"accept","visitor_id":"v1"}`))
		req.Header.Set("Content-Type", "text/plain")
		w = httptest.NewRecorder()
		engine.ServeHTTP(w, req)
		Expect(w.Code).To(Equal(http.StatusNoContent))

		decisions, total, err := pageRepo.ListDecisions(bannerPageID, 0, 0)
		Expect(err).NotTo(HaveOccurred())
		Expect(total).To(Equal(int64(1)))
		Expect(decisions).To(HaveLen(1))

		// 3. Snippet is served and points at the page endpoints.
		req = httptest.NewRequest(http.MethodGet, "/page/snippet/"+contextID+".js", nil)
		w = httptest.NewRecorder()
		engine.ServeHTTP(w, req)
		Expect(w.Code).To(Equal(http.StatusOK))
		Expect(w.Body.String()).To(ContainSubstring("/page/config/" + contextID))
		Expect(w.Body.String()).NotTo(ContainSubstring("/banner/"))

		// 4. Plain page is served as a minimal HTML document.
		req = httptest.NewRequest(http.MethodGet, "/page/"+contextID+"/cookie-policy", nil)
		w = httptest.NewRecorder()
		engine.ServeHTTP(w, req)
		Expect(w.Code).To(Equal(http.StatusOK))
		Expect(w.Body.String()).To(ContainSubstring("Cookie Policy"))
		Expect(w.Body.String()).To(ContainSubstring("Cookie policy text"))
	})

	It("returns 404 for retired banner URLs", func() {
		for _, path := range []string{
			"/banner/config/" + contextID,
			"/banner/snippet/" + contextID + ".js",
			"/track/banner/list",
		} {
			req := httptest.NewRequest(http.MethodGet, path, nil)
			w = httptest.NewRecorder()
			engine.ServeHTTP(w, req)
			Expect(w.Code).To(Equal(http.StatusNotFound), "path %s got %d", path, w.Code)
		}
	})
})