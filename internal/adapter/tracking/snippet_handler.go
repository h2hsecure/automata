package tracking

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/hekemen/automata/pkg/snippet"
)

// SnippetHandler serves the JavaScript tracking snippet.
type SnippetHandler struct{}

// ServeSnippet handles GET /snippet/:id.js — returns the tracking JS for a context.
func (h *SnippetHandler) ServeSnippet(c *gin.Context) {
	path := c.Request.URL.Path
	// Extract context ID from /snippet/{id}.js
	if len(path) > 10 && path[:9] == "/snippet/" && strings.HasSuffix(path, ".js") {
		contextID := path[9 : len(path)-3]
		if contextID == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "context ID required"})
			return
		}
		// Derive the tracking origin from the request so the snippet posts
		// back to the same origin it was served from (e.g. the website's own
		// host). Behind a TLS-terminating proxy, X-Forwarded-Proto carries
		// the public scheme; without it the snippet would post over http and
		// be blocked as mixed content on https pages.
		host := c.GetHeader("Host")
		if host == "" {
			host = c.Request.Host
		}
		scheme := "http"
		if proto := c.GetHeader("X-Forwarded-Proto"); proto != "" {
			scheme = strings.ToLower(strings.TrimSpace(strings.Split(proto, ",")[0]))
		}
		js, err := snippet.Generate(contextID, scheme+"://"+host, nil)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.Data(http.StatusOK, "application/javascript; charset=utf-8", []byte(js))
		return
	}
	c.JSON(http.StatusBadRequest, gin.H{"error": "invalid path"})
}
