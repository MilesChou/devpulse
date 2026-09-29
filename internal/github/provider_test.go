package github_test

import (
	"context"
	"errors"
	"fmt"
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
	if pr.Title != `Revert "Add fetch orchestrator"` {
		t.Fatalf("title: %q", pr.Title)
	}
	if pr.BaseRef != "main" || pr.HeadRef != "revert-40-fetch" {
		t.Fatalf("refs: base=%q head=%q", pr.BaseRef, pr.HeadRef)
	}
	if pr.MergeCommitSHA != "0123456789abcdef0123456789abcdef01234567" {
		t.Fatalf("merge sha: %q", pr.MergeCommitSHA)
	}
	if len(pr.Labels) != 2 || pr.Labels[0] != "bug" || pr.Labels[1] != "hotfix" {
		t.Fatalf("labels: %v", pr.Labels)
	}
	if pr.RevertsNumber == nil || *pr.RevertsNumber != 40 {
		t.Fatalf("reverts number: %v", pr.RevertsNumber)
	}
	if want := time.Date(2026, 5, 15, 14, 0, 0, 0, time.UTC); !pr.SourceUpdatedAt.Equal(want) {
		t.Fatalf("source updated_at: %v, want %v", pr.SourceUpdatedAt, want)
	}
}

// TestGetFirstCommitAt_MinAuthorDate asserts the lead-time start is the
// earliest author date on the page, not the first element: rebases can
// reorder commits relative to their author dates.
func TestGetFirstCommitAt_MinAuthorDate(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/repos/MilesChou/devpulse/pulls/42/commits" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		if r.URL.Query().Get("per_page") != "100" {
			t.Errorf("per_page: %q", r.URL.Query().Get("per_page"))
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`[
			{"commit": {"author": {"date": "2026-05-14T11:00:00+08:00"}}},
			{"commit": {"author": {"date": "2026-05-14T02:00:00Z"}}},
			{"commit": {"author": null}}
		]`))
	}))
	defer srv.Close()

	c := newClient(t, srv)
	repoName, _ := repo.ParseFullName("MilesChou/devpulse")
	got, err := c.GetFirstCommitAt(context.Background(), repoName, 42)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	want := time.Date(2026, 5, 14, 2, 0, 0, 0, time.UTC)
	if got == nil || !got.Equal(want) || got.Location() != time.UTC {
		t.Fatalf("got %v, want %v (UTC)", got, want)
	}
}

// TestGetFirstCommitAt_IgnoresImplausibleDates asserts an author date
// from a broken clock (the Unix epoch) does not become the lead-time
// start, so it can neither skew the metric nor overflow MySQL TIMESTAMP.
func TestGetFirstCommitAt_IgnoresImplausibleDates(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`[
			{"commit": {"author": {"date": "1970-01-01T00:00:00Z"}}},
			{"commit": {"author": {"date": "2026-05-14T02:00:00Z"}}}
		]`))
	}))
	defer srv.Close()

	c := newClient(t, srv)
	repoName, _ := repo.ParseFullName("MilesChou/devpulse")
	got, err := c.GetFirstCommitAt(context.Background(), repoName, 42)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	want := time.Date(2026, 5, 14, 2, 0, 0, 0, time.UTC)
	if got == nil || !got.Equal(want) {
		t.Fatalf("got %v, want %v", got, want)
	}
}

func TestGetFirstCommitAt_NoCommits(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`[]`))
	}))
	defer srv.Close()

	c := newClient(t, srv)
	repoName, _ := repo.ParseFullName("MilesChou/devpulse")
	got, err := c.GetFirstCommitAt(context.Background(), repoName, 42)
	if err != nil || got != nil {
		t.Fatalf("want (nil, nil), got (%v, %v)", got, err)
	}
}

