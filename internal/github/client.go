// Package github implements fetching.VCSProvider against the GitHub REST
// and GraphQL APIs. Pagination, retry, and OTel instrumentation come from
// internal/x/httpx; this package just owns the request shapes.
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

	"github.com/mileschou/devpulse/internal/x/httpx"
)

const (
	DefaultBaseURL   = "https://api.github.com"
	DefaultUserAgent = "devpulse/0 (+https://github.com/MilesChou/devpulse)"
	DefaultTimeout   = 30 * time.Second
	DefaultRetryMax  = 3
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
	RetryMax  int
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
		SpanPrefix: "github",
		Logger:     cfg.Logger,
	})

	return &Client{
		base:       cfg.BaseURL,
		token:      cfg.Token,
		userAgent:  cfg.UserAgent,
		httpClient: hc,
		logger:     cfg.Logger,
	}
}

// rest issues a REST request and decodes the JSON body into out. The
// Link header (if any) is returned for pagination.
//
// query is appended as ?k=v; pass nil if none.
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
	c.applyAuth(req)
	req.Header.Set("Accept", "application/vnd.github+json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("github: do: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode/100 != 2 {
		body, _ := io.ReadAll(resp.Body)
		return resp.Header, fmt.Errorf("github: %s %s: status %d: %s",
			method, path, resp.StatusCode, httpx.Snippet(body))
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
	c.applyAuth(req)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/vnd.github+json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("github: graphql do: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode/100 != 2 {
		raw, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("github: graphql status %d: %s", resp.StatusCode, httpx.Snippet(raw))
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

func (c *Client) applyAuth(req *http.Request) {
	if c.token != "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
	}
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")
}
