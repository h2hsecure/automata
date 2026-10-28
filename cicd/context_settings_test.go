package cicd_test

import (
	"bytes"
	"net/http"
	"net/http/httptest"

	"github.com/gin-gonic/gin"
	"github.com/h2hsecure/automata/cicd/support"
	bhandler "github.com/h2hsecure/automata/internal/adapter/api/handler"
	"github.com/h2hsecure/automata/internal/domain/settings"
	settingsrepo "github.com/h2hsecure/automata/internal/infrastructure/settings/repo"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func setupContextSettingsEngine() *gin.Engine {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	h := bhandler.NewContextSettingsHandler(settingsrepo.NewContext(pool))
	admin := engine.Group("/api/admin")
	admin.GET("/contexts/:id/settings", h.List)
	admin.PUT("/contexts/:id/settings", h.Upsert)
	admin.DELETE("/contexts/:id/settings/:key", h.Delete)
	return engine
}

var _ = Describe("Context Settings", func() {
	var (
		w         *httptest.ResponseRecorder
		engine    *gin.Engine
		repo      settings.ContextRepository
		contextID string
		otherID   string
	)

	BeforeEach(func() {
		w = httptest.NewRecorder()
		repo = settingsrepo.NewContext(pool)
		engine = setupContextSettingsEngine()
		contextID = support.NewTestContextID()
		otherID = support.NewTestContextID()
	})

	AfterEach(func() {
		_, _ = pool.Exec(ctx, "DELETE FROM context_settings")
	})

	Describe("repo", func() {
		It("upserts, gets, lists, and deletes", func() {
			Expect(repo.Upsert(contextID, "smtp.host", "relay.example.com")).To(Succeed())
			v, err := repo.Get(contextID, "smtp.host")
			Expect(err).NotTo(HaveOccurred())
			Expect(v).To(Equal("relay.example.com"))

			Expect(repo.Upsert(contextID, "smtp.host", "other.example.com")).To(Succeed())
			v, _ = repo.Get(contextID, "smtp.host")
			Expect(v).To(Equal("other.example.com"))

			all, err := repo.List(contextID)
			Expect(err).NotTo(HaveOccurred())
			Expect(all["smtp.host"]).To(Equal("other.example.com"))

			Expect(repo.Delete(contextID, "smtp.host")).To(Succeed())
			_, err = repo.Get(contextID, "smtp.host")
			Expect(err).To(MatchError(settings.ErrNotFound))
			Expect(repo.Delete(contextID, "smtp.host")).To(MatchError(settings.ErrNotFound))
		})

		It("returns ErrNotFound for an unknown key", func() {
			_, err := repo.Get(contextID, "nope")
			Expect(err).To(MatchError(settings.ErrNotFound))
		})

		It("keeps settings isolated across contexts", func() {
			Expect(repo.Upsert(contextID, "smtp.host", "a.example.com")).To(Succeed())
			Expect(repo.Upsert(otherID, "smtp.host", "b.example.com")).To(Succeed())

			v, err := repo.Get(contextID, "smtp.host")
			Expect(err).NotTo(HaveOccurred())
			Expect(v).To(Equal("a.example.com"))

			v, err = repo.Get(otherID, "smtp.host")
			Expect(err).NotTo(HaveOccurred())
			Expect(v).To(Equal("b.example.com"))

			all, err := repo.List(contextID)
			Expect(err).NotTo(HaveOccurred())
			Expect(all).To(HaveLen(1))
			Expect(all["smtp.host"]).To(Equal("a.example.com"))

			Expect(repo.Delete(contextID, "smtp.host")).To(Succeed())
			_, err = repo.Get(otherID, "smtp.host")
			Expect(err).NotTo(HaveOccurred())
		})
	})

	Describe("HTTP", func() {
		It("upserts and lists settings", func() {
			req := httptest.NewRequest("PUT", "/api/admin/contexts/"+contextID+"/settings",
				bytes.NewBufferString(`{"key":"smtp.from","value":"noreply@automata.local"}`))
			req.Header.Set("Content-Type", "application/json")
			engine.ServeHTTP(w, req)
			Expect(w.Code).To(Equal(http.StatusOK))

			w = httptest.NewRecorder()
			req = httptest.NewRequest("GET", "/api/admin/contexts/"+contextID+"/settings", nil)
			engine.ServeHTTP(w, req)
			Expect(w.Code).To(Equal(http.StatusOK))
			Expect(w.Body.String()).To(ContainSubstring(`"smtp.from":"noreply@automata.local"`))
		})

		It("rejects an empty key with 400", func() {
			req := httptest.NewRequest("PUT", "/api/admin/contexts/"+contextID+"/settings",
				bytes.NewBufferString(`{"key":"  ","value":"x"}`))
			req.Header.Set("Content-Type", "application/json")
			engine.ServeHTTP(w, req)
			Expect(w.Code).To(Equal(http.StatusBadRequest))
		})

		It("rejects an invalid context ID with 400", func() {
			req := httptest.NewRequest("GET", "/api/admin/contexts/not-a-uuid/settings", nil)
			engine.ServeHTTP(w, req)
			Expect(w.Code).To(Equal(http.StatusBadRequest))

			w = httptest.NewRecorder()
			req = httptest.NewRequest("PUT", "/api/admin/contexts/not-a-uuid/settings",
				bytes.NewBufferString(`{"key":"k","value":"v"}`))
			req.Header.Set("Content-Type", "application/json")
			engine.ServeHTTP(w, req)
			Expect(w.Code).To(Equal(http.StatusBadRequest))

			w = httptest.NewRecorder()
			req = httptest.NewRequest("DELETE", "/api/admin/contexts/not-a-uuid/settings/k", nil)
			engine.ServeHTTP(w, req)
			Expect(w.Code).To(Equal(http.StatusBadRequest))
		})

		It("deletes a setting and 404s when missing", func() {
			Expect(repo.Upsert(contextID, "k1", "v1")).To(Succeed())
			req := httptest.NewRequest("DELETE", "/api/admin/contexts/"+contextID+"/settings/k1", nil)
			engine.ServeHTTP(w, req)
			Expect(w.Code).To(Equal(http.StatusNoContent))

			w = httptest.NewRecorder()
			req = httptest.NewRequest("DELETE", "/api/admin/contexts/"+contextID+"/settings/nope", nil)
			engine.ServeHTTP(w, req)
			Expect(w.Code).To(Equal(http.StatusNotFound))
		})
	})
})
