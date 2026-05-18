package fetching

// RepoFetchOutcome is the per-repo result of orchestrator.Fetch.
// Error is non-nil only on transport/persistence errors that aborted the
// run before completion; per-PR enrichment failures are logged, not folded
// here, so a partial month still records what succeeded.
type RepoFetchOutcome struct {
	RepoFullName        string
	BuildsWritten       int
	PullRequestsWritten int
	Error               error
}

