package http

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"github.com/mileschou/devpulse/internal/build"
	"github.com/mileschou/devpulse/internal/incident"
	"github.com/mileschou/devpulse/internal/metrics"
	"github.com/mileschou/devpulse/internal/persistence"
	"github.com/mileschou/devpulse/internal/persistence/persistencetest"
	"github.com/mileschou/devpulse/internal/pullrequest"
	"github.com/mileschou/devpulse/internal/repo"
	"github.com/mileschou/devpulse/internal/repoadmin"
	"github.com/mileschou/devpulse/internal/x/commitsha"
)

// -update rewrites the golden files under testdata/ from the current
// handler output. The desktop client decodes the same files in its
// contract test, so review the diff before committing.
var update = flag.Bool("update", false, "rewrite golden files")

const testToken = "s3cret"

var (
	windowFrom = time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC)
	fixedNow   = windowFrom.Add(10 * 24 * time.Hour)
)

func TestStart_NoAddrIsNoOp(t *testing.T) {
	s := New(Config{})
	ctx, cancel := context.WithCancel(context.Background())

	done := make(chan error, 1)
	go func() { done <- s.Start(ctx) }()

	cancel()
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("server should return nil after cancel, got: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("server did not return on cancel")
	}
}

func TestCheckBind(t *testing.T) {
	tests := []struct {
		addr, token string
		wantErr     bool
	}{
		{"127.0.0.1:8080", "", false},
		{"localhost:8080", "", false},
		{"[::1]:8080", "", false},
		{"0.0.0.0:8080", "", true},
		{":8080", "", true},
		{"192.168.1.10:8080", "", true},
		{"0.0.0.0:8080", "t", false},
		{"not-an-addr", "", true},
	}
	for _, tt := range tests {
		err := CheckBind(tt.addr, tt.token)
		if (err != nil) != tt.wantErr {
			t.Errorf("CheckBind(%q, %q) = %v, wantErr %v", tt.addr, tt.token, err, tt.wantErr)
		}
	}
}

// newTestServer seeds one repo with builds and PRs in May 2026 and
// serves the API over it. persistencetest honours DEVPULSE_DSN, so the
// CI matrix replays these tests on PostgreSQL and MySQL too.
func newTestServer(t *testing.T) *httptest.Server {
	t.Helper()
	return newTestServerWith(t, nil)
}

// newTestServerWith is newTestServer with a hook to adjust the Config,
// e.g. to plug in a sync runner.
func newTestServerWith(t *testing.T, adjust func(*Config)) *httptest.Server {
	t.Helper()
	p := persistencetest.NewMemoryPersister(t)
	ctx := context.Background()

	repos := persistence.NewRepoPersister(p)
	name, err := repo.ParseFullName("MilesChou/devpulse")
	if err != nil {
		t.Fatalf("parse name: %v", err)
	}
	r, err := repos.EnsureID(ctx, "github", name)
	if err != nil {
		t.Fatalf("ensure repo: %v", err)
	}
	desc := "CI and PR metrics"
	if err := repos.UpdateMetadata(ctx, r.ID, repo.Repo{Description: &desc, DefaultBranch: "main"}); err != nil {
		t.Fatalf("update metadata: %v", err)
	}

	seedBuilds(t, persistence.NewBuildPersister(p), r.ID)
	seedPullRequests(t, persistence.NewPullRequestPersister(p), persistence.NewReviewPersister(p), r.ID)
	seedIncidents(t, persistence.NewIncidentPersister(p), r.ID)

	mp := persistence.NewMetricsPersister(p)
	cfg := Config{
		Token:   testToken,
		Repos:   repos,
		Metrics: mp,
		Now:     func() time.Time { return fixedNow },
		Admin:   repoadmin.New(repos, fakeMetadata{}),
		People:  persistence.NewPeoplePersister(p),
		Authors: mp,
		Scoped:  func(accounts []string) metrics.Source { return mp.Scoped(accounts) },
	}
	if adjust != nil {
		adjust(&cfg)
	}
	srv := httptest.NewServer(NewHandler(cfg))
	t.Cleanup(srv.Close)
	return srv
}

func seedBuilds(t *testing.T, bp *persistence.BuildPersister, repoID string) {
	t.Helper()
	sha, err := commitsha.Parse("aaa1234567890abcdef1234567890abcdef12345")
	if err != nil {
		t.Fatalf("parse sha: %v", err)
	}
	day1 := windowFrom.Add(10 * time.Hour)
	day2 := windowFrom.AddDate(0, 0, 1).Add(10 * time.Hour)
	finished := func(start time.Time, d time.Duration) *time.Time {
		f := start.Add(d)
		return &f
	}
	builds := []build.Build{
		{ExternalID: "1", CommitSHA: sha, Trigger: build.TriggerPullRequest, PRNumber: 1,
			Status: build.StatusPassed, StartedAt: day1, FinishedAt: finished(day1, 60*time.Second)},
		{ExternalID: "2", CommitSHA: sha, Trigger: build.TriggerPullRequest, PRNumber: 1,
			Status: build.StatusFailed, StartedAt: day1.Add(time.Hour), FinishedAt: finished(day1.Add(time.Hour), 120*time.Second)},
		{ExternalID: "3", CommitSHA: sha, Trigger: build.TriggerPullRequest, PRNumber: 2,
			Status: build.StatusErrored, StartedAt: day2, FinishedAt: finished(day2, 30*time.Second)},
		{ExternalID: "4", CommitSHA: sha, Trigger: build.TriggerPush,
			Status: build.StatusFailed, StartedAt: day2.Add(time.Hour), FinishedAt: finished(day2.Add(time.Hour), 90*time.Second)},
	}
	if _, err := bp.UpsertMany(context.Background(), repoID, "github-actions", builds); err != nil {
		t.Fatalf("seed builds: %v", err)
	}
}

func seedPullRequests(t *testing.T, pp *persistence.PullRequestPersister, rvp *persistence.ReviewPersister, repoID string) {
	t.Helper()
	mk := func(number int, leadHours float64, lines int) pullrequest.PullRequest {
		created := windowFrom.Add(time.Duration(number) * 24 * time.Hour)
		ready := created.Add(30 * time.Minute)
		firstReview := ready.Add(2 * time.Hour)
		merged := created.Add(time.Duration(leadHours * float64(time.Hour)))
		firstCommit := created.Add(-2 * time.Hour)
		return pullrequest.PullRequest{
			RepoID:            repoID,
			Number:            number,
			Author:            "alice",
			Status:            pullrequest.StatusMerged,
			Additions:         lines,
			TotalChangedLines: lines,
			SizeBucket:        pullrequest.SizeBucket(lines),
			CreatedAt:         created,
			ReadyAt:           &ready,
			FirstReviewAt:     &firstReview,
			MergedAt:          &merged,
			// Every PR merges into the default branch, so each is a DORA
			// deployment with a lead time of (lead + 2h).
			Title:         fmt.Sprintf("feat: change %d", number),
			BaseRef:       "main",
			HeadRef:       fmt.Sprintf("feature/%d", number),
			FirstCommitAt: &firstCommit,
		}
	}
	// PR #3 reverts #1: one failed change out of three deployments, and
	// a recovery sample from #1's merge to #3's.
	revert := mk(3, 30, 600)
	revert.Title = `Revert "feat: change 1"`
	one := 1
	revert.RevertsNumber = &one
	prs := []pullrequest.PullRequest{mk(1, 10, 10), mk(2, 20, 100), revert}
	if _, err := pp.UpsertMany(context.Background(), prs); err != nil {
		t.Fatalf("seed prs: %v", err)
	}
	// Review wait reads review rows: bob reviews each PR 2h after ready,
	// and a Copilot review after one minute must not count.
	for _, pr := range prs {
		for _, rv := range []pullrequest.Review{
			{ReviewerAccount: "copilot-pull-request-reviewer", State: pullrequest.ReviewStateCommented, SubmittedAt: pr.ReadyAt.Add(time.Minute)},
			{ReviewerAccount: "bob", State: pullrequest.ReviewStateApproved, SubmittedAt: *pr.FirstReviewAt},
		} {
			if err := rvp.Upsert(context.Background(), pr.ID, rv); err != nil {
				t.Fatalf("seed review: %v", err)
			}
		}
	}
}

func seedIncidents(t *testing.T, ip *persistence.IncidentPersister, repoID string) {
	t.Helper()
	opened := windowFrom.Add(5 * 24 * time.Hour)
	resolved := opened.Add(4 * time.Hour)
	if _, err := ip.ReplaceForRepo(context.Background(), repoID, []incident.Incident{
		{Source: incident.SourceGitHubIssue, Number: 42, Title: "outage", OpenedAt: opened, ResolvedAt: &resolved},
	}); err != nil {
		t.Fatalf("seed incidents: %v", err)
	}
}

func get(t *testing.T, srv *httptest.Server, path, token string) (int, []byte) {
	t.Helper()
	req, err := http.NewRequest(http.MethodGet, srv.URL+path, nil)
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	resp, err := srv.Client().Do(req)
	if err != nil {
		t.Fatalf("GET %s: %v", path, err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}
	if ct := resp.Header.Get("Content-Type"); ct != "application/json" {
		t.Fatalf("GET %s: content-type %q", path, ct)
	}
	return resp.StatusCode, body
}

// assertGolden compares body with testdata/<name> as JSON values, so
// the golden file can stay pretty-printed for review.
func assertGolden(t *testing.T, name string, body []byte) {
	t.Helper()
	path := filepath.Join("testdata", name)

	if *update {
		var v any
		if err := json.Unmarshal(body, &v); err != nil {
			t.Fatalf("decode body: %v", err)
		}
		pretty, err := json.MarshalIndent(v, "", "  ")
		if err != nil {
			t.Fatalf("encode golden: %v", err)
		}
		if err := os.WriteFile(path, append(pretty, '\n'), 0o644); err != nil {
			t.Fatalf("write golden: %v", err)
		}
	}

	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read golden (run with -update to create): %v", err)
	}
	var gotV, wantV any
	if err := json.Unmarshal(body, &gotV); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if err := json.Unmarshal(want, &wantV); err != nil {
		t.Fatalf("decode golden: %v", err)
	}
	if !reflect.DeepEqual(gotV, wantV) {
		t.Fatalf("%s mismatch\n got: %s\nwant: %s", name, body, want)
	}
}

func TestAPI_Healthz_NoAuth(t *testing.T) {
	srv := newTestServer(t)
	status, body := get(t, srv, "/healthz", "")
	if status != http.StatusOK {
		t.Fatalf("status %d: %s", status, body)
	}
}

func TestAPI_RequiresToken(t *testing.T) {
	srv := newTestServer(t)
	for _, token := range []string{"", "wrong"} {
		status, body := get(t, srv, "/api/v1/repos", token)
		if status != http.StatusUnauthorized {
			t.Fatalf("token %q: status %d: %s", token, status, body)
		}
	}
	// Unknown API paths are still behind auth, so they don't reveal
	// which routes exist.
	if status, _ := get(t, srv, "/api/v1/nope", ""); status != http.StatusUnauthorized {
		t.Fatalf("unknown path without token: status %d", status)
	}
}

func TestAPI_ListRepos(t *testing.T) {
	srv := newTestServer(t)
	status, body := get(t, srv, "/api/v1/repos", testToken)
	if status != http.StatusOK {
		t.Fatalf("status %d: %s", status, body)
	}
	var got struct {
		Repos []repoJSON `json:"repos"`
	}
	if err := json.Unmarshal(body, &got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(got.Repos) != 1 {
		t.Fatalf("repos: %+v", got.Repos)
	}
	r := got.Repos[0]
	if r.FullName != "MilesChou/devpulse" || r.Owner != "MilesChou" || r.Name != "devpulse" ||
		r.DefaultBranch != "main" || r.Description == nil || *r.Description != "CI and PR metrics" || r.ID == "" {
		t.Fatalf("repo: %+v", r)
	}
}

func TestAPI_Metrics_Golden(t *testing.T) {
	srv := newTestServer(t)
	status, body := get(t, srv, "/api/v1/repos/MilesChou/devpulse/metrics?from=2026-05", testToken)
	if status != http.StatusOK {
		t.Fatalf("status %d: %s", status, body)
	}
	assertGolden(t, "metrics.json", body)

	// The default window is the month of Now, which the seed targets.
	status, defaultBody := get(t, srv, "/api/v1/repos/MilesChou/devpulse/metrics", testToken)
	if status != http.StatusOK {
		t.Fatalf("default window status %d", status)
	}
	assertGolden(t, "metrics.json", defaultBody)
}

func TestAPI_MonthlyMetrics_Golden(t *testing.T) {
	srv := newTestServer(t)
	status, body := get(t, srv, "/api/v1/repos/MilesChou/devpulse/metrics/monthly?from=2026-04&to=2026-06", testToken)
	if status != http.StatusOK {
		t.Fatalf("status %d: %s", status, body)
	}
	assertGolden(t, "metrics_monthly.json", body)
}

// TestAPI_WideWindow asserts the 36-month limit applies to monthly
// trends only: a single wide window is one set of queries.
func TestAPI_WideWindow(t *testing.T) {
	srv := newTestServer(t)
	if status, body := get(t, srv, "/api/v1/repos/MilesChou/devpulse/metrics?from=2020-01&to=2026-06", testToken); status != http.StatusOK {
		t.Fatalf("wide single window: status %d: %s", status, body)
	}
	if status, _ := get(t, srv, "/api/v1/repos/MilesChou/devpulse/metrics/monthly?from=2023-06&to=2026-06", testToken); status != http.StatusOK {
		t.Fatalf("36-month trend: status %d", status)
	}
}

func TestAPI_Errors(t *testing.T) {
	srv := newTestServer(t)
	tests := []struct {
		path   string
		status int
	}{
		{"/api/v1/repos/acme/unknown", http.StatusNotFound},
		{"/api/v1/repos/acme/unknown/metrics", http.StatusNotFound},
		{"/api/v1/repos/MilesChou/devpulse/metrics?from=May", http.StatusBadRequest},
		{"/api/v1/repos/MilesChou/devpulse/metrics?from=2026-05&to=2026-04", http.StatusBadRequest},
		{"/api/v1/repos/MilesChou/devpulse/metrics/monthly?from=2020-01&to=2026-01", http.StatusBadRequest},
		{"/api/v1/nope", http.StatusNotFound},
	}
	for _, tt := range tests {
		status, body := get(t, srv, tt.path, testToken)
		if status != tt.status {
			t.Errorf("GET %s: status %d, want %d (%s)", tt.path, status, tt.status, body)
			continue
		}
		var e struct {
			Error string `json:"error"`
		}
		if err := json.Unmarshal(body, &e); err != nil || e.Error == "" {
			t.Errorf("GET %s: want JSON error body, got %s", tt.path, body)
		}
	}
}
