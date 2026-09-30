package cli

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/mileschou/devpulse/internal/incident"
	"github.com/mileschou/devpulse/internal/persistence"
	"github.com/mileschou/devpulse/internal/pullrequest"
	"github.com/mileschou/devpulse/internal/repo"
)

// seedStore opens the DB the CLI points at so a test can write rows the
// sync would normally produce. The connection is closed on cleanup.
func seedStore(t *testing.T) *persistence.Persister {
	t.Helper()
	conn, err := persistence.Open(context.Background(), os.Getenv("DEVPULSE_DSN"))
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { _ = conn.DB.Close() })
	return persistence.New(conn, nil)
}

func TestMetrics_DORASection(t *testing.T) {
	setEnvSharedSQLite(t)
	if _, err := runCmd(t, "migrate", "up"); err != nil {
		t.Fatalf("migrate up: %v", err)
	}

	ctx := context.Background()
	p := seedStore(t)
	rp := persistence.NewRepoPersister(p)
	name, _ := repo.ParseFullName("MilesChou/devpulse")
	r, err := rp.EnsureID(ctx, "github", name)
	if err != nil {
		t.Fatalf("ensure: %v", err)
	}

	// Default branch not fetched yet: the section must say so.
	out, err := runCmd(t, "metrics", "MilesChou/devpulse", "--from", "2026-05")
	if err != nil {
		t.Fatalf("metrics: %v", err)
	}
	if !strings.Contains(out, "Default branch unknown") {
		t.Fatalf("expected unknown-default-branch hint, got:\n%s", out)
	}

	if err := rp.UpdateMetadata(ctx, r.ID, repo.Repo{DefaultBranch: "main"}); err != nil {
		t.Fatalf("metadata: %v", err)
	}

	merged := func(n int, at time.Time, title string) pullrequest.PullRequest {
		first := at.Add(-6 * time.Hour)
		return pullrequest.PullRequest{
			RepoID: r.ID, Number: n, Author: "alice", Status: pullrequest.StatusMerged,
			CreatedAt: at.Add(-time.Hour), MergedAt: &at, FirstCommitAt: &first,
			Title: title, BaseRef: "main", HeadRef: "feature",
		}
	}
	d1 := time.Date(2026, 5, 10, 10, 0, 0, 0, time.UTC)
	d2 := time.Date(2026, 5, 10, 12, 30, 0, 0, time.UTC)
	revert := merged(2, d2, `Revert "feat: x"`)
	revert.RevertsNumber = func(n int) *int { return &n }(1)
	if _, err := persistence.NewPullRequestPersister(p).UpsertMany(ctx, []pullrequest.PullRequest{
		merged(1, d1, "feat: x"), revert,
	}); err != nil {
		t.Fatalf("seed prs: %v", err)
	}

	opened := time.Date(2026, 5, 12, 8, 0, 0, 0, time.UTC)
	closed := opened.Add(3 * time.Hour)
	if _, err := persistence.NewIncidentPersister(p).ReplaceForRepo(ctx, r.ID, []incident.Incident{
		{Source: incident.SourceGitHubIssue, Number: 9, Title: "down", OpenedAt: opened, ResolvedAt: &closed},
	}); err != nil {
		t.Fatalf("seed incidents: %v", err)
	}

	out, err = runCmd(t, "metrics", "MilesChou/devpulse", "--from", "2026-05")
	if err != nil {
		t.Fatalf("metrics: %v", err)
	}
	for _, want := range []string{
		"Deployment Frequency:   2 deploys into main  (0.45/week, 1 deploy days)",
		"Lead Time for Changes:  avg 6.0h  p50 6.0h  p90 6.0h  (2 deploys)",
		`Change Failure Rate:    50.0% (1/2)  reverts=1 hotfixes=0 (label "hotfix")`,
		`Recovery Time:          avg 2.8h  p50 2.8h  p90 3.0h  (2 samples)  from reverts=1 incidents=1 (label "incident")`,
	} {
		if !strings.Contains(out, want) {
			t.Errorf("missing line %q in:\n%s", want, out)
		}
	}
}
