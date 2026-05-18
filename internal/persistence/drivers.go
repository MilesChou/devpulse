package persistence

// Side-effect imports register the three supported database/sql drivers
// under the names dsn.Parse emits ("pgx", "mysql", "sqlite"). Keep these
// in a dedicated file so swapping a driver is a single-file change and
// the dependency surface is obvious in `go mod why` output.

import (
	_ "github.com/go-sql-driver/mysql"
	_ "github.com/jackc/pgx/v5/stdlib"
	_ "modernc.org/sqlite"
)

