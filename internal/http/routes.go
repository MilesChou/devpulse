package http

import "net/http"

// registerRoutes attaches handlers to the mux.
//
// v1 stays empty. v2 wires real endpoints once we know what the dashboard
// or external API surface needs. Keep this file as the single hook so it
// is obvious where to add a route.
func registerRoutes(mux *http.ServeMux) {
	_ = mux // placeholder
}
