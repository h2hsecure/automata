package cicd_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"

	"github.com/gin-gonic/gin"
	"github.com/h2hsecure/automata/cicd/support"
	bhandler "github.com/h2hsecure/automata/internal/adapter/api/handler"
	trepo "github.com/h2hsecure/automata/internal/infrastructure/tracking/repo"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("HTTP Integration Tests - Tracking Dashboard", func() {
	var (
		w         *httptest.ResponseRecorder
		engine    *gin.Engine
		contextID string
	)

	BeforeEach(func() {
		gin.SetMode(gin.TestMode)
		w = httptest.NewRecorder()
		trackingRepo := trepo.New(pool)
		trackingHandler := bhandler.NewTrackingHandler(trackingRepo)

		engine = gin.New()
		engine.Use(func(c *gin.Context) {
			if contextID := c.GetHeader("X-Context-ID"); contextID != "" {
				c.Set("context_id", contextID)
				c.Next()
				return
			}
			c.JSON(http.StatusBadRequest, gin.H{"error": "context required"})
			c.Abort()
		})
		engine.GET("/api/context/tracking/dashboard", trackingHandler.GetDashboard)

		contextID = support.NewTestContextID()
	})

	AfterEach(func() {
		_, _ = pool.Exec(ctx, "DELETE FROM tracking_events WHERE context_id = $1", contextID)
		_, _ = pool.Exec(ctx, "DELETE FROM tracking_visitors WHERE context_id = $1", contextID)
	})

	It("should return merged metrics with page, referrer and UA breakdowns", func() {
		// Three pageviews from a single visitor: iPhone Safari, Chrome on Android, Firefox on Linux.
		_, err := pool.Exec(ctx, `
			INSERT INTO tracking_events
				(id, context_id, visitor_id, type, url, title, referrer, event_name, properties, user_agent, ip_hash, utm_source, utm_medium, utm_campaign, created_at)
			VALUES
				($1, $2, 'v-dash-1', 'pageview', '/home', 'Home', 'https://google.com', '', '{}'::jsonb,
				 'Mozilla/5.0 (iPhone; CPU iPhone OS 17_0 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/17.0 Mobile/15E148 Safari/604.1',
				 '', '', '', '', NOW()),
				($3, $2, 'v-dash-1', 'pageview', '/home', 'Home', 'https://google.com', '', '{}'::jsonb,
				 'Mozilla/5.0 (Linux; Android 14; Pixel 8) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/126.0.0.0 Mobile Safari/537.36',
				 '', '', '', '', NOW()),
				($4, $2, 'v-dash-1', 'pageview', '/about', 'About', '', '', '{}'::jsonb,
				 'Mozilla/5.0 (X11; Linux x86_64; rv:126.0) Gecko/20100101 Firefox/126.0',
				 '', '', '', '', NOW())
		`, support.NewTestUUID(), contextID, support.NewTestUUID(), support.NewTestUUID())
		Expect(err).NotTo(HaveOccurred())

		req := httptest.NewRequest("GET", "/api/context/tracking/dashboard", nil)
		req.Header.Set("X-Context-ID", contextID)
		engine.ServeHTTP(w, req)

		Expect(w.Code).To(Equal(http.StatusOK))

		var resp struct {
			TotalVisitors  int64             `json:"total_visitors"`
			ActiveVisitors int64             `json:"active_visitors"`
			PageViews      int64             `json:"page_views"`
			TopPages       []struct {
				URL   string `json:"URL"`
				Count int64  `json:"Count"`
			} `json:"top_pages"`
			TopReferrers []struct {
				Referrer string `json:"Referrer"`
				Count    int64  `json:"Count"`
			} `json:"top_referrers"`
			DeviceBreakdown  map[string]int `json:"device_breakdown"`
			BrowserBreakdown map[string]int `json:"browser_breakdown"`
		}
		Expect(json.Unmarshal(w.Body.Bytes(), &resp)).To(Succeed())

		Expect(resp.TotalVisitors).To(Equal(int64(1)))
		Expect(resp.ActiveVisitors).To(Equal(int64(1)))
		Expect(resp.PageViews).To(Equal(int64(3)))

		Expect(len(resp.TopPages)).To(Equal(2))
		Expect(resp.TopPages[0].URL).To(Equal("/home"))
		Expect(resp.TopPages[0].Count).To(Equal(int64(2)))
		Expect(resp.TopPages[1].URL).To(Equal("/about"))
		Expect(resp.TopPages[1].Count).To(Equal(int64(1)))

		Expect(len(resp.TopReferrers)).To(Equal(1))
		Expect(resp.TopReferrers[0].Referrer).To(Equal("https://google.com"))
		Expect(resp.TopReferrers[0].Count).To(Equal(int64(2)))

		Expect(resp.DeviceBreakdown).To(Equal(map[string]int{"mobile": 2, "desktop": 1}))
		Expect(resp.BrowserBreakdown).To(Equal(map[string]int{"Safari": 1, "Chrome": 1, "Firefox": 1}))
	})
})