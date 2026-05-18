package migrator

import (
	"testing"

	"github.com/mileschou/devpulse/internal/persistence/dialect"
)

func TestParseFileName(t *testing.T) {
	tests := []struct {
		name     string
		in       string
		wantTS   int64
		wantEnt  string
		wantDia  dialect.Dialect
		wantDir  Direction
		wantErr  bool
	}{
		{"generic up", "20260518000001_repos.up.sql", 20260518000001, "repos", dialect.Unknown, DirUp, false},
		{"generic down", "20260518000001_repos.down.sql", 20260518000001, "repos", dialect.Unknown, DirDown, false},
		{"pg up", "20260518000002_builds.postgres.up.sql", 20260518000002, "builds", dialect.Postgres, DirUp, false},
		{"mysql up", "20260518000002_builds.mysql.up.sql", 20260518000002, "builds", dialect.MySQL, DirUp, false},
		{"sqlite down", "20260518000003_pull_requests.sqlite.down.sql", 20260518000003, "pull_requests", dialect.SQLite, DirDown, false},
		{"multi underscore entity", "20260518000004_pull_request_reviews.up.sql", 20260518000004, "pull_request_reviews", dialect.Unknown, DirUp, false},

		{"missing direction", "20260518_repos.sql", 0, "", 0, 0, true},
		{"bad ts", "abc_repos.up.sql", 0, "", 0, 0, true},
		{"unknown dialect", "20260518_repos.oracle.up.sql", 0, "", 0, 0, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseFileName(tt.in)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected error")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got.Timestamp != tt.wantTS {
				t.Fatalf("ts %d want %d", got.Timestamp, tt.wantTS)
			}
			if got.Entity != tt.wantEnt {
				t.Fatalf("entity %q want %q", got.Entity, tt.wantEnt)
			}
			if got.Dialect != tt.wantDia {
				t.Fatalf("dialect %v want %v", got.Dialect, tt.wantDia)
			}
			if got.Direction != tt.wantDir {
				t.Fatalf("dir %v want %v", got.Direction, tt.wantDir)
			}
		})
	}
}

