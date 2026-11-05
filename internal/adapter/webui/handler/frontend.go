package handler

import (
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/gin-gonic/gin"
)

// FrontendHandler serves the Vue.js SPA static files.
type FrontendHandler struct {
	webDir string
}

// NewFrontendHandler creates a new FrontendHandler.
func NewFrontendHandler(webDir string) *FrontendHandler {
	return &FrontendHandler{webDir: webDir}
}

// serveIndex serves index.html with Cache-Control: no-cache so browsers
// always revalidate the shell (the ETag ServeFile sets makes the
// revalidation a cheap 304). Without this, heuristic caching based on
// Last-Modified can keep serving a pre-deploy shell for hours after a
// release, whose content-hashed chunks no longer exist on the server.
func (h *FrontendHandler) serveIndex(c *gin.Context) {
	c.Header("Cache-Control", "no-cache")
	http.ServeFile(c.Writer, c.Request, filepath.Join(h.webDir, "index.html"))
}

// HandleIndex serves the main index.html.
func (h *FrontendHandler) HandleIndex(c *gin.Context) {
	h.serveIndex(c)
}

// HandleLogin serves the login page (redirects to SPA).
func (FrontendHandler) HandleLogin(c *gin.Context) {
	c.Redirect(http.StatusFound, "/")
}

// HandleSPA serves index.html for all non-API routes (Vue Router history mode).
func (h *FrontendHandler) HandleSPA(c *gin.Context) {
	path := c.Request.URL.Path

	// If it's a static asset, serve it directly
	fullPath := filepath.Join(h.webDir, path)
	if _, err := os.Stat(fullPath); err == nil {
		if strings.HasPrefix(path, "/assets/") {
			// Content-hashed build output: safe to cache indefinitely.
			c.Header("Cache-Control", "public, max-age=31536000, immutable")
		} else {
			// Unhashed files (e.g. favicon.svg): revalidate.
			c.Header("Cache-Control", "no-cache")
		}
		contentType := mime.TypeByExtension(filepath.Ext(path))
		if contentType == "" {
			contentType = "application/octet-stream"
		}
		c.Header("Content-Type", contentType)
		http.ServeFile(c.Writer, c.Request, fullPath)
		return
	}

	// A missing file that looks like a static asset request (has a file
	// extension) gets an honest 404. Falling back to index.html here
	// serves HTML in response to a JS/CSS request, which the browser
	// cannot execute ("error loading dynamically imported module").
	if filepath.Ext(path) != "" {
		c.Header("Cache-Control", "no-store")
		c.AbortWithStatus(http.StatusNotFound)
		return
	}

	// For SPA routes, serve index.html
	h.serveIndex(c)
}