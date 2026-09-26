package repo

import "time"

// Repo is the application-side representation of a tracked repository.
// ID is the ULID stored in repos.id. Name is the canonical "owner/name".
//
// Metadata fields mirror the GitHub REST /repos response and are filled
// by the VCS provider; they remain at their zero value until the first
// successful fetch.
//
// IncidentLabel and HotfixLabel are operator settings for DORA: the
// issue label that marks an incident and the PR label that marks a
// hotfix. Both are matched case-insensitively; see DefaultIncidentLabel
// and DefaultHotfixLabel.
//
// PRUpdatedWatermark is sync state, not an operator setting: the
// upstream updated_at of the most recently updated PR seen by the last
// fully successful PR refresh. The next refresh re-syncs every stored
// PR updated at or after it. Nil until the first refresh completes.
//
// PRSyncStartNumber is the floor for PR sync: the orchestrator backfills
// PRs starting from this number and stops nothing else from being
// fetched. Default 1 (full history). Bump it to skip early history that
// predates CI adoption; values below 1 are nonsensical.
type Repo struct {
	ID                string
	Name              FullName
	Provider          string  // VCS host: "github", "gitlab", etc.
	Description       *string // nullable
	DefaultBranch     string
	Disabled          bool
	PRSyncStartNumber int
	IncidentLabel     string
	HotfixLabel       string

	PRUpdatedWatermark *time.Time
}

// Defaults for the DORA label settings. They mirror the DB column
// defaults on repos.incident_label / repos.hotfix_label.
const (
	DefaultIncidentLabel = "incident"
	DefaultHotfixLabel   = "hotfix"
)
