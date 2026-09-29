package pullrequest

import "time"

// Status is the lifecycle state of a PR. Maps to pull_requests.status.
type Status int

const (
	StatusUnknown Status = iota
	StatusOpen
	StatusMerged
	StatusClosed
)

func (s Status) String() string {
	switch s {
	case StatusOpen:
		return "open"
	case StatusMerged:
		return "merged"
	case StatusClosed:
		return "closed"
	default:
		return "unknown"
	}
}

// ParseStatus is the inverse of Status.String. Unknown inputs map to
// StatusUnknown so a stored row with a stale enum value is recoverable
// rather than fatal.
func ParseStatus(s string) Status {
	switch s {
	case "open":
		return StatusOpen
	case "merged":
		return StatusMerged
	case "closed":
		return StatusClosed
	default:
		return StatusUnknown
	}
}

// PullRequest mirrors pull_requests. Enrichment-derived fields
// (FirstReviewAt, FirstApprovedAt, TimeToApproval, TimeToMerge) may be
// zero/nil until enrichment runs.
type PullRequest struct {
	ID                string
	RepoID            string
	Number            int
	Author            string
	Status            Status
	Additions         int
	Deletions         int
	TotalChangedLines int
	SizeBucket        string
	IsDraft           bool

	// DORA facts. Title / Labels / HeadRef drive remediation
	// classification (revert, hotfix); BaseRef decides whether a merge
	// is a deployment; FirstCommitAt is the lead-time start and is only
	// fetched once the PR is merged. RevertsNumber is the PR this one
	// reverts, parsed from GitHub's "Reverts owner/repo#N" body.
	Title          string
	Labels         []string
	BaseRef        string
	HeadRef        string
	MergeCommitSHA string
	FirstCommitAt  *time.Time
	RevertsNumber  *int

	CreatedAt       time.Time
	ReadyAt         *time.Time
	FirstReviewAt   *time.Time
	FirstApprovedAt *time.Time
	TimeToApproval  *int // seconds
	TimeToMerge     *int // seconds
	MergedAt        *time.Time
	ClosedAt        *time.Time

	// SourceUpdatedAt is the upstream updated_at of the fetched version.
	// It is not persisted; the sync compares it with the listing that
	// asked for the refresh, to reject a stale (cached) response.
	SourceUpdatedAt time.Time
}

// Stamp identifies one upstream version of a PR: its number and the
// upstream updated_at of that version.
type Stamp struct {
	Number    int
	UpdatedAt time.Time
}

func (p PullRequest) ChangeStats() ChangeStats {
	return ChangeStats{Additions: p.Additions, Deletions: p.Deletions}
}
