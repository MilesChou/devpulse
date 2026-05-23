package github_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/mileschou/devpulse/internal/github"
	"github.com/mileschou/devpulse/internal/repo"
)

// TestNewClient_EmptyToken_SucceedsAndOmitsAuthHeader verifies that
// constructing a client without a token does not error and does not send
// a forged "Authorization: Bearer " header to GitHub. Subcommands that do
// not call the GitHub API (init, migrate) rely on this contract — they
// build the dependency graph without a token configured.
func TestNewClient_EmptyToken_SucceedsAndOmitsAuthHeader(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Authorization"); got != "" {
			t.Errorf("expected no Authorization header for empty token, got %q", got)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":1,"full_name":"o/r","default_branch":"main"}`))
	}))
	defer srv.Close()

	c, err := github.NewClient(github.Config{
		BaseURL: srv.URL,
		Token:   "",
		Timeout: 5 * time.Second,
	})
	if err != nil {
		t.Fatalf("new client with empty token: %v", err)
	}

	name, _ := repo.ParseFullName("o/r")
	if _, err := github.NewProvider(c).GetRepo(context.Background(), name); err != nil {
		t.Fatalf("get repo: %v", err)
	}
}
