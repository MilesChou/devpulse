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
	PRNumber   int // PR number when Trigger == TriggerPullRequest; 0 otherwise
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

// Status is the CI build outcome. The failure-rate metric counts a build
// iff IsFailure reports true, so provider adapters must map any provider
// state that should count toward that metric to StatusFailed or StatusErrored.
type Status int

const (
	// StatusUnknown covers in-progress runs and unmapped provider values.
	StatusUnknown Status = iota
	StatusPassed
	// StatusFailed is a script/test failure: the build ran and its work failed.
	StatusFailed
	// StatusErrored is an infra-level failure: the build could not run to
	// completion (Travis `errored`, Actions `timed_out` / `startup_failure`).
	StatusErrored
	StatusCanceled
)

// IsFailure reports whether the build counts toward the failure-rate metric.
func (s Status) IsFailure() bool { return s == StatusFailed || s == StatusErrored }

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

// ParseStatus is the inverse of Status.String. Unknown inputs map to
// StatusUnknown so downstream code can still operate on stale rows.
func ParseStatus(s string) Status {
	switch s {
	case "passed":
		return StatusPassed
	case "failed":
		return StatusFailed
	case "errored":
		return StatusErrored
	case "canceled":
		return StatusCanceled
	default:
		return StatusUnknown
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

// ParseTrigger is the inverse of Trigger.String. Unknown inputs map to
// TriggerUnknown.
func ParseTrigger(s string) Trigger {
	switch s {
	case "push":
		return TriggerPush
	case "pull_request":
		return TriggerPullRequest
	case "api":
		return TriggerAPI
	case "cron":
		return TriggerCron
	default:
		return TriggerUnknown
	}
}
