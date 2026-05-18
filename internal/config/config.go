// Package config loads runtime configuration from environment variables.
// Values are read once at startup; no live-reloading.
package config

import (
	"errors"
	"os"
	"strconv"
	"strings"
	"time"
)

// Config is the merged, validated runtime configuration.
type Config struct {
	// DSN: postgres://, mysql://, sqlite://, "memory", ":memory:".
	DSN string

	// GitHub / Travis API credentials.
	GitHubToken string
	GitHubBase  string
	TravisToken string
	TravisBase  string

	// Observability.
	LogLevel    string
	LogFormat   string // "json" or "text"
	OTLPEndpoint string
	OTELSample  float64

	// Service identity.
	ServiceName    string
	ServiceVersion string
	Environment    string

	// HTTP client tuning.
	HTTPTimeout time.Duration
	HTTPRetry   int
}

// Load reads environment variables and returns Config.
func Load() (Config, error) {
	cfg := Config{
		DSN:            getenv("DEVPULSE_DSN", "memory"),
		GitHubToken:    os.Getenv("GITHUB_TOKEN"),
		GitHubBase:     getenv("GITHUB_BASE_URL", "https://api.github.com"),
		TravisToken:    os.Getenv("TRAVIS_TOKEN"),
		TravisBase:     getenv("TRAVIS_BASE_URL", "https://api.travis-ci.com"),
		LogLevel:       getenv("LOG_LEVEL", "info"),
		LogFormat:      getenv("LOG_FORMAT", "json"),
		OTLPEndpoint:   os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT"),
		ServiceName:    getenv("OTEL_SERVICE_NAME", "devpulse"),
		ServiceVersion: getenv("SERVICE_VERSION", "dev"),
		Environment:    getenv("ENV", "local"),
		HTTPTimeout:    getenvDuration("HTTP_TIMEOUT", 30*time.Second),
		HTTPRetry:      getenvInt("HTTP_RETRY_MAX", 3),
		OTELSample:     getenvFloat("OTEL_SAMPLE_RATE", 1.0),
	}
	return cfg.validate()
}

func (c Config) validate() (Config, error) {
	if strings.TrimSpace(c.DSN) == "" {
		return c, errors.New("config: DEVPULSE_DSN is required")
	}
	if c.LogFormat != "json" && c.LogFormat != "text" {
		return c, errors.New("config: LOG_FORMAT must be json or text")
	}
	return c, nil
}

func getenv(key, def string) string {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		return v
	}
	return def
}

func getenvDuration(key string, def time.Duration) time.Duration {
	if v := os.Getenv(key); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			return d
		}
	}
	return def
}

func getenvInt(key string, def int) int {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return def
}

func getenvFloat(key string, def float64) float64 {
	if v := os.Getenv(key); v != "" {
		if f, err := strconv.ParseFloat(v, 64); err == nil {
			return f
		}
	}
	return def
}

