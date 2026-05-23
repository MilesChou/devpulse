package github_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mileschou/devpulse/internal/github"
	"github.com/mileschou/devpulse/internal/pullrequest"
	"github.com/mileschou/devpulse/internal/repo"
	"github.com/mileschou/devpulse/internal/x/commitsha"
)

func loadFixture(t *testing.T, name string) []byte {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("testdata", name))
	if err != nil {
		t.Fatalf("read fixture %s: %v", name, err)
	}
	return data
}

func newClient(t *testing.T, server *httptest.Server) *github.Client {
	t.Helper()
	c, err := github.NewClient(github.Config{
		BaseURL: server.URL,
		Token:   "test-token",
		Timeout: 5 * time.Second,
	})
	if err != nil {
		t.Fatalf("new client: %v", err)
	}
	return c
}

func TestListAllPullRequests_PagesThroughAll(t *testing.T) {
	page1 := loadFixture(t, "list_pulls_page1.json")
	page2 := loadFixture(t, "list_pulls_page2.json")

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/repos/MilesChou/devpulse/pulls" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer test-token" {
			t.Errorf("auth header missing: %q", got)
		}
		if r.URL.Query().Get("state") != "all" {
			t.Errorf("expected state=all, got %q", r.URL.Query().Get("state"))
		}
		page := r.URL.Query().Get("page")
		w.Header().Set("Content-Type", "application/json")
		switch page {
		case "1":
			w.Header().Set("Link", `<next>; rel="next"`)
			_, _ = w.Write(page1)
		case "2":
			_, _ = w.Write(page2)
		default:
			t.Errorf("unexpected page %q", page)
		}
	}))
	defer srv.Close()

	c := newClient(t, srv)
	repoName, _ := repo.ParseFullName("MilesChou/devpulse")

	pulls, err := c.ListAllPullRequests(context.Background(), "repo-1", repoName)
	if err != nil {
		t.Fatalf("list: %v", err)
	}

	// Without month filtering, every PR across both pages is returned.
	if len(pulls) != 3 {
		t.Fatalf("expected 3 pulls, got %d", len(pulls))
	}
	if pulls[0].Number != 42 || pulls[0].Status != pullrequest.StatusMerged {
		t.Fatalf("first pull mismatch: %+v", pulls[0])
	}
	if pulls[1].Number != 41 || !pulls[1].IsDraft {
		t.Fatalf("draft pull mismatch: %+v", pulls[1])
	}
	// Draft PR has no ReadyAt.
	if pulls[1].ReadyAt != nil {
		t.Fatalf("draft PR should have ReadyAt nil, got %v", *pulls[1].ReadyAt)
	}
}

func TestGetPullRequest_DecodesAdditionsDeletions(t *testing.T) {
	fx := loadFixture(t, "get_pull.json")

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/repos/MilesChou/devpulse/pulls/42" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(fx)
	}))
	defer srv.Close()

	c := newClient(t, srv)
	repoName, _ := repo.ParseFullName("MilesChou/devpulse")
	pr, err := c.GetPullRequest(context.Background(), "repo-1", repoName, 42)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if pr.Additions != 120 || pr.Deletions != 35 {
		t.Fatalf("change stats: +%d / -%d", pr.Additions, pr.Deletions)
	}
	if pr.TotalChangedLines != 155 {
		t.Fatalf("total: %d", pr.TotalChangedLines)
	}
	if pr.Status != pullrequest.StatusMerged {
		t.Fatalf("status: %v", pr.Status)
	}
}

func TestListReviews_FiltersPendingAndGhost(t *testing.T) {
	fx := loadFixture(t, "reviews.json")

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/graphql" || r.Method != "POST" {
			t.Errorf("unexpected %s %s", r.Method, r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(fx)
	}))
	defer srv.Close()

	c := newClient(t, srv)
	repoName, _ := repo.ParseFullName("MilesChou/devpulse")
	reviews, err := c.ListReviews(context.Background(), repoName, 42)
	if err != nil {
		t.Fatalf("list reviews: %v", err)
	}

	if len(reviews) != 2 {
		t.Fatalf("expected 2 (pending + ghost dropped), got %d", len(reviews))
	}
	if reviews[0].ReviewerAccount != "bob" || reviews[0].State != pullrequest.ReviewStateCommented {
		t.Fatalf("first review: %+v", reviews[0])
	}
	if reviews[1].ReviewerAccount != "carol" || reviews[1].State != pullrequest.ReviewStateApproved {
		t.Fatalf("second review: %+v", reviews[1])
	}
}

func TestGetCommitAuthorAccountsBulk(t *testing.T) {
	fx := loadFixture(t, "bulk_authors.json")

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/graphql" || r.Method != "POST" {
			t.Errorf("unexpected %s %s", r.Method, r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(fx)
	}))
	defer srv.Close()

	c := newClient(t, srv)
	repoName, _ := repo.ParseFullName("MilesChou/devpulse")
	shaA, _ := commitsha.Parse("aaa1234567890abcdef1234567890abcdef12345")
	shaB, _ := commitsha.Parse("bbb1234567890abcdef1234567890abcdef12345")
	shaC, _ := commitsha.Parse("ccc1234567890abcdef1234567890abcdef12345")

	got, err := c.GetCommitAuthorAccountsBulk(context.Background(), repoName, []commitsha.SHA{shaA, shaB, shaC})
	if err != nil {
		t.Fatalf("bulk: %v", err)
	}

	if got[shaA] == nil || *got[shaA] != "alice" {
		t.Fatalf("shaA: %v", got[shaA])
	}
	if got[shaB] != nil {
		// Ghost user.
		t.Fatalf("shaB should be nil, got %v", *got[shaB])
	}
	if got[shaC] == nil || *got[shaC] != "bob" {
		t.Fatalf("shaC: %v", got[shaC])
	}
}

func TestREST_ErrorResponsePropagated(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, `{"message": "not found"}`, http.StatusNotFound)
	}))
	defer srv.Close()

	c := newClient(t, srv)
	repoName, _ := repo.ParseFullName("MilesChou/devpulse")
	_, err := c.GetPullRequest(context.Background(), "repo-1", repoName, 999)
	if err == nil {
		t.Fatalf("expected error")
	}
	if !strings.Contains(err.Error(), "404") {
		t.Fatalf("expected 404 in error, got: %v", err)
	}
}
