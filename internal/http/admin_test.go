package http

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/mileschou/devpulse/internal/repo"
	"github.com/mileschou/devpulse/internal/syncrun"
)

// fakeMetadata answers GitHub metadata lookups without the network.
// Repos owned by "missing" fail, like a typo'd or private repo.
type fakeMetadata struct{}

func (fakeMetadata) GetRepo(_ context.Context, name repo.FullName) (repo.Repo, error) {
	if name.Owner == "missing" {
		return repo.Repo{}, errors.New("not found")
	}
	desc := "fetched"
	return repo.Repo{Name: name, Description: &desc, DefaultBranch: "trunk"}, nil
}

// fakeSync records Start calls and can pretend to be busy.
type fakeSync struct {
	started []string
	busy    bool
}

func (f *fakeSync) Start(r repo.Repo) error {
	if f.busy {
		return syncrun.ErrBusy
	}
	f.started = append(f.started, r.Name.String())
	return nil
}

func (f *fakeSync) Status() syncrun.Status {
	if f.busy {
		return syncrun.Status{Running: "acme/other"}
	}
	if len(f.started) > 0 {
		return syncrun.Status{Running: f.started[len(f.started)-1]}
	}
	return syncrun.Status{}
}

func do(t *testing.T, srv *httptest.Server, method, path, token, body string) (int, []byte) {
	t.Helper()
	var rd io.Reader
	if body != "" {
		rd = bytes.NewBufferString(body)
	}
	req, err := http.NewRequest(method, srv.URL+path, rd)
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	resp, err := srv.Client().Do(req)
	if err != nil {
		t.Fatalf("%s %s: %v", method, path, err)
	}
	defer resp.Body.Close()
	out, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}
	return resp.StatusCode, out
}

func TestAPI_RegisterRepo(t *testing.T) {
	srv := newTestServer(t)

	status, body := do(t, srv, "POST", "/api/v1/repos", testToken, `{"full_name":"acme/web"}`)
	if status != http.StatusCreated {
		t.Fatalf("register: status %d: %s", status, body)
	}
	var reg registrationJSON
	if err := json.Unmarshal(body, &reg); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if !reg.Created || reg.MetadataError != nil || reg.Repo.FullName != "acme/web" ||
		reg.Repo.DefaultBranch != "trunk" || reg.Repo.PRStart != 1 || reg.Repo.HotfixLabel != "hotfix" {
		t.Fatalf("registration: %+v", reg)
	}

	// Registering again is idempotent and answers 200.
	if status, body := do(t, srv, "POST", "/api/v1/repos", testToken, `{"full_name":"acme/web"}`); status != http.StatusOK {
		t.Fatalf("re-register: status %d: %s", status, body)
	}

	// A metadata failure still registers the repo, and says why.
	status, body = do(t, srv, "POST", "/api/v1/repos", testToken, `{"full_name":"missing/repo"}`)
	if status != http.StatusCreated {
		t.Fatalf("register missing: status %d: %s", status, body)
	}
	reg = registrationJSON{}
	_ = json.Unmarshal(body, &reg)
	if reg.MetadataError == nil {
		t.Fatalf("want metadata_error, got %s", body)
	}

	for _, bad := range []string{`{"full_name":"not-a-slug"}`, `{"name":"acme/web"}`, `not json`, `{"full_name":"a/b"} {}`} {
		if status, body := do(t, srv, "POST", "/api/v1/repos", testToken, bad); status != http.StatusBadRequest {
			t.Errorf("body %s: status %d, want 400 (%s)", bad, status, body)
		}
	}

	if status, _ := do(t, srv, "POST", "/api/v1/repos", "", `{"full_name":"acme/web"}`); status != http.StatusUnauthorized {
		t.Fatalf("write without token: status %d, want 401", status)
	}
}

func TestAPI_UpdateRepo(t *testing.T) {
	srv := newTestServer(t)
	path := "/api/v1/repos/MilesChou/devpulse"

	status, body := do(t, srv, "PATCH", path, testToken, `{"pr_start":500,"incident_label":" sev1 "}`)
	if status != http.StatusOK {
		t.Fatalf("patch: status %d: %s", status, body)
	}
	var got repoJSON
	_ = json.Unmarshal(body, &got)
	if got.PRStart != 500 || got.IncidentLabel != "sev1" || got.HotfixLabel != "hotfix" {
		t.Fatalf("patched repo: %+v", got)
	}

	// Invalid values and unknown fields change nothing.
	for _, bad := range []string{`{"pr_start":0}`, `{"hotfix_label":"  "}`, `{"pr-start":10}`} {
		if status, body := do(t, srv, "PATCH", path, testToken, bad); status != http.StatusBadRequest {
			t.Errorf("patch %s: status %d, want 400 (%s)", bad, status, body)
		}
	}
	_, body = get(t, srv, path, testToken)
	got = repoJSON{}
	_ = json.Unmarshal(body, &got)
	if got.PRStart != 500 || got.HotfixLabel != "hotfix" {
		t.Fatalf("rejected patches changed the repo: %+v", got)
	}

	if status, _ := do(t, srv, "PATCH", "/api/v1/repos/acme/nope", testToken, `{"pr_start":2}`); status != http.StatusNotFound {
		t.Fatalf("patch unknown repo: status %d, want 404", status)
	}
}

func TestAPI_RemoveRepo(t *testing.T) {
	srv := newTestServer(t)
	path := "/api/v1/repos/MilesChou/devpulse"

	if status, body := do(t, srv, "DELETE", path, testToken, ""); status != http.StatusNoContent {
		t.Fatalf("delete: status %d: %s", status, body)
	}
	if status, _ := get(t, srv, path, testToken); status != http.StatusNotFound {
		t.Fatalf("after delete: status %d, want 404", status)
	}
	if status, _ := do(t, srv, "DELETE", path, testToken, ""); status != http.StatusNotFound {
		t.Fatalf("delete twice: status %d, want 404", status)
	}
}

func TestAPI_Sync(t *testing.T) {
	// No runner (server without GITHUB_TOKEN): 503 with a reason.
	srv := newTestServer(t)
	if status, body := do(t, srv, "POST", "/api/v1/repos/MilesChou/devpulse/sync", testToken, ""); status != http.StatusServiceUnavailable {
		t.Fatalf("sync without runner: status %d: %s", status, body)
	}
	if status, _ := get(t, srv, "/api/v1/sync", testToken); status != http.StatusServiceUnavailable {
		t.Fatalf("status without runner: %d", status)
	}

	fs := &fakeSync{}
	srv = newTestServerWith(t, func(c *Config) { c.Sync = fs })
	status, body := do(t, srv, "POST", "/api/v1/repos/MilesChou/devpulse/sync", testToken, "")
	if status != http.StatusAccepted || len(fs.started) != 1 || fs.started[0] != "MilesChou/devpulse" {
		t.Fatalf("sync: status %d, started %v: %s", status, fs.started, body)
	}
	var st syncrun.Status
	if err := json.Unmarshal(body, &st); err != nil || st.Running != "MilesChou/devpulse" {
		t.Fatalf("sync status body: %s (%v)", body, err)
	}
	if status, _ := do(t, srv, "POST", "/api/v1/repos/acme/nope/sync", testToken, ""); status != http.StatusNotFound {
		t.Fatalf("sync unknown repo: status %d, want 404", status)
	}

	fs.busy = true
	if status, body := do(t, srv, "POST", "/api/v1/repos/MilesChou/devpulse/sync", testToken, ""); status != http.StatusConflict {
		t.Fatalf("sync while busy: status %d: %s", status, body)
	}
}
