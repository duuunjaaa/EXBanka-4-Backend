package middleware

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
)

// RateLimit returns a middleware that limits requests per user (or IP for unauthenticated) per endpoint path.
// limit is the max number of requests allowed in window.
func RateLimit(rdb *redis.Client, limit int, window time.Duration) gin.HandlerFunc {
	return func(c *gin.Context) {
		identity := identityFromContext(c)
		key := fmt.Sprintf("ratelimit:%s:%s", identity, c.FullPath())

		ctx := context.Background()
		count, err := rdb.Incr(ctx, key).Result()
		if err != nil {
			// Redis error — fail open rather than blocking all traffic
			c.Next()
			return
		}
		if count == 1 {
			rdb.Expire(ctx, key, window)
		}

		if count > int64(limit) {
			c.Header("Retry-After", fmt.Sprintf("%.0f", window.Seconds()))
			c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{"error": "rate limit exceeded"})
			return
		}

		c.Next()
	}
}

// identityFromContext returns a stable identifier for the caller:
// the user_id from the JWT if present, otherwise the client IP.
func identityFromContext(c *gin.Context) string {
	header := c.GetHeader("Authorization")
	if strings.HasPrefix(header, "Bearer ") {
		if id, err := GetUserIDFromToken(c); err == nil {
			return fmt.Sprintf("uid:%d", id)
		}
	}
	return "ip:" + c.ClientIP()
}
