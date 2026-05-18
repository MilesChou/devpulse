package repo

// Repo is the application-side representation of a tracked repository.
// ID is the ULID stored in repos.id. Name is the canonical "owner/name".
type Repo struct {
	ID   string
	Name FullName
}
