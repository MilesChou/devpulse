package persistence_test

import (
	"context"
	"database/sql"
	"errors"
	"testing"
	"time"

	"github.com/mileschou/devpulse/internal/build"
	"github.com/mileschou/devpulse/internal/persistence"
	"github.com/mileschou/devpulse/internal/pullrequest"
	"github.com/mileschou/devpulse/internal/x/commitsha"
)

// TestBuildPersister_LinkPullRequestsByBranch covers every rule of the
// branch + time-window match: a reused branch name resolves by time,
// ambiguous and out-of-window builds stay unlinked, push builds and
// already-linked builds are untouched, and another repo's PRs never
// match.
func TestBuildPersister_LinkPullRequestsByBranch(t *testing.T) {
	p := setup(t)
	rp := persistence.NewRepoPersister(p)
	pp := persistence.NewPullRequestPersister(p)
	bp := persistence.NewBuildPersister(p)
	ctx := context.Background()

	r, _ := rp.EnsureID(ctx, "github", mustFullName(t, "MilesChou/devpulse"))
	other, _ := rp.EnsureID(ctx, "github", mustFullName(t, "MilesChou/other"))

	at := func(h int) time.Time { return time.Date(2026, 5, 1, h, 0, 0, 0, time.UTC) }
	pr := func(repoID string, n int, head string, created int, closed *time.Time) pullrequest.PullRequest {
		status := pullrequest.StatusOpen
		if closed != nil {
			status = pullrequest.StatusClosed
		}
		return pullrequest.PullRequest{
			RepoID: repoID, Number: n, Author: "alice", Status: status,
			CreatedAt: at(created), ClosedAt: closed, HeadRef: head,
		}
	}
	if _, err := pp.UpsertMany(ctx, []pullrequest.PullRequest{
		pr(r.ID, 1, "feat/a", 10, timePtr(at(12))), // feat/a, first use
		pr(r.ID, 2, "feat/a", 13, nil),             // feat/a reused later
		pr(r.ID, 3, "main", 8, nil),                // two fork PRs open at once
		pr(r.ID, 4, "main", 8, nil),                //   from a branch named main
		pr(other.ID, 1, "feat/x", 8, nil),          // other repo only
	}); err != nil {
		t.Fatalf("seed prs: %v", err)
	}

	sha, _ := commitsha.Parse("aaa1234567890abcdef1234567890abcdef12345")
	b := func(id, branch string, trigger build.Trigger, started, prNumber int) build.Build {
		return build.Build{
			ExternalID: id, CommitSHA: sha, Status: build.StatusPassed,
			Trigger: trigger, Branch: branch, StartedAt: at(started), PRNumber: prNumber,
		}
	}
	if _, err := bp.UpsertMany(ctx, r.ID, "github-actions", []build.Build{
		b("in-first", "feat/a", build.TriggerPullRequest, 11, 0),
		b("in-second", "feat/a", build.TriggerPullRequest, 14, 0),
		b("before-any", "feat/a", build.TriggerPullRequest, 9, 0),
		b("ambiguous", "main", build.TriggerPullRequest, 11, 0),
		b("push", "feat/a", build.TriggerPush, 11, 0),
		b("preset", "feat/a", build.TriggerPullRequest, 11, 99),
		b("no-branch", "", build.TriggerPullRequest, 11, 0),
		b("other-repo", "feat/x", build.TriggerPullRequest, 11, 0),
	}); err != nil {
		t.Fatalf("seed builds: %v", err)
	}

	linked, err := bp.LinkPullRequestsByBranch(ctx, r.ID)
	if err != nil {
		t.Fatalf("link: %v", err)
	}
	if linked != 2 {
		t.Fatalf("linked: %d, want 2", linked)
	}

	want := map[string]int{ // 0 = NULL
		"in-first": 1, "in-second": 2, "before-any": 0, "ambiguous": 0,
		"push": 0, "preset": 99, "no-branch": 0, "other-repo": 0,
	}
	for id, wantPR := range want {
		var got sql.NullInt64
		if err := p.QueryRowCtx(ctx, `SELECT pr_number FROM builds WHERE external_id = ?`, id).Scan(&got); err != nil {
			t.Fatalf("read %s: %v", id, err)
		}
		if int(got.Int64) != wantPR || got.Valid != (wantPR != 0) {
			t.Errorf("build %s: pr_number=%v, want %d", id, got, wantPR)
		}
	}

	// Idempotent: nothing left to link.
	if again, err := bp.LinkPullRequestsByBranch(ctx, r.ID); err != nil || again != 0 {
		t.Fatalf("rerun: (%d, %v), want (0, nil)", again, err)
	}
}

func TestRepoPersister_PRUpdatedWatermark(t *testing.T) {
	p := setup(t)
	rp := persistence.NewRepoPersister(p)
	ctx := context.Background()

	r, err := rp.EnsureID(ctx, "github", mustFullName(t, "MilesChou/devpulse"))
	if err != nil {
		t.Fatalf("ensure: %v", err)
	}
	if r.PRUpdatedWatermark != nil {
		t.Fatalf("new repo watermark: %v, want nil", r.PRUpdatedWatermark)
	}

	w := time.Date(2026, 5, 9, 12, 30, 0, 0, time.UTC)
	if err := rp.UpdatePRUpdatedWatermark(ctx, r.ID, w); err != nil {
		t.Fatalf("update: %v", err)
	}
	// Same value again: MySQL may report 0 affected rows.
	if err := rp.UpdatePRUpdatedWatermark(ctx, r.ID, w); err != nil {
		t.Fatalf("idempotent update: %v", err)
	}
	got, err := rp.FindByID(ctx, r.ID)
	if err != nil {
		t.Fatalf("find: %v", err)
	}
	if got.PRUpdatedWatermark == nil || !got.PRUpdatedWatermark.Equal(w) {
		t.Fatalf("watermark: %v, want %v", got.PRUpdatedWatermark, w)
	}

	if err := rp.UpdatePRUpdatedWatermark(ctx, "01NOTEXIST0000000000000000", w); !errors.Is(err, persistence.ErrRepoNotFound) {
		t.Fatalf("want ErrRepoNotFound, got %v", err)
	}
}
