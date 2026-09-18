package banner

import (
	"strings"

	"github.com/gin-gonic/gin"
)

// ResolveTenant extracts the tenant ID from the request.
// It tries the X-Tenant-ID header first, then falls back to subdomain extraction
// from the Host header. Returns empty string and sets a 400 response if neither is present.
func ResolveTenant(c *gin.Context) string {
	// Try X-Tenant-ID header first
	if tenantID := c.GetHeader("X-Tenant-ID"); tenantID != "" {
		return tenantID
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
		c.JSON(400, gin.H{"error": "tenant ID required: provide X-Tenant-ID header or a valid subdomain"})
		c.Abort()
		return ""
	}

	subdomain := parts[0]

	// Skip common non-tenant subdomains
	if subdomain == "www" || subdomain == "api" || subdomain == "mail" {
		c.JSON(400, gin.H{"error": "tenant ID required: provide X-Tenant-ID header or a valid subdomain"})
		c.Abort()
		return ""
	}

	return subdomain
}
