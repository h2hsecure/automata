package cicd_test

import (
	"bytes"
	"net/http"
	"net/http/httptest"

	"github.com/gin-gonic/gin"
	bhandler "github.com/h2hsecure/automata/internal/adapter/api/handler"
	"github.com/h2hsecure/automata/internal/domain/settings"
	settingsrepo "github.com/h2hsecure/automata/internal/infrastructure/settings/repo"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func setupSettingsEngine() *gin.Engine {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	h := bhandler.NewSettingsHandler(settingsrepo.New(pool))
	admin := engine.Group("/api/admin")
	admin.GET("/settings", h.List)
	admin.PUT("/settings", h.Upsert)
	admin.DELETE("/settings/:key", h.Delete)
	return engine
}

var _ = Describe("Global Settings", func() {
	var (
		w      *httptest.ResponseRecorder
		engine *gin.Engine
		repo   settings.Repository
	)

	BeforeEach(func() {
		w = httptest.NewRecorder()
		repo = settingsrepo.New(pool)
		engine = setupSettingsEngine()
	})

	AfterEach(func() {
		_, _ = pool.Exec(ctx, "DELETE FROM global_settings")
	})

	Describe("repo", func() {
		It("upserts, gets, lists, and deletes", func() {
			Expect(repo.Upsert("smtp.host", "relay.example.com")).To(Succeed())
			v, err := repo.Get("smtp.host")
			Expect(err).NotTo(HaveOccurred())
			Expect(v).To(Equal("relay.example.com"))

			Expect(repo.Upsert("smtp.host", "other.example.com")).To(Succeed())
			v, _ = repo.Get("smtp.host")
			Expect(v).To(Equal("other.example.com"))

			all, err := repo.List()
			Expect(err).NotTo(HaveOccurred())
			Expect(all["smtp.host"]).To(Equal("other.example.com"))

			Expect(repo.Delete("smtp.host")).To(Succeed())
			_, err = repo.Get("smtp.host")
			Expect(err).NotTo(HaveOccurred())
			Expect(repo.Delete("smtp.host")).ToNot(Succeed()) // missing key
		})
	})

	Describe("HTTP", func() {
		It("upserts and lists settings", func() {
			req := httptest.NewRequest("PUT", "/api/admin/settings",
				bytes.NewBufferString(`{"key":"smtp.from","value":"noreply@automata.local"}`))
			req.Header.Set("Content-Type", "application/json")
			engine.ServeHTTP(w, req)
			Expect(w.Code).To(Equal(http.StatusOK))

			w = httptest.NewRecorder()
			req = httptest.NewRequest("GET", "/api/admin/settings", nil)
			engine.ServeHTTP(w, req)
			Expect(w.Code).To(Equal(http.StatusOK))
			Expect(w.Body.String()).To(ContainSubstring(`"smtp.from":"noreply@automata.local"`))
		})

		It("rejects an empty key with 400", func() {
			req := httptest.NewRequest("PUT", "/api/admin/settings",
				bytes.NewBufferString(`{"key":"  ","value":"x"}`))
			req.Header.Set("Content-Type", "application/json")
			engine.ServeHTTP(w, req)
			Expect(w.Code).To(Equal(http.StatusBadRequest))
		})

		It("deletes a setting and 404s when missing", func() {
			Expect(repo.Upsert("k1", "v1")).To(Succeed())
			req := httptest.NewRequest("DELETE", "/api/admin/settings/k1", nil)
			engine.ServeHTTP(w, req)
			Expect(w.Code).To(Equal(http.StatusNoContent))

			w = httptest.NewRecorder()
			req = httptest.NewRequest("DELETE", "/api/admin/settings/nope", nil)
			engine.ServeHTTP(w, req)
			Expect(w.Code).To(Equal(http.StatusNotFound))
		})
	})
})
