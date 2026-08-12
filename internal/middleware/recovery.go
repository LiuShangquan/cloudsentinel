package middleware

import (
	"log/slog"

	"cloudsentinel/internal/httpserver/response"

	"github.com/gin-gonic/gin"
)

func Recovery(log *slog.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		defer func() {
			if recovered := recover(); recovered != nil {
				log.ErrorContext(c.Request.Context(), "panic recovered",
					"request_id", GetRequestID(c),
					"panic", recovered,
				)
				response.InternalError(c)
			}
		}()
		c.Next()
	}
}
