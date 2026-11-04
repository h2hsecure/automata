package cicd_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"

	"github.com/gin-gonic/gin"
	"github.com/h2hsecure/automata/cicd/support"
	bhandler "github.com/h2hsecure/automata/internal/adapter/api/handler"
	formRepo "github.com/h2hsecure/automata/internal/infrastructure/form/repo"
	ttracking "github.com/h2hsecure/automata/internal/adapter/tracking"
	tdomain "github.com/h2hsecure/automata/internal/domain/tracking"
	trepo "github.com/h2hsecure/automata/internal/infrastructure/tracking/repo"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Tracking Server Integration Tests", func() {
	var (
		w            *httptest.ResponseRecorder
		engine       *gin.Engine
		trackingRepo tdomain.Repository
		formHandler  *bhandler.FormHandler
	)

	BeforeEach(func() {
		gin.SetMode(gin.TestMode)
		w = httptest.NewRecorder()
		trackingRepo = trepo.New(pool)
		formHandler = bhandler.NewFormHandler(formRepo.New(pool))
		engine = ttracking.NewServer(formHandler, trackingRepo, nil)
	})

	AfterEach(func() {
		_, err := pool.Exec(ctx, "DELETE FROM tracking_events")
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

			// Batch tracking returns 200 with accepted/rejected counts.
			Expect(w.Code).To(Equal(http.StatusOK))
			Expect(w.Body.String()).To(ContainSubstring(`"accepted":2`))
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
