package middleware

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/hekemen/automata/internal/domain/auth"
	"github.com/hekemen/automata/internal/infrastructure/config"
)

// AuthMiddleware validates JWT tokens and API keys.
// It also performs session revocation checks by comparing JWT iat against password_changed_at.
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

		secretKey := config.Get("auth.secret_key")
		if secretKey == "" {
			secretKey = "automata-dev-secret-key-change-in-production"
		}

		userID, err := authService.VerifyTokenUserOnly(token)
		if err == nil {
			c.Set("user_id", userID)

			// Parse token to get is_admin and iat claims
			parsedToken, parseErr := jwt.Parse(token, func(t *jwt.Token) (interface{}, error) {
				return []byte(secretKey), nil
			})
			if parseErr == nil {
				if claims, ok := parsedToken.Claims.(jwt.MapClaims); ok {
					if isAdmin, ok := claims["is_admin"].(bool); ok {
						c.Set("is_admin", isAdmin)
					}

					// Session revocation check: compare JWT iat with password_changed_at
					if user, userErr := authService.GetUserByID(userID); userErr == nil {
						if iat, ok := claims["iat"].(float64); ok {
							if int64(iat) < user.PasswordChangedAt.Unix() {
								c.JSON(http.StatusUnauthorized, gin.H{"error": "session revoked"})
								c.Abort()
								return
							}
						}
					}
				}
			}

			c.Next()
			return
		}

		apiKey, err := authService.ValidateAPIKey(token)
		if err == nil {
			c.Set("user_id", apiKey.UserID)
			c.Set("context_id", apiKey.ContextID)
			c.Next()
			return
		}

		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid token or API key"})
		c.Abort()
	}
}
