// Package config loads runtime configuration from environment variables.
// Values are read once at startup; no live-reloading.
package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
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
	LogLevel     string
	LogFormat    string // "json" or "text"
	OTLPEndpoint string
	OTELSample   float64

	// Service identity.
	ServiceName    string
	ServiceVersion string
	Environment    string

	// HTTP client tuning.
	HTTPTimeout time.Duration
	HTTPRetry   int

	// HTTP response cache. When enabled, successful API responses
	// are stored on disk so DB rebuilds can replay without network.
	CacheEnabled bool
	CacheDir     string        // empty → os.UserCacheDir()/devpulse
	CacheTTL     time.Duration // 0 means entries never expire
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
		CacheEnabled:   getenvBool("CACHE_ENABLED", false),
		CacheDir:       os.Getenv("CACHE_DIR"),
		CacheTTL:       getenvDuration("CACHE_TTL", 24*time.Hour), // 0 = never expire (always fresh)
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

	// Resolve the default cache directory so every consumer of
	// Config.CacheDir gets a fully resolved path — no caller needs
	// to re-implement the os.UserCacheDir fallback.
	if c.CacheEnabled && c.CacheDir == "" {
		userCache, err := os.UserCacheDir()
		if err != nil {
			return c, fmt.Errorf("config: resolve cache dir: %w", err)
		}
		c.CacheDir = filepath.Join(userCache, "devpulse")
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

func getenvBool(key string, def bool) bool {
	v := strings.TrimSpace(os.Getenv(key))
	if v == "" {
		return def
	}
	// strconv.ParseBool accepts 1/t/T/TRUE/true/True and the false
	// equivalents; anything else ("yes", "on") falls back to def
	// rather than silently meaning false.
	b, err := strconv.ParseBool(v)
	if err != nil {
		return def
	}
	return b
}

func getenvFloat(key string, def float64) float64 {
	if v := os.Getenv(key); v != "" {
		if f, err := strconv.ParseFloat(v, 64); err == nil {
			return f
		}
	}
	return def
}
