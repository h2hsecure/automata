package middleware

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/h2hsecure/automata/internal/domain/context"
)

// RequireContextMembership enforces tenant isolation for context-resolved
// routes. After ContextResolver has set context_id, it verifies that the
// authenticated user is a member of that context (or is a platform admin,
// DB-verified). API-key requests are skipped: AuthMiddleware already binds
// them to their own context, so a different context cannot be addressed.
func RequireContextMembership(userRepo context.UserRepository) gin.HandlerFunc {
	return func(c *gin.Context) {
		if _, ok := c.Get("auth_via_api_key"); ok {
			c.Next()
			return
		}
		userID := c.GetString("user_id")
		contextID := c.GetString("context_id")
		if userID == "" || contextID == "" {
			c.Next() // anonymous/public route
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