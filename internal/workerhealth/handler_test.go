package workerhealth

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

type testChecker struct {
	name string
	err  error
}

func (c testChecker) Name() string                { return c.name }
func (c testChecker) Check(context.Context) error { return c.err }

func TestHealthAndReadiness(t *testing.T) {
	handler := New(testChecker{name: "mysql"}, testChecker{name: "redis", err: errors.New("unavailable")})

	healthRecorder := httptest.NewRecorder()
	handler.Health(healthRecorder, httptest.NewRequest(http.MethodGet, "/healthz", nil))
	if healthRecorder.Code != http.StatusOK || !strings.Contains(healthRecorder.Body.String(), `"status":"alive"`) {
		t.Fatalf("health response = %d %s", healthRecorder.Code, healthRecorder.Body.String())
	}

	readyRecorder := httptest.NewRecorder()
	handler.Ready(readyRecorder, httptest.NewRequest(http.MethodGet, "/readyz", nil))
	if readyRecorder.Code != http.StatusServiceUnavailable ||
		!strings.Contains(readyRecorder.Body.String(), `"status":"not_ready"`) ||
		!strings.Contains(readyRecorder.Body.String(), `"redis":"unavailable"`) {
		t.Fatalf("ready response = %d %s", readyRecorder.Code, readyRecorder.Body.String())
	}
}
