package httpcache

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"time"
)

// Transport is a caching http.RoundTripper that sits between the
// caller and a wrapped base transport. Successful (2xx) responses
// are stored on disk; subsequent requests for the same resource
// are served from cache when within TTL, or revalidated via
// conditional requests (ETag / If-Modified-Since) when stale.
//
// A TTL of zero means entries never expire — they are always
// served from cache when present. This is useful for replaying
// previously fetched data (e.g. rebuilding a DB from cached API
// responses without making any network calls).
type Transport struct {
	// Base is the underlying transport for cache-miss requests.
	// Defaults to http.DefaultTransport when nil.
	Base http.RoundTripper

	// Store is the disk-backed cache store.
	Store *DiskStore

	// TTL controls the default freshness window for cached entries.
	// Zero means entries never expire (always fresh).
	TTL time.Duration

	// TTLFunc, if non-nil, returns the TTL for a specific request,
	// overriding the global TTL. Use it to give stable resources
	// (e.g. a closed PR's detail, commit author lookups) a longer —
	// or infinite (0) — lifetime than volatile ones (e.g. latest PR
	// number). When nil, every request uses TTL.
	TTLFunc func(req *http.Request) time.Duration

	// Logger receives cache hit/miss diagnostics. Nil disables logging.
	Logger *slog.Logger
}

// NewTransport returns a caching transport backed by store. The
// Base transport defaults to http.DefaultTransport; callers
// typically override it when composing with other transports.
func NewTransport(store *DiskStore, ttl time.Duration, logger *slog.Logger) *Transport {
	return &Transport{
		Base:   http.DefaultTransport,
		Store:  store,
		TTL:    ttl,
		Logger: logger,
	}
}

func (t *Transport) base() http.RoundTripper {
	if t.Base != nil {
		return t.Base
	}
	return http.DefaultTransport
}

// RoundTrip implements http.RoundTripper with the two-layer cache
// strategy:
//
//  1. Fresh cache hit (within TTL) → return from disk, zero network.
//  2. Stale hit → conditional request with ETag / If-Modified-Since.
//     304 refreshes the timestamp; 2xx replaces the entry.
//  3. Cache miss → normal request; 2xx responses are cached.
func (t *Transport) RoundTrip(req *http.Request) (*http.Response, error) {
	key, reqBody, err := cacheKey(req)
	if err != nil {
		return t.base().RoundTrip(req)
	}

	entry, body, getErr := t.Store.Get(key)

	ttl := t.ttlFor(req)

	// Fresh cache hit: return directly without network I/O.
	if getErr == nil && isFresh(entry, ttl) {
		t.log(req.Context(), slog.LevelDebug, "cache hit (fresh)", req, key)
		return toResponse(req, entry, body), nil
	}

	// Stale entry available: clone the request and attach
	// conditional headers so a 304 saves bandwidth and rate limit.
	if getErr == nil {
		req = req.Clone(req.Context())
		if etag := entry.Header.Get("ETag"); etag != "" {
			req.Header.Set("If-None-Match", etag)
		}
		if !entry.LastModified.IsZero() {
			req.Header.Set("If-Modified-Since", entry.LastModified.Format(http.TimeFormat))
		}
	}

	// Restore body once — on whichever req (original or clone)
	// will actually be sent. Fresh-hit returns above, so this
	// only runs when a network call is needed.
	if reqBody != nil {
		restoreBody(req, reqBody)
	}

	resp, netErr := t.base().RoundTrip(req)
	if netErr != nil {
		// Network error with stale cache: serve stale as fallback.
		if getErr == nil {
			t.log(req.Context(), slog.LevelWarn, "cache hit (stale, network error)", req, key)
			return toResponse(req, entry, body), nil
		}
		return nil, netErr
	}

	// 304 Not Modified: refresh cache timestamp and return cached body.
	if resp.StatusCode == http.StatusNotModified && getErr == nil {
		resp.Body.Close()
		entry.CachedAt = time.Now()
		if putErr := t.Store.PutMeta(key, entry); putErr != nil {
			t.log(req.Context(), slog.LevelWarn, "cache put failed on 304", req, key)
		}
		t.log(req.Context(), slog.LevelDebug, "cache hit (revalidated)", req, key)
		return toResponse(req, entry, body), nil
	}

	// Only cache successful (2xx) responses.
	if resp.StatusCode/100 == 2 {
		respBody, readErr := io.ReadAll(resp.Body)
		resp.Body.Close()
		if readErr != nil {
			return nil, readErr
		}

		newEntry := &Entry{
			StatusCode: resp.StatusCode,
			Header:     resp.Header.Clone(),
			CachedAt:   time.Now(),
		}
		if lm := resp.Header.Get("Last-Modified"); lm != "" {
			if parsed, parseErr := http.ParseTime(lm); parseErr == nil {
				newEntry.LastModified = parsed
			}
		}
		if putErr := t.Store.Put(key, newEntry, respBody); putErr != nil {
			t.log(req.Context(), slog.LevelWarn, "cache put failed", req, key)
		}
		t.log(req.Context(), slog.LevelDebug, "cached", req, key)

		resp.Body = io.NopCloser(bytes.NewReader(respBody))
		resp.ContentLength = int64(len(respBody))
		return resp, nil
	}

	return resp, nil
}

// ttlFor returns the TTL to apply for this request. TTLFunc takes
// precedence; if nil, the global TTL is used.
func (t *Transport) ttlFor(req *http.Request) time.Duration {
	if t.TTLFunc != nil {
		return t.TTLFunc(req)
	}
	return t.TTL
}

// isFresh returns true when the entry should be served directly.
// A zero TTL means "never expire".
func isFresh(e *Entry, ttl time.Duration) bool {
	if ttl == 0 {
		return true
	}
	return time.Since(e.CachedAt) < ttl
}

// cacheKey computes a hex-encoded SHA-256 from the request method,
// URL, and — for requests with a body (e.g. GraphQL POST) — the
// request body. Returns the key string and, when the body was
// consumed, its bytes so the caller can restore req.Body.
func cacheKey(req *http.Request) (string, []byte, error) {
	h := sha256.New()
	fmt.Fprintf(h, "%s %s", req.Method, req.URL.String())

	var body []byte
	if req.Body != nil && req.Body != http.NoBody {
		var err error
		body, err = io.ReadAll(req.Body)
		if err != nil {
			return "", nil, err
		}
		req.Body.Close()
		h.Write([]byte("\n"))
		h.Write(body)
	}

	return hex.EncodeToString(h.Sum(nil)), body, nil
}

// restoreBody resets req.Body (and GetBody) so the request can be
// re-sent after its body was consumed for cache-key hashing.
func restoreBody(req *http.Request, body []byte) {
	req.Body = io.NopCloser(bytes.NewReader(body))
	req.GetBody = func() (io.ReadCloser, error) {
		return io.NopCloser(bytes.NewReader(body)), nil
	}
}

// toResponse constructs an *http.Response from a cache entry.
func toResponse(req *http.Request, e *Entry, body []byte) *http.Response {
	return &http.Response{
		StatusCode:    e.StatusCode,
		Status:        fmt.Sprintf("%d %s", e.StatusCode, http.StatusText(e.StatusCode)),
		Proto:         "HTTP/1.1",
		ProtoMajor:    1,
		ProtoMinor:    1,
		Header:        e.Header.Clone(),
		Body:          io.NopCloser(bytes.NewReader(body)),
		ContentLength: int64(len(body)),
		Request:       req,
	}
}

func (t *Transport) log(ctx context.Context, level slog.Level, msg string, req *http.Request, key string) {
	if t.Logger == nil {
		return
	}
	t.Logger.Log(ctx, level, msg,
		slog.String("method", req.Method),
		slog.String("url", req.URL.String()),
		slog.String("cache_key", key[:12]),
	)
}
