// Package github implements fetching.VCSProvider against the GitHub REST
// and GraphQL APIs. The HTTP plumbing — auth header injection, default
// headers, and an ASCII-sanitizing transport — comes from
// github.com/cli/go-gh; this package owns the request shapes (paths,
// queries, GraphQL bodies) and adapts responses into the domain model.
//
// Note: go-gh's pkg/api does not implement rate-limit-aware retries. The
// client currently does not retry on any failure (5xx, network errors,
// primary or secondary GitHub rate limits). Adding retry is tracked
// separately; do not assume retries are happening here.
//
// OpenTelemetry instrumentation is injected via api.ClientOptions.Transport
// using the same otelhttp wrapper used by the rest of the codebase, so spans
// continue to be emitted for every GitHub request.
package github

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"time"

	"github.com/cli/go-gh/v2/pkg/api"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
)

const (
	DefaultBaseURL   = "https://api.github.com"
	DefaultUserAgent = "devpulse/0 (+https://github.com/MilesChou/devpulse)"
	DefaultTimeout   = 30 * time.Second
	graphqlEndpoint  = "/graphql"
	defaultPerPage   = 100
	graphqlMaxBatch  = 80 // alias batch limit for GetCommitAuthorsBulk
)

// Config controls the GitHub client.
type Config struct {
	BaseURL   string // override for tests / GitHub Enterprise
	Token     string // personal access token or App installation token
	UserAgent string
	Timeout   time.Duration
	Logger    *slog.Logger
}

// Client wraps the HTTP plumbing. Methods on it issue REST or GraphQL
// requests; higher-level operations live in pulls.go, reviews.go, etc.
type Client struct {
	base       string
	userAgent  string
	httpClient *http.Client
	logger     *slog.Logger
}

// NewClient returns a configured Client.
//
// When cfg.Token is non-empty, the HTTP client is built by api.NewHTTPClient
// from go-gh, which provides default headers and a sanitizing transport. The
// Authorization header is set explicitly to "Bearer <token>" — go-gh's
// default is "token <token>", but we keep Bearer for parity with the GitHub
// REST API documentation and existing test expectations. The host handed to
// go-gh's same-domain check on the Authorization header is derived from
// BaseURL, so a test server at http://127.0.0.1:PORT still receives auth
// headers.
//
// When cfg.Token is empty, go-gh's NewHTTPClient would otherwise try to
// resolve a token from gh's on-disk config (~/.config/gh/hosts.yml) and
// fail on machines without gh installed; to preserve the "deps build even
// without a GitHub token, fail later when an API call is actually made"
// contract that subcommands like `init` and `migrate` depend on, we bypass
// go-gh entirely and build a minimal *http.Client with the OTel transport.
// In that branch no Authorization header is sent, so requests against
// api.github.com will receive 401 — but local-only subcommands never reach
// that code path.
//
// An error is returned if api.NewHTTPClient fails; callers should treat
// that as fatal rather than fall back to a bare client, because the
// fallback would silently drop OTel instrumentation and every default
// header.
func NewClient(cfg Config) (*Client, error) {
	if cfg.BaseURL == "" {
		cfg.BaseURL = DefaultBaseURL
	}
	if cfg.UserAgent == "" {
		cfg.UserAgent = DefaultUserAgent
	}
	if cfg.Timeout == 0 {
		cfg.Timeout = DefaultTimeout
	}

	transport := otelhttp.NewTransport(
		http.DefaultTransport,
		otelhttp.WithSpanNameFormatter(func(_ string, r *http.Request) string {
			return "github." + r.Method + " " + r.URL.Path
		}),
	)

	var hc *http.Client
	if cfg.Token == "" {
		hc = &http.Client{
			Transport: transport,
			Timeout:   cfg.Timeout,
		}
	} else {
		host := hostFromBaseURL(cfg.BaseURL)
		// go-gh's default is "token <token>"; override to "Bearer <token>"
		// for parity with the GitHub REST API documentation.
		headers := map[string]string{
			"User-Agent":    cfg.UserAgent,
			"Authorization": "Bearer " + cfg.Token,
		}
		var err error
		hc, err = api.NewHTTPClient(api.ClientOptions{
			Host:      host,
			AuthToken: cfg.Token,
			Headers:   headers,
			Timeout:   cfg.Timeout,
			Transport: transport,
		})
		if err != nil {
			return nil, fmt.Errorf("github: build http client: %w", err)
		}
	}

	return &Client{
		base:       cfg.BaseURL,
		userAgent:  cfg.UserAgent,
		httpClient: hc,
		logger:     cfg.Logger,
	}, nil
}

