package proxy

import (
	"fmt"
	"net/http"
	"regexp"
	"strings"

	"github.com/gin-gonic/gin"
	domainctx "github.com/h2hsecure/automata/internal/domain/context"
)

// uuidPattern matches a lowercase UUID. The snippet handler embeds the
// context ID in generated JavaScript, so only a strict UUID is accepted —
// anything else cannot break out of the string literal.
var uuidPattern = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`)
type Proxy struct {
	contextRepo domainctx.Repository
}

// NewProxy creates a Gin router group for context content routing.
func NewProxy(contextRepo domainctx.Repository, router gin.IRouter) *Proxy {
	p := &Proxy{contextRepo: contextRepo}

	// Form rendering: /form/<slug>
	router.GET("/form/*path", p.formHandler())

	// Snippet serving: /snippet/<context-id>.js
	router.GET("/snippet/*path", p.snippetHandler())

	// Static assets: /static/*path
	router.StaticFS("/static", http.Dir("./static"))

	return p
}

func (p *Proxy) formHandler() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Extract context from context (set by context middleware)
		ctx, exists := c.Get("context")
		if !exists {
			c.JSON(http.StatusBadRequest, gin.H{"error": "context not found"})
			c.Abort()
			return
		}

		context := ctx.(*domainctx.Context)
		_ = strings.TrimPrefix(c.Param("path"), "/")

		// For now, return a simple HTML response
		// Full implementation will render forms from database
		c.Header("Content-Type", "text/html")
		c.String(http.StatusOK, fmt.Sprintf("<html><body>Form for context: %s</body></html>", context.Name))
	}
}

func (p *Proxy) snippetHandler() gin.HandlerFunc {
	return func(c *gin.Context) {
		contextID := strings.TrimPrefix(c.Param("path"), "/")
		contextID = strings.TrimSuffix(contextID, ".js")

		// The context ID is interpolated into generated JavaScript. Reject
		// anything that is not a lowercase UUID so the path segment can
		// never inject code into the snippet.
		if !uuidPattern.MatchString(contextID) {
			c.AbortWithStatus(http.StatusNotFound)
			return
		}

		// Generate tracking snippet for context
		snippet := generateSnippet(contextID)

		c.Header("Content-Type", "application/javascript")
		c.String(http.StatusOK, snippet)
	}
}

// escapeJS escapes a value for interpolation into a single-quoted JS
// string literal. Defense in depth: the snippet handler already rejects
// non-UUID input, but the generated code treats the value as untrusted.
func escapeJS(s string) string {
	s = strings.ReplaceAll(s, `\`, `\\`)
	s = strings.ReplaceAll(s, `'`, `\'`)
	s = strings.ReplaceAll(s, "\n", `\n`)
	s = strings.ReplaceAll(s, "\r", `\r`)
	return s
}

func generateSnippet(contextID string) string {
	id := escapeJS(contextID)
	return fmt.Sprintf(`(function(w,d,s,l,i){w[l]=w[l]||[];w[l].push({'%s':new Date().getTime(),event:'nag'});var f=d.getElementsByTagName(s)[0],j=d.createElement(s),dl!='%s'?'&dl='+dl:'';j.async=true;j.src='https://automata.example.com/track.js?id='+i+dl;f.parentNode.insertBefore(j,f);})(window,document,'script','dataLayer','%s');`, id, id, id)
}
