// Package fetching orchestrates CI and VCS fetch + enrich flows.
//
// It owns the application-level orchestration plus the interfaces it needs
// from adapters and persistence (CIProvider, VCSProvider, BuildWriter,
// PullRequestWriter, ReviewWriter). Interface definitions live here, with
// the consumer, per Go convention.
package fetching