// hostFromBaseURL extracts the hostname from a BaseURL so we can hand it to
// api.ClientOptions.Host. go-gh uses Host purely for its same-domain safety
// check on the Authorization header — without it, requests to a httptest
// server would have their auth header stripped.
func hostFromBaseURL(baseURL string) string {
	u, err := url.Parse(baseURL)
	if err != nil || u.Hostname() == "" {
		return "api.github.com"
	}
	return u.Hostname()
}

// rest issues a REST request and decodes the JSON body into out. The
// Link header (if any) is returned for pagination.
//
// query is appended as ?k=v; pass nil if none.
//
// When a token is configured, the request is sent through go-gh's
// http.Client, which applies the Authorization, User-Agent, Content-Type,
// and Time-Zone headers. The Accept and X-GitHub-Api-Version pins are set
// explicitly here — they are not part of go-gh's defaults.
func (c *Client) rest(ctx context.Context, method, path string, query url.Values, out any) (http.Header, error) {
	u, err := url.Parse(c.base + path)
	if err != nil {
		return nil, fmt.Errorf("github: build url: %w", err)
	}
	if query != nil {
		u.RawQuery = query.Encode()
	}

	req, err := http.NewRequestWithContext(ctx, method, u.String(), nil)
	if err != nil {
		return nil, fmt.Errorf("github: build request: %w", err)
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("github: do: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode/100 != 2 {
		body, _ := io.ReadAll(resp.Body)
		return resp.Header, fmt.Errorf("github: %s %s: status %d: %s",
			method, path, resp.StatusCode, snippet(body))
	}

	if out != nil {
		if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
			return resp.Header, fmt.Errorf("github: decode body: %w", err)
		}
	}
	return resp.Header, nil
}

// graphqlRequest is the wire shape of the POST /graphql body.
type graphqlRequest struct {
	Query     string         `json:"query"`
	Variables map[string]any `json:"variables,omitempty"`
}

// graphqlResponse captures the canonical "data + errors" envelope.
type graphqlResponse struct {
	Data   json.RawMessage `json:"data"`
	Errors []graphqlError  `json:"errors,omitempty"`
}

type graphqlError struct {
	Message string `json:"message"`
	Path    []any  `json:"path,omitempty"`
}

// graphql issues a POST /graphql and decodes the "data" key into out.
// An empty errors slice is considered success; otherwise the first error
// message is surfaced.
func (c *Client) graphql(ctx context.Context, query string, vars map[string]any, out any) error {
	body, err := json.Marshal(graphqlRequest{Query: query, Variables: vars})
	if err != nil {
		return fmt.Errorf("github: marshal graphql: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.base+graphqlEndpoint, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("github: graphql do: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode/100 != 2 {
		raw, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("github: graphql status %d: %s", resp.StatusCode, snippet(raw))
	}

	var env graphqlResponse
	if err := json.NewDecoder(resp.Body).Decode(&env); err != nil {
		return fmt.Errorf("github: graphql decode: %w", err)
	}
	if len(env.Errors) > 0 {
		return fmt.Errorf("github: graphql error: %s", env.Errors[0].Message)
	}
	if out != nil {
		if err := json.Unmarshal(env.Data, out); err != nil {
			return fmt.Errorf("github: graphql data decode: %w", err)
		}
	}
	return nil
}

// snippet returns a printable prefix of an HTTP response body for inclusion
// in error messages. Capped at 512 bytes; oversize bodies are truncated.
//
// Previously sourced from internal/x/httpx, inlined here so the github
// package can drop its dependency on httpx without forcing the Travis
// client (which still uses httpx) to change.
func snippet(b []byte) string {
	const max = 512
	if len(b) > max {
		return string(b[:max]) + "..."
	}
	return string(b)
}
