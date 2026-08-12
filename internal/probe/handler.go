package probe

import (
	"cloudsentinel/internal/auth"
	"cloudsentinel/internal/httpserver/response"
	appmiddleware "cloudsentinel/internal/middleware"
	"errors"
	"github.com/gin-gonic/gin"
	"net/http"
	"strconv"
)

type Handler struct{ service *TaskService }

func NewHandler(service *TaskService) *Handler { return &Handler{service: service} }
func (h *Handler) Create(c *gin.Context) {
	var input TaskInput
	if c.ShouldBindJSON(&input) != nil {
		bad(c)
		return
	}
	principal, metadata := identity(c)
	item, err := h.service.Create(c.Request.Context(), input, principal.UserID, metadata)
	if handle(c, err) {
		return
	}
	response.Success(c, http.StatusCreated, item)
}
func (h *Handler) List(c *gin.Context) {
	page, size, ok := pages(c)
	if !ok {
		return
	}
	items, err := h.service.List(c.Request.Context(), page, size)
	if handle(c, err) {
		return
	}
	response.Success(c, http.StatusOK, items)
}
func (h *Handler) Get(c *gin.Context) {
	id, ok := id(c)
	if !ok {
		return
	}
	item, err := h.service.Get(c.Request.Context(), id)
	if handle(c, err) {
		return
	}
	response.Success(c, http.StatusOK, item)
}
func (h *Handler) Update(c *gin.Context) {
	identifier, ok := id(c)
	if !ok {
		return
	}
	var input TaskInput
	if c.ShouldBindJSON(&input) != nil {
		bad(c)
		return
	}
	principal, metadata := identity(c)
	item, err := h.service.Update(c.Request.Context(), identifier, input, principal.UserID, metadata)
	if handle(c, err) {
		return
	}
	response.Success(c, http.StatusOK, item)
}
func (h *Handler) Disable(c *gin.Context) {
	identifier, ok := id(c)
	if !ok {
		return
	}
	principal, metadata := identity(c)
	if handle(c, h.service.Disable(c.Request.Context(), identifier, principal.UserID, metadata)) {
		return
	}
	response.Success(c, http.StatusOK, gin.H{"id": identifier, "status": TaskDisabled})
}
func (h *Handler) ListResults(c *gin.Context) {
	page, size, ok := pages(c)
	if !ok {
		return
	}
	items, err := h.service.ListResults(c.Request.Context(), page, size)
	if handle(c, err) {
		return
	}
	response.Success(c, http.StatusOK, items)
}
func (h *Handler) GetResult(c *gin.Context) {
	identifier, ok := id(c)
	if !ok {
		return
	}
	item, err := h.service.GetResult(c.Request.Context(), identifier)
	if handle(c, err) {
		return
	}
	response.Success(c, http.StatusOK, item)
}
func identity(c *gin.Context) (auth.Principal, Metadata) {
	principal, _ := auth.PrincipalFromContext(c)
	return principal, Metadata{RequestID: appmiddleware.GetRequestID(c), ClientIP: c.ClientIP(), UserAgent: c.Request.UserAgent()}
}
func id(c *gin.Context) (uint64, bool) {
	value, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil || value == 0 {
		bad(c)
		return 0, false
	}
	return value, true
}
func pages(c *gin.Context) (int, int, bool) {
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
	default:
		response.InternalError(c)
	}
	return true
}
