package persistence

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/mileschou/devpulse/internal/persistence/dialect"
	"github.com/mileschou/devpulse/internal/persistence/dsn"
)

// Connection bundles a *sql.DB with the metadata callers need to write
// dialect-aware queries against it.
type Connection struct {
	DB       *sql.DB
	Dialect  dialect.Dialect
	IsMemory bool
}

// Open parses the DSN, registers the right driver under the hood, and
// returns a ready-to-use Connection. The caller is responsible for
// calling DB.Close().
func Open(ctx context.Context, raw string) (*Connection, error) {
	p, err := dsn.Parse(raw)
	if err != nil {
		return nil, err
	}

	db, err := sql.Open(p.DriverName, p.DataSourceName)
	if err != nil {
		return nil, fmt.Errorf("persistence: open %s: %w", p.DriverName, err)
	}

	// Reasonable defaults; callers can tune on the returned *sql.DB.
	db.SetConnMaxLifetime(30 * time.Minute)
	db.SetConnMaxIdleTime(5 * time.Minute)
	db.SetMaxOpenConns(25)
	db.SetMaxIdleConns(5)

	pingCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	if err := db.PingContext(pingCtx); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("persistence: ping: %w", err)
	}

	return &Connection{
		DB:       db,
		Dialect:  p.Dialect,
		IsMemory: p.IsMemory,
	}, nil
}
