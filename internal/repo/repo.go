package repo

// Repo is the application-side representation of a tracked repository.
// ID is the ULID stored in repos.id. Name is the canonical "owner/name".
//
// Metadata fields mirror the GitHub REST /repos response and are filled
// by the VCS provider; they remain at their zero value until the first
// successful fetch.
type Repo struct {
	ID            string
	Name          FullName
	Description   *string // nullable
	DefaultBranch string
	Disabled      bool
}
