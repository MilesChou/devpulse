package persistence

import (
	"context"
	"crypto/rand"
	"database/sql"
	"log/slog"
	"time"

	"github.com/oklog/ulid/v2"

	"github.com/mileschou/devpulse/internal/persistence/dialect"
)

// Persister bundles the *sql.DB plus dialect for downstream entity
// persisters. Constructors should embed *Persister rather than re-import
// these dependencies.
type Persister struct {
	DB      *sql.DB
	Dialect dialect.Dialect
	Logger  *slog.Logger
	Now     func() time.Time
}

// New builds a Persister from an open Connection.
func New(conn *Connection, logger *slog.Logger) *Persister {
	if logger == nil {
		logger = slog.Default()
	}
	return &Persister{
		DB:      conn.DB,
		Dialect: conn.Dialect,
		Logger:  logger,
		Now:     func() time.Time { return time.Now().UTC() },
	}
}

// Rebind rewrites `?` placeholders for the persister's dialect.
func (p *Persister) Rebind(query string) string {
	return p.Dialect.Rebind(query)
}

// NewID returns a new ULID string suitable for CHAR(26) primary keys.
// Time source comes from p.Now so tests can stub it.
func (p *Persister) NewID() string {
	t := p.Now()
	entropy := ulid.Monotonic(rand.Reader, 0)
	return ulid.MustNew(ulid.Timestamp(t), entropy).String()
}

// ExecCtx wraps DB.ExecContext with placeholder rebinding.
func (p *Persister) ExecCtx(ctx context.Context, q string, args ...any) (sql.Result, error) {
	return p.DB.ExecContext(ctx, p.Rebind(q), args...)
}

// QueryCtx wraps DB.QueryContext with placeholder rebinding.
func (p *Persister) QueryCtx(ctx context.Context, q string, args ...any) (*sql.Rows, error) {
	return p.DB.QueryContext(ctx, p.Rebind(q), args...)
}

// QueryRowCtx wraps DB.QueryRowContext with placeholder rebinding.
func (p *Persister) QueryRowCtx(ctx context.Context, q string, args ...any) *sql.Row {
	return p.DB.QueryRowContext(ctx, p.Rebind(q), args...)
}

