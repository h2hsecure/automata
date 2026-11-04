package middleware

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// LimitBody rejects requests whose declared body size exceeds limit with
// 413, and caps the actual body read so oversized uploads that omit or lie
// about Content-Length are aborted mid-stream.
func LimitBody(limit int64) gin.HandlerFunc {
	return func(c *gin.Context) {
		if cl := c.Request.ContentLength; cl > limit {
			c.AbortWithStatus(http.StatusRequestEntityTooLarge)
			return
		}
		c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, limit)
		c.Next()
	}
}