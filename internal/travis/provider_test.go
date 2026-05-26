package travis_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
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

// TestListBuildsSince_StopsAtWatermark asserts the incremental path:
// once a page contains a build whose started_at is at-or-before the
// watermark, the client stops paging — even if the page is full. The
// boundary page is still returned in full so callers can rely on
// DB-side dedupe to absorb the overlap.
func TestListBuildsSince_StopsAtWatermark(t *testing.T) {
	// Two-build inline page: the second build sits exactly at the
	// watermark, so paging must stop after this page even though the
	// real Travis API would normally serve more.
	page := `{
	  "builds": [
	    {"id": 9100, "number": "1240", "state": "passed", "event_type": "push", "pull_request_number": 0,
	     "duration": 100, "started_at": "2026-05-20T10:00:00Z", "finished_at": "2026-05-20T10:01:40Z",
	     "commit": {"sha": "aaa1234567890abcdef1234567890abcdef12345"}, "branch": {"name": "main"},
	     "repository": {"slug": "MilesChou/devpulse"}},
	    {"id": 9099, "number": "1239", "state": "passed", "event_type": "push", "pull_request_number": 0,
	     "duration": 100, "started_at": "2026-05-15T10:00:00Z", "finished_at": "2026-05-15T10:01:40Z",
	     "commit": {"sha": "bbb1234567890abcdef1234567890abcdef12345"}, "branch": {"name": "main"},
	     "repository": {"slug": "MilesChou/devpulse"}}
	  ]
	}`

	var hits int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits++
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(page))
	}))
	defer srv.Close()

	c := travis.NewClient(travis.Config{BaseURL: srv.URL, Token: "t", Timeout: 5 * time.Second})

	// Watermark sits between the two builds; the second build is == watermark,
	// which counts as reached (the boundary is inclusive).
	since := time.Date(2026, 5, 15, 10, 0, 0, 0, time.UTC)
	builds, err := c.ListBuildsSince(context.Background(), "MilesChou/devpulse", since)
	if err != nil {
		t.Fatalf("list: %v", err)
	}

	if hits != 1 {
		t.Fatalf("expected exactly 1 page hit (stop on watermark), got %d", hits)
	}
	if len(builds) != 2 {
		t.Fatalf("expected boundary page returned in full (2 builds), got %d", len(builds))
	}
	// Caller is responsible for dedupe; provider does not filter.
	if builds[1].ExternalID != "9099" {
		t.Fatalf("boundary build ExternalID: %q", builds[1].ExternalID)
	}
}

func TestListBuildsSince_ColdStart_ParsesAndSkipsUnparseable(t *testing.T) {
	page1 := loadFixture(t, "builds_page1.json")

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
		// Page 1 has 3 entries; less than defaultLimit (100) so pagination
		// terminates after a single fetch.
		_, _ = w.Write(page1)
	}))
	defer srv.Close()

	c := travis.NewClient(travis.Config{BaseURL: srv.URL, Token: "test-token", Timeout: 5 * time.Second})

	// Zero `since` exercises the cold-start branch — no watermark, walk
	// until short page / empty page.
	builds, err := c.ListBuildsSince(context.Background(), "MilesChou/devpulse", time.Time{})
	if err != nil {
		t.Fatalf("list: %v", err)
	}

	// id 9001 (push to main) and id 9000 (PR) should parse. id 8500 has
	// no started_at and is silently dropped.
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
