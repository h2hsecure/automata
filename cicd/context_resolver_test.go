package cicd_test

import (
	"net/http"
	"net/http/httptest"

	"github.com/gin-gonic/gin"
	apiadapter "github.com/hekemen/automata/internal/adapter/api"
	ctxrepo "github.com/hekemen/automata/internal/infrastructure/context/repo"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("HTTP Integration Tests - Context Resolver", func() {
	var w *httptest.ResponseRecorder

	BeforeEach(func() {
		gin.SetMode(gin.TestMode)
		w = httptest.NewRecorder()
	})

	AfterEach(func() {
		_, _ = pool.Exec(ctx, "DELETE FROM contexts")
	})

	It("should return 400 when no context exists and none is provided", func() {
		// The global AfterEach guarantees the contexts table is empty here.
		engine := gin.New()
		engine.Use(apiadapter.ContextResolver(ctxrepo.NewPostgresRepo(pool)))
		engine.GET("/api/contacts", func(c *gin.Context) {
			c.JSON(http.StatusOK, gin.H{"ok": true})
		})

		// No X-Context-ID header, no context_id query param, host example.com
		// resolves no subdomain context, and there is no default context.
		req := httptest.NewRequest("GET", "/api/contacts", nil)
		engine.ServeHTTP(w, req)

		Expect(w.Code).To(Equal(http.StatusBadRequest))
		Expect(w.Body.String()).To(ContainSubstring("no context available"))
	})
})