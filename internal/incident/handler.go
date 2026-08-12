package incident

import (
	"cloudsentinel/internal/auth"
	"cloudsentinel/internal/httpserver/response"
	"context"
	"encoding/json"
	"errors"
	"github.com/gin-gonic/gin"
	"net/http"
	"strconv"
)

type Handler struct{ service *Service }

func NewHandler(service *Service) *Handler { return &Handler{service: service} }
func (h *Handler) Webhook(c *gin.Context) {
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 1<<20)
	var payload Webhook
	if err := json.NewDecoder(c.Request.Body).Decode(&payload); err != nil {
		response.Error(c, http.StatusBadRequest, 40010, "invalid webhook")
		return
	}
	if err := h.service.Ingest(c.Request.Context(), payload); err != nil {
		response.Error(c, http.StatusBadRequest, 40010, "invalid webhook")
		return
	}
	response.Success(c, http.StatusOK, gin.H{"processed": len(payload.Alerts)})
}
func (h *Handler) List(c *gin.Context) {
	page, size, ok := pages(c)
	if !ok {
		return
	}
	items, err := h.service.List(c.Request.Context(), page, size)
	if fail(c, err) {
		return
	}
	response.Success(c, http.StatusOK, items)
}
func (h *Handler) Get(c *gin.Context) {
	id, ok := pathID(c)
	if !ok {
		return
	}
	item, err := h.service.Get(c.Request.Context(), id)
	if fail(c, err) {
		return
	}
	response.Success(c, http.StatusOK, item)
}
func (h *Handler) Acknowledge(c *gin.Context) { h.action(c, h.service.Acknowledge) }
func (h *Handler) Process(c *gin.Context)     { h.action(c, h.service.Process) }
func (h *Handler) Close(c *gin.Context)       { h.action(c, h.service.Close) }
func (h *Handler) action(c *gin.Context, operation func(context.Context, uint64, uint64, string) (Incident, error)) {
	id, ok := pathID(c)
	if !ok {
		return
	}
	principal, _ := auth.PrincipalFromContext(c)
	item, err := operation(c.Request.Context(), id, principal.UserID, principal.Username)
	if fail(c, err) {
		return
	}
	response.Success(c, http.StatusOK, item)
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
func pathID(c *gin.Context) (uint64, bool) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil || id == 0 {
		bad(c)
		return 0, false
	}
	return id, true
}
func bad(c *gin.Context) { response.Error(c, http.StatusBadRequest, 40000, "invalid request") }
func fail(c *gin.Context, err error) bool {
	if err == nil {
		return false
	}
	switch {
	case errors.Is(err, ErrNotFound):
		response.Error(c, http.StatusNotFound, 40400, "not found")
	case errors.Is(err, ErrConflict):
		response.Error(c, http.StatusConflict, 40900, "invalid incident transition")
	default:
		response.InternalError(c)
	}
	return true
}
