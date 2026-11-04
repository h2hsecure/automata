package middleware

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
)

func newRateTestEngine(l *Limiter, capacity int, window time.Duration, keyFn func(*gin.Context) string) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(RateLimit(l, capacity, window, keyFn))
	r.POST("/login", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"ok": true})
	})
	return r
}

func doRateRequest(r *gin.Engine, body string) *httptest.ResponseRecorder {
	req := httptest.NewRequest("POST", "/login", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

func TestRateLimit_AllowsUnderCapacity(t *testing.T) {
	r := newRateTestEngine(NewLimiter(), 10, time.Minute, IPKeyFn)

	for i := 0; i < 10; i++ {
		w := doRateRequest(r, `{"email":"a@b.com"}`)
		if w.Code != http.StatusOK {
			t.Fatalf("request %d: expected 200, got %d", i+1, w.Code)
		}
	}

	w := doRateRequest(r, `{"email":"a@b.com"}`)
	if w.Code != http.StatusTooManyRequests {
		t.Fatalf("11th request: expected 429, got %d", w.Code)
	}
	if retryAfter := w.Header().Get("Retry-After"); retryAfter == "" {
		t.Error("expected Retry-After header on 429")
	}
}

func TestRateLimit_RefillsAfterWindow(t *testing.T) {
	r := newRateTestEngine(NewLimiter(), 2, 100*time.Millisecond, IPKeyFn)

	for i := 0; i < 2; i++ {
		if w := doRateRequest(r, `{"email":"a@b.com"}`); w.Code != http.StatusOK {
			t.Fatalf("request %d: expected 200, got %d", i+1, w.Code)
		}
	}
	if w := doRateRequest(r, `{"email":"a@b.com"}`); w.Code != http.StatusTooManyRequests {
		t.Fatalf("expected 429 after capacity exhausted, got %d", w.Code)
	}

	time.Sleep(150 * time.Millisecond)

	if w := doRateRequest(r, `{"email":"a@b.com"}`); w.Code != http.StatusOK {
		t.Fatalf("expected 200 after refill window, got %d", w.Code)
	}
}

func TestRateLimit_TrackingPerIP(t *testing.T) {
	r := newRateTestEngine(NewLimiter(), 300, time.Minute, IPKeyFn)

	for i := 0; i < 300; i++ {
		w := doRateRequest(r, `{}`)
		if w.Code != http.StatusOK {
			t.Fatalf("request %d: expected 200, got %d", i+1, w.Code)
		}
	}

	if w := doRateRequest(r, `{}`); w.Code != http.StatusTooManyRequests {
		t.Fatalf("301st request: expected 429, got %d", w.Code)
	}
}

func TestRateLimit_DistinctKeysHaveSeparateBuckets(t *testing.T) {
	l := NewLimiter()
	if !l.Allow("user-a", 1, time.Minute) {
		t.Fatal("expected first token for user-a")
	}
	if l.Allow("user-a", 1, time.Minute) {
		t.Fatal("expected user-a bucket exhausted")
	}
	if !l.Allow("user-b", 1, time.Minute) {
		t.Fatal("expected user-b to have its own bucket")
	}
}

func TestRateLimit_LoginKeyFn_UsesIPAndEmail(t *testing.T) {
	// LoginKeyFn peeks the body for the email; the body must be restored so
	// downstream handlers can read it.
	r := gin.New()
	r.POST("/login", func(c *gin.Context) {
		var body struct {
			Email string `json:"email"`
		}
		if err := c.ShouldBindJSON(&body); err != nil || body.Email != "peek@peek.com" {
			c.String(http.StatusInternalServerError, "body not restored: %v", body.Email)
			return
		}
		c.JSON(http.StatusOK, gin.H{"ok": true})
	})

	req := httptest.NewRequest("POST", "/login", strings.NewReader(`{"email":"peek@peek.com"}`))
	req.Header.Set("Content-Type", "application/json")
	req.RemoteAddr = "203.0.113.7:1234"
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	// Key must include both IP and (lowercased) email. Use a fresh request:
	// the one above had its body consumed by the handler.
	freshReq := httptest.NewRequest("POST", "/login", strings.NewReader(`{"email":"Peek@Peek.com"}`))
	freshReq.RemoteAddr = "203.0.113.7:1234"
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = freshReq
	key := LoginKeyFn(c)
	if !strings.Contains(key, "203.0.113.7") || !strings.Contains(key, "peek@peek.com") {
		t.Errorf("key %q should contain client IP and email", key)
	}
}

func TestRateLimit_LoginKeyFn_FallsBackToIPWithoutEmail(t *testing.T) {
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest("POST", "/login", strings.NewReader(`not-json`))
	c.Request.RemoteAddr = "198.51.100.9:9999"

	key := LoginKeyFn(c)
	if !strings.Contains(key, "198.51.100.9") {
		t.Errorf("key %q should contain client IP", key)
	}
	if strings.Contains(key, "|") {
		t.Errorf("key %q should not contain an email separator without a parseable email", key)
	}
}

func TestRateLimit_PurgeIdleRemovesStaleBuckets(t *testing.T) {
	l := NewLimiter()
	l.Allow("stale", 1, time.Minute)

	// Backdate the bucket's last access.
	l.mu.Lock()
	l.buckets["stale"].lastTime = time.Now().Add(-time.Hour)
	l.mu.Unlock()

	l.PurgeIdle(time.Now().Add(-30 * time.Minute))

	l.mu.Lock()
	_, exists := l.buckets["stale"]
	l.mu.Unlock()
	if exists {
		t.Error("expected idle bucket to be purged")
	}
}