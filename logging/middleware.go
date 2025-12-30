package logging

import (
	"context"
	"log/slog"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type loggerKeyType struct{}

var loggerKey = loggerKeyType{}

func LoggerMiddleware(baseLogger *slog.Logger) gin.HandlerFunc {

	return func(c *gin.Context) {
		start := time.Now()
		requestID := uuid.NewString()
		path := c.FullPath()
		if path == "" {
			path = c.Request.URL.Path
		}

		reqLogger := baseLogger.With(
			slog.String("request_id", requestID),
			slog.String("method", c.Request.Method),
			slog.String("path", path),
		)
		ctx := context.WithValue(c.Request.Context(), loggerKey, reqLogger)
		c.Request = c.Request.WithContext(ctx)

		c.Writer.Header().Set("X-Request-ID", requestID)

		c.Next()

		duration := time.Since(start)
		status := c.Writer.Status()

		level := slog.LevelInfo

		if status >= 500 {
			level = slog.LevelError
		} else if status >= 400 {
			level = slog.LevelWarn
		}

		reqLogger.Log(
			c.Request.Context(),
			level,
			"http request completed",
			slog.Int("status", status),
			slog.Duration("duration", duration),
		)

	}
}
