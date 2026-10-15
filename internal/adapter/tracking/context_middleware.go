package tracking

import (
	"github.com/gin-gonic/gin"
)

// ContextMiddleware resolves the context for public tracking endpoints from
// an explicit X-Context-ID header or context_id query parameter. It fails
// closed: when neither is present, context_id is left unset and the handler
// responds with its own error (401/400). Subdomain and default-context
// fallbacks are intentionally excluded.
func ContextMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		contextID := c.GetHeader("X-Context-ID")
		if contextID == "" {
			contextID = c.Query("context_id")
		}
		if contextID != "" {
			c.Set("context_id", contextID)
		}
		c.Next()
	}
}