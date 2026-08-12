package auth

import (
	"errors"
	"net/http"

	"cloudsentinel/internal/httpserver/response"
	appmiddleware "cloudsentinel/internal/middleware"
	"github.com/gin-gonic/gin"
)

type Handler struct{ service *Service }

func NewHandler(service *Service) *Handler { return &Handler{service: service} }

type loginRequest struct {
	Username string `json:"username" binding:"required,max=100"`
	Password string `json:"password" binding:"required,max=1024"`
}

func (h *Handler) Login(c *gin.Context) {
	var input loginRequest
	if err := c.ShouldBindJSON(&input); err != nil {
		response.Error(c, http.StatusBadRequest, 40000, "invalid request")
		return
	}
	result, err := h.service.Login(c.Request.Context(), input.Username, input.Password, RequestMetadata{RequestID: appmiddleware.GetRequestID(c), ClientIP: c.ClientIP(), UserAgent: c.Request.UserAgent()})
	if err != nil {
		switch {
		case errors.Is(err, ErrInvalidCredentials):
			response.Error(c, http.StatusUnauthorized, 40101, ErrInvalidCredentials.Error())
		case errors.Is(err, ErrUserDisabled):
			response.Error(c, http.StatusForbidden, 40301, ErrUserDisabled.Error())
		default:
			response.InternalError(c)
		}
		return
	}
	response.Success(c, http.StatusOK, result)
}

func (h *Handler) Me(c *gin.Context) {
	principal, ok := PrincipalFromContext(c)
	if !ok {
		response.Error(c, http.StatusUnauthorized, 40100, "unauthorized")
		return
	}
	user, err := h.service.Me(c.Request.Context(), principal.UserID)
	if err != nil {
		if errors.Is(err, ErrUnauthenticated) {
			response.Error(c, http.StatusUnauthorized, 40100, "unauthorized")
		} else {
			response.InternalError(c)
		}
		return
	}
	response.Success(c, http.StatusOK, user)
}
