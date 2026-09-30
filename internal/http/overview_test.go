package http

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/mileschou/devpulse/internal/metrics"
)

// TestAPI_AllRepos checks the cross-repo report. The test store has one
// repo, so every metric must equal that repo's; only the repo label and
// the DORA section differ.
func TestAPI_AllRepos(t *testing.T) {
	srv := newTestServer(t)
	_, one := get(t, srv, "/api/v1/repos/MilesChou/devpulse/metrics?from=2026-05", testToken)
	status, all := get(t, srv, "/api/v1/metrics?from=2026-05", testToken)
	if status != http.StatusOK {
		t.Fatalf("status %d: %s", status, all)
	}
	var a, b metrics.Report
	if err := json.Unmarshal(one, &a); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(all, &b); err != nil {
		t.Fatal(err)
	}
	if b.Repo != metrics.AllReposName || b.DORA != nil {
		t.Fatalf("cross-repo report: repo %q, dora %v", b.Repo, b.DORA)
	}
	if a.BuildFailure != b.BuildFailure || a.PRLeadTime != b.PRLeadTime ||
		a.AvgBuildsPerPR != b.AvgBuildsPerPR || a.ReviewWait != b.ReviewWait {
		t.Fatalf("one repo pooled must equal the repo:\n%+v\n%+v", a, b)
	}

	status, body := get(t, srv, "/api/v1/metrics/monthly?from=2026-04&to=2026-06", testToken)
	var monthly struct {
		Repo   string           `json:"repo"`
		Months []metrics.Report `json:"months"`
	}
	if status != http.StatusOK || json.Unmarshal(body, &monthly) != nil || monthly.Repo != "*" || len(monthly.Months) != 2 {
		t.Fatalf("cross-repo monthly: %d %s", status, body)
	}
	if status, _ := get(t, srv, "/api/v1/metrics/monthly?from=2015-01&to=2026-01", testToken); status != http.StatusBadRequest {
		t.Fatalf("over 120 months: status %d, want 400", status)
	}
}

func TestAPI_RepoOverview_Golden(t *testing.T) {
	srv := newTestServer(t)
	status, body := get(t, srv, "/api/v1/overview/repos?from=2026-05", testToken)
	if status != http.StatusOK {
		t.Fatalf("status %d: %s", status, body)
	}
	assertGolden(t, "overview_repos.json", body)
}

// TestAPI_MemberOverview_Golden maps alice to a member; bob only
// reviews and copilot is an excluded bot, so Alice is the only row.
func TestAPI_MemberOverview_Golden(t *testing.T) {
	srv := newTestServer(t)
	if status, body := do(t, srv, http.MethodPost, "/api/v1/members", testToken,
		`{"display_name": "Alice", "accounts": ["alice"]}`); status != http.StatusCreated {
		t.Fatalf("create member: %d %s", status, body)
	}
	status, body := get(t, srv, "/api/v1/overview/members?from=2026-05", testToken)
	if status != http.StatusOK {
		t.Fatalf("status %d: %s", status, body)
	}
	var ov metrics.Overview[metrics.MemberRow]
	if err := json.Unmarshal(body, &ov); err != nil {
		t.Fatal(err)
	}
	if len(ov.Rows) != 1 || ov.Rows[0].Name != "Alice" || ov.Rows[0].MemberID == nil {
		t.Fatalf("rows: %+v", ov.Rows)
	}
	// IDs are generated; blank them so the golden file is stable.
	blank := ""
	ov.Rows[0].MemberID = &blank
	stable, err := json.MarshalIndent(ov, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	assertGolden(t, "overview_members.json", append(stable, '\n'))

	if status, _ := get(t, srv, "/api/v1/overview/members?from=2015-01&to=2026-01", testToken); status != http.StatusBadRequest {
		t.Fatalf("over 120 months: status %d, want 400", status)
	}
}

// TestAPI_AccountScope looks at an account no member claims, across all
// repos, the way the Overview's unmapped rows open the dashboard.
func TestAPI_AccountScope(t *testing.T) {
	srv := newTestServer(t)
	status, body := get(t, srv, "/api/v1/metrics?from=2026-05&account=Alice", testToken)
	if status != http.StatusOK {
		t.Fatalf("status %d: %s", status, body)
	}
	var rep metrics.Report
	if err := json.Unmarshal(body, &rep); err != nil {
		t.Fatal(err)
	}
	if rep.Scope == nil || rep.Scope.Kind != "account" || rep.Scope.Name != "alice" {
		t.Fatalf("scope: %+v", rep.Scope)
	}
	if rep.PRLeadTime.Count != 3 {
		t.Fatalf("alice's merged PRs: got %d, want 3", rep.PRLeadTime.Count)
	}

	_, body = get(t, srv, "/api/v1/metrics?from=2026-05&account=nobody", testToken)
	if err := json.Unmarshal(body, &rep); err != nil || rep.PRLeadTime.Count != 0 {
		t.Fatalf("unknown account covers nothing: %s", body)
	}
	for _, q := range []string{"account=alice&member=x", "account=%20"} {
		if status, _ := get(t, srv, "/api/v1/metrics?from=2026-05&"+q, testToken); status != http.StatusBadRequest {
			t.Fatalf("%s: status %d, want 400", q, status)
		}
	}
}
