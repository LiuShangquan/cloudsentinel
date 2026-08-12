package workerhealth

import (
	"context"
	"encoding/json"
	"net/http"
)

type DependencyChecker interface {
	Name() string
	Check(context.Context) error
}

type Handler struct {
	checkers []DependencyChecker
}

type envelope struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    any    `json:"data"`
}

func New(checkers ...DependencyChecker) *Handler {
	return &Handler{checkers: checkers}
}

func (h *Handler) Health(w http.ResponseWriter, _ *http.Request) {
	write(w, http.StatusOK, map[string]any{"status": "alive"})
}

func (h *Handler) Ready(w http.ResponseWriter, r *http.Request) {
	dependencies := make(map[string]string, len(h.checkers))
	ready := true
	for _, checker := range h.checkers {
		if err := checker.Check(r.Context()); err != nil {
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
	write(w, status, map[string]any{"status": state, "dependencies": dependencies})
}

func write(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(envelope{Code: 0, Message: "success", Data: data})
}
