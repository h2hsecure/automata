package cicd_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"

	"github.com/gin-gonic/gin"
	"github.com/hekemen/automata/cicd/support"
	bhandler "github.com/hekemen/automata/internal/adapter/api/handler"
	brepo "github.com/hekemen/automata/internal/infrastructure/banner/repo"
	formRepo "github.com/hekemen/automata/internal/infrastructure/form/repo"
	ttracking "github.com/hekemen/automata/internal/adapter/tracking"
	trepo "github.com/hekemen/automata/internal/infrastructure/tracking/repo"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("HTTP Integration Tests - Public Form Endpoints", func() {
	var (
		w      *httptest.ResponseRecorder
		engine *gin.Engine
	)

	BeforeEach(func() {
		gin.SetMode(gin.TestMode)
		w = httptest.NewRecorder()
		bannerRepo := brepo.New(pool)
		engine = ttracking.NewServer(
			bhandler.NewFormHandler(formRepo.New(pool)),
			bhandler.NewBannerHandler(bannerRepo),
			bhandler.NewPlacementHandler(bannerRepo),
			bhandler.NewCampaignHandler(bannerRepo),
			trepo.New(pool),
		)
	})

	AfterEach(func() {
		_, _ = pool.Exec(ctx, "DELETE FROM form_submissions")
		_, _ = pool.Exec(ctx, "DELETE FROM forms")
	})

	createForm := func(contextID, slug string) {
		fields, _ := json.Marshal([]map[string]interface{}{
			{"slug": "email", "type": "email", "required": true, "label": "Email"},
			{"slug": "name", "type": "text", "required": false, "label": "Name"},
		})
		settings, _ := json.Marshal(map[string]interface{}{})
		_, err := pool.Exec(ctx,
			`INSERT INTO forms (id, context_id, slug, name, description, fields, settings)
			 VALUES (gen_random_uuid(), $1, $2, 'Snippet Test Form', '', $3, $4)`,
			contextID, slug, fields, settings)
		Expect(err).ToNot(HaveOccurred())
	}

	Describe("GET /form/definition/:contextID/:slug", func() {
		It("should return the form definition as JSON", func() {
			contextID := support.NewTestContextID()
			createForm(contextID, "snip-form")

			engine.ServeHTTP(w, httptest.NewRequest("GET", "/form/definition/"+contextID+"/snip-form", nil))
			Expect(w.Code).To(Equal(http.StatusOK))

			var def map[string]interface{}
			Expect(json.Unmarshal(w.Body.Bytes(), &def)).To(Succeed())
			Expect(def["slug"]).To(Equal("snip-form"))
			Expect(def["name"]).To(Equal("Snippet Test Form"))
		})

		It("should 404 for an unknown form", func() {
			contextID := support.NewTestContextID()
			engine.ServeHTTP(w, httptest.NewRequest("GET", "/form/definition/"+contextID+"/nope", nil))
			Expect(w.Code).To(Equal(http.StatusNotFound))
		})
	})

	Describe("GET /form/snippet/:contextID/:slug", func() {
		It("should return a JavaScript snippet containing the context ID and URLs", func() {
			contextID := support.NewTestContextID()
			createForm(contextID, "snip-form")

			engine.ServeHTTP(w, httptest.NewRequest("GET", "/form/snippet/"+contextID+"/snip-form", nil))
			Expect(w.Code).To(Equal(http.StatusOK))
			Expect(w.Header().Get("Content-Type")).To(ContainSubstring("javascript"))

			body := w.Body.String()
			Expect(body).To(ContainSubstring(contextID))
			Expect(body).To(ContainSubstring("/form/definition/" + contextID + "/snip-form"))
			Expect(body).To(ContainSubstring("/form/submit/snip-form"))
		})

		It("should 404 for an unknown form", func() {
			contextID := support.NewTestContextID()
			engine.ServeHTTP(w, httptest.NewRequest("GET", "/form/snippet/"+contextID+"/nope", nil))
			Expect(w.Code).To(Equal(http.StatusNotFound))
		})

		It("should serve the snippet for a .js-suffixed slug", func() {
			contextID := support.NewTestContextID()
			createForm(contextID, "snip-form")

			engine.ServeHTTP(w, httptest.NewRequest("GET", "/form/snippet/"+contextID+"/snip-form.js", nil))
			Expect(w.Code).To(Equal(http.StatusOK))
			Expect(w.Header().Get("Content-Type")).To(ContainSubstring("javascript"))
		})
	})

	Describe("existing form routes", func() {
		It("should still serve /form/list with the context header", func() {
			contextID := support.NewTestContextID()
			createForm(contextID, "snip-form")

			req := httptest.NewRequest("GET", "/form/list", nil)
			req.Header.Set("X-Context-ID", contextID)
			engine.ServeHTTP(w, req)
			Expect(w.Code).To(Equal(http.StatusOK))
		})

		It("should 401 /form/list without a context", func() {
			engine.ServeHTTP(w, httptest.NewRequest("GET", "/form/list", nil))
			Expect(w.Code).To(Equal(http.StatusUnauthorized))
		})
	})
})