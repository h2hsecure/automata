package handler

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

// newTestFrontend builds a FrontendHandler backed by a temp webDir
// containing an index.html and a hashed asset under assets/.
func newTestFrontend(t *testing.T) *FrontendHandler {
	t.Helper()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "index.html"), []byte("<html>index</html>"), 0o644); err != nil {
		t.Fatalf("write index.html: %v", err)
	}
	assets := filepath.Join(dir, "assets")
	if err := os.MkdirAll(assets, 0o755); err != nil {
		t.Fatalf("mkdir assets: %v", err)
	}
	if err := os.WriteFile(filepath.Join(assets, "app-abc123.js"), []byte("console.log('app')"), 0o644); err != nil {
		t.Fatalf("write asset: %v", err)
	}
	return NewFrontendHandler(dir)
}

// doGET runs one GET against the handler and returns the gin context
// plus the underlying recorder for assertions.
func doGET(t *testing.T, path string) (*gin.Context, *httptest.ResponseRecorder) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, path, nil)
	return c, w
}

func TestFrontend_IndexHasNoCacheHeader(t *testing.T) {
	h := newTestFrontend(t)
	c, w := doGET(t, "/")
	h.HandleIndex(c)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", w.Code)
	}
	if got := w.Header().Get("Cache-Control"); got != "no-cache" {
		t.Errorf("Cache-Control = %q, want %q", got, "no-cache")
	}
	if ct := w.Header().Get("Content-Type"); ct != "text/html; charset=utf-8" {
		t.Errorf("Content-Type = %q, want text/html", ct)
	}
}

func TestFrontend_HashedAssetIsImmutable(t *testing.T) {
	h := newTestFrontend(t)
	c, w := doGET(t, "/assets/app-abc123.js")
	h.HandleSPA(c)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", w.Code)
	}
	want := "public, max-age=31536000, immutable"
	if got := w.Header().Get("Cache-Control"); got != want {
		t.Errorf("Cache-Control = %q, want %q", got, want)
	}
	if ct := w.Header().Get("Content-Type"); !strings.HasPrefix(ct, "application/javascript") && !strings.HasPrefix(ct, "text/javascript") {
		t.Errorf("Content-Type = %q, want a javascript type", ct)
	}
}

func TestFrontend_MissingAssetReturns404NotHTML(t *testing.T) {
	h := newTestFrontend(t)
	c, w := doGET(t, "/assets/gone-lc3fkc0g.js")
	h.HandleSPA(c)
	if w.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404 (SPA fallback must not mask missing assets)", w.Code)
	}
	if ct := w.Header().Get("Content-Type"); ct == "text/html; charset=utf-8" {
		t.Errorf("Content-Type = %q, must not serve index.html for a missing asset", ct)
	}
}

func TestFrontend_SPAFallbackStillServesIndex(t *testing.T) {
	h := newTestFrontend(t)
	c, w := doGET(t, "/some/spa/route")
	h.HandleSPA(c)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", w.Code)
	}
	if ct := w.Header().Get("Content-Type"); ct != "text/html; charset=utf-8" {
		t.Errorf("Content-Type = %q, want text/html", ct)
	}
	if body := w.Body.String(); body != "<html>index</html>" {
		t.Errorf("body = %q, want index.html content", body)
	}
	if got := w.Header().Get("Cache-Control"); got != "no-cache" {
		t.Errorf("Cache-Control = %q, want %q", got, "no-cache")
	}
}