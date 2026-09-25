package handler

import (
	"mime"
	"net/http"
	"os"
	"path/filepath"

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

// HandleIndex serves the main index.html.
func (h *FrontendHandler) HandleIndex(c *gin.Context) {
	http.ServeFile(c.Writer, c.Request, filepath.Join(h.webDir, "index.html"))
}

// HandleLogin serves the login page.
func (FrontendHandler) HandleLogin(c *gin.Context) {
	c.HTML(http.StatusOK, "login.tmpl", gin.H{})
}

// HandleSPA serves index.html for all non-API routes (Vue Router history mode).
func (h *FrontendHandler) HandleSPA(c *gin.Context) {
	path := c.Request.URL.Path

	// If it's a static asset, serve it directly
	fullPath := filepath.Join(h.webDir, path)
	if _, err := os.Stat(fullPath); err == nil {
		contentType := mime.TypeByExtension(filepath.Ext(path))
		if contentType == "" {
			contentType = "application/octet-stream"
		}
		c.Header("Content-Type", contentType)
		http.ServeFile(c.Writer, c.Request, fullPath)
		return
	}

	// For SPA routes, serve index.html
	http.ServeFile(c.Writer, c.Request, filepath.Join(h.webDir, "index.html"))
}
