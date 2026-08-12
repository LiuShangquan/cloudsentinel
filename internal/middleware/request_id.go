package middleware

import (
	"crypto/rand"
	"encoding/hex"
	"strings"

	"cloudsentinel/internal/httpserver/response"
	"github.com/gin-gonic/gin"
)

const (
	RequestIDHeader = "X-Request-ID"
	RequestIDKey    = "request_id"
)

func RequestID() gin.HandlerFunc {
	return func(c *gin.Context) {
		requestID := strings.TrimSpace(c.GetHeader(RequestIDHeader))
		if requestID == "" {
			generated, err := newRequestID()
			if err != nil {
				response.InternalError(c)
				return
			}
			requestID = generated
		}
		c.Set(RequestIDKey, requestID)
		c.Header(RequestIDHeader, requestID)
		c.Next()
	}
}

func GetRequestID(c *gin.Context) string {
	requestID, _ := c.Get(RequestIDKey)
	value, _ := requestID.(string)
	return value
}

func newRequestID() (string, error) {
	buffer := make([]byte, 16)
	if _, err := rand.Read(buffer); err != nil {
		return "", err
	}
	return hex.EncodeToString(buffer), nil
}
