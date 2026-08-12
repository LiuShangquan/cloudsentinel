package auth

import (
	"net/http"
	"strings"

	"cloudsentinel/internal/httpserver/response"
	"github.com/gin-gonic/gin"
)

const principalKey = "auth_principal"

func (m *TokenManager) Middleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		authorization := c.GetHeader("Authorization")
		parts := strings.SplitN(authorization, " ", 2)
		if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") || strings.TrimSpace(parts[1]) == "" {
			response.Error(c, http.StatusUnauthorized, 40100, "unauthorized")
			return
		}
		principal, err := m.Parse(strings.TrimSpace(parts[1]))
		if err != nil {
			response.Error(c, http.StatusUnauthorized, 40100, "unauthorized")
			return
		}
		c.Set(principalKey, principal)
		c.Next()
	}
}

func PrincipalFromContext(c *gin.Context) (Principal, bool) {
	value, ok := c.Get(principalKey)
	if !ok {
		return Principal{}, false
	}
	principal, ok := value.(Principal)
	return principal, ok
}
