package config

import (
	"testing"
	"time"
)

func TestLoad_DefaultsWhenUnset(t *testing.T) {
	t.Setenv("DEVPULSE_DSN", "")
	cfg, err := Load()
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if cfg.DSN != "memory" {
		t.Fatalf("default dsn: %q", cfg.DSN)
	}
	if cfg.LogLevel != "info" {
		t.Fatalf("default log level: %q", cfg.LogLevel)
	}
	if cfg.HTTPTimeout != 30*time.Second {
		t.Fatalf("default timeout: %v", cfg.HTTPTimeout)
	}
}

func TestLoad_ReadsEnv(t *testing.T) {
	t.Setenv("DEVPULSE_DSN", "postgres://host/db")
	t.Setenv("LOG_LEVEL", "debug")
	t.Setenv("HTTP_TIMEOUT", "5s")
	t.Setenv("HTTP_RETRY_MAX", "7")
	t.Setenv("OTEL_SAMPLE_RATE", "0.5")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if cfg.DSN != "postgres://host/db" {
		t.Fatalf("dsn: %q", cfg.DSN)
	}
	if cfg.LogLevel != "debug" {
		t.Fatalf("log level: %q", cfg.LogLevel)
	}
	if cfg.HTTPTimeout != 5*time.Second {
		t.Fatalf("timeout: %v", cfg.HTTPTimeout)
	}
	if cfg.HTTPRetry != 7 {
		t.Fatalf("retry: %d", cfg.HTTPRetry)
	}
	if cfg.OTELSample != 0.5 {
		t.Fatalf("sample: %v", cfg.OTELSample)
	}
}

func TestLoad_InvalidLogFormat(t *testing.T) {
	t.Setenv("LOG_FORMAT", "xml")
	if _, err := Load(); err == nil {
		t.Fatalf("expected error")
	}
}

