package api

import (
	"testing"

	"github.com/gin-gonic/gin"
	infraauth "github.com/hekemen/automata/internal/infrastructure/auth"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestServerRoutes(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Server Routes Suite")
}

// routeExists reports whether the engine registers the given method+path.
func routeExists(engine *gin.Engine, method, path string) bool {
	for _, r := range engine.Routes() {
		if r.Method == method && r.Path == path {
			return true
		}
	}
	return false
}

// newTestServer builds the production server with nil dependencies (all
// repo/service construction is lazy) so route registration can be verified
// without a database.
func newTestServer() *gin.Engine {
	gin.SetMode(gin.TestMode)
	infraauth.InitForTesting("test-secret-key-for-jwt-signing-32bytes", "test-cookie-secret-32bytes-xxxx!")
	return NewServer(gin.New(), nil, nil, nil, nil, nil, "/api")
}

var _ = Describe("NewServer contact import/export/tag routes", func() {
	var engine *gin.Engine

	BeforeEach(func() {
		engine = newTestServer()
	})

	It("registers the contact import route", func() {
		Expect(routeExists(engine, "POST", "/api/context/contacts/import")).To(BeTrue())
	})

	It("registers the contact export route", func() {
		Expect(routeExists(engine, "GET", "/api/context/contacts/export")).To(BeTrue())
	})

	It("registers the contact tag routes", func() {
		Expect(routeExists(engine, "GET", "/api/context/contacts/tags")).To(BeTrue())
		Expect(routeExists(engine, "POST", "/api/context/contacts/tags")).To(BeTrue())
		Expect(routeExists(engine, "DELETE", "/api/context/contacts/tags/:id")).To(BeTrue())
	})
})