package metrics

import "github.com/mileschou/devpulse/internal/repo"

// AllReposName is Report.Repo for a report across all repos.
const AllReposName = "*"

// Target is what a report covers: one repo, or several repos whose
// rows are pooled so averages and percentiles are exact across them.
type Target struct {
	Repos []repo.Repo
	all   bool
}

// Single targets one repo; its report is the per-repo report, DORA
// included.
func Single(rp repo.Repo) Target {
	return Target{Repos: []repo.Repo{rp}}
}

// AllRepos targets every repo given. DORA is not computed: a deployment
// is a merge into one repo's default branch, and pooling repos with
// different release practices gives a number nobody can act on.
func AllRepos(repos []repo.Repo) Target {
	return Target{Repos: repos, all: true}
}

// Name is the report's repo label: "owner/name", or AllReposName.
func (t Target) Name() string {
	if r, ok := t.single(); ok {
		return r.Name.String()
	}
	return AllReposName
}

func (t Target) ids() []string {
	ids := make([]string, len(t.Repos))
	for i, r := range t.Repos {
		ids[i] = r.ID
	}
	return ids
}

// single returns the repo of a single-repo target.
func (t Target) single() (repo.Repo, bool) {
	if t.all || len(t.Repos) != 1 {
		return repo.Repo{}, false
	}
	return t.Repos[0], true
}
