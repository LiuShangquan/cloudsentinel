package health

import (
	"context"
	"net/http"

	"cloudsentinel/internal/httpserver/response"
	"github.com/gin-gonic/gin"
)

type DependencyChecker interface {
	Name() string
	Check(context.Context) error
}

type Handler struct {
	checkers []DependencyChecker
}

func NewHandler(checkers ...DependencyChecker) *Handler {
	return &Handler{checkers: checkers}
}

func (h *Handler) Health(c *gin.Context) {
	response.Success(c, http.StatusOK, gin.H{"status": "alive"})
}

func (h *Handler) Ready(c *gin.Context) {
	dependencies := make(map[string]string, len(h.checkers))
	ready := true
	for _, checker := range h.checkers {
		if err := checker.Check(c.Request.Context()); err != nil {
			dependencies[checker.Name()] = "unavailable"
			ready = false
		} else {
			dependencies[checker.Name()] = "ready"
		}
	}
	status := http.StatusOK
	state := "ready"
	if !ready {
		status = http.StatusServiceUnavailable
		state = "not_ready"
	}
	response.Success(c, status, gin.H{"status": state, "dependencies": dependencies})
}
