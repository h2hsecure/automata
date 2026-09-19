package middleware

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/hekemen/automata/internal/domain/auth"
)

// AuthMiddleware validates JWT tokens and API keys.
func AuthMiddleware(authService auth.AuthService) gin.HandlerFunc {
	return func(c *gin.Context) {
		path := c.Request.URL.Path
		if strings.HasPrefix(path, "/auth/login") || strings.HasPrefix(path, "/health") {
			c.Next()
			return
		}

		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "missing authorization header"})
			c.Abort()
			return
		}

		token := strings.TrimPrefix(authHeader, "Bearer ")

		userID, err := authService.VerifyTokenUserOnly(token)
		if err == nil {
			c.Set("user_id", userID)
			c.Next()
			return
		}

		apiKey, err := authService.ValidateAPIKey(token)
		if err == nil {
			c.Set("user_id", apiKey.UserID)
			c.Set("tenant_id", apiKey.TenantID)
			c.Next()
			return
		}

		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid token or API key"})
		c.Abort()
	}
}
