package middleware

import (
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/Yulian302/lfusys-services-commons/ratelimit"
	"github.com/gin-gonic/gin"
)

func RateLimiterMiddleware(limiter ratelimit.RateLimiter, limit int, window time.Duration) gin.HandlerFunc {
	return func(c *gin.Context) {
		ip := c.ClientIP()
		key := fmt.Sprintf("rate:ip:%s", ip)

		count, err := limiter.Incr(c, key)
		if err != nil {
			c.Next()
			return
		}

		if count == 1 {
			err = limiter.Expire(c, key, window)
			if err != nil {
				log.Println("could not set expiration for rate limiting")
			}
		}

		if count > int64(limit) {
			c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{
				"erorr": "Too many requests. Please try again later",
			})
			return
		}

		c.Next()
	}
}
