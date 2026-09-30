package persistence_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/mileschou/devpulse/internal/build"
	"github.com/mileschou/devpulse/internal/incident"
	"github.com/mileschou/devpulse/internal/persistence"
	"github.com/mileschou/devpulse/internal/pullrequest"
	"github.com/mileschou/devpulse/internal/x/commitsha"
)

// TestRepoPersister_Delete seeds two repos with every kind of synced
// row, deletes one, and asserts its rows are all gone while the other
// repo is untouched.
func TestRepoPersister_Delete(t *testing.T) {
	p := setup(t)
	ctx := context.Background()
	rp := persistence.NewRepoPersister(p)

	sha, _ := commitsha.Parse("aaa1234567890abcdef1234567890abcdef12345")
	at := time.Date(2026, 5, 1, 10, 0, 0, 0, time.UTC)
	seed := func(name string) string {
		r, err := rp.EnsureID(ctx, "github", mustFullName(t, name))
		if err != nil {
			t.Fatalf("ensure %s: %v", name, err)
		}
		prs := []pullrequest.PullRequest{{RepoID: r.ID, Number: 1, Author: "alice", Status: pullrequest.StatusOpen, CreatedAt: at}}
		if _, err := persistence.NewPullRequestPersister(p).UpsertMany(ctx, prs); err != nil {
			t.Fatalf("seed pr: %v", err)
		}
		if err := persistence.NewReviewPersister(p).Upsert(ctx, prs[0].ID, pullrequest.Review{
			ReviewerAccount: "bob", State: pullrequest.ReviewStateApproved, SubmittedAt: at,
		}); err != nil {
			t.Fatalf("seed review: %v", err)
		}
		if _, err := persistence.NewBuildPersister(p).UpsertMany(ctx, r.ID, "github-actions", []build.Build{
			{ExternalID: "1", CommitSHA: sha, Status: build.StatusPassed, StartedAt: at},
		}); err != nil {
			t.Fatalf("seed build: %v", err)
		}
		if _, err := persistence.NewIncidentPersister(p).ReplaceForRepo(ctx, r.ID, []incident.Incident{
			{Source: incident.SourceGitHubIssue, Number: 7, Title: "down", OpenedAt: at},
		}); err != nil {
			t.Fatalf("seed incident: %v", err)
		}
		return r.ID
	}
	gone, kept := seed("acme/gone"), seed("acme/kept")

	count := func(q, repoID string) int {
		t.Helper()
		var n int
		if err := p.DB.QueryRowContext(ctx, p.Rebind(q), repoID).Scan(&n); err != nil {
			t.Fatalf("count: %v", err)
		}
		return n
	}
	counts := func(repoID string) [5]int {
		return [5]int{
			count(`SELECT COUNT(*) FROM repos WHERE id = ?`, repoID),
			count(`SELECT COUNT(*) FROM pull_requests WHERE repo_id = ?`, repoID),
			count(`SELECT COUNT(*) FROM pull_request_reviews r JOIN pull_requests pr ON pr.id = r.pull_request_id WHERE pr.repo_id = ?`, repoID),
			count(`SELECT COUNT(*) FROM builds WHERE repo_id = ?`, repoID),
			count(`SELECT COUNT(*) FROM incidents WHERE repo_id = ?`, repoID),
		}
	}
	var reviewsBefore int
	if err := p.DB.QueryRowContext(ctx, `SELECT COUNT(*) FROM pull_request_reviews`).Scan(&reviewsBefore); err != nil {
		t.Fatalf("count reviews: %v", err)
	}

	if err := rp.Delete(ctx, gone); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if got := counts(gone); got != [5]int{} {
		t.Fatalf("deleted repo still has rows (repos, prs, reviews, builds, incidents): %v", got)
	}
	if got := counts(kept); got != [5]int{1, 1, 1, 1, 1} {
		t.Fatalf("other repo lost rows: %v", got)
	}
	var reviewsAfter int
	if err := p.DB.QueryRowContext(ctx, `SELECT COUNT(*) FROM pull_request_reviews`).Scan(&reviewsAfter); err != nil {
		t.Fatalf("count reviews: %v", err)
	}
	if reviewsAfter != reviewsBefore-1 {
		t.Fatalf("reviews: before %d, after %d; want exactly one deleted", reviewsBefore, reviewsAfter)
	}

	if err := rp.Delete(ctx, gone); !errors.Is(err, persistence.ErrRepoNotFound) {
		t.Fatalf("second delete: want ErrRepoNotFound, got %v", err)
	}
}
