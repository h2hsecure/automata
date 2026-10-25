package cicd_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"

	"github.com/gin-gonic/gin"
	"github.com/hekemen/automata/cicd/support"
	bhandler "github.com/hekemen/automata/internal/adapter/api/handler"
	brepo "github.com/hekemen/automata/internal/infrastructure/banner/repo"
	crepo "github.com/hekemen/automata/internal/infrastructure/contact/repo"
	formRepo "github.com/hekemen/automata/internal/infrastructure/form/repo"
	ttracking "github.com/hekemen/automata/internal/adapter/tracking"
	ct "github.com/hekemen/automata/internal/domain/contact"
	tdomain "github.com/hekemen/automata/internal/domain/tracking"
	trepo "github.com/hekemen/automata/internal/infrastructure/tracking/repo"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Shadow User Tracking", func() {
	var (
		w            *httptest.ResponseRecorder
		engine       *gin.Engine
		trackingRepo tdomain.Repository
	)

	BeforeEach(func() {
		gin.SetMode(gin.TestMode)
		w = httptest.NewRecorder()
		trackingRepo = trepo.New(pool)
		bannerRepo := brepo.New(pool)
		formHandler := bhandler.NewFormHandler(formRepo.New(pool))
		bannerHandler := bhandler.NewBannerHandler(bannerRepo)
		placementHandler := bhandler.NewPlacementHandler(bannerRepo)
		campaignHandler := bhandler.NewCampaignHandler(bannerRepo)
		engine = ttracking.NewServer(formHandler, bannerHandler, placementHandler, campaignHandler, trackingRepo, nil)
	})

	AfterEach(func() {
		_, err := pool.Exec(ctx, "DELETE FROM tracking_events")
		Expect(err).NotTo(HaveOccurred())
		_, err = pool.Exec(ctx, "DELETE FROM tracking_visitors")
		Expect(err).NotTo(HaveOccurred())
		_, err = pool.Exec(ctx, "DELETE FROM form_submissions")
		Expect(err).NotTo(HaveOccurred())
		_, err = pool.Exec(ctx, "DELETE FROM forms")
		Expect(err).NotTo(HaveOccurred())
		_, err = pool.Exec(ctx, "DELETE FROM contacts")
		Expect(err).NotTo(HaveOccurred())
	})

	Describe("POST /track", func() {
		It("stores the client IP on the event and on the visitor last_ip", func() {
			contextID := support.NewTestContextID()
			vid := support.NewTestUUID()
			body := `{"contextId":"` + contextID + `","type":"page_view","url":"/home","visitor_id":"` + vid + `"}`

			req := httptest.NewRequest("POST", "/track", bytes.NewBufferString(body))
			req.Header.Set("Content-Type", "application/json")
			req.RemoteAddr = "203.0.113.7:1234"
			engine.ServeHTTP(w, req)

			Expect(w.Code).To(Equal(http.StatusNoContent))

			var ip string
			err := pool.QueryRow(ctx, "SELECT ip FROM tracking_events WHERE context_id = $1", contextID).Scan(&ip)
			Expect(err).NotTo(HaveOccurred())
			Expect(ip).To(Equal("203.0.113.7"))

			var lastIP string
			err = pool.QueryRow(ctx, "SELECT last_ip FROM tracking_visitors WHERE cookie_value = $1", vid).Scan(&lastIP)
			Expect(err).NotTo(HaveOccurred())
			Expect(lastIP).To(Equal("203.0.113.7"))
		})

		It("keeps contact_id intact when the visitor is upserted again", func() {
			contextID := support.NewTestContextID()
			vid := support.NewTestUUID()
			contactID := support.NewTestUUID()
			body := `{"contextId":"` + contextID + `","type":"page_view","url":"/home","visitor_id":"` + vid + `"}`

			req := httptest.NewRequest("POST", "/track", bytes.NewBufferString(body))
			req.Header.Set("Content-Type", "application/json")
			req.RemoteAddr = "203.0.113.7:1234"
			engine.ServeHTTP(w, req)
			Expect(w.Code).To(Equal(http.StatusNoContent))

			Expect(trackingRepo.SetVisitorContactID(ctx, contextID, vid, contactID)).To(Succeed())

			// A second track for the same visitor must not un-identify them.
			req2 := httptest.NewRequest("POST", "/track", bytes.NewBufferString(body))
			req2.Header.Set("Content-Type", "application/json")
			req2.RemoteAddr = "203.0.113.8:1234"
			engine.ServeHTTP(w, req2)
			Expect(w.Code).To(Equal(http.StatusNoContent))

			var cid string
			err := pool.QueryRow(ctx, "SELECT contact_id FROM tracking_visitors WHERE cookie_value = $1", vid).Scan(&cid)
			Expect(err).NotTo(HaveOccurred())
			Expect(cid).To(Equal(contactID))
		})
	})

	Describe("POST /form/submit/:slug (identifies the visitor)", func() {
		var (
			contextID  string
			vid        string
			contactRepo ct.Repository
			formEngine *gin.Engine
		)

		BeforeEach(func() {
			contextID = support.NewTestContextID()
			vid = support.NewTestUUID()

			// Form with slug "test" and email/first_name fields.
			fieldsJSON, _ := json.Marshal([]map[string]interface{}{
				{"slug": "email", "type": "email", "required": true},
				{"slug": "first_name", "type": "text"},
			})
			_, err := pool.Exec(ctx, `
				INSERT INTO forms (id, context_id, slug, name, description, fields, settings, created_at, updated_at)
				VALUES ($1, $2, 'test', 'Shadow Form', '', $3, '{}', NOW(), NOW())
			`, support.NewTestUUID(), contextID, fieldsJSON)
			Expect(err).NotTo(HaveOccurred())

			// Seed the visitor via the tracking server.
			body := `{"contextId":"` + contextID + `","type":"page_view","url":"/home","visitor_id":"` + vid + `"}`
			req := httptest.NewRequest("POST", "/track", bytes.NewBufferString(body))
			req.Header.Set("Content-Type", "application/json")
			req.RemoteAddr = "203.0.113.7:1234"
			engine.ServeHTTP(w, req)
			Expect(w.Code).To(Equal(http.StatusNoContent))

			// Main-API-style engine mounting the form submit route.
			contactRepo = crepo.New(pool)
			formHandler := bhandler.NewFormHandlerWithIdentifiers(formRepo.New(pool), contactRepo, trackingRepo)
			formEngine = gin.New()
			formEngine.Use(func(c *gin.Context) {
				c.Set("context_id", contextID)
				c.Next()
			})
			formEngine.POST("/form/submit/:slug", formHandler.SubmitForm)
		})

		It("creates a contact from the submission and links the visitor", func() {
			body := `{"data":{"email":"shadow@example.com","first_name":"Ada"},"visitor_id":"` + vid + `"}`
			req := httptest.NewRequest("POST", "/form/submit/test", bytes.NewBufferString(body))
			req.Header.Set("Content-Type", "application/json")
			fw := httptest.NewRecorder()
			formEngine.ServeHTTP(fw, req)
			Expect(fw.Code).To(Equal(http.StatusCreated))

			c, err := contactRepo.FindByEmail(contextID, "shadow@example.com")
			Expect(err).NotTo(HaveOccurred())
			Expect(c.Source).To(Equal("form:test"))
			Expect(c.SourceID).To(Equal(vid))

			var cid string
			err = pool.QueryRow(ctx, "SELECT contact_id FROM tracking_visitors WHERE cookie_value = $1", vid).Scan(&cid)
			Expect(err).NotTo(HaveOccurred())
			Expect(cid).To(Equal(c.ID))
		})

		It("dedupes the contact by email and preserves the first visitor's link", func() {
			submit := func(visitorID string) {
				body := `{"data":{"email":"shadow@example.com","first_name":"Ada"},"visitor_id":"` + visitorID + `"}`
				req := httptest.NewRequest("POST", "/form/submit/test", bytes.NewBufferString(body))
				req.Header.Set("Content-Type", "application/json")
				fw := httptest.NewRecorder()
				formEngine.ServeHTTP(fw, req)
				Expect(fw.Code).To(Equal(http.StatusCreated))
			}

			vid2 := support.NewTestUUID()
			submit(vid)
			submit(vid2)

			c, err := contactRepo.FindByEmail(contextID, "shadow@example.com")
			Expect(err).NotTo(HaveOccurred())
			Expect(c.SourceID).To(Equal(vid)) // dedupe: first submission wins

			var cid string
			err = pool.QueryRow(ctx, "SELECT contact_id FROM tracking_visitors WHERE cookie_value = $1", vid).Scan(&cid)
			Expect(err).NotTo(HaveOccurred())
			Expect(cid).To(Equal(c.ID))
		})
	})

	Describe("GET /context/tracking/visitors (context-scoped)", func() {
		var (
			contextID  string
			vid1       string
			vid2       string
			visitorAPI *gin.Engine
		)

		BeforeEach(func() {
			contextID = support.NewTestContextID()
			vid1 = support.NewTestUUID()
			vid2 = support.NewTestUUID()

			// Seed two visitors via the tracking server with distinct client IPs.
			track := func(visitorID, remoteAddr string) {
				body := `{"contextId":"` + contextID + `","type":"page_view","url":"/home","visitor_id":"` + visitorID + `"}`
				req := httptest.NewRequest("POST", "/track", bytes.NewBufferString(body))
				req.Header.Set("Content-Type", "application/json")
				req.RemoteAddr = remoteAddr
				tw := httptest.NewRecorder()
				engine.ServeHTTP(tw, req)
				Expect(tw.Code).To(Equal(http.StatusNoContent))
			}
			track(vid1, "203.0.113.7:1234")
			track(vid2, "203.0.113.9:1234")

			// Main-API-style engine mounting the context tracking visitor routes.
			visitorAPI = gin.New()
			visitorAPI.Use(func(c *gin.Context) {
				c.Set("context_id", contextID)
				c.Next()
			})
			trackingHandler := bhandler.NewTrackingHandler(trepo.New(pool))
			visitorAPI.GET("/context/tracking/visitors", trackingHandler.ListVisitors)
			visitorAPI.GET("/context/tracking/visitors/:id/events", trackingHandler.GetVisitorEvents)
		})

		It("lists visitors with LastIP and ContactID fields", func() {
			req := httptest.NewRequest("GET", "/context/tracking/visitors?limit=10", nil)
			vw := httptest.NewRecorder()
			visitorAPI.ServeHTTP(vw, req)
			Expect(vw.Code).To(Equal(http.StatusOK))

			var resp struct {
				Visitors []map[string]interface{} `json:"visitors"`
				Total    int                      `json:"total"`
			}
			Expect(json.Unmarshal(vw.Body.Bytes(), &resp)).To(Succeed())
			Expect(resp.Total).To(Equal(2))
			Expect(resp.Visitors).To(HaveLen(2))
			for _, v := range resp.Visitors {
				_, hasLastIP := v["LastIP"]
				Expect(hasLastIP).To(BeTrue())
				_, hasContactID := v["ContactID"]
				Expect(hasContactID).To(BeTrue())
			}
		})

		It("returns the visitor's events with the stored IP", func() {
			// The route is keyed by the visitor UUID PK; events are stored
			// under the cookie value, so resolve the UUID first.
			var visitorUUID string
			err := pool.QueryRow(ctx, "SELECT id FROM tracking_visitors WHERE context_id = $1 AND cookie_value = $2", contextID, vid1).Scan(&visitorUUID)
			Expect(err).NotTo(HaveOccurred())

			req := httptest.NewRequest("GET", "/context/tracking/visitors/"+visitorUUID+"/events?limit=10", nil)
			vw := httptest.NewRecorder()
			visitorAPI.ServeHTTP(vw, req)
			Expect(vw.Code).To(Equal(http.StatusOK))

			var resp struct {
				Events []map[string]interface{} `json:"events"`
				Total  int                      `json:"total"`
			}
			Expect(json.Unmarshal(vw.Body.Bytes(), &resp)).To(Succeed())
			Expect(resp.Total).To(Equal(1))
			Expect(resp.Events).To(HaveLen(1))
			Expect(resp.Events[0]["IP"]).To(Equal("203.0.113.7"))
		})
	})

	Describe("legacy rows with NULL ip/last_ip/contact_id (pre-migration data)", func() {
		var (
			contextID  string
			legacyID   string
			visitorAPI *gin.Engine
		)

		BeforeEach(func() {
			contextID = support.NewTestContextID()
			legacyID = support.NewTestUUID()

			// Simulate a visitor row created before the ip/last_ip/contact_id migration:
// fingerprint is '' (Go always writes empty strings), new columns are NULL.
			_, err := pool.Exec(ctx, `
				INSERT INTO tracking_visitors (id, context_id, cookie_value, fingerprint, first_seen, last_seen, page_views)
				VALUES ($1, $2, 'legacy-cookie', '', NOW(), NOW(), 1)
			`, legacyID, contextID)
			Expect(err).NotTo(HaveOccurred())

			// Simulate an event row created before the ip column migration: text
// columns are '' (Go always writes empty strings), ip is NULL, and
// visitor_id holds the cookie value (the real data model).
			_, err = pool.Exec(ctx, `
				INSERT INTO tracking_events (context_id, visitor_id, type, url, title, referrer, event_name, properties, user_agent, ip_hash, utm_source, utm_medium, utm_campaign, created_at)
				VALUES ($1, 'legacy-cookie', 'page_view', '/legacy', '', '', '', '{}', '', '', '', '', '', NOW())
			`, contextID)
			Expect(err).NotTo(HaveOccurred())

			visitorAPI = gin.New()
			visitorAPI.Use(func(c *gin.Context) {
				c.Set("context_id", contextID)
				c.Next()
			})
			trackingHandler := bhandler.NewTrackingHandler(trepo.New(pool))
			visitorAPI.GET("/context/tracking/visitors", trackingHandler.ListVisitors)
			visitorAPI.GET("/context/tracking/visitors/:id/events", trackingHandler.GetVisitorEvents)
		})

		It("lists the legacy visitor with empty LastIP/ContactID instead of erroring", func() {
			req := httptest.NewRequest("GET", "/context/tracking/visitors?limit=10", nil)
			vw := httptest.NewRecorder()
			visitorAPI.ServeHTTP(vw, req)
			Expect(vw.Code).To(Equal(http.StatusOK))

			var resp struct {
				Visitors []map[string]interface{} `json:"visitors"`
				Total    int                      `json:"total"`
			}
			Expect(json.Unmarshal(vw.Body.Bytes(), &resp)).To(Succeed())
			Expect(resp.Total).To(Equal(1))
			Expect(resp.Visitors[0]["LastIP"]).To(Equal(""))
			Expect(resp.Visitors[0]["ContactID"]).To(Equal(""))
		})

		It("returns the legacy visitor's events with an empty IP instead of erroring", func() {
			req := httptest.NewRequest("GET", "/context/tracking/visitors/"+legacyID+"/events?limit=10", nil)
			vw := httptest.NewRecorder()
			visitorAPI.ServeHTTP(vw, req)
			Expect(vw.Code).To(Equal(http.StatusOK))

			var resp struct {
				Events []map[string]interface{} `json:"events"`
				Total  int                      `json:"total"`
			}
			Expect(json.Unmarshal(vw.Body.Bytes(), &resp)).To(Succeed())
			Expect(resp.Total).To(Equal(1))
			Expect(resp.Events[0]["IP"]).To(Equal(""))
		})

		It("returns the legacy visitor by ID via the repo without a NULL scan error", func() {
			v, err := trackingRepo.GetVisitorByID(ctx, contextID, legacyID)
			Expect(err).NotTo(HaveOccurred())
			Expect(v.CookieValue).To(Equal("legacy-cookie"))
			Expect(v.LastIP).To(Equal(""))
			Expect(v.ContactID).To(Equal(""))
		})
	})
})