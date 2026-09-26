// Package incident holds the Incident domain type: a production problem
// recorded upstream (today a GitHub issue carrying the repo's incident
// label). Incidents feed DORA's failed-deployment recovery time.
package incident

import "time"

// SourceGitHubIssue identifies incidents mirrored from GitHub issues.
const SourceGitHubIssue = "github-issue"

// Incident mirrors one incidents row. ResolvedAt is nil while the
// incident is still open.
type Incident struct {
	ID         string
	RepoID     string
	Source     string
	Number     int
	Title      string
	OpenedAt   time.Time
	ResolvedAt *time.Time
}
