package migrator

import (
	"fmt"
	"io/fs"
	"sort"
	"strings"

	"github.com/mileschou/devpulse/internal/persistence/dialect"
)

// Step is one logical migration after dialect selection. Each (timestamp,
// direction) pair gets one Step — the resolver picks the dialect-specific
// file if it exists, falling back to the generic version.
type Step struct {
	Timestamp int64
	Entity    string
	Direction Direction
	Path      string // path within the source FS
	SQL       string
}

// Plan returns the ordered list of Steps for a given direction.
//
// Selection rules per (timestamp, entity, direction):
//  1. If a file with the target dialect exists, use it.
//  2. Else, use the generic file (no dialect segment).
//  3. Else, the migration is missing — return an error.
//
// Up plans are sorted ascending by timestamp; Down plans descending.
func Plan(srcFS fs.FS, d dialect.Dialect, dir Direction) ([]Step, error) {
	entries, err := fs.ReadDir(srcFS, ".")
	if err != nil {
		return nil, fmt.Errorf("migrator: read source: %w", err)
	}

	// Group all files by (timestamp, entity, direction).
	type key struct {
		ts     int64
		entity string
		dir    Direction
	}
	groups := map[key][]fileName{}

	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		base := e.Name()
		if !strings.HasSuffix(base, ".sql") {
			continue
		}
		fn, err := parseFileName(base)
		if err != nil {
			return nil, err
		}
		if fn.Direction != dir {
			continue
		}
		k := key{ts: fn.Timestamp, entity: fn.Entity, dir: fn.Direction}
		groups[k] = append(groups[k], fn)
	}

	steps := make([]Step, 0, len(groups))
	for k, files := range groups {
		picked, ok := pickForDialect(files, d)
		if !ok {
			return nil, fmt.Errorf("migrator: no migration for ts=%d entity=%s dir=%s dialect=%s", k.ts, k.entity, k.dir, d)
		}

		body, err := fs.ReadFile(srcFS, picked.Raw)
		if err != nil {
			return nil, fmt.Errorf("migrator: read %s: %w", picked.Raw, err)
		}

		steps = append(steps, Step{
			Timestamp: picked.Timestamp,
			Entity:    picked.Entity,
			Direction: picked.Direction,
			Path:      picked.Raw,
			SQL:       string(body),
		})
	}

	sort.Slice(steps, func(i, j int) bool {
		if dir == DirDown {
			return steps[i].Timestamp > steps[j].Timestamp
		}
		return steps[i].Timestamp < steps[j].Timestamp
	})

	return steps, nil
}

// pickForDialect returns the dialect-specific entry if present, else the
// generic entry (dialect.Unknown).
func pickForDialect(files []fileName, d dialect.Dialect) (fileName, bool) {
	var generic *fileName
	for i := range files {
		f := files[i]
		if f.Dialect == d {
			return f, true
		}
		if f.Dialect == dialect.Unknown {
			generic = &files[i]
		}
	}
	if generic != nil {
		return *generic, true
	}
	return fileName{}, false
}

