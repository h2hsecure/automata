package api

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/hekemen/automata/internal/domain/context"
)

// ContextKeyType is the type for context keys used in the Gin context.
type ContextKeyType string

// ContextKey is the key used to store the resolved context in the Gin context.
const ContextKey ContextKeyType = "context"

// ErrNoContext indicates that no context could be resolved from the request.
var ErrNoContext = fmt.Errorf("no context found")

// ContextResolver is a Gin middleware that resolves the context from the request
// and sets it in the context. It tries X-Context-ID header first, then subdomain, then path-based fallback.
func ContextResolver(contextRepo context.Repository) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Skip context resolution for admin API routes
		if strings.HasPrefix(c.Request.URL.Path, "/admin") ||
			strings.HasPrefix(c.Request.URL.Path, "/auth") ||
			strings.HasPrefix(c.Request.URL.Path, "/api/auth") ||
			strings.HasPrefix(c.Request.URL.Path, "/api/admin") ||
			c.Request.URL.Path == "/health" ||
			strings.HasPrefix(c.Request.URL.Path, "/api/health") {
			c.Next()
			return
		}

		// Try X-Context-ID header first (preferred)
		if contextID := c.GetHeader("X-Context-ID"); contextID != "" {
			t, err := contextRepo.GetByID(contextID)
			if err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"error": "context not found"})
				c.Abort()
				return
			}
			c.Set(string(ContextKey), *t)
			c.Set("context_id", contextID)
			c.Next()
			return
		}

		// Try subdomain resolution
		t, err := resolveBySubdomain(c, contextRepo)
		if err != nil && err != ErrNoContext {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			c.Abort()
			return
		}
		if t != nil {
			c.Set(string(ContextKey), *t)
			c.Set("context_id", t.ID)
			c.Next()
			return
		}

		// Fall back to path-based resolution
		t, err = resolveByPath(c, contextRepo)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			c.Abort()
			return
		}
		if t != nil {
			c.Set(string(ContextKey), *t)
			c.Set("context_id", t.ID)
			c.Next()
			return
		}

		c.JSON(http.StatusBadRequest, gin.H{"error": "context not found"})
		c.Abort()
	}
}

// resolveBySubdomain extracts the subdomain from the Host header and looks up the context.
func resolveBySubdomain(c *gin.Context, contextRepo context.Repository) (*context.Context, error) {
	host := c.Request.Host

	// Strip port if present
	if idx := strings.LastIndex(host, ":"); idx != -1 {
		host = host[:idx]
	}

	parts := strings.Split(host, ".")
	if len(parts) < 2 {
		return nil, ErrNoContext
	}

	subdomain := parts[0]

	// Skip common non-context subdomains
	if subdomain == "www" || subdomain == "api" || subdomain == "mail" {
		return nil, ErrNoContext
	}

	t, err := contextRepo.GetBySlug(subdomain)
	if err != nil {
		return nil, ErrNoContext
	}
	return t, nil
}

// resolveByPath extracts the context slug from /context/<slug>/... or /t/<slug>/... paths.
func resolveByPath(c *gin.Context, contextRepo context.Repository) (*context.Context, error) {
	path := c.Request.URL.Path

	var slug string
	if strings.HasPrefix(path, "/context/") {
		rest := strings.TrimPrefix(path, "/context/")
		parts := strings.Split(rest, "/")
		if len(parts) == 0 || parts[0] == "" {
			return nil, ErrNoContext
		}
		slug = parts[0]
	} else if strings.HasPrefix(path, "/t/") {
		rest := strings.TrimPrefix(path, "/t/")
		parts := strings.Split(rest, "/")
		if len(parts) == 0 || parts[0] == "" {
			return nil, ErrNoContext
		}
		slug = parts[0]
	} else {
		return nil, ErrNoContext
	}

	t, err := contextRepo.GetBySlug(slug)
	if err != nil {
		return nil, ErrNoContext
	}
	return t, nil
}
