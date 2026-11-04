package middleware

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

func newLimitTestEngine(limit int64) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(LimitBody(limit))
	r.POST("/echo", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"ok": true})
	})
	return r
}

func TestLimitBody_RejectsOversizedBody(t *testing.T) {
	r := newLimitTestEngine(100)

	// 111 bytes: 11 over the limit.
	req := httptest.NewRequest("POST", "/echo", strings.NewReader(strings.Repeat("a", 111)))
	req.Header.Set("Content-Type", "application/octet-stream")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("expected 413, got %d: %s", w.Code, w.Body.String())
	}
}

func TestLimitBody_AcceptsBodyUnderLimit(t *testing.T) {
	r := newLimitTestEngine(100)

	req := httptest.NewRequest("POST", "/echo", strings.NewReader(strings.Repeat("a", 50)))
	req.Header.Set("Content-Type", "application/octet-stream")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
}