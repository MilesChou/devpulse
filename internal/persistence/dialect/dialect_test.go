package dialect

import "testing"

func TestParse(t *testing.T) {
	tests := []struct {
		in   string
		want Dialect
		ok   bool
	}{
		{"postgres", Postgres, true},
		{"postgresql", Postgres, true},
		{"PGX", Postgres, true},
		{"mysql", MySQL, true},
		{"sqlite", SQLite, true},
		{"sqlite3", SQLite, true},
		{"oracle", Unknown, false},
		{"", Unknown, false},
	}
	for _, tt := range tests {
		t.Run(tt.in, func(t *testing.T) {
			got, err := Parse(tt.in)
			if tt.ok && err != nil {
				t.Fatalf("unexpected err: %v", err)
			}
			if !tt.ok && err == nil {
				t.Fatalf("expected err, got dialect=%v", got)
			}
			if got != tt.want {
				t.Fatalf("got %v want %v", got, tt.want)
			}
		})
	}
}

func TestPlaceholder(t *testing.T) {
	if got := Postgres.Placeholder(3); got != "$3" {
		t.Fatalf("pg got %q", got)
	}
	if got := MySQL.Placeholder(3); got != "?" {
		t.Fatalf("mysql got %q", got)
	}
	if got := SQLite.Placeholder(3); got != "?" {
		t.Fatalf("sqlite got %q", got)
	}
}

func TestRebind(t *testing.T) {
	in := "SELECT * FROM t WHERE a = ? AND b = ? AND c IN (?, ?)"
	wantPG := "SELECT * FROM t WHERE a = $1 AND b = $2 AND c IN ($3, $4)"

	if got := Postgres.Rebind(in); got != wantPG {
		t.Fatalf("pg got %q want %q", got, wantPG)
	}
	if got := MySQL.Rebind(in); got != in {
		t.Fatalf("mysql got %q want unchanged", got)
	}
	if got := SQLite.Rebind(in); got != in {
		t.Fatalf("sqlite got %q want unchanged", got)
	}
}
