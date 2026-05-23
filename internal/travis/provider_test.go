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

func TestListAllBuilds_ParsesAndSkipsUnparseable(t *testing.T) {
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

	builds, err := c.ListAllBuilds(context.Background(), "MilesChou/devpulse")
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
