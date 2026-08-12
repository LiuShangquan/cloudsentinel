package incident

import (
	"cloudsentinel/internal/httpserver/response"
	"crypto/subtle"
	"github.com/gin-gonic/gin"
	"net/http"
	"strings"
)

func MachineAuth(token string) gin.HandlerFunc {
	return func(c *gin.Context) {
		authorization := c.GetHeader("Authorization")
		parts := strings.SplitN(authorization, " ", 2)
		if len(parts) != 2 || parts[0] != "Bearer" || subtle.ConstantTimeCompare([]byte(parts[1]), []byte(token)) != 1 {
			response.Error(c, http.StatusUnauthorized, 40110, "unauthorized")
			return
		}
		c.Next()
	}
}
