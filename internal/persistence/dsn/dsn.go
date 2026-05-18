// Package dsn parses DevPulse connection strings.
//
// The format mirrors ory/hydra:
//
//	postgres://user:pass@host:5432/db?sslmode=disable
//	postgresql://...                                   (alias for postgres)
//	mysql://user:pass@host:3306/db?parseTime=true
//	sqlite://./devpulse.db?_fk=true                    (on-disk)
//	sqlite://:memory:?_fk=true                         (in-memory)
//	memory                                             (alias for SQLite :memory:)
//	:memory:                                           (alias for SQLite :memory:)
//
// Parsed DSNs surface the dialect and a driver-specific data source name
// ready to hand to sql.Open with the matching registered driver.
package dsn

import (
	"errors"
	"fmt"
	"net/url"
	"regexp"
	"strings"

	"github.com/mileschou/devpulse/internal/persistence/dialect"
)

// Parsed is the result of Parse — a connection plan that the caller can
// hand to sql.Open without further reshuffling.
type Parsed struct {
	// Dialect is the SQL flavor (postgres/mysql/sqlite).
	Dialect dialect.Dialect

	// DriverName is the registered database/sql driver to use:
	// "pgx", "mysql", "sqlite".
	DriverName string

	// DataSourceName is the driver-specific connection string. For
	// MySQL it is in go-sql-driver/mysql DSN format. For Postgres it is
	// kept as a libpq URL (pgx accepts it). For SQLite it is the path or
	// :memory: target plus query string.
	DataSourceName string

	// IsMemory is true if the DSN resolves to in-memory SQLite.
	IsMemory bool
}

var (
	// memoryRE accepts the same family of strings ory/x/dbal does:
	// "memory", ":memory:", "sqlite://:memory:?...", "sqlite://file:...&mode=memory...".
	memoryRE = regexp.MustCompile(
		`^sqlite://file:.+\?.*&?mode=memory($|&.*)` +
			`|^sqlite://(file:)?:memory:(\?.*)?$` +
			`|^(:memory:|memory)$`,
	)

	ErrEmpty       = errors.New("dsn is empty")
	ErrUnsupported = errors.New("dsn scheme not supported")
)

// IsMemory returns true if the DSN points at in-memory SQLite, without
// fully parsing the rest of the URL.
func IsMemory(raw string) bool { return memoryRE.MatchString(raw) }

// Parse normalizes a user-supplied DSN into a connection plan.
func Parse(raw string) (Parsed, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return Parsed{}, ErrEmpty
	}

	if IsMemory(raw) {
		return Parsed{
			Dialect:        dialect.SQLite,
			DriverName:     "sqlite",
			DataSourceName: memorySQLiteDataSource(raw),
			IsMemory:       true,
		}, nil
	}

	u, err := url.Parse(raw)
	if err != nil {
		return Parsed{}, fmt.Errorf("dsn: parse url: %w", err)
	}

	switch strings.ToLower(u.Scheme) {
	case "postgres", "postgresql":
		// pgx/stdlib accepts the URL form directly.
		return Parsed{
			Dialect:        dialect.Postgres,
			DriverName:     "pgx",
			DataSourceName: raw,
		}, nil

	case "mysql":
		return Parsed{
			Dialect:        dialect.MySQL,
			DriverName:     "mysql",
			DataSourceName: mysqlURLToDSN(u),
		}, nil

	case "sqlite", "sqlite3":
		return Parsed{
			Dialect:        dialect.SQLite,
			DriverName:     "sqlite",
			DataSourceName: sqliteURLToDataSource(u),
		}, nil

	default:
		return Parsed{}, fmt.Errorf("%w: %q", ErrUnsupported, u.Scheme)
	}
}

// memorySQLiteDataSource returns a stable :memory: DSN for the SQLite
// driver. We rely on the driver's default unless the caller supplied a
// query string in which case we forward it.
func memorySQLiteDataSource(raw string) string {
	switch {
	case raw == "memory", raw == ":memory:":
		return ":memory:"
	case strings.HasPrefix(raw, "sqlite://:memory:"):
		// Forward the original query string so flags like ?_fk=true pass through.
		return ":memory:" + strings.TrimPrefix(raw, "sqlite://:memory:")
	case strings.HasPrefix(raw, "sqlite://file::memory:"):
		return strings.TrimPrefix(raw, "sqlite://")
	default:
		// sqlite://file:foo.db?mode=memory&cache=shared
		return strings.TrimPrefix(raw, "sqlite://")
	}
}

// mysqlURLToDSN converts mysql://user:pass@host:port/db?params into the
// go-sql-driver/mysql DSN: user:pass@tcp(host:port)/db?params.
func mysqlURLToDSN(u *url.URL) string {
	var b strings.Builder
	if u.User != nil {
		b.WriteString(u.User.Username())
		if pwd, ok := u.User.Password(); ok {
			b.WriteString(":")
			b.WriteString(pwd)
		}
		b.WriteString("@")
	}
	b.WriteString("tcp(")
	b.WriteString(u.Host)
	b.WriteString(")")

	if u.Path == "" {
		b.WriteString("/")
	} else {
		b.WriteString(u.Path)
	}

	if q := u.RawQuery; q != "" {
		b.WriteString("?")
		b.WriteString(q)
	}
	return b.String()
}

// sqliteURLToDataSource strips the sqlite:// prefix while preserving the
// path and query string. modernc.org/sqlite consumes the result directly.
func sqliteURLToDataSource(u *url.URL) string {
	path := u.Host + u.Path
	if u.RawQuery != "" {
		return path + "?" + u.RawQuery
	}
	return path
}

