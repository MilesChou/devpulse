package pullrequest

// ChangeStats is the size of a PR in additions/deletions.
// PR size distribution (CLAUDE.md) is computed off of this.
type ChangeStats struct {
	Additions int
	Deletions int
}

func (c ChangeStats) Total() int { return c.Additions + c.Deletions }

