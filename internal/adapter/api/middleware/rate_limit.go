package middleware

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

// bucket is a token bucket refilled linearly: capacity tokens per refill
// window, capped at capacity.
type bucket struct {
	tokens   float64
	lastTime time.Time
}

// Limiter is an in-process token-bucket rate limiter keyed by string.
// It is per-pod (no cross-replica state) — a documented limitation of the
// stdlib-only design (spec 4.2 / §9).
type Limiter struct {
	mu         sync.Mutex
	buckets    map[string]*bucket
	lastPurge  time.Time
	purgeEvery time.Duration
}

// NewLimiter creates an empty rate limiter.
func NewLimiter() *Limiter {
	return &Limiter{
		buckets:    make(map[string]*bucket),
		purgeEvery: time.Minute,
	}
}

// Allow consumes one token from the bucket for key. The bucket holds
// capacity tokens and refills capacity tokens per refill window. It returns
// false when no token is available.
func (l *Limiter) Allow(key string, capacity int, refill time.Duration) bool {
	l.mu.Lock()
	defer l.mu.Unlock()

	now := time.Now()

	// Lazy purge keeps the map bounded (idle keys are dropped).
	if now.Sub(l.lastPurge) >= l.purgeEvery {
		l.lastPurge = now
		idleBefore := now.Add(-10 * l.purgeEvery)
		for k, b := range l.buckets {
			if b.lastTime.Before(idleBefore) {
				delete(l.buckets, k)
			}
		}
	}

	b, ok := l.buckets[key]
	if !ok {
		b = &bucket{tokens: float64(capacity), lastTime: now}
		l.buckets[key] = b
	} else {
		// Linear refill: capacity tokens per refill window.
		b.tokens += float64(capacity) * float64(now.Sub(b.lastTime)) / float64(refill)
		if b.tokens > float64(capacity) {
			b.tokens = float64(capacity)
		}
		b.lastTime = now
	}

	if b.tokens >= 1 {
		b.tokens--
		return true
	}
	return false
}

// PurgeIdle removes buckets not touched since olderThan.
func (l *Limiter) PurgeIdle(olderThan time.Time) {
	l.mu.Lock()
	defer l.mu.Unlock()
	for k, b := range l.buckets {
		if b.lastTime.Before(olderThan) {
			delete(l.buckets, k)
		}
	}
}

// RateLimit returns a gin middleware that answers 429 with a Retry-After
// header when the key's bucket is empty.
func RateLimit(l *Limiter, capacity int, window time.Duration, keyFn func(*gin.Context) string) gin.HandlerFunc {
	return func(c *gin.Context) {
		if l.Allow(keyFn(c), capacity, window) {
			c.Next()
			return
		}
		c.Header("Retry-After", "60")
		c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{"error": "rate limit exceeded"})
	}
}

// IPKeyFn keys by client IP.
func IPKeyFn(c *gin.Context) string {
	return c.ClientIP()
}

// loginKeyPeek is the max body size inspected for the email claim.
const loginKeyPeek = 4096

// LoginKeyFn keys by client IP plus the email from the JSON body (peeked and
// re-wrapped so the handler can still read it). Malformed or missing email
// falls back to client IP only.
func LoginKeyFn(c *gin.Context) string {
	email := ""
	if c.Request != nil && c.Request.Body != nil {
		buf, err := io.ReadAll(io.LimitReader(c.Request.Body, loginKeyPeek))
		if err == nil && len(buf) > 0 {
			// Restore the body for downstream handlers.
			c.Request.Body = io.NopCloser(bytes.NewReader(buf))
			var payload struct {
				Email string `json:"email"`
			}
			if json.Unmarshal(buf, &payload) == nil && payload.Email != "" {
				email = strings.ToLower(strings.TrimSpace(payload.Email))
			}
		}
	}

	key := c.ClientIP()
	if email != "" {
		key += "|" + email
	}
	return key
}