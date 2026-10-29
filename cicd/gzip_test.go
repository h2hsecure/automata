package cicd_test

import (
	"bytes"
	"compress/gzip"
	"io"
	"net/http"
	"net/http/httptest"

	"github.com/gin-gonic/gin"
	infraauth "github.com/h2hsecure/automata/internal/infrastructure/auth"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Gzip compression on the API server", func() {
	var engine *gin.Engine

	BeforeEach(func() {
		infraauth.InitForTesting("test-secret-key-for-jwt-signing-32bytes", "test-cookie-secret-32bytes-xxxx!")
		engine = setupCreationServer()
	})

	It("compresses JSON responses when the client accepts gzip", func() {
		w := httptest.NewRecorder()
		req := httptest.NewRequest("GET", "/api/health", nil)
		req.Header.Set("Accept-Encoding", "gzip")
		engine.ServeHTTP(w, req)

		Expect(w.Code).To(Equal(http.StatusOK))
		Expect(w.Header().Get("Content-Encoding")).To(Equal("gzip"))

		zr, err := gzip.NewReader(bytes.NewReader(w.Body.Bytes()))
		Expect(err).NotTo(HaveOccurred())
		body, err := io.ReadAll(zr)
		Expect(err).NotTo(HaveOccurred())
		Expect(string(body)).To(ContainSubstring(`"status":"ok"`))
	})

	It("does not compress when the client does not accept gzip", func() {
		w := httptest.NewRecorder()
		req := httptest.NewRequest("GET", "/api/health", nil)
		engine.ServeHTTP(w, req)

		Expect(w.Code).To(Equal(http.StatusOK))
		Expect(w.Header().Get("Content-Encoding")).To(BeEmpty())
		Expect(w.Body.String()).To(ContainSubstring(`"status":"ok"`))
	})
})