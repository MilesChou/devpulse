// Package httpcache provides a disk-backed HTTP caching transport.
// It implements http.RoundTripper and can be inserted into any HTTP
// client's transport chain to cache successful responses on disk.
//
// Two layers of caching are supported:
//
//   - Disk cache with configurable TTL: responses within the TTL
//     window are served directly from disk without any network I/O.
//   - Conditional requests (ETag / If-Modified-Since): when the TTL
//     has expired, the transport attaches validators from the cached
//     entry and revalidates with the origin; a 304 refreshes the
//     local entry without re-downloading the body.
//
// Typical use: wrap http.DefaultTransport with a Transport, then
// hand the result to an outer transport (OTel, retryablehttp, etc.):
//
//	store := httpcache.NewDiskStore("/tmp/cache")
//	cache := httpcache.NewTransport(store, 24*time.Hour, logger)
//	otel  := otelhttp.NewTransport(cache, ...)
package httpcache
