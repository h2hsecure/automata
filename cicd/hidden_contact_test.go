package cicd_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"

	"github.com/gin-gonic/gin"
	"github.com/h2hsecure/automata/cicd/support"
	bhandler "github.com/h2hsecure/automata/internal/adapter/api/handler"
	brepo "github.com/h2hsecure/automata/internal/infrastructure/banner/repo"
	crepo "github.com/h2hsecure/automata/internal/infrastructure/contact/repo"
	geoInfra "github.com/h2hsecure/automata/internal/infrastructure/geo"
	formRepo "github.com/h2hsecure/automata/internal/infrastructure/form/repo"
	ttracking "github.com/h2hsecure/automata/internal/adapter/tracking"
	trepo "github.com/h2hsecure/automata/internal/infrastructure/tracking/repo"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("HTTP Integration Tests - Hidden Visitor Contacts", func() {
	var (
		w             *httptest.ResponseRecorder
		trackEngine   *gin.Engine
		contactEngine *gin.Engine
		contextID     string
	)

	BeforeEach(func() {
		gin.SetMode(gin.TestMode)
		w = httptest.NewRecorder()

		trackingRepo := trepo.New(pool)
		bannerRepo := brepo.New(pool)
		formHandler := bhandler.NewFormHandler(formRepo.New(pool))
		bannerHandler := bhandler.NewBannerHandler(bannerRepo)
		placementHandler := bhandler.NewPlacementHandler(bannerRepo)
		campaignHandler := bhandler.NewCampaignHandler(bannerRepo)
		trackEngine = ttracking.NewServer(formHandler, bannerHandler, placementHandler, campaignHandler, trackingRepo, geoInfra.New(), crepo.New(pool))
		contactEngine = setupContactEngine()

		contextID = support.NewTestContextID()
	})

	AfterEach(func() {
		_, _ = pool.Exec(ctx, "DELETE FROM contact_activities WHERE context_id = $1", contextID)
		_, _ = pool.Exec(ctx, "DELETE FROM contacts WHERE context_id = $1", contextID)
		_, _ = pool.Exec(ctx, "DELETE FROM tracking_events WHERE context_id = $1", contextID)
		_, _ = pool.Exec(ctx, "DELETE FROM tracking_visitors WHERE context_id = $1", contextID)
	})

	It("should create a hidden visitor contact and a page_visit activity on pageview", func() {
		body := `{"type":"pageview","url":"/home","title":"Home","referrer":"https://google.com","visitor_id":"v-hc-1"}`
		req := httptest.NewRequest("POST", "/track", bytes.NewBufferString(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-Context-ID", contextID)
		trackEngine.ServeHTTP(w, req)

		Expect(w.Code).To(Equal(http.StatusNoContent))

		var contactCount int
		err := pool.QueryRow(ctx, `
			SELECT COUNT(*) FROM contacts
			WHERE context_id = $1 AND source = 'visitor' AND source_id = 'v-hc-1' AND hidden = true AND email IS NULL
		`, contextID).Scan(&contactCount)
		Expect(err).NotTo(HaveOccurred())
		Expect(contactCount).To(Equal(1))

		var activityCount int
		err = pool.QueryRow(ctx, `
			SELECT COUNT(*) FROM contact_activities WHERE context_id = $1 AND type = 'page_visit'
		`, contextID).Scan(&activityCount)
		Expect(err).NotTo(HaveOccurred())
		Expect(activityCount).To(Equal(1))
	})

	It("should enrich the hidden contact with user-agent, device, browser, ip, and geo", func() {
		body := `{"type":"pageview","url":"/home","title":"Home","visitor_id":"v-hc-geo"}`
		req := httptest.NewRequest("POST", "/track", bytes.NewBufferString(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-Context-ID", contextID)
		req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/126.0.0.0 Safari/537.36")
		// 208.65.44.17 resolves to Naples, Florida, United States in GeoLite2-City.
		req.RemoteAddr = "208.65.44.17:51234"
		trackEngine.ServeHTTP(w, req)

		Expect(w.Code).To(Equal(http.StatusNoContent))

		var userAgent, device, browser, ip, country, countryCode, city, region string
		err := pool.QueryRow(ctx, `
			SELECT COALESCE(user_agent,''), COALESCE(device_type,''), COALESCE(browser,''), COALESCE(host(ip_address),''),
			       COALESCE(geo_country,''), COALESCE(geo_country_code,''), COALESCE(geo_city,''), COALESCE(geo_region,'')
			FROM contacts WHERE context_id = $1 AND source_id = 'v-hc-geo'
		`, contextID).Scan(&userAgent, &device, &browser, &ip, &country, &countryCode, &city, &region)
		Expect(err).NotTo(HaveOccurred())
		Expect(userAgent).To(ContainSubstring("Chrome"))
		Expect(device).To(Equal("desktop"))
		Expect(browser).To(Equal("Chrome"))
		Expect(ip).To(Equal("208.65.44.17"))
		Expect(country).To(Equal("United States"))
		Expect(countryCode).To(Equal("US"))
		Expect(city).To(Equal("Naples"))
		Expect(region).To(Equal("Florida"))
	})

	It("should not duplicate the hidden contact on subsequent pageviews", func() {
		for i := 0; i < 2; i++ {
			body := `{"type":"pageview","url":"/home","title":"Home","visitor_id":"v-hc-dup"}`
			req := httptest.NewRequest("POST", "/track", bytes.NewBufferString(body))
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("X-Context-ID", contextID)
			trackEngine.ServeHTTP(w, req)
			Expect(w.Code).To(Equal(http.StatusNoContent))
		}

		var contactCount, activityCount int
		err := pool.QueryRow(ctx, "SELECT COUNT(*) FROM contacts WHERE context_id = $1 AND source_id = 'v-hc-dup'", contextID).Scan(&contactCount)
		Expect(err).NotTo(HaveOccurred())
		Expect(contactCount).To(Equal(1))
		err = pool.QueryRow(ctx, "SELECT COUNT(*) FROM contact_activities WHERE context_id = $1 AND type = 'page_visit'", contextID).Scan(&activityCount)
		Expect(err).NotTo(HaveOccurred())
		Expect(activityCount).To(Equal(2))
	})

	It("should include hidden contacts by default and support the hidden filter", func() {
		hiddenID := support.NewTestUUID()
		visibleID := support.NewTestUUID()
		_, err := pool.Exec(ctx, `
			INSERT INTO contacts (id, context_id, source, source_id, hidden)
			VALUES ($1, $2, 'visitor', 'v-hc-list', true)
		`, hiddenID, contextID)
		Expect(err).NotTo(HaveOccurred())
		_, err = pool.Exec(ctx, `
			INSERT INTO contacts (id, context_id, email, hidden)
			VALUES ($1, $2, 'visible@example.com', false)
		`, visibleID, contextID)
		Expect(err).NotTo(HaveOccurred())

		var listResp struct {
			Contacts []struct {
				ID     string `json:"ID"`
				Hidden bool   `json:"Hidden"`
			} `json:"contacts"`
			Total int64 `json:"total"`
		}

		// No hidden param: hidden contacts are included.
		req := httptest.NewRequest("GET", "/api/contacts", nil)
		req.Header.Set("X-Context-ID", contextID)
		contactEngine.ServeHTTP(w, req)
		Expect(w.Code).To(Equal(http.StatusOK))
		Expect(json.Unmarshal(w.Body.Bytes(), &listResp)).To(Succeed())
		Expect(listResp.Total).To(Equal(int64(2)))

		// ?hidden=false: hidden contacts are excluded.
		w = httptest.NewRecorder()
		req = httptest.NewRequest("GET", "/api/contacts?hidden=false", nil)
		req.Header.Set("X-Context-ID", contextID)
		contactEngine.ServeHTTP(w, req)
		Expect(w.Code).To(Equal(http.StatusOK))
		Expect(json.Unmarshal(w.Body.Bytes(), &listResp)).To(Succeed())
		Expect(listResp.Total).To(Equal(int64(1)))
		Expect(listResp.Contacts[0].ID).To(Equal(visibleID))

		// ?hidden=true: only hidden contacts.
		w = httptest.NewRecorder()
		req = httptest.NewRequest("GET", "/api/contacts?hidden=true", nil)
		req.Header.Set("X-Context-ID", contextID)
		contactEngine.ServeHTTP(w, req)
		Expect(w.Code).To(Equal(http.StatusOK))
		Expect(json.Unmarshal(w.Body.Bytes(), &listResp)).To(Succeed())
		Expect(listResp.Total).To(Equal(int64(1)))
		Expect(listResp.Contacts[0].ID).To(Equal(hiddenID))
		Expect(listResp.Contacts[0].Hidden).To(BeTrue())
	})
})