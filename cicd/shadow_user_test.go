package cicd_test

import (
	"bytes"
	"net/http"
	"net/http/httptest"

	"github.com/gin-gonic/gin"
	"github.com/hekemen/automata/cicd/support"
	bhandler "github.com/hekemen/automata/internal/adapter/api/handler"
	brepo "github.com/hekemen/automata/internal/infrastructure/banner/repo"
	formRepo "github.com/hekemen/automata/internal/infrastructure/form/repo"
	ttracking "github.com/hekemen/automata/internal/adapter/tracking"
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
		engine = ttracking.NewServer(formHandler, bannerHandler, placementHandler, campaignHandler, trackingRepo)
	})

	AfterEach(func() {
		_, err := pool.Exec(ctx, "DELETE FROM tracking_events")
		Expect(err).NotTo(HaveOccurred())
		_, err = pool.Exec(ctx, "DELETE FROM tracking_visitors")
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

			var visitorPK string
err := pool.QueryRow(ctx, "SELECT id FROM tracking_visitors WHERE cookie_value = $1", vid).Scan(&visitorPK)
Expect(err).NotTo(HaveOccurred())
Expect(trackingRepo.SetVisitorContactID(ctx, contextID, visitorPK, contactID)).To(Succeed())

			// A second track for the same visitor must not un-identify them.
			req2 := httptest.NewRequest("POST", "/track", bytes.NewBufferString(body))
			req2.Header.Set("Content-Type", "application/json")
			req2.RemoteAddr = "203.0.113.8:1234"
			engine.ServeHTTP(w, req2)
			Expect(w.Code).To(Equal(http.StatusNoContent))

			var cid string
			err = pool.QueryRow(ctx, "SELECT contact_id FROM tracking_visitors WHERE cookie_value = $1", vid).Scan(&cid)
			Expect(err).NotTo(HaveOccurred())
			Expect(cid).To(Equal(contactID))
		})
	})
})