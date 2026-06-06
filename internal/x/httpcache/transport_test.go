package httpcache_test

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/mileschou/devpulse/internal/x/httpcache"
)

// roundTripFunc adapts a function to http.RoundTripper.
type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) { return f(req) }

func TestTransport_CacheHitFresh(t *testing.T) {
	dir := t.TempDir()
	store := httpcache.NewDiskStore(dir)
	calls := 0

	upstream := roundTripFunc(func(req *http.Request) (*http.Response, error) {
		calls++
		rec := httptest.NewRecorder()
		rec.Header().Set("ETag", `"v1"`)
		rec.WriteHeader(http.StatusOK)
		rec.WriteString(`{"ok":true}`)
		return rec.Result(), nil
	})

	tr := &httpcache.Transport{
		Base:  upstream,
		Store: store,
		TTL:   time.Hour,
	}

	req := must(http.NewRequest("GET", "https://api.example.com/test", nil))

	// First request: cache miss, hits upstream.
	resp1 := mustDo(t, tr, req)
	assertBody(t, resp1, `{"ok":true}`)
	if calls != 1 {
		t.Fatalf("expected 1 upstream call, got %d", calls)
	}

	// Second request: cache hit, no upstream call.
	resp2 := mustDo(t, tr, req)
	assertBody(t, resp2, `{"ok":true}`)
	if calls != 1 {
		t.Fatalf("expected still 1 upstream call after cache hit, got %d", calls)
	}
	if resp2.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp2.StatusCode)
	}
}

func TestTransport_ConditionalRequest304(t *testing.T) {
	dir := t.TempDir()
	store := httpcache.NewDiskStore(dir)
	calls := 0

	upstream := roundTripFunc(func(req *http.Request) (*http.Response, error) {
		calls++
		rec := httptest.NewRecorder()

		if req.Header.Get("If-None-Match") == `"v1"` {
			rec.WriteHeader(http.StatusNotModified)
			return rec.Result(), nil
		}

		rec.Header().Set("ETag", `"v1"`)
		rec.WriteHeader(http.StatusOK)
		rec.WriteString(`{"ok":true}`)
		return rec.Result(), nil
	})

	tr := &httpcache.Transport{
		Base:  upstream,
		Store: store,
		TTL:   time.Nanosecond, // immediately stale
	}

	req := must(http.NewRequest("GET", "https://api.example.com/cond", nil))

	// First request: populate cache.
	mustDo(t, tr, req)
	if calls != 1 {
		t.Fatalf("expected 1 call, got %d", calls)
	}

	// Wait for TTL to expire.
	time.Sleep(2 * time.Millisecond)

	// Second request: stale → conditional request → 304.
	resp := mustDo(t, tr, req)
	assertBody(t, resp, `{"ok":true}`)
	if calls != 2 {
		t.Fatalf("expected 2 calls (conditional revalidation), got %d", calls)
	}
}

func TestTransport_ZeroTTLNeverExpires(t *testing.T) {
	dir := t.TempDir()
	store := httpcache.NewDiskStore(dir)
	calls := 0

	upstream := roundTripFunc(func(req *http.Request) (*http.Response, error) {
		calls++
		rec := httptest.NewRecorder()
		rec.WriteHeader(http.StatusOK)
		rec.WriteString(`{"v":1}`)
		return rec.Result(), nil
	})

	tr := &httpcache.Transport{
		Base:  upstream,
		Store: store,
		TTL:   0, // never expires
	}

	req := must(http.NewRequest("GET", "https://api.example.com/zero", nil))

	mustDo(t, tr, req)
	if calls != 1 {
		t.Fatalf("expected 1, got %d", calls)
	}

	// Even after "time passes", TTL=0 means always fresh.
	mustDo(t, tr, req)
	if calls != 1 {
		t.Fatalf("expected still 1 (TTL=0 never expires), got %d", calls)
	}
}

func TestTransport_POSTBodyInCacheKey(t *testing.T) {
	dir := t.TempDir()
	store := httpcache.NewDiskStore(dir)
	calls := 0

	upstream := roundTripFunc(func(req *http.Request) (*http.Response, error) {
		calls++
		body, _ := io.ReadAll(req.Body)
		rec := httptest.NewRecorder()
		rec.WriteHeader(http.StatusOK)
		rec.WriteString(string(body))
		return rec.Result(), nil
	})

	tr := &httpcache.Transport{
		Base:  upstream,
		Store: store,
		TTL:   time.Hour,
	}

	url := "https://api.example.com/graphql"

	// Two POST requests with different bodies → different cache keys.
	req1 := must(http.NewRequest("POST", url, strings.NewReader(`{"query":"a"}`)))
	req2 := must(http.NewRequest("POST", url, strings.NewReader(`{"query":"b"}`)))

	resp1 := mustDo(t, tr, req1)
	assertBody(t, resp1, `{"query":"a"}`)

	resp2 := mustDo(t, tr, req2)
	assertBody(t, resp2, `{"query":"b"}`)

	if calls != 2 {
		t.Fatalf("expected 2 upstream calls for different POST bodies, got %d", calls)
	}

	// Repeat req1 → should hit cache.
	req1again := must(http.NewRequest("POST", url, strings.NewReader(`{"query":"a"}`)))
	resp3 := mustDo(t, tr, req1again)
	assertBody(t, resp3, `{"query":"a"}`)
	if calls != 2 {
		t.Fatalf("expected still 2 (req1 cached), got %d", calls)
	}
}

func TestTransport_NonSuccessNotCached(t *testing.T) {
	dir := t.TempDir()
	store := httpcache.NewDiskStore(dir)
	calls := 0

	upstream := roundTripFunc(func(req *http.Request) (*http.Response, error) {
		calls++
		rec := httptest.NewRecorder()
		rec.WriteHeader(http.StatusNotFound)
		rec.WriteString(`not found`)
		return rec.Result(), nil
	})

	tr := &httpcache.Transport{
		Base:  upstream,
		Store: store,
		TTL:   time.Hour,
	}

	req := must(http.NewRequest("GET", "https://api.example.com/missing", nil))

	resp := mustDo(t, tr, req)
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", resp.StatusCode)
	}

	// Second call should NOT be cached — 404 is not a 2xx.
	mustDo(t, tr, req)
	if calls != 2 {
		t.Fatalf("expected 2 upstream calls (404 not cached), got %d", calls)
	}
}

func TestTransport_TTLFuncOverride(t *testing.T) {
	dir := t.TempDir()
	store := httpcache.NewDiskStore(dir)
	calls := 0

	upstream := roundTripFunc(func(req *http.Request) (*http.Response, error) {
		calls++
		rec := httptest.NewRecorder()
		rec.WriteHeader(http.StatusOK)
		rec.WriteString(`ok`)
		return rec.Result(), nil
	})

	tr := &httpcache.Transport{
		Base:  upstream,
		Store: store,
		TTL:   time.Nanosecond, // global: expire immediately
		TTLFunc: func(req *http.Request) time.Duration {
			if strings.Contains(req.URL.Path, "/stable") {
				return 0 // never expire
			}
			return time.Nanosecond // expire immediately
		},
	}

	stable := must(http.NewRequest("GET", "https://api.example.com/stable", nil))
	volatile := must(http.NewRequest("GET", "https://api.example.com/volatile", nil))

	// Populate both caches.
	mustDo(t, tr, stable)
	mustDo(t, tr, volatile)
	if calls != 2 {
		t.Fatalf("expected 2, got %d", calls)
	}

	time.Sleep(2 * time.Millisecond)

	// /stable → TTLFunc returns 0 (never expire) → still cached.
	mustDo(t, tr, stable)
	if calls != 2 {
		t.Fatalf("expected still 2 (stable cached forever), got %d", calls)
	}

	// /volatile → TTLFunc returns nanosecond → stale → upstream call.
	mustDo(t, tr, volatile)
	if calls != 3 {
		t.Fatalf("expected 3 (volatile expired), got %d", calls)
	}
}

// --- helpers ---

func must[T any](v T, err error) T {
	if err != nil {
		panic(err)
	}
	return v
}

func mustDo(t *testing.T, tr http.RoundTripper, req *http.Request) *http.Response {
	t.Helper()
	// Clone the request to avoid mutation across calls.
	clone := req.Clone(req.Context())
	if req.Body != nil {
		body, _ := io.ReadAll(req.Body)
		req.Body = io.NopCloser(strings.NewReader(string(body)))
		clone.Body = io.NopCloser(strings.NewReader(string(body)))
	}
	resp, err := tr.RoundTrip(clone)
	if err != nil {
		t.Fatalf("RoundTrip failed: %v", err)
	}
	return resp
}

func assertBody(t *testing.T, resp *http.Response, want string) {
	t.Helper()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}
	resp.Body.Close()
	if got := string(body); got != want {
		t.Fatalf("body = %q, want %q", got, want)
	}
}
