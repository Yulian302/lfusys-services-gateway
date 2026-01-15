package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Yulian302/lfusys-services-commons/ratelimit"
	"github.com/alicebob/miniredis/v2"
	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
)

func TestRateLimiterMiddleware(t *testing.T) {
	s, _ := miniredis.Run()
	defer s.Close()

	rdb := redis.NewClient(&redis.Options{
		Addr: s.Addr(),
	})
	limiter := ratelimit.NewRedisRateLimiter(rdb)

	gin.SetMode(gin.TestMode)

	r := gin.Default()
	r.Use(RateLimiterMiddleware(limiter, 3, time.Minute))

	r.GET("/test", func(c *gin.Context) {
		c.String(200, "ok")
	})

	for i := 0; i < 3; i++ {
		req, _ := http.NewRequest("GET", "/test", nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		require.Equal(t, 200, w.Code)
	}

	// rate limit should work
	req, _ := http.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	require.Equal(t, 429, w.Code)
}
