package http

import (
	"encoding/json"
	"net/http"
	"slices"
	"testing"

	"github.com/mileschou/devpulse/internal/metrics"
)

func decode[T any](t *testing.T, body []byte) T {
	t.Helper()
	var v T
	if err := json.Unmarshal(body, &v); err != nil {
		t.Fatalf("decode %s: %v", body, err)
	}
	return v
}

func TestAPI_Members(t *testing.T) {
	srv := newTestServer(t)

	status, body := do(t, srv, "POST", "/api/v1/members", testToken,
		`{"display_name":" Alice ","accounts":["Alice","alice-work","alice"]}`)
	if status != http.StatusCreated {
		t.Fatalf("create: status %d: %s", status, body)
	}
	alice := decode[memberJSON](t, body)
	if alice.DisplayName != "Alice" || !slices.Equal(alice.Accounts, []string{"alice", "alice-work"}) {
		t.Fatalf("created member: %+v", alice)
	}

	for _, tc := range []struct {
		body   string
		status int
	}{
		{`{"display_name":"Alice","accounts":[]}`, http.StatusConflict},        // name taken
		{`{"display_name":"Other","accounts":["ALICE"]}`, http.StatusConflict}, // account taken
		{`{"display_name":"  ","accounts":[]}`, http.StatusBadRequest},
		{`{"display_name":"X","accounts":["not valid"]}`, http.StatusBadRequest},
		{`{"name":"X"}`, http.StatusBadRequest},
	} {
		if status, body := do(t, srv, "POST", "/api/v1/members", testToken, tc.body); status != tc.status {
			t.Errorf("create %s: status %d, want %d (%s)", tc.body, status, tc.status, body)
		}
	}

	status, body = do(t, srv, "PUT", "/api/v1/members/"+alice.ID, testToken,
		`{"display_name":"Alice Chen","accounts":["alice"]}`)
	if status != http.StatusOK {
		t.Fatalf("update: status %d: %s", status, body)
	}
	if got := decode[memberJSON](t, body); got.DisplayName != "Alice Chen" || !slices.Equal(got.Accounts, []string{"alice"}) {
		t.Fatalf("updated member: %+v", got)
	}
	if status, _ := do(t, srv, "PUT", "/api/v1/members/nope", testToken, `{"display_name":"X","accounts":[]}`); status != http.StatusNotFound {
		t.Fatalf("update unknown: status %d, want 404", status)
	}

	_, body = get(t, srv, "/api/v1/members", testToken)
	list := decode[struct {
		Members []memberJSON `json:"members"`
	}](t, body)
	if len(list.Members) != 1 || list.Members[0].ID != alice.ID {
		t.Fatalf("list: %+v", list)
	}

	if status, _ := do(t, srv, "DELETE", "/api/v1/members/"+alice.ID, testToken, ""); status != http.StatusNoContent {
		t.Fatalf("delete: status %d", status)
	}
	if status, _ := do(t, srv, "DELETE", "/api/v1/members/"+alice.ID, testToken, ""); status != http.StatusNotFound {
		t.Fatalf("delete twice: status %d, want 404", status)
	}
}

func TestAPI_TeamsAndExcluded(t *testing.T) {
	srv := newTestServer(t)
	_, body := do(t, srv, "POST", "/api/v1/members", testToken, `{"display_name":"Alice","accounts":["alice"]}`)
	alice := decode[memberJSON](t, body)

	status, body := do(t, srv, "POST", "/api/v1/teams", testToken, `{"name":"Web","member_ids":["`+alice.ID+`"]}`)
	if status != http.StatusCreated {
		t.Fatalf("create team: status %d: %s", status, body)
	}
	team := decode[teamJSON](t, body)
	if status, body := do(t, srv, "POST", "/api/v1/teams", testToken, `{"name":"Ghosts","member_ids":["nope"]}`); status != http.StatusBadRequest {
		t.Fatalf("team with unknown member: status %d (%s)", status, body)
	}
	// Updating a member keeps reporting its teams.
	status, body = do(t, srv, "PUT", "/api/v1/members/"+alice.ID, testToken, `{"display_name":"Alice Chen","accounts":["alice"]}`)
	if got := decode[memberJSON](t, body); status != http.StatusOK || !slices.Equal(got.TeamIDs, []string{team.ID}) {
		t.Fatalf("update member in a team: status %d: %s", status, body)
	}

	status, body = do(t, srv, "PUT", "/api/v1/teams/"+team.ID, testToken, `{"name":"Frontend","member_ids":[]}`)
	if got := decode[teamJSON](t, body); status != http.StatusOK || got.Name != "Frontend" || len(got.MemberIDs) != 0 {
		t.Fatalf("update team: status %d: %s", status, body)
	}
	if status, _ := do(t, srv, "DELETE", "/api/v1/teams/"+team.ID, testToken, ""); status != http.StatusNoContent {
		t.Fatalf("delete team: status %d", status)
	}

	_, body = get(t, srv, "/api/v1/excluded-accounts", testToken)
	ex := decode[map[string][]string](t, body)
	if !slices.Equal(ex["accounts"], []string{"copilot-pull-request-reviewer", "dependabot", "github-actions"}) {
		t.Fatalf("default excluded: %v", ex)
	}
	status, body = do(t, srv, "PUT", "/api/v1/excluded-accounts", testToken, `{"accounts":["Renovate[bot]","dependabot"]}`)
	if ex := decode[map[string][]string](t, body); status != http.StatusOK || !slices.Equal(ex["accounts"], []string{"dependabot", "renovate"}) {
		t.Fatalf("replace excluded: status %d: %s", status, body)
	}
}

