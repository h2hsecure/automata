package proxy

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/hekemen/automata/internal/domain/tenant"
)
type Proxy struct {
	tenantRepo tenant.Repository
}

// NewProxy creates a Gin router group for tenant content routing.
func NewProxy(tenantRepo tenant.Repository, router gin.IRouter) *Proxy {
	p := &Proxy{tenantRepo: tenantRepo}

	// Form rendering: /form/<slug>
	router.GET("/form/*path", p.formHandler())

	// Snippet serving: /snippet/<tenant-id>.js
	router.GET("/snippet/*path", p.snippetHandler())

	// Static assets: /static/*path
	router.StaticFS("/static", http.Dir("./static"))

	return p
}

func (p *Proxy) formHandler() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Extract tenant from context (set by tenant middleware)
		t, exists := c.Get("tenant")
		if !exists {
			c.JSON(http.StatusBadRequest, gin.H{"error": "tenant not found"})
			c.Abort()
			return
		}

		tenant := t.(*tenant.Tenant)
		_ = strings.TrimPrefix(c.Param("path"), "/")

		// For now, return a simple HTML response
		// Full implementation will render forms from database
		c.Header("Content-Type", "text/html")
		c.String(http.StatusOK, fmt.Sprintf("<html><body>Form for tenant: %s</body></html>", tenant.Name))
	}
}

func (p *Proxy) snippetHandler() gin.HandlerFunc {
	return func(c *gin.Context) {
		tenantID := strings.TrimPrefix(c.Param("path"), "/")
		tenantID = strings.TrimSuffix(tenantID, ".js")

		// Generate tracking snippet for tenant
		snippet := generateSnippet(tenantID)

		c.Header("Content-Type", "application/javascript")
		c.String(http.StatusOK, snippet)
	}
}

func generateSnippet(tenantID string) string {
	return fmt.Sprintf(`(function(w,d,s,l,i){w[l]=w[l]||[];w[l].push({'%s':new Date().getTime(),event:'nag'});var f=d.getElementsByTagName(s)[0],j=d.createElement(s),dl!='%s'?'&dl='+dl:'';j.async=true;j.src='https://automata.example.com/track.js?id='+i+dl;f.parentNode.insertBefore(j,f);})(window,document,'script','dataLayer','%s');`, tenantID, tenantID, tenantID)
}
