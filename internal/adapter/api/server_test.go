package api

import (
	"testing"

	"github.com/gin-gonic/gin"
	infraauth "github.com/h2hsecure/automata/internal/infrastructure/auth"
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

// The MCP skill file is served at two paths with different credential models:
//   /api/mcp-skill.md       — direct to the API engine (Bearer API key; external agents)
//   /api/webui/mcp-skill.md — via the BFF (session cookie → injected JWT; the Web UI)
// The Web UI only ever holds a session cookie, and the ingress routes the BFF
// catch-all for /api/* except the two Exact-match MCP bypass paths, so the SPA
// must use the webui path. Both are registered on the engine.
var _ = Describe("NewServer MCP skill file routes", func() {
	var engine *gin.Engine

	BeforeEach(func() {
		engine = newTestServer()
	})

	It("registers the API-key path /api/mcp-skill.md", func() {
		Expect(routeExists(engine, "GET", "/api/mcp-skill.md")).To(BeTrue())
	})

	It("registers the Web UI path /api/webui/mcp-skill.md", func() {
		Expect(routeExists(engine, "GET", "/api/webui/mcp-skill.md")).To(BeTrue())
	})
})

// gin >= 1.12 no longer auto-maps HEAD to GET, so the public health endpoint
// is registered for both methods (same fix as the tracking server, b2746ec).
var _ = Describe("NewServer health route", func() {
	var engine *gin.Engine

	BeforeEach(func() {
		engine = newTestServer()
	})

	It("registers /api/health for GET and HEAD", func() {
		Expect(routeExists(engine, "GET", "/api/health")).To(BeTrue())
		Expect(routeExists(engine, "HEAD", "/api/health")).To(BeTrue())
	})
})