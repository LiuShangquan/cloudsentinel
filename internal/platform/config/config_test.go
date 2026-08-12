package config

import (
	"strings"
	"testing"
	"time"
)

func TestLoadDefaults(t *testing.T) {
	t.Setenv("SERVER_PORT", "")
	t.Setenv("SERVER_PORT", "8080")
	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.App.Name != "cloudsentinel-api" || cfg.Server.Port != 8080 {
		t.Fatalf("unexpected defaults: %+v", cfg)
	}
	if cfg.Server.ShutdownTimeout != 10*time.Second {
		t.Fatalf("shutdown timeout = %v", cfg.Server.ShutdownTimeout)
	}
}

func TestLoadReportsFieldWithoutSecret(t *testing.T) {
	t.Setenv("MYSQL_PASSWORD", "do-not-leak")
	t.Setenv("SERVER_READ_TIMEOUT", "not-a-duration")
	_, err := Load()
	if err == nil || !strings.Contains(err.Error(), "SERVER_READ_TIMEOUT") {
		t.Fatalf("expected field context, got %v", err)
	}
	if strings.Contains(err.Error(), "do-not-leak") {
		t.Fatal("configuration error leaked a secret")
	}
}

func TestLoadTLSConfiguration(t *testing.T) {
	t.Setenv("MYSQL_TLS_ENABLED", "true")
	t.Setenv("REDIS_TLS_ENABLED", "true")
	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if !cfg.Database.TLSEnabled || !cfg.Redis.TLSEnabled {
		t.Fatalf("TLS flags = mysql:%t redis:%t", cfg.Database.TLSEnabled, cfg.Redis.TLSEnabled)
	}
}

func TestLoadRejectsInvalidTLSConfiguration(t *testing.T) {
	t.Setenv("MYSQL_TLS_ENABLED", "skip-verify")
	_, err := Load()
	if err == nil || !strings.Contains(err.Error(), "MYSQL_TLS_ENABLED") {
		t.Fatalf("expected safe TLS configuration error, got %v", err)
	}
}
