package api

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/hekemen/automata/internal/domain/tenant"
)

// ContextKey is the type for context keys used in the Gin context.
type ContextKey string

// TenantContextKey is the key used to store the resolved tenant in the Gin context.
const TenantContextKey ContextKey = "tenant"

// ErrNoTenant indicates that no tenant could be resolved from the request.
var ErrNoTenant = fmt.Errorf("no tenant found")

// TenantResolver is a Gin middleware that resolves the tenant from the request
// and sets it in the context. It tries X-Tenant-ID header first, then subdomain, then path-based fallback.
func TenantResolver(tenantRepo tenant.Repository) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Skip tenant resolution for admin API routes
		if strings.HasPrefix(c.Request.URL.Path, "/admin") ||
			strings.HasPrefix(c.Request.URL.Path, "/auth") ||
			strings.HasPrefix(c.Request.URL.Path, "/api/auth") ||
			strings.HasPrefix(c.Request.URL.Path, "/api/admin") ||
			c.Request.URL.Path == "/health" ||
			strings.HasPrefix(c.Request.URL.Path, "/api/health") {
			c.Next()
			return
		}

		// Try X-Tenant-ID header first (preferred)
		if tenantID := c.GetHeader("X-Tenant-ID"); tenantID != "" {
			t, err := tenantRepo.GetByID(tenantID)
			if err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"error": "tenant not found"})
				c.Abort()
				return
			}
			c.Set(string(TenantContextKey), t)
			c.Set("tenant_id", tenantID)
			c.Next()
			return
		}

		// Try subdomain resolution
		t, err := resolveBySubdomain(c, tenantRepo)
		if err != nil && err != ErrNoTenant {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			c.Abort()
			return
		}
		if t != nil {
			c.Set(string(TenantContextKey), t)
			c.Set("tenant_id", t.ID)
			c.Next()
			return
		}

		// Fall back to path-based resolution
		t, err = resolveByPath(c, tenantRepo)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			c.Abort()
			return
		}
		if t != nil {
			c.Set(string(TenantContextKey), t)
			c.Set("tenant_id", t.ID)
			c.Next()
			return
		}

		c.JSON(http.StatusBadRequest, gin.H{"error": "tenant not found"})
		c.Abort()
	}
}

// resolveBySubdomain extracts the subdomain from the Host header and looks up the tenant.
func resolveBySubdomain(c *gin.Context, tenantRepo tenant.Repository) (*tenant.Tenant, error) {
	host := c.Request.Host

	// Strip port if present
	if idx := strings.LastIndex(host, ":"); idx != -1 {
		host = host[:idx]
	}

	parts := strings.Split(host, ".")
	if len(parts) < 2 {
		return nil, ErrNoTenant
	}

	subdomain := parts[0]

	// Skip common non-tenant subdomains
	if subdomain == "www" || subdomain == "api" || subdomain == "mail" {
		return nil, ErrNoTenant
	}

	t, err := tenantRepo.GetBySlug(subdomain)
	if err != nil {
		return nil, ErrNoTenant
	}
	return t, nil
}

// resolveByPath extracts the tenant slug from /tenant/<slug>/... or /t/<slug>/... paths.
func resolveByPath(c *gin.Context, tenantRepo tenant.Repository) (*tenant.Tenant, error) {
	path := c.Request.URL.Path

	var slug string
	if strings.HasPrefix(path, "/tenant/") {
		rest := strings.TrimPrefix(path, "/tenant/")
		parts := strings.Split(rest, "/")
		if len(parts) == 0 || parts[0] == "" {
			return nil, ErrNoTenant
		}
		slug = parts[0]
	} else if strings.HasPrefix(path, "/t/") {
		rest := strings.TrimPrefix(path, "/t/")
		parts := strings.Split(rest, "/")
		if len(parts) == 0 || parts[0] == "" {
			return nil, ErrNoTenant
		}
		slug = parts[0]
	} else {
		return nil, ErrNoTenant
	}

	t, err := tenantRepo.GetBySlug(slug)
	if err != nil {
		return nil, ErrNoTenant
	}
	return t, nil
}
