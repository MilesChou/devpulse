// Package migrations embeds the DevPulse migration SQL files into the
// binary. Files follow the hydra-style naming:
//
//	<timestamp>_<entity>.up.sql                          generic
//	<timestamp>_<entity>.down.sql
//	<timestamp>_<entity>.<dialect>.up.sql                dialect-specific overrides
//	<timestamp>_<entity>.<dialect>.down.sql
//
// internal/persistence/migrator picks the dialect-specific file when
// available, falling back to the generic version.
package migrations

import "embed"

//go:embed *.sql
var FS embed.FS
