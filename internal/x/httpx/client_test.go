package httpx

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"
)

func TestNew_SetsUserAgent(t *testing.T) {
	var seen atomic.Value
	srv := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		seen.Store(r.Header.Get("User-Agent"))
	}))
	defer srv.Close()

	c := New(Config{UserAgent: "devpulse-test/1.0"})
	req, _ := http.NewRequestWithContext(context.Background(), http.MethodGet, srv.URL, nil)
	resp, err := c.Do(req)
	if err != nil {
		t.Fatalf("do: %v", err)
	}
	resp.Body.Close()

	if got := seen.Load().(string); got != "devpulse-test/1.0" {
		t.Fatalf("UA got %q want devpulse-test/1.0", got)
	}
}

func TestNew_RetriesOn500(t *testing.T) {
	var attempts atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		n := attempts.Add(1)
		if n < 3 {
			http.Error(w, "boom", http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	c := New(Config{
		RetryMax:     3,
		RetryWaitMin: 5 * time.Millisecond,
		RetryWaitMax: 20 * time.Millisecond,
		Timeout:      5 * time.Second,
	})
	req, _ := http.NewRequestWithContext(context.Background(), http.MethodGet, srv.URL, nil)
	resp, err := c.Do(req)
	if err != nil {
		t.Fatalf("do: %v", err)
	}
	resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("final status %d", resp.StatusCode)
	}
	if got := attempts.Load(); got != 3 {
		t.Fatalf("attempts got %d want 3", got)
	}
}
