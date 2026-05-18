// Package travis adapts the Travis CI v3 API to fetching.CIProvider.
package travis

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/mileschou/devpulse/internal/x/httpx"
)

const (
	DefaultBaseURL   = "https://api.travis-ci.com"
	DefaultUserAgent = "devpulse/0 (+https://github.com/MilesChou/devpulse)"
	DefaultTimeout   = 30 * time.Second
	DefaultRetryMax  = 3
	defaultLimit     = 100
)

// Config controls the Travis client.
type Config struct {
	BaseURL   string
	Token     string
	UserAgent string
	Timeout   time.Duration
	RetryMax  int
	Logger    *slog.Logger
}

// Client is the Travis HTTP plumbing. Higher-level operations live in
// builds.go / provider.go.
type Client struct {
	base       string
	token      string
	httpClient *http.Client
	logger     *slog.Logger
}

// NewClient returns a configured Travis client.
func NewClient(cfg Config) *Client {
	if cfg.BaseURL == "" {
		cfg.BaseURL = DefaultBaseURL
	}
	if cfg.UserAgent == "" {
		cfg.UserAgent = DefaultUserAgent
	}
	if cfg.Timeout == 0 {
		cfg.Timeout = DefaultTimeout
	}
	if cfg.RetryMax == 0 {
		cfg.RetryMax = DefaultRetryMax
	}

	hc := httpx.New(httpx.Config{
		RetryMax:   cfg.RetryMax,
		Timeout:    cfg.Timeout,
		UserAgent:  cfg.UserAgent,
		SpanPrefix: "travis",
		Logger:     cfg.Logger,
	})

	return &Client{
		base:       cfg.BaseURL,
		token:      cfg.Token,
		httpClient: hc,
		logger:     cfg.Logger,
	}
}

// get issues a GET to the Travis API and decodes JSON into out.
func (c *Client) get(ctx context.Context, path string, query url.Values, out any) error {
	u, err := url.Parse(c.base + path)
	if err != nil {
		return fmt.Errorf("travis: build url: %w", err)
	}
	if query != nil {
		u.RawQuery = query.Encode()
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return err
	}
	c.applyAuth(req)
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Travis-API-Version", "3")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("travis: do: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode/100 != 2 {
		raw, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("travis: GET %s: status %d: %s", path, resp.StatusCode, snippet(raw))
	}

	if out != nil {
		if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
			return fmt.Errorf("travis: decode: %w", err)
		}
	}
	return nil
}

func (c *Client) applyAuth(req *http.Request) {
	if c.token != "" {
		req.Header.Set("Authorization", "token "+c.token)
	}
}

func snippet(b []byte) string {
	const max = 512
	if len(b) > max {
		return string(b[:max]) + "..."
	}
	return string(b)
}

// repoSlugEscaped URL-encodes the slug for use in a /repo/:slug path.
// Travis expects "owner%2Fname".
func repoSlugEscaped(slug string) string {
	return strings.ReplaceAll(url.PathEscape(slug), "/", "%2F")
}

// itoa is a tiny convenience.
func itoa(n int) string { return strconv.Itoa(n) }

