package persistence

import (
	"context"
	"crypto/rand"
	"database/sql"
	"log/slog"
	"sync"
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

	// rebindCache memoizes Rebind output keyed on the raw `?`-placeholder
	// query. Persisters call ExecCtx/QueryCtx in tight loops with the
	// same SQL each iteration; without the cache every call allocates a
	// fresh strings.Builder for the same result.
	rebindCache sync.Map // map[string]string
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

// Rebind rewrites `?` placeholders for the persister's dialect. Results
// are memoized per raw query string so high-frequency call sites do not
// re-walk the string on every invocation.
func (p *Persister) Rebind(query string) string {
	if v, ok := p.rebindCache.Load(query); ok {
		return v.(string)
	}
	rebound := p.Dialect.Rebind(query)
	p.rebindCache.Store(query, rebound)
	return rebound
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

// rowScanner is the Scan-shaped interface common to *sql.Row and
// *sql.Rows, so per-aggregate scan helpers can serve both single-row
// lookups and iterator loops without duplication.
type rowScanner interface {
	Scan(dest ...any) error
}
