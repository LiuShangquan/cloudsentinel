package httpserver

import (
	"bytes"
	"context"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"cloudsentinel/internal/health"
	appmiddleware "cloudsentinel/internal/middleware"
	"github.com/gin-gonic/gin"
)

type checker struct {
	name string
	err  error
}

func (c checker) Name() string                { return c.name }
func (c checker) Check(context.Context) error { return c.err }

func testLogger(output io.Writer) *slog.Logger {
	return slog.New(slog.NewTextHandler(output, nil))
}

func TestHealthAndReadyRoutes(t *testing.T) {
	gin.SetMode(gin.TestMode)
	tests := []struct {
		path     string
		checkers []health.DependencyChecker
		want     int
	}{
		{path: "/healthz", want: http.StatusOK},
		{path: "/readyz", want: http.StatusOK},
		{path: "/readyz", checkers: []health.DependencyChecker{checker{name: "mysql", err: errors.New("down")}}, want: http.StatusServiceUnavailable},
	}
	for _, tt := range tests {
		router := NewRouter(testLogger(io.Discard), health.NewHandler(tt.checkers...))
		response := httptest.NewRecorder()
		router.ServeHTTP(response, httptest.NewRequest(http.MethodGet, tt.path, nil))
		if response.Code != tt.want {
			t.Fatalf("%s status = %d, want %d; body=%s", tt.path, response.Code, tt.want, response.Body.String())
		}
	}
}

func TestRequestIDIsPreservedOrGenerated(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := NewRouter(testLogger(io.Discard), health.NewHandler())

	provided := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	request.Header.Set(appmiddleware.RequestIDHeader, "client-id")
	router.ServeHTTP(provided, request)
	if got := provided.Header().Get(appmiddleware.RequestIDHeader); got != "client-id" {
		t.Fatalf("provided request ID = %q", got)
	}

	generated := httptest.NewRecorder()
	router.ServeHTTP(generated, httptest.NewRequest(http.MethodGet, "/healthz", nil))
	if got := generated.Header().Get(appmiddleware.RequestIDHeader); len(got) != 32 {
		t.Fatalf("generated request ID = %q", got)
	}
}

func TestRecoveryReturnsSafeResponseAndLogsRequestID(t *testing.T) {
	gin.SetMode(gin.TestMode)
	var logs bytes.Buffer
	router := gin.New()
	router.Use(appmiddleware.RequestID(), appmiddleware.Recovery(testLogger(&logs)))
	router.GET("/panic", func(*gin.Context) { panic("sensitive-stack-marker") })

	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/panic", nil)
	request.Header.Set(appmiddleware.RequestIDHeader, "panic-request")
	router.ServeHTTP(response, request)
	if response.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d", response.Code)
	}
	if bytes.Contains(response.Body.Bytes(), []byte("sensitive-stack-marker")) {
		t.Fatal("panic detail leaked to response")
	}
	if !bytes.Contains(logs.Bytes(), []byte("panic-request")) {
		t.Fatal("recovery log does not contain request ID")
	}
}
