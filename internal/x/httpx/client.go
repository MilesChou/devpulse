package httpx

import (
	"context"
	"log/slog"
	"net/http"
	"time"

	"github.com/hashicorp/go-retryablehttp"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
)

// Config controls outbound HTTP client construction.
type Config struct {
	RetryMax      int           // 0 disables retry
	RetryWaitMin  time.Duration // default 1s
	RetryWaitMax  time.Duration // default 30s
	Timeout       time.Duration // default 30s
	UserAgent     string
	SpanPrefix    string // OTel span name prefix, e.g. "github" or "travis"
	Logger        *slog.Logger
	BaseTransport http.RoundTripper // optional; wraps this instead of http.DefaultTransport
}

// New returns a configured *http.Client whose transport stack is:
//
//	otelhttp -> retryablehttp -> http.DefaultTransport
//
// otelhttp goes outermost so retries are folded into a single client span.
func New(cfg Config) *http.Client {
	retryClient := retryablehttp.NewClient()
	if cfg.BaseTransport != nil {
		retryClient.HTTPClient.Transport = cfg.BaseTransport
	}
	retryClient.RetryMax = cfg.RetryMax
	retryClient.RetryWaitMin = orDefault(cfg.RetryWaitMin, time.Second)
	retryClient.RetryWaitMax = orDefault(cfg.RetryWaitMax, 30*time.Second)
	retryClient.Logger = slogAdapter{cfg.Logger}

	base := retryClient.StandardClient().Transport

	transport := otelhttp.NewTransport(
		base,
		otelhttp.WithSpanNameFormatter(func(_ string, r *http.Request) string {
			if cfg.SpanPrefix == "" {
				return r.Method + " " + r.URL.Host
			}
			return cfg.SpanPrefix + "." + r.Method + " " + r.URL.Path
		}),
	)

	c := &http.Client{
		Transport: transport,
		Timeout:   orDefault(cfg.Timeout, 30*time.Second),
	}
	if cfg.UserAgent != "" {
		c.Transport = &uaTransport{base: transport, ua: cfg.UserAgent}
	}
	return c
}

func orDefault(v, def time.Duration) time.Duration {
	if v == 0 {
		return def
	}
	return v
}

type uaTransport struct {
	base http.RoundTripper
	ua   string
}

func (t *uaTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	req = req.Clone(req.Context())
	if req.Header.Get("User-Agent") == "" {
		req.Header.Set("User-Agent", t.ua)
	}
	return t.base.RoundTrip(req)
}

// slogAdapter satisfies retryablehttp.LeveledLogger using slog.
type slogAdapter struct{ logger *slog.Logger }

func (a slogAdapter) Error(msg string, keysAndValues ...any) {
	a.log(slog.LevelError, msg, keysAndValues)
}
func (a slogAdapter) Info(msg string, keysAndValues ...any) {
	a.log(slog.LevelInfo, msg, keysAndValues)
}
func (a slogAdapter) Debug(msg string, keysAndValues ...any) {
	a.log(slog.LevelDebug, msg, keysAndValues)
}
func (a slogAdapter) Warn(msg string, keysAndValues ...any) {
	a.log(slog.LevelWarn, msg, keysAndValues)
}

func (a slogAdapter) log(level slog.Level, msg string, kv []any) {
	if a.logger == nil {
		return
	}
	a.logger.Log(context.Background(), level, msg, kv...)
}
