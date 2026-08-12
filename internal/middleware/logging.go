package middleware

import (
	"log/slog"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
)

func AccessLog(log *slog.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		started := time.Now()
		c.Next()
		route := c.FullPath()
		if route == "" {
			route = "unmatched"
		}
		log.InfoContext(c.Request.Context(), "http request",
			"request_id", GetRequestID(c),
			"method", c.Request.Method,
			"route", route,
			"status", strconv.Itoa(c.Writer.Status()),
			"duration", time.Since(started),
		)
	}
}
