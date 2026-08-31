package http

import (
	"net"
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

type attemptWindow struct {
	started time.Time
	count   int
}

func RateLimit(maxAttempts int, window time.Duration) gin.HandlerFunc {
	var mu sync.Mutex
	attempts := make(map[string]attemptWindow)
	return func(c *gin.Context) {
		host, _, err := net.SplitHostPort(c.Request.RemoteAddr)
		if err != nil {
			host = c.Request.RemoteAddr
		}
		key, now := host+"|"+c.FullPath(), time.Now()
		mu.Lock()
		entry := attempts[key]
		if entry.started.IsZero() || now.Sub(entry.started) >= window {
			entry = attemptWindow{started: now}
		}
		entry.count++
		attempts[key] = entry
		limited := entry.count > maxAttempts
		if len(attempts) > 10000 {
			for candidate, value := range attempts {
				if now.Sub(value.started) >= window {
					delete(attempts, candidate)
				}
			}
		}
		mu.Unlock()
		if limited {
			c.Header("Retry-After", "60")
			c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{"error": "too many authentication attempts"})
			return
		}
		c.Next()
	}
}

func LimitRequestBody(bytes int64) gin.HandlerFunc {
	return func(c *gin.Context) {
		if c.Request.Body != nil {
			c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, bytes)
		}
		c.Next()
	}
}
