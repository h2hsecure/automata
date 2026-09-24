package proxy

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	domainctx "github.com/hekemen/automata/internal/domain/context"
)
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

		// Generate tracking snippet for context
		snippet := generateSnippet(contextID)

		c.Header("Content-Type", "application/javascript")
		c.String(http.StatusOK, snippet)
	}
}

func generateSnippet(contextID string) string {
	return fmt.Sprintf(`(function(w,d,s,l,i){w[l]=w[l]||[];w[l].push({'%s':new Date().getTime(),event:'nag'});var f=d.getElementsByTagName(s)[0],j=d.createElement(s),dl!='%s'?'&dl='+dl:'';j.async=true;j.src='https://automata.example.com/track.js?id='+i+dl;f.parentNode.insertBefore(j,f);})(window,document,'script','dataLayer','%s');`, contextID, contextID, contextID)
}
