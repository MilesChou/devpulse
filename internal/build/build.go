package build

import (
	"time"

	"github.com/mileschou/devpulse/internal/x/commitsha"
)

// Build is one CI run for a commit. Author may be empty until enrichment
// resolves the GitHub login for CommitSHA (Travis payload does not carry it).
type Build struct {
	ID         string // local ULID (PK)
	ExternalID string // provider-side ID (Travis build number / id)
	RepoID     string
	Number     int // PR number (when triggered by PR)
	CommitSHA  commitsha.SHA
	Branch     string
	Status     Status
	Trigger    Trigger
	Author     string
	StartedAt  time.Time
	FinishedAt *time.Time
}

// Started returns the wall-clock start of the build.
func (b Build) Started() time.Time { return b.StartedAt }

// DurationSeconds returns the elapsed seconds when FinishedAt is set,
// otherwise nil — semantically "still in progress / unknown."
func (b Build) DurationSeconds() any {
	if b.FinishedAt == nil {
		return nil
	}
	return int(b.FinishedAt.Sub(b.StartedAt).Seconds())
}

// Status is the CI build outcome. Unknown covers in-progress / unmapped values.
type Status int

const (
	StatusUnknown Status = iota
	StatusPassed
	StatusFailed
	StatusErrored
	StatusCanceled
)

func (s Status) String() string {
	switch s {
	case StatusPassed:
		return "passed"
	case StatusFailed:
		return "failed"
	case StatusErrored:
		return "errored"
	case StatusCanceled:
		return "canceled"
	default:
		return "unknown"
	}
}

// Trigger identifies why the CI build ran.
type Trigger int

const (
	TriggerUnknown Trigger = iota
	TriggerPush
	TriggerPullRequest
	TriggerAPI
	TriggerCron
)

func (t Trigger) String() string {
	switch t {
	case TriggerPush:
		return "push"
	case TriggerPullRequest:
		return "pull_request"
	case TriggerAPI:
		return "api"
	case TriggerCron:
		return "cron"
	default:
		return "unknown"
	}
}
