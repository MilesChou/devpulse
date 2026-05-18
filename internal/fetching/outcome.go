package fetching

// BuildsFetchOutcome is the per-repo result of orchestrator.FetchBuilds.
// Error is non-nil only on a transport / persistence failure that aborted
// the run before completion; per-row author-backfill failures are logged
// rather than folded in so a partial run still records the builds it
// could fetch.
type BuildsFetchOutcome struct {
	RepoFullName  string
	BuildsWritten int
	Error         error
}

// PullRequestsFetchOutcome is the per-repo result of
// orchestrator.FetchPullRequests. Same error semantics as
// BuildsFetchOutcome: per-PR enrichment failures are logged, not folded.
type PullRequestsFetchOutcome struct {
	RepoFullName        string
	PullRequestsWritten int
	Error               error
}
