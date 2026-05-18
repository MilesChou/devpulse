// Package dialect labels the SQL flavor for query rewriting and migration
// selection. Three dialects are supported in v1: postgres, mysql, sqlite.
package dialect

import (
	"fmt"
	"strings"
)

type Dialect int

const (
	Unknown Dialect = iota
	Postgres
	MySQL
	SQLite
)

func (d Dialect) String() string {
	switch d {
	case Postgres:
		return "postgres"
	case MySQL:
		return "mysql"
	case SQLite:
		return "sqlite"
	default:
		return "unknown"
	}
}

// Parse accepts the common driver and short-name spellings.
func Parse(s string) (Dialect, error) {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "postgres", "postgresql", "pgx":
		return Postgres, nil
	case "mysql":
		return MySQL, nil
	case "sqlite", "sqlite3":
		return SQLite, nil
	default:
		return Unknown, fmt.Errorf("dialect: unknown %q", s)
	}
}

// Placeholder returns the parameter placeholder for the 1-based index `n`.
//
//	pg:     $1, $2, ...
//	mysql:  ?,  ?,  ... (n ignored)
//	sqlite: ?,  ?,  ... (n ignored)
func (d Dialect) Placeholder(n int) string {
	if d == Postgres {
		return fmt.Sprintf("$%d", n)
	}
	return "?"
}

// Rebind walks a `?`-placeholder query and rewrites it for the dialect.
// MySQL/SQLite return the input unchanged; Postgres swaps each `?` with $N.
func (d Dialect) Rebind(query string) string {
	if d != Postgres {
		return query
	}

	var b strings.Builder
	b.Grow(len(query) + 8)
	n := 1
	for i := 0; i < len(query); i++ {
		if query[i] == '?' {
			b.WriteString(fmt.Sprintf("$%d", n))
			n++
			continue
		}
		b.WriteByte(query[i])
	}
	return b.String()
}
