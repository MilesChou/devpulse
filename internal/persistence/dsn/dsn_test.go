package dsn

import (
	"strings"
	"testing"

	"github.com/mileschou/devpulse/internal/persistence/dialect"
)

func TestIsMemory(t *testing.T) {
	memory := []string{
		"memory",
		":memory:",
		"sqlite://:memory:",
		"sqlite://:memory:?_fk=true",
		"sqlite://file::memory:?cache=shared",
		"sqlite://file:any.db?mode=memory",
		"sqlite://file:any.db?cache=shared&mode=memory",
	}
	for _, m := range memory {
		t.Run(m, func(t *testing.T) {
			if !IsMemory(m) {
				t.Fatalf("expected memory, got false")
			}
		})
	}

	notMemory := []string{
		"sqlite://./devpulse.db",
		"postgres://localhost/foo",
		"mysql://localhost/foo",
		"",
		"memorial",
	}
	for _, n := range notMemory {
		t.Run("not:"+n, func(t *testing.T) {
			if IsMemory(n) {
				t.Fatalf("expected not memory, got true")
			}
		})
	}
}

func TestParse_Memory(t *testing.T) {
	p, err := Parse("memory")
	if err != nil {
		t.Fatalf("unexpected: %v", err)
	}
	if p.Dialect != dialect.SQLite {
		t.Fatalf("dialect %v", p.Dialect)
	}
	if p.DriverName != "sqlite" {
		t.Fatalf("driver %q", p.DriverName)
	}
	if p.DataSourceName != ":memory:" {
		t.Fatalf("dsn %q", p.DataSourceName)
	}
	if !p.IsMemory {
		t.Fatalf("expected IsMemory=true")
	}
}

func TestParse_MemoryURL_PreservesQuery(t *testing.T) {
	p, err := Parse("sqlite://:memory:?_fk=true")
	if err != nil {
		t.Fatalf("unexpected: %v", err)
	}
	if p.DataSourceName != ":memory:?_fk=true" {
		t.Fatalf("dsn %q", p.DataSourceName)
	}
	if !p.IsMemory {
		t.Fatalf("expected IsMemory")
	}
}

func TestParse_Postgres(t *testing.T) {
	in := "postgres://devpulse:secret@localhost:5432/devpulse?sslmode=disable"
	p, err := Parse(in)
	if err != nil {
		t.Fatalf("unexpected: %v", err)
	}
	if p.Dialect != dialect.Postgres {
		t.Fatalf("dialect %v", p.Dialect)
	}
	if p.DriverName != "pgx" {
		t.Fatalf("driver %q", p.DriverName)
	}
	if p.DataSourceName != in {
		t.Fatalf("dsn round-trip: %q", p.DataSourceName)
	}
	if p.IsMemory {
		t.Fatalf("should not be memory")
	}
}

func TestParse_PostgresAlias(t *testing.T) {
	p, err := Parse("postgresql://localhost/foo")
	if err != nil {
		t.Fatalf("unexpected: %v", err)
	}
	if p.Dialect != dialect.Postgres {
		t.Fatalf("dialect %v", p.Dialect)
	}
}

func TestParse_MySQL(t *testing.T) {
	p, err := Parse("mysql://devpulse:devpulse@localhost:3306/devpulse?parseTime=true")
	if err != nil {
		t.Fatalf("unexpected: %v", err)
	}
	if p.Dialect != dialect.MySQL {
		t.Fatalf("dialect %v", p.Dialect)
	}
	if p.DriverName != "mysql" {
		t.Fatalf("driver %q", p.DriverName)
	}
	want := "devpulse:devpulse@tcp(localhost:3306)/devpulse?parseTime=true"
	if p.DataSourceName != want {
		t.Fatalf("dsn got %q want %q", p.DataSourceName, want)
	}
}

func TestParse_MySQL_NoPasswordNoQuery(t *testing.T) {
	p, err := Parse("mysql://devpulse@localhost/devpulse")
	if err != nil {
		t.Fatalf("unexpected: %v", err)
	}
	want := "devpulse@tcp(localhost)/devpulse"
	if p.DataSourceName != want {
		t.Fatalf("dsn got %q want %q", p.DataSourceName, want)
	}
}

func TestParse_SQLiteFile(t *testing.T) {
	p, err := Parse("sqlite://./devpulse.db?_fk=true")
	if err != nil {
		t.Fatalf("unexpected: %v", err)
	}
	if p.Dialect != dialect.SQLite {
		t.Fatalf("dialect %v", p.Dialect)
	}
	if !strings.Contains(p.DataSourceName, "devpulse.db") {
		t.Fatalf("dsn lost path: %q", p.DataSourceName)
	}
	if !strings.Contains(p.DataSourceName, "_fk=true") {
		t.Fatalf("dsn lost query: %q", p.DataSourceName)
	}
	if p.IsMemory {
		t.Fatalf("file is not memory")
	}
}

func TestParse_Empty(t *testing.T) {
	if _, err := Parse(""); err != ErrEmpty {
		t.Fatalf("expected ErrEmpty, got %v", err)
	}
	if _, err := Parse("   "); err != ErrEmpty {
		t.Fatalf("expected ErrEmpty for whitespace, got %v", err)
	}
}

func TestParse_Unsupported(t *testing.T) {
	_, err := Parse("oracle://localhost/foo")
	if err == nil {
		t.Fatalf("expected error")
	}
}

