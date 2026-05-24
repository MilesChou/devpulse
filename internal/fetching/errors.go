package fetching

import "errors"

// ErrNotFound signals that an upstream resource (PR, repo, commit, ...)
// does not exist. VCS providers MUST wrap (not return bare) so callers
// can classify via errors.Is while retaining the original error context
// in logs.
//
// The PR backfill loop uses this to distinguish "this number is an
// issue / deleted" (skip silently) from real transport or 5xx failures
// (fail-fast). Other classifications can be added as needed; keep them
// in this file so the contract stays in one place.
var ErrNotFound = errors.New("fetching: upstream resource not found")
