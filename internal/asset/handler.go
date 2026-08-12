package asset

import (
	"errors"
	"net/http"
	"strconv"

	"cloudsentinel/internal/auth"
	"cloudsentinel/internal/httpserver/response"
	appmiddleware "cloudsentinel/internal/middleware"
	"github.com/gin-gonic/gin"
)

type Handler struct{ service *AssetService }

func NewHandler(service *AssetService) *Handler { return &Handler{service: service} }

type hostRequest struct {
	Name        string `json:"name" binding:"required,max=100"`
	Address     string `json:"address" binding:"required,max=255"`
	Description string `json:"description"`
}
type serviceRequest struct {
	HostID      uint64 `json:"host_id" binding:"required"`
	Name        string `json:"name" binding:"required,max=100"`
	Type        string `json:"type" binding:"required"`
	Target      string `json:"target" binding:"required,max=2048"`
	Description string `json:"description"`
}

func (h *Handler) CreateHost(c *gin.Context) {
	var input hostRequest
	if c.ShouldBindJSON(&input) != nil {
		bad(c)
		return
	}
	principal, metadata := identity(c)
	item, err := h.service.CreateHost(c.Request.Context(), HostInput(input), principal.UserID, metadata)
	if handle(c, err) {
		return
	}
	response.Success(c, http.StatusCreated, item)
}
func (h *Handler) ListHosts(c *gin.Context) {
	page, size, ok := pageParams(c)
	if !ok {
		return
	}
	items, err := h.service.ListHosts(c.Request.Context(), page, size)
	if handle(c, err) {
		return
	}
	response.Success(c, http.StatusOK, items)
}
func (h *Handler) GetHost(c *gin.Context) {
	id, ok := pathID(c)
	if !ok {
		return
	}
	item, err := h.service.GetHost(c.Request.Context(), id)
	if handle(c, err) {
		return
	}
	response.Success(c, http.StatusOK, item)
}
func (h *Handler) UpdateHost(c *gin.Context) {
	id, ok := pathID(c)
	if !ok {
		return
	}
	var input hostRequest
	if c.ShouldBindJSON(&input) != nil {
		bad(c)
		return
	}
	principal, metadata := identity(c)
	item, err := h.service.UpdateHost(c.Request.Context(), id, HostInput(input), principal.UserID, metadata)
	if handle(c, err) {
		return
	}
	response.Success(c, http.StatusOK, item)
}
func (h *Handler) DisableHost(c *gin.Context) {
	id, ok := pathID(c)
	if !ok {
		return
	}
	principal, metadata := identity(c)
	if handle(c, h.service.DisableHost(c.Request.Context(), id, principal.UserID, metadata)) {
		return
	}
	response.Success(c, http.StatusOK, gin.H{"id": id, "status": StatusDisabled})
}
func (h *Handler) CreateService(c *gin.Context) {
	var input serviceRequest
	if c.ShouldBindJSON(&input) != nil {
		bad(c)
		return
	}
	principal, metadata := identity(c)
	item, err := h.service.CreateMonitoredService(c.Request.Context(), ServiceInput(input), principal.UserID, metadata)
	if handle(c, err) {
		return
	}
	response.Success(c, http.StatusCreated, item)
}
func (h *Handler) ListServices(c *gin.Context) {
	page, size, ok := pageParams(c)
	if !ok {
		return
	}
	items, err := h.service.ListMonitoredServices(c.Request.Context(), page, size)
	if handle(c, err) {
		return
	}
	response.Success(c, http.StatusOK, items)
}
func (h *Handler) GetService(c *gin.Context) {
	id, ok := pathID(c)
	if !ok {
		return
	}
	item, err := h.service.GetMonitoredService(c.Request.Context(), id)
	if handle(c, err) {
		return
	}
	response.Success(c, http.StatusOK, item)
}
func (h *Handler) UpdateService(c *gin.Context) {
	id, ok := pathID(c)
	if !ok {
		return
	}
	var input serviceRequest
	if c.ShouldBindJSON(&input) != nil {
		bad(c)
		return
	}
	principal, metadata := identity(c)
	item, err := h.service.UpdateMonitoredService(c.Request.Context(), id, ServiceInput(input), principal.UserID, metadata)
	if handle(c, err) {
		return
	}
	response.Success(c, http.StatusOK, item)
}
func (h *Handler) DisableService(c *gin.Context) {
	id, ok := pathID(c)
	if !ok {
		return
	}
	principal, metadata := identity(c)
	if handle(c, h.service.DisableMonitoredService(c.Request.Context(), id, principal.UserID, metadata)) {
		return
	}
	response.Success(c, http.StatusOK, gin.H{"id": id, "status": StatusDisabled})
}

func identity(c *gin.Context) (auth.Principal, Metadata) {
	principal, _ := auth.PrincipalFromContext(c)
	return principal, Metadata{RequestID: appmiddleware.GetRequestID(c), ClientIP: c.ClientIP(), UserAgent: c.Request.UserAgent()}
}
func pathID(c *gin.Context) (uint64, bool) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil || id == 0 {
		bad(c)
		return 0, false
	}
	return id, true
}
func pageParams(c *gin.Context) (int, int, bool) {
	page, err := strconv.Atoi(c.DefaultQuery("page", "1"))
	if err != nil || page < 1 {
		bad(c)
		return 0, 0, false
	}
	size, err := strconv.Atoi(c.DefaultQuery("page_size", "20"))
	if err != nil || size < 1 || size > 100 {
		bad(c)
		return 0, 0, false
	}
	return page, size, true
}
func bad(c *gin.Context) { response.Error(c, http.StatusBadRequest, 40000, "invalid request") }
func handle(c *gin.Context, err error) bool {
	if err == nil {
		return false
	}
	switch {
	case errors.Is(err, ErrInvalidInput):
		bad(c)
	case errors.Is(err, ErrNotFound):
		response.Error(c, http.StatusNotFound, 40400, "not found")
	case errors.Is(err, ErrConflict):
		response.Error(c, http.StatusConflict, 40900, "conflict")
	case errors.Is(err, ErrHostDisabled):
		response.Error(c, http.StatusConflict, 40901, "host is disabled")
	default:
		response.InternalError(c)
	}
	return true
}
