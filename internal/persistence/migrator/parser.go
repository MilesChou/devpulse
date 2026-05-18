package migrator

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/mileschou/devpulse/internal/persistence/dialect"
)

// fileName is the parsed structure of a migration file name in the
// hydra-style layout:
//
//	<timestamp>_<entity>.up.sql                          generic up
//	<timestamp>_<entity>.down.sql                        generic down
//	<timestamp>_<entity>.<dialect>.up.sql                dialect-specific up
//	<timestamp>_<entity>.<dialect>.down.sql              dialect-specific down
//
// Example: 20260518000001_repos.up.sql, 20260518000001_repos.postgres.up.sql.
type fileName struct {
	Timestamp int64
	Entity    string
	Dialect   dialect.Dialect // dialect.Unknown means "generic"
	Direction Direction
	Raw       string
}

// Direction is up or down.
type Direction int

const (
	DirUnknown Direction = iota
	DirUp
	DirDown
)

func (d Direction) String() string {
	switch d {
	case DirUp:
		return "up"
	case DirDown:
		return "down"
	default:
		return "unknown"
	}
}

// filePattern: 12345_entity[.dialect].up.sql
// Captures: 1=timestamp, 2=entity, 3=dialect (optional, e.g. ".postgres"), 4=direction.
var filePattern = regexp.MustCompile(
	`^(\d+)_([a-zA-Z0-9_]+?)(\.[a-zA-Z0-9]+)?\.(up|down)\.sql$`,
)

// parseFileName extracts metadata from the file's base name.
func parseFileName(base string) (fileName, error) {
	m := filePattern.FindStringSubmatch(base)
	if m == nil {
		return fileName{}, fmt.Errorf("migrator: unrecognized file name %q", base)
	}

	ts, err := strconv.ParseInt(m[1], 10, 64)
	if err != nil {
		return fileName{}, fmt.Errorf("migrator: bad timestamp in %q: %w", base, err)
	}

	d := dialect.Unknown
	if m[3] != "" {
		parsed, err := dialect.Parse(strings.TrimPrefix(m[3], "."))
		if err != nil {
			// Treat unrecognized middle segment as part of entity name —
			// e.g. "20260518_pull_request_reviews.up.sql" must still parse.
			return fileName{}, fmt.Errorf("migrator: %s in %q", err, base)
		}
		d = parsed
	}

	dir := DirUnknown
	switch m[4] {
	case "up":
		dir = DirUp
	case "down":
		dir = DirDown
	}

	return fileName{
		Timestamp: ts,
		Entity:    m[2],
		Dialect:   d,
		Direction: dir,
		Raw:       base,
	}, nil
}

