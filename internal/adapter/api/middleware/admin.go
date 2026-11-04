package middleware

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/h2hsecure/automata/internal/domain/context"
)

// RequireAdmin gates admin routes with a DB-verified check of the caller's
// is_admin flag. It must run after AuthMiddleware (which sets user_id).
// The admin status is read from the user row, never from the JWT claim, so a
// forged or stale token cannot grant admin access.
func RequireAdmin(userRepo context.UserRepository) gin.HandlerFunc {
	return func(c *gin.Context) {
		userID := c.GetString("user_id")
		user, err := userRepo.GetByID(userID)
		if err != nil || user == nil || !user.IsAdmin {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "admin required"})
			return
		}
		c.Set("user", user) // DB-verified user for downstream
		c.Next()
	}
}