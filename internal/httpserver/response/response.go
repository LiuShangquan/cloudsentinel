package response

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

type Envelope struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    any    `json:"data"`
}

func Success(c *gin.Context, status int, data any) {
	c.JSON(status, Envelope{Code: 0, Message: "success", Data: data})
}

func InternalError(c *gin.Context) {
	c.AbortWithStatusJSON(http.StatusInternalServerError, Envelope{Code: 50000, Message: "internal server error", Data: nil})
}

func Error(c *gin.Context, status, code int, message string) {
	c.AbortWithStatusJSON(status, Envelope{Code: code, Message: message, Data: nil})
}
