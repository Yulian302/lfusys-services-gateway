package middleware

import (
	"fmt"
	"net/http"
	"time"

	"github.com/Yulian302/lfusys-services-commons/ratelimit"
	logger "github.com/Yulian302/lfusys-services-commons/logging"
	"github.com/gin-gonic/gin"
)

func RateLimiterMiddleware(limiter ratelimit.RateLimiter, limit int, window time.Duration, l logger.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		ip := c.ClientIP()
		key := fmt.Sprintf("rate:ip:%s", ip)

		count, err := limiter.Incr(c, key)
		if err != nil {
			l.Error("rate limiter increment failed",
				"ip", ip,
				"error", err,
			)
			c.Next()
			return
		}

		if count == 1 {
			err = limiter.Expire(c, key, window)
			if err != nil {
				l.Error("rate limiter expiration failed",
					"ip", ip,
					"error", err,
				)
			}
		}

		if count > int64(limit) {
			l.Warn("rate limit exceeded",
				"ip", ip,
				"count", count,
				"limit", limit,
			)
			c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{
				"error": "Too many requests. Please try again later",
			})
			return
		}

		c.Next()
	}
}
