package travis_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"github.com/mileschou/devpulse/internal/build"
	"github.com/mileschou/devpulse/internal/travis"
)

func loadFixture(t *testing.T, name string) []byte {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("testdata", name))
	if err != nil {
		t.Fatalf("read fixture %s: %v", name, err)
	}
	return data
}

func TestListBuildsInMonth_FiltersAndSkipsUnparseable(t *testing.T) {
	page1 := loadFixture(t, "builds_page1.json")
	page2 := loadFixture(t, "builds_page2.json")

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Travis URL-encodes "/" as "%2F" — the raw path keeps the encoded form.
		if r.URL.EscapedPath() != "/repo/MilesChou%2Fdevpulse/builds" {
			t.Errorf("unexpected escaped path: %s", r.URL.EscapedPath())
		}
		if got := r.Header.Get("Authorization"); got != "token test-token" {
			t.Errorf("auth header: %q", got)
		}
		if got := r.Header.Get("Travis-Api-Version"); got != "3" {
			t.Errorf("api version: %q", got)
		}
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Query().Get("offset") {
		case "0":
			_, _ = w.Write(page1)
		case "100":
			_, _ = w.Write(page2)
		default:
			t.Errorf("unexpected offset %q", r.URL.Query().Get("offset"))
		}
	}))
	defer srv.Close()

	c := travis.NewClient(travis.Config{BaseURL: srv.URL, Token: "test-token", Timeout: 5 * time.Second})

	start := time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC)
	end := time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)
	builds, err := c.ListBuildsInMonth(context.Background(), "MilesChou/devpulse", start, end)
	if err != nil {
		t.Fatalf("list: %v", err)
	}

	// Expected: id 9001 (May 20 push to main), id 9000 (May 15 PR). The
	// canceled build with no started_at is dropped; the April build on
	// page 2 is outside the window but consecutiveBelow stays at 1 so
	// pagination terminates because page 2 is short (len < limit).
	if len(builds) != 2 {
		t.Fatalf("expected 2 builds, got %d", len(builds))
	}

	if builds[0].ExternalID != "9001" {
		t.Fatalf("first ExternalID: %q", builds[0].ExternalID)
	}
	if builds[0].Trigger != build.TriggerPush {
		t.Fatalf("trunk push trigger: %v", builds[0].Trigger)
	}
	if builds[0].Branch != "main" {
		t.Fatalf("branch: %q", builds[0].Branch)
	}

	if builds[1].ExternalID != "9000" {
		t.Fatalf("second ExternalID: %q", builds[1].ExternalID)
	}
	if builds[1].Trigger != build.TriggerPullRequest {
		t.Fatalf("PR trigger: %v", builds[1].Trigger)
	}
	if builds[1].PRNumber != 42 {
		t.Fatalf("PR number: %d", builds[1].PRNumber)
	}
	if builds[1].Status != build.StatusFailed {
		t.Fatalf("status: %v", builds[1].Status)
	}
}

func TestListBuildsInMonth_TerminatesOn50ConsecutiveBelow(t *testing.T) {
	// Build a synthetic page where every build is below the window.
	// Travis pages of size 100 → we need 50+ consecutive below entries.
	// Generate 100 builds at id 1..100, all dated April 2026.

	var body []byte
	body = append(body, []byte(`{"builds": [`)...)
	for i := 99; i >= 0; i-- {
		comma := ","
		if i == 0 {
			comma = ""
		}
		entry := `{
		    "id": ` + strconv.Itoa(i) + `,
		    "number": "n",
		    "state": "passed",
		    "event_type": "push",
		    "pull_request_number": 0,
		    "duration": 100,
		    "started_at": "2026-04-15T10:00:00Z",
		    "finished_at": "2026-04-15T10:01:40Z",
		    "commit": {"sha": "aaa1234567890abcdef1234567890abcdef12345"},
		    "branch": {"name": "main"},
		    "repository": {"slug": "MilesChou/devpulse"}
		}` + comma
		body = append(body, []byte(entry)...)
	}
	body = append(body, []byte(`]}`)...)

	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(body)
	}))
	defer srv.Close()

	c := travis.NewClient(travis.Config{BaseURL: srv.URL, Token: "t", Timeout: 5 * time.Second})
	start := time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC)
	end := time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)
	builds, err := c.ListBuildsInMonth(context.Background(), "MilesChou/devpulse", start, end)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(builds) != 0 {
		t.Fatalf("expected 0 builds inside window, got %d", len(builds))
	}
	if calls > 1 {
		t.Fatalf("expected 1 API call before cutoff fires, got %d", calls)
	}
}