func TestAPI_ScopedMetrics(t *testing.T) {
	srv := newTestServer(t)
	path := "/api/v1/repos/MilesChou/devpulse/metrics?from=2026-05"

	// All seeded PRs are alice's, so her member scope sees all of them.
	_, body := do(t, srv, "POST", "/api/v1/members", testToken, `{"display_name":"Alice","accounts":["alice"]}`)
	alice := decode[memberJSON](t, body)
	_, body = do(t, srv, "POST", "/api/v1/members", testToken, `{"display_name":"Carol","accounts":["carol"]}`)
	carol := decode[memberJSON](t, body)
	_, body = do(t, srv, "POST", "/api/v1/teams", testToken, `{"name":"Web","member_ids":["`+alice.ID+`","`+carol.ID+`"]}`)
	team := decode[teamJSON](t, body)

	status, body := get(t, srv, path+"&member="+alice.ID, testToken)
	if status != http.StatusOK {
		t.Fatalf("member scope: status %d: %s", status, body)
	}
	r := decode[metrics.Report](t, body)
	if r.Scope == nil || r.Scope.Kind != "member" || r.Scope.Name != "Alice" || r.PRLeadTime.Count != 3 || r.DORA != nil {
		t.Fatalf("alice report: scope=%+v lead=%+v dora=%v", r.Scope, r.PRLeadTime, r.DORA)
	}

	_, body = get(t, srv, path+"&member="+carol.ID, testToken)
	if r := decode[metrics.Report](t, body); r.PRLeadTime.Count != 0 || r.BuildFailure.Total != 0 {
		t.Fatalf("carol has no work: %+v %+v", r.PRLeadTime, r.BuildFailure)
	}

	_, body = get(t, srv, path+"&team="+team.ID, testToken)
	if r := decode[metrics.Report](t, body); r.Scope.Kind != "team" || !slices.Equal(r.Scope.Accounts, []string{"alice", "carol"}) || r.PRLeadTime.Count != 3 {
		t.Fatalf("team report: %+v %+v", r.Scope, r.PRLeadTime)
	}

	monthly := "/api/v1/repos/MilesChou/devpulse/metrics/monthly?from=2026-04&to=2026-06&member=" + alice.ID
	if status, body := get(t, srv, monthly, testToken); status != http.StatusOK {
		t.Fatalf("scoped monthly: status %d: %s", status, body)
	}

	for p, want := range map[string]int{
		path + "&member=nope":                             http.StatusNotFound,
		path + "&team=nope":                               http.StatusNotFound,
		path + "&member=" + alice.ID + "&team=" + team.ID: http.StatusBadRequest,
	} {
		if status, body := get(t, srv, p, testToken); status != want {
			t.Errorf("GET %s: status %d, want %d (%s)", p, status, want, body)
		}
	}
}

func TestAPI_MetricsByMember(t *testing.T) {
	srv := newTestServer(t)
	path := "/api/v1/repos/MilesChou/devpulse/metrics/by-member?from=2026-05"
	type resp struct {
		Rows []metrics.Row `json:"rows"`
	}

	// Nobody mapped yet: alice shows up as an unmapped account.
	status, body := get(t, srv, path, testToken)
	if status != http.StatusOK {
		t.Fatalf("by-member: status %d: %s", status, body)
	}
	rows := decode[resp](t, body).Rows
	if len(rows) != 1 || rows[0].MemberID != nil || rows[0].Name != "alice" || rows[0].Report.PRLeadTime.Count != 3 {
		t.Fatalf("unmapped rows: %+v", rows)
	}

	// Once mapped, the row is the member; an idle member gets no row.
	_, body = do(t, srv, "POST", "/api/v1/members", testToken, `{"display_name":"Alice","accounts":["alice"]}`)
	alice := decode[memberJSON](t, body)
	do(t, srv, "POST", "/api/v1/members", testToken, `{"display_name":"Idle","accounts":["idle"]}`)

	_, body = get(t, srv, path, testToken)
	rows = decode[resp](t, body).Rows
	if len(rows) != 1 || rows[0].MemberID == nil || *rows[0].MemberID != alice.ID || rows[0].Name != "Alice" {
		t.Fatalf("mapped rows: %+v", rows)
	}
	if rows[0].Report.BuildFailure.Total != 3 || rows[0].Report.DORA != nil {
		t.Fatalf("alice row report: %+v dora=%v", rows[0].Report.BuildFailure, rows[0].Report.DORA)
	}
}
