package cicd_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"

	"github.com/gin-gonic/gin"
	"github.com/hekemen/automata/cicd/support"
	ttracking "github.com/hekemen/automata/internal/adapter/tracking"
	tdomain "github.com/hekemen/automata/internal/domain/tracking"
	trepo "github.com/hekemen/automata/internal/infrastructure/tracking/repo"
	"github.com/hekemen/automata/pkg/snippet"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Tracking context resolution and snippet host", func() {
	var (
		w            *httptest.ResponseRecorder
		engine       *gin.Engine
		trackingRepo tdomain.Repository
	)

	BeforeEach(func() {
		gin.SetMode(gin.TestMode)
		w = httptest.NewRecorder()
		trackingRepo = trepo.New(pool)
		engine = ttracking.NewServer(nil, nil, nil, nil, trackingRepo)
	})

	AfterEach(func() {
		_, err := pool.Exec(ctx, "DELETE FROM tracking_events")
		Expect(err).NotTo(HaveOccurred())
		_, err = pool.Exec(ctx, "DELETE FROM tracking_visitors")
		Expect(err).NotTo(HaveOccurred())
	})

	Describe("POST /track", func() {
		It("should prefer a valid contextId in the body over host resolution", func() {
			contextID := support.NewTestContextID()
			body := `{"contextId":"` + contextID + `","type":"page_view","url":"/home","visitor_id":"v-1"}`

			req := httptest.NewRequest("POST", "/track", bytes.NewBufferString(body))
			req.Header.Set("Content-Type", "application/json")
			// Host would resolve to "pweb" (not a UUID) if used
			req.Header.Set("Host", "pweb.home.arpa")
			engine.ServeHTTP(w, req)

			Expect(w.Code).To(Equal(http.StatusNoContent))

			var count int
			err := pool.QueryRow(ctx, "SELECT count(*) FROM tracking_events WHERE context_id=$1", contextID).Scan(&count)
			Expect(err).NotTo(HaveOccurred())
			Expect(count).To(Equal(1))
		})

		It("should fall back to X-Context-ID header when body contextId is not a valid UUID", func() {
			contextID := support.NewTestContextID()
			body := `{"contextId":"not-a-uuid","type":"page_view","url":"/home"}`

			req := httptest.NewRequest("POST", "/track", bytes.NewBufferString(body))
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("X-Context-ID", contextID)
			engine.ServeHTTP(w, req)

			Expect(w.Code).To(Equal(http.StatusNoContent))

			var count int
			err := pool.QueryRow(ctx, "SELECT count(*) FROM tracking_events WHERE context_id=$1", contextID).Scan(&count)
			Expect(err).NotTo(HaveOccurred())
			Expect(count).To(Equal(1))
		})

		It("should fall back to host resolution when the body has no contextId", func() {
			contextID := support.NewTestContextID()
			body := `{"type":"page_view","url":"/home"}`

			req := httptest.NewRequest("POST", "/track", bytes.NewBufferString(body))
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("X-Context-ID", contextID)
			engine.ServeHTTP(w, req)

			Expect(w.Code).To(Equal(http.StatusNoContent))
		})
	})

	Describe("POST /track/batch", func() {
		It("should use a per-event contextId when it is a valid UUID", func() {
			contextID := support.NewTestContextID()
			body := `{"events":[{"contextId":"` + contextID + `","type":"page_view","url":"/home"}]}`

			req := httptest.NewRequest("POST", "/track/batch", bytes.NewBufferString(body))
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("Host", "pweb.home.arpa")
			engine.ServeHTTP(w, req)

			Expect(w.Code).To(Equal(http.StatusOK))

			var resp struct {
				Accepted int `json:"accepted"`
				Rejected int `json:"rejected"`
			}
			Expect(json.Unmarshal(w.Body.Bytes(), &resp)).To(Succeed())
			Expect(resp.Accepted).To(Equal(1))
			Expect(resp.Rejected).To(Equal(0))

			var count int
			err := pool.QueryRow(ctx, "SELECT count(*) FROM tracking_events WHERE context_id=$1", contextID).Scan(&count)
			Expect(err).NotTo(HaveOccurred())
			Expect(count).To(Equal(1))
		})
	})

	Describe("GET /snippet/:id.js", func() {
		It("should emit an apiHost derived from the request Host header", func() {
			contextID := support.NewTestContextID()

			req := httptest.NewRequest("GET", "/snippet/"+contextID+".js", nil)
			req.Header.Set("Host", "pweb.home.arpa")
			engine.ServeHTTP(w, req)

			Expect(w.Code).To(Equal(http.StatusOK))
			Expect(w.Body.String()).To(ContainSubstring(`"http://pweb.home.arpa"`))
		})

		It("should keep localhost apiHost for localhost requests", func() {
			contextID := support.NewTestContextID()

			req := httptest.NewRequest("GET", "/snippet/"+contextID+".js", nil)
			req.Header.Set("Host", "localhost:8081")
			engine.ServeHTTP(w, req)

			Expect(w.Code).To(Equal(http.StatusOK))
			Expect(w.Body.String()).To(ContainSubstring(`"http://localhost:8081"`))
		})
	})

	Describe("snippet generator", func() {
		It("should send handler field names in the fetch body", func() {
			contextID := support.NewTestContextID()

			js, err := snippet.Generate(contextID, "", nil)
			Expect(err).NotTo(HaveOccurred())
			Expect(js).To(ContainSubstring(`v:v,t:t,u:`))
			Expect(js).To(ContainSubstring(`r:r||document.referrer`))
			Expect(js).To(ContainSubstring(`p:p||{}`))
		})
	})
})