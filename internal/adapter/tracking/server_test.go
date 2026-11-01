package tracking

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

// NewServer without handlers/repos: these tests only exercise route
// registration (method handling), not handler behavior.
func newTrackingServerForRouteTests() http.Handler {
	return NewServer(nil, nil, nil, nil, nil, nil)
}

func TestSnippetRoute_GetServes200(t *testing.T) {
	gin.SetMode(gin.TestMode)

	req := httptest.NewRequest("GET", "/snippet/"+testContextID+".js", nil)
	req.Host = "www.example.com"
	w := httptest.NewRecorder()
	newTrackingServerForRouteTests().ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
}

// gin >= 1.12 no longer auto-maps HEAD to GET, so GET-only routes 404 HEAD
// requests (curl -I, health checks, proxies). Public GET routes must accept
// HEAD.
func TestSnippetRoute_HeadServes200LikeGet(t *testing.T) {
	gin.SetMode(gin.TestMode)

	req := httptest.NewRequest("HEAD", "/snippet/"+testContextID+".js", nil)
	req.Host = "www.example.com"
	w := httptest.NewRecorder()
	newTrackingServerForRouteTests().ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
}

func TestHealthRoute_HeadServes200LikeGet(t *testing.T) {
	gin.SetMode(gin.TestMode)

	req := httptest.NewRequest("HEAD", "/health", nil)
	w := httptest.NewRecorder()
	newTrackingServerForRouteTests().ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
}