package tracking

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestTrackingSnippet(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Tracking Snippet Suite")
}

const testContextID = "550e8400-e29b-41d4-a716-446655440000"

var _ = Describe("SnippetHandler.ServeSnippet", func() {
	var engine *gin.Engine

	BeforeEach(func() {
		gin.SetMode(gin.TestMode)
		engine = gin.New()
		engine.GET("/snippet/:id.js", (&SnippetHandler{}).ServeSnippet)
	})

	newSnippetRequest := func(host string) *http.Request {
		req := httptest.NewRequest("GET", "/snippet/"+testContextID+".js", nil)
		req.Host = host
		return req
	}

	It("uses the forwarded https scheme in the generated tracking URL", func() {
		req := newSnippetRequest("m.h2hsecure.com")
		req.Header.Set("X-Forwarded-Proto", "https")
		w := httptest.NewRecorder()

		engine.ServeHTTP(w, req)

		Expect(w.Code).To(Equal(http.StatusOK))
		Expect(w.Body.String()).To(ContainSubstring("https://m.h2hsecure.com"))
	})

	It("uses the forwarded http scheme in the generated tracking URL", func() {
		req := newSnippetRequest("m.h2hsecure.com")
		req.Header.Set("X-Forwarded-Proto", "http")
		w := httptest.NewRecorder()

		engine.ServeHTTP(w, req)

		Expect(w.Code).To(Equal(http.StatusOK))
		Expect(w.Body.String()).To(ContainSubstring("http://m.h2hsecure.com"))
	})

	It("falls back to http when no forwarded proto is present", func() {
		req := newSnippetRequest("m.h2hsecure.com")
		w := httptest.NewRecorder()

		engine.ServeHTTP(w, req)

		Expect(w.Code).To(Equal(http.StatusOK))
		Expect(w.Body.String()).To(ContainSubstring("http://m.h2hsecure.com"))
	})
})