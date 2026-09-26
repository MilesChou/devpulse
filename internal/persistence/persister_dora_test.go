package persistence_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/mileschou/devpulse/internal/incident"
	"github.com/mileschou/devpulse/internal/persistence"
	"github.com/mileschou/devpulse/internal/pullrequest"
	"github.com/mileschou/devpulse/internal/repo"
)

func intPtr(n int) *int { return &n }

func timePtr(t time.Time) *time.Time { return &t }

// mergedPR builds a merged PR carrying the DORA columns.
func mergedPR(repoID string, number int, base string, merged time.Time) pullrequest.PullRequest {
	created := merged.Add(-2 * time.Hour)
	return pullrequest.PullRequest{
		RepoID:    repoID,
		Number:    number,
		Author:    "alice",
		Status:    pullrequest.StatusMerged,
		CreatedAt: created,
		MergedAt:  timePtr(merged),
		Title:     "feat: x",
		BaseRef:   base,
		HeadRef:   "feature/x",
	}
}

func TestPullRequestPersister_Upsert_RoundTripsDORAFields(t *testing.T) {
	p := setup(t)
	rp := persistence.NewRepoPersister(p)
	pp := persistence.NewPullRequestPersister(p)
	ctx := context.Background()

	r, _ := rp.EnsureID(ctx, "github", mustFullName(t, "MilesChou/devpulse"))

	merged := time.Date(2026, 5, 10, 15, 0, 0, 0, time.UTC)
	first := time.Date(2026, 5, 10, 9, 0, 0, 0, time.UTC)
	pr := mergedPR(r.ID, 11, "main", merged)
	pr.Title = `Revert "feat: x"`
	pr.Labels = []string{"bug", "hot fix"}
	pr.HeadRef = "revert-10"
	pr.MergeCommitSHA = "0123456789abcdef0123456789abcdef01234567"
	pr.FirstCommitAt = &first
	pr.RevertsNumber = intPtr(10)

	if _, err := pp.UpsertMany(ctx, []pullrequest.PullRequest{pr}); err != nil {
		t.Fatalf("upsert: %v", err)
	}
	got, err := pp.FindByNumber(ctx, r.ID, 11)
	if err != nil {
		t.Fatalf("find: %v", err)
	}
	if got.Title != pr.Title || got.BaseRef != "main" || got.HeadRef != "revert-10" || got.MergeCommitSHA != pr.MergeCommitSHA {
		t.Fatalf("text fields: %+v", got)
	}
	if len(got.Labels) != 2 || got.Labels[0] != "bug" || got.Labels[1] != "hot fix" {
		t.Fatalf("labels: %q", got.Labels)
	}
	if got.FirstCommitAt == nil || !got.FirstCommitAt.Equal(first) {
		t.Fatalf("first_commit_at: %v", got.FirstCommitAt)
	}
	if got.RevertsNumber == nil || *got.RevertsNumber != 10 {
		t.Fatalf("reverts_number: %v", got.RevertsNumber)
	}

	// A PR without DORA facts reads back as zero values, not "".
	bare := mergedPR(r.ID, 12, "", merged)
	bare.Title, bare.HeadRef = "", ""
	if _, err := pp.UpsertMany(ctx, []pullrequest.PullRequest{bare}); err != nil {
		t.Fatalf("upsert bare: %v", err)
	}
	got, err = pp.FindByNumber(ctx, r.ID, 12)
	if err != nil {
		t.Fatalf("find bare: %v", err)
	}
	if got.Labels != nil || got.FirstCommitAt != nil || got.RevertsNumber != nil || got.BaseRef != "" {
		t.Fatalf("bare PR: %+v", got)
	}
}

func TestPullRequestPersister_ListOpenNumbers(t *testing.T) {
	p := setup(t)
	rp := persistence.NewRepoPersister(p)
	pp := persistence.NewPullRequestPersister(p)
	ctx := context.Background()

	r, _ := rp.EnsureID(ctx, "github", mustFullName(t, "MilesChou/devpulse"))
	other, _ := rp.EnsureID(ctx, "github", mustFullName(t, "MilesChou/other"))

	created := time.Date(2026, 5, 1, 9, 0, 0, 0, time.UTC)
	open := func(repoID string, n int) pullrequest.PullRequest {
		return pullrequest.PullRequest{RepoID: repoID, Number: n, Author: "a", Status: pullrequest.StatusOpen, CreatedAt: created}
	}
	prs := []pullrequest.PullRequest{
		open(r.ID, 9), open(r.ID, 3),
		mergedPR(r.ID, 5, "main", created.Add(time.Hour)),
		open(other.ID, 1),
	}
	if _, err := pp.UpsertMany(ctx, prs); err != nil {
		t.Fatalf("upsert: %v", err)
	}

	got, err := pp.ListOpenNumbers(ctx, r.ID)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(got) != 2 || got[0] != 3 || got[1] != 9 {
		t.Fatalf("want [3 9], got %v", got)
	}
}

func TestRepoPersister_Labels(t *testing.T) {
	p := setup(t)
	rp := persistence.NewRepoPersister(p)
	ctx := context.Background()

	r, err := rp.EnsureID(ctx, "github", mustFullName(t, "MilesChou/devpulse"))
	if err != nil {
		t.Fatalf("ensure: %v", err)
	}
	if r.IncidentLabel != repo.DefaultIncidentLabel || r.HotfixLabel != repo.DefaultHotfixLabel {
		t.Fatalf("create defaults: %q / %q", r.IncidentLabel, r.HotfixLabel)
	}

	if err := rp.UpdateLabels(ctx, r.ID, " sev-1 ", "urgent-fix"); err != nil {
		t.Fatalf("update: %v", err)
	}
	// Same values again: MySQL may report 0 affected rows; must not be
	// mistaken for "not found".
	if err := rp.UpdateLabels(ctx, r.ID, "sev-1", "urgent-fix"); err != nil {
		t.Fatalf("idempotent update: %v", err)
	}
	got, err := rp.FindByID(ctx, r.ID)
	if err != nil {
		t.Fatalf("find: %v", err)
	}
	if got.IncidentLabel != "sev-1" || got.HotfixLabel != "urgent-fix" {
		t.Fatalf("labels: %q / %q", got.IncidentLabel, got.HotfixLabel)
	}

	if err := rp.UpdateLabels(ctx, r.ID, "  ", "x"); err == nil {
		t.Fatal("blank incident label accepted")
	}
	if err := rp.UpdateLabels(ctx, "01NOTEXIST0000000000000000", "a", "b"); !errors.Is(err, persistence.ErrRepoNotFound) {
		t.Fatalf("want ErrRepoNotFound, got %v", err)
	}
}

func TestIncidentPersister_ReplaceForRepo(t *testing.T) {
	p := setup(t)
	rp := persistence.NewRepoPersister(p)
	ip := persistence.NewIncidentPersister(p)
	ctx := context.Background()

	r, _ := rp.EnsureID(ctx, "github", mustFullName(t, "MilesChou/devpulse"))
	other, _ := rp.EnsureID(ctx, "github", mustFullName(t, "MilesChou/other"))

	opened := time.Date(2026, 5, 2, 8, 0, 0, 0, time.UTC)
	inc := func(n int, resolved *time.Time) incident.Incident {
		return incident.Incident{Source: incident.SourceGitHubIssue, Number: n, Title: "down", OpenedAt: opened, ResolvedAt: resolved}
	}

	if _, err := ip.ReplaceForRepo(ctx, r.ID, []incident.Incident{inc(1, nil), inc(2, timePtr(opened.Add(3*time.Hour)))}); err != nil {
		t.Fatalf("replace 1: %v", err)
	}
	if _, err := ip.ReplaceForRepo(ctx, other.ID, []incident.Incident{inc(1, nil)}); err != nil {
		t.Fatalf("replace other: %v", err)
	}
	// Second sync: #1 lost its label, #3 appeared.
	n, err := ip.ReplaceForRepo(ctx, r.ID, []incident.Incident{inc(2, timePtr(opened.Add(3*time.Hour))), inc(3, nil)})
	if err != nil || n != 2 {
		t.Fatalf("replace 2: n=%d err=%v", n, err)
	}

	got, err := ip.List(ctx, r.ID)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(got) != 2 || got[0].Number != 2 || got[1].Number != 3 {
		t.Fatalf("want [2 3], got %+v", got)
	}
	if got[0].ResolvedAt == nil || got[0].ResolvedAt.Sub(got[0].OpenedAt) != 3*time.Hour || got[1].ResolvedAt != nil {
		t.Fatalf("timestamps: %+v", got)
	}
	if others, _ := ip.List(ctx, other.ID); len(others) != 1 {
		t.Fatalf("other repo touched: %+v", others)
	}
}

func TestMetricsPersister_DORAInput(t *testing.T) {
	p := setup(t)
	rp := persistence.NewRepoPersister(p)
	pp := persistence.NewPullRequestPersister(p)
	ip := persistence.NewIncidentPersister(p)
	m := persistence.NewMetricsPersister(p)
	ctx := context.Background()

	r, _ := rp.EnsureID(ctx, "github", mustFullName(t, "MilesChou/devpulse"))

	april := time.Date(2026, 4, 30, 10, 0, 0, 0, time.UTC)
	may10 := time.Date(2026, 5, 10, 12, 30, 0, 0, time.UTC)

	reverted := mergedPR(r.ID, 10, "main", april) // before the window
	revert := mergedPR(r.ID, 11, "main", may10)
	revert.Title = `Revert "feat: x"`
	revert.RevertsNumber = intPtr(10)
	revert.Labels = []string{"hotfix"}
	dangling := mergedPR(r.ID, 12, "main", may10.Add(time.Hour))
	dangling.RevertsNumber = intPtr(999)                // unknown PR
	release := mergedPR(r.ID, 13, "release/1.x", may10) // not the default branch
	openPR := pullrequest.PullRequest{RepoID: r.ID, Number: 14, Author: "a", Status: pullrequest.StatusOpen, CreatedAt: may10, BaseRef: "main"}

	if _, err := pp.UpsertMany(ctx, []pullrequest.PullRequest{reverted, revert, dangling, release, openPR}); err != nil {
		t.Fatalf("upsert: %v", err)
	}

	opened := time.Date(2026, 5, 2, 8, 0, 0, 0, time.UTC)
	if _, err := ip.ReplaceForRepo(ctx, r.ID, []incident.Incident{
		{Source: incident.SourceGitHubIssue, Number: 1, Title: "in window", OpenedAt: opened, ResolvedAt: timePtr(opened.Add(3 * time.Hour))},
		{Source: incident.SourceGitHubIssue, Number: 2, Title: "open", OpenedAt: opened},
		{Source: incident.SourceGitHubIssue, Number: 3, Title: "june", OpenedAt: opened, ResolvedAt: timePtr(metricsTo.Add(time.Hour))},
	}); err != nil {
		t.Fatalf("incidents: %v", err)
	}

	in, err := m.DORAInput(ctx, r.ID, "main", metricsFrom, metricsTo)
	if err != nil {
		t.Fatalf("DORAInput: %v", err)
	}

	if len(in.Deployments) != 2 {
		t.Fatalf("want deployments [11 12], got %+v", in.Deployments)
	}
	d := in.Deployments[0]
	if d.Number != 11 || !d.MergedAt.Equal(may10) || d.Title != revert.Title || len(d.Labels) != 1 {
		t.Fatalf("revert deployment: %+v", d)
	}
	if d.RevertedMergedAt == nil || !d.RevertedMergedAt.Equal(april) {
		t.Fatalf("reverted merged_at: %v", d.RevertedMergedAt)
	}
	if in.Deployments[1].RevertedMergedAt != nil {
		t.Fatalf("dangling revert resolved: %v", in.Deployments[1].RevertedMergedAt)
	}
	if len(in.Incidents) != 1 || in.Incidents[0].Number != 1 {
		t.Fatalf("incidents: %+v", in.Incidents)
	}
}
