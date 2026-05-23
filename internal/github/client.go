// Package github implements fetching.VCSProvider against the GitHub REST
// and GraphQL APIs. The HTTP plumbing — auth headers, default headers,
// rate-limit-aware behavior — comes from github.com/cli/go-gh; this package
// just owns the request shapes (paths, queries, GraphQL bodies) and adapts
// responses into the domain model.
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
	token      string
	userAgent  string
	httpClient *http.Client
	logger     *slog.Logger
}

// NewClient returns a configured Client.
//
// The HTTP client is built by api.NewHTTPClient from go-gh, which provides
// GitHub-aware rate-limit handling (secondary rate limit + Retry-After
// header), default headers, and a sanitizing transport. The Authorization
// header is set explicitly to "Bearer <token>" — go-gh's default is
// "token <token>", but we keep Bearer for parity with the REST API docs and
// existing test expectations.
//
// The host used for the token-domain safety check is derived from BaseURL,
// so a test server at http://127.0.0.1:PORT still receives auth headers.
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

	host := hostFromBaseURL(cfg.BaseURL)

	headers := map[string]string{
		"User-Agent": cfg.UserAgent,
		// go-gh's default is "token <token>"; we override to "Bearer <token>"
		// for parity with the GitHub REST API documentation.
		"Authorization": "Bearer " + cfg.Token,
	}

	hc, err := api.NewHTTPClient(api.ClientOptions{
		Host:      host,
		AuthToken: cfg.Token,
		Headers:   headers,
		Timeout:   cfg.Timeout,
		Transport: otelhttp.NewTransport(
			http.DefaultTransport,
			otelhttp.WithSpanNameFormatter(func(_ string, r *http.Request) string {
				return "github." + r.Method + " " + r.URL.Path
			}),
		),
	})
	if err != nil {
		// api.NewHTTPClient only fails when resolving options from gh's
		// on-disk config; we provide all required options ourselves, so
		// this path is unreachable in practice. Fall back to a bare client
		// to avoid panicking on a misconfigured environment.
		hc = &http.Client{Timeout: cfg.Timeout}
	}

	return &Client{
		base:       cfg.BaseURL,
		token:      cfg.Token,
		userAgent:  cfg.UserAgent,
		httpClient: hc,
		logger:     cfg.Logger,
	}
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
// The request is sent through go-gh's http.Client, which applies the
// Authorization, Accept, User-Agent, Content-Type, and Time-Zone headers
// and handles GitHub's rate-limit responses. We still need to set the
// API-version pin explicitly — it is not part of go-gh's defaults.
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
