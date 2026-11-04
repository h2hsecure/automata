package middleware

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/h2hsecure/automata/internal/domain/context"
)

// RequireContextAccess validates access to an explicitly addressed context on
// admin routes (which skip ContextResolver). The context ID is resolved from
// the given path params in order, falling back to the X-Context-ID header.
// When no context can be resolved, the check is skipped (the handler decides
// the target). Platform admins (DB-verified) pass; everyone else needs a
// membership row for the target context.
//
// Callers must only pass param names that carry a context ID on their routes
// (e.g. "id" for /admin/contexts/:id/...), never param names with other
// meanings (e.g. the API-key id in /admin/api-keys/:id — pass no params and
// let the header be validated).
func RequireContextAccess(userRepo context.UserRepository, paramNames ...string) gin.HandlerFunc {
	return func(c *gin.Context) {
		userID := c.GetString("user_id")
		if userID == "" {
			c.Next() // unauthenticated: AuthMiddleware will reject
			return
		}
		contextID := ""
		for _, name := range paramNames {
			if v := c.Param(name); v != "" {
				contextID = v
				break
			}
		}
		if contextID == "" {
			contextID = c.GetHeader("X-Context-ID")
		}
		if contextID == "" {
			c.Next()
			return
		}
		user, err := userRepo.GetByID(userID)
		if err == nil && user != nil && user.IsAdmin {
			c.Next() // platform admin
			return
		}
		mc, err := userRepo.GetByUserIDContext(c.Request.Context(), userID, contextID)
		if err != nil || mc == nil {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "no access to this context"})
			return
		}
		c.Next()
	}
}