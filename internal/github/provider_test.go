package github_test

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mileschou/devpulse/internal/fetching"
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

// TestGetLatestPRNumber_ReturnsHighest asserts the upper-bound probe
// for the by-number backfill: sort=created direction=desc per_page=1
// returns one row, the loop reads its Number.
func TestGetLatestPRNumber_ReturnsHighest(t *testing.T) {
	page1 := loadFixture(t, "list_pulls_page1.json")

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/repos/MilesChou/devpulse/pulls" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		if r.URL.Query().Get("direction") != "desc" {
			t.Errorf("expected direction=desc, got %q", r.URL.Query().Get("direction"))
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(page1)
	}))
	defer srv.Close()

	c := newClient(t, srv)
	repoName, _ := repo.ParseFullName("MilesChou/devpulse")
	got, err := c.GetLatestPRNumber(context.Background(), repoName)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if got != 42 {
		t.Fatalf("expected 42 (largest in fixture), got %d", got)
	}
}

// TestGetLatestPRNumber_EmptyRepo asserts the zero-PR case returns 0
// without an error — the orchestrator uses this to short-circuit the
// loop on a fresh repo.
func TestGetLatestPRNumber_EmptyRepo(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`[]`))
	}))
	defer srv.Close()

	c := newClient(t, srv)
	repoName, _ := repo.ParseFullName("MilesChou/devpulse")
	got, err := c.GetLatestPRNumber(context.Background(), repoName)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if got != 0 {
		t.Fatalf("expected 0 for empty repo, got %d", got)
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

// TestListReviews_FollowsCursorAcrossPages asserts that when GitHub
// reports hasNextPage=true the client passes endCursor to the next
// call and concatenates results. This is the safety net behind the
// "always overwrite enrichment on upsert" contract — large PRs (>100
// reviews) MUST get every review, otherwise enrichment computed from
// a truncated set could overwrite a more complete previous snapshot.
func TestListReviews_FollowsCursorAcrossPages(t *testing.T) {
	const (
		page1 = `{
			"data": {"repository": {"pullRequest": {"reviews": {
				"nodes": [
					{"state": "COMMENTED", "submittedAt": "2026-05-15T11:00:00Z", "author": {"login": "alice"}}
				],
				"pageInfo": {"hasNextPage": true, "endCursor": "CURSOR_AFTER_PAGE1"}
			}}}}}`
		page2 = `{
			"data": {"repository": {"pullRequest": {"reviews": {
				"nodes": [
					{"state": "APPROVED", "submittedAt": "2026-05-15T12:00:00Z", "author": {"login": "bob"}}
				],
				"pageInfo": {"hasNextPage": false, "endCursor": null}
			}}}}}`
	)

	var (
		calls         int
		cursorOnCall2 string
	)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		body, _ := io.ReadAll(r.Body)
		w.Header().Set("Content-Type", "application/json")
		if calls == 1 {
			if strings.Contains(string(body), "CURSOR_AFTER_PAGE1") {
				t.Errorf("first call should not carry a cursor; body=%s", body)
			}
			_, _ = w.Write([]byte(page1))
			return
		}
		// Capture the cursor the client passed on the second call so we
		// can assert it matches page1's endCursor.
		if strings.Contains(string(body), "CURSOR_AFTER_PAGE1") {
			cursorOnCall2 = "CURSOR_AFTER_PAGE1"
		}
		_, _ = w.Write([]byte(page2))
	}))
	defer srv.Close()

	c := newClient(t, srv)
	repoName, _ := repo.ParseFullName("MilesChou/devpulse")
	reviews, err := c.ListReviews(context.Background(), repoName, 42)
	if err != nil {
		t.Fatalf("list reviews: %v", err)
	}
	if calls != 2 {
		t.Fatalf("expected 2 calls (one per page), got %d", calls)
	}
	if cursorOnCall2 != "CURSOR_AFTER_PAGE1" {
		t.Fatalf("second call did not carry endCursor from page 1")
	}
	if len(reviews) != 2 || reviews[0].ReviewerAccount != "alice" || reviews[1].ReviewerAccount != "bob" {
		t.Fatalf("expected [alice, bob], got %+v", reviews)
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

// TestREST_NotFoundWrapsErrNotFound asserts that a 404 from REST is
// classified via the fetching.ErrNotFound sentinel — so the orchestrator
// can skip-vs-fail without depending on error message text.
func TestREST_NotFoundWrapsErrNotFound(t *testing.T) {
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
	if !errors.Is(err, fetching.ErrNotFound) {
		t.Fatalf("expected errors.Is(err, fetching.ErrNotFound), got: %v", err)
	}
	// Upstream body should still be present for log forensics — not a
	// hard contract, but useful to confirm we didn't swallow context.
	if !strings.Contains(err.Error(), "not found") {
		t.Fatalf("expected upstream message preserved in error, got: %v", err)
	}
}

// TestREST_Non2xxNon404PreservesStatus asserts that non-404 failures
// still surface the upstream status code in the error message —
// regression guard for the original behaviour.
func TestREST_Non2xxNon404PreservesStatus(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, `{"message": "boom"}`, http.StatusInternalServerError)
	}))
	defer srv.Close()

	c := newClient(t, srv)
	repoName, _ := repo.ParseFullName("MilesChou/devpulse")
	_, err := c.GetPullRequest(context.Background(), "repo-1", repoName, 999)
	if err == nil {
		t.Fatalf("expected error")
	}
	if errors.Is(err, fetching.ErrNotFound) {
		t.Fatalf("500 should NOT be classified as ErrNotFound: %v", err)
	}
	if !strings.Contains(err.Error(), "500") {
		t.Fatalf("expected status 500 in error, got: %v", err)
	}
}
