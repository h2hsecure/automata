package banner

import (
	"strings"

	"github.com/gin-gonic/gin"
)

// ResolveContext extracts the context ID from the request.
// It tries the X-Context-ID header first, then falls back to subdomain extraction
// from the Host header. Returns empty string and sets a 400 response if neither is present.
func ResolveContext(c *gin.Context) string {
	// Try X-Context-ID header first
	if contextID := c.GetHeader("X-Context-ID"); contextID != "" {
		return contextID
	}

	// Fall back to subdomain extraction from Host header
	host := c.GetHeader("Host")
	if host == "" {
		host = c.Request.Host
	}

	// Strip port if present
	if idx := strings.LastIndex(host, ":"); idx != -1 {
		host = host[:idx]
	}

	parts := strings.Split(host, ".")
	if len(parts) < 2 {
		c.JSON(400, gin.H{"error": "context ID required: provide X-Context-ID header or a valid subdomain"})
		c.Abort()
		return ""
	}

	subdomain := parts[0]

	// Skip common non-context subdomains
	if subdomain == "www" || subdomain == "api" || subdomain == "mail" {
		c.JSON(400, gin.H{"error": "context ID required: provide X-Context-ID header or a valid subdomain"})
		c.Abort()
		return ""
	}

	return subdomain
}