// TestListIncidentIssues_PaginatesAndDropsPRs walks a full first page and
// a short second page, and asserts that pull requests carrying the label
// are dropped.
func TestListIncidentIssues_PaginatesAndDropsPRs(t *testing.T) {
	var pages []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/repos/MilesChou/devpulse/issues" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		q := r.URL.Query()
		if q.Get("labels") != "sev-1" || q.Get("state") != "all" {
			t.Errorf("query: %s", r.URL.RawQuery)
		}
		pages = append(pages, q.Get("page"))
		w.Header().Set("Content-Type", "application/json")

		if q.Get("page") == "1" {
			items := make([]string, 0, 100)
			for i := 1; i <= 100; i++ {
				if i == 2 {
					items = append(items, `{"number": 2, "title": "pr", "created_at": "2026-05-01T00:00:00Z", "pull_request": {}}`)
					continue
				}
				items = append(items, fmt.Sprintf(`{"number": %d, "title": "t", "created_at": "2026-05-01T00:00:00Z"}`, i))
			}
			_, _ = w.Write([]byte("[" + strings.Join(items, ",") + "]"))
			return
		}
		_, _ = w.Write([]byte(`[{"number": 101, "title": "db down", "created_at": "2026-05-02T08:00:00Z", "closed_at": "2026-05-02T11:00:00Z"}]`))
	}))
	defer srv.Close()

	c := newClient(t, srv)
	repoName, _ := repo.ParseFullName("MilesChou/devpulse")
	got, err := c.ListIncidentIssues(context.Background(), repoName, "sev-1")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if strings.Join(pages, ",") != "1,2" {
		t.Fatalf("pages: %v", pages)
	}
	if len(got) != 100 {
		t.Fatalf("want 100 incidents (101 issues minus 1 PR), got %d", len(got))
	}
	last := got[len(got)-1]
	if last.Number != 101 || last.Title != "db down" || last.ResolvedAt == nil ||
		last.ResolvedAt.Sub(last.OpenedAt) != 3*time.Hour {
		t.Fatalf("last incident: %+v", last)
	}
	if got[0].ResolvedAt != nil {
		t.Fatalf("open issue has resolved_at: %v", got[0].ResolvedAt)
	}
	for _, inc := range got {
		if inc.Number == 2 {
			t.Fatal("pull request leaked into incidents")
		}
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

// TestListPullRequestsUpdatedSince_StopsAtSince asserts the walk reads
// sort=updated pages newest first, keeps every PR updated at or after
// since (the boundary is inclusive), stops at the first older one
// without requesting another page, and reports the head of the list as
// newest.
func TestListPullRequestsUpdatedSince_StopsAtSince(t *testing.T) {
	head := time.Date(2026, 6, 1, 12, 0, 0, 0, time.UTC)
	since := head.Add(-101 * time.Minute)

	var pages []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/repos/MilesChou/devpulse/pulls" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		q := r.URL.Query()
		if q.Get("state") != "all" || q.Get("sort") != "updated" || q.Get("direction") != "desc" || q.Get("per_page") != "100" {
			t.Errorf("query: %s", r.URL.RawQuery)
		}
		pages = append(pages, q.Get("page"))

		// PR #(1000-i) is updated i minutes before head: page 1 holds
		// i = 0..99, page 2 holds i = 100..199.
		offset := 0
		if q.Get("page") == "2" {
			offset = 100
		}
		items := make([]string, 0, 100)
		for i := offset; i < offset+100; i++ {
			items = append(items, fmt.Sprintf(`{"number": %d, "updated_at": %q}`,
				1000-i, head.Add(-time.Duration(i)*time.Minute).Format(time.RFC3339)))
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte("[" + strings.Join(items, ",") + "]"))
	}))
	defer srv.Close()

	c := newClient(t, srv)
	repoName, _ := repo.ParseFullName("MilesChou/devpulse")
	updated, newest, err := c.ListPullRequestsUpdatedSince(context.Background(), repoName, since)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if strings.Join(pages, ",") != "1,2" {
		t.Fatalf("pages: %v, want 1,2 (stop inside page 2)", pages)
	}
	if !newest.Equal(head) {
		t.Fatalf("newest: %v, want %v", newest, head)
	}
	// i = 0..101 are at or after since: 102 PRs, #1000 down to #899,
	// the last one exactly at since.
	if len(updated) != 102 || updated[0].Number != 1000 || updated[101].Number != 899 {
		t.Fatalf("updated: len=%d first=%v last=%v", len(updated), updated[0], updated[len(updated)-1])
	}
	if !updated[0].UpdatedAt.Equal(head) || !updated[101].UpdatedAt.Equal(since) {
		t.Fatalf("updated_at: first=%v last=%v", updated[0].UpdatedAt, updated[101].UpdatedAt)
	}
}

// TestListPullRequestsUpdatedSince_ZeroSinceReadsNewestOnly asserts the
// bootstrap call costs one single-item request and lists nothing.
func TestListPullRequestsUpdatedSince_ZeroSinceReadsNewestOnly(t *testing.T) {
	var calls int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if got := r.URL.Query().Get("per_page"); got != "1" {
			t.Errorf("per_page: %q, want 1", got)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`[{"number": 7, "updated_at": "2026-06-01T12:00:00Z"}]`))
	}))
	defer srv.Close()

	c := newClient(t, srv)
	repoName, _ := repo.ParseFullName("MilesChou/devpulse")
	updated, newest, err := c.ListPullRequestsUpdatedSince(context.Background(), repoName, time.Time{})
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if calls != 1 || len(updated) != 0 {
		t.Fatalf("calls=%d updated=%v, want 1 call and no PRs", calls, updated)
	}
	if want := time.Date(2026, 6, 1, 12, 0, 0, 0, time.UTC); !newest.Equal(want) {
		t.Fatalf("newest: %v", newest)
	}
}

// TestListPullRequestsUpdatedSince_EmptyRepo asserts a repo without PRs
// yields a zero newest, which the orchestrator reads as "keep the
// watermark".
func TestListPullRequestsUpdatedSince_EmptyRepo(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`[]`))
	}))
	defer srv.Close()

	c := newClient(t, srv)
	repoName, _ := repo.ParseFullName("MilesChou/devpulse")
	updated, newest, err := c.ListPullRequestsUpdatedSince(context.Background(), repoName,
		time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC))
	if err != nil || len(updated) != 0 || !newest.IsZero() {
		t.Fatalf("got (%v, %v, %v), want (nil, zero, nil)", updated, newest, err)
	}
}
