package pullrequest

import "time"

// ReviewState is the GitHub review verdict.
type ReviewState int

const (
	ReviewStateUnknown ReviewState = iota
	ReviewStateCommented
	ReviewStateApproved
	ReviewStateChangesRequested
	ReviewStateDismissed
)

func (s ReviewState) String() string {
	switch s {
	case ReviewStateCommented:
		return "commented"
	case ReviewStateApproved:
		return "approved"
	case ReviewStateChangesRequested:
		return "changes_requested"
	case ReviewStateDismissed:
		return "dismissed"
	default:
		return "unknown"
	}
}

// Review is a single PR review submission.
type Review struct {
	ReviewerAccount string
	State           ReviewState
	SubmittedAt     time.Time
}

