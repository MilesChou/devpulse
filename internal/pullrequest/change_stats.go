package pullrequest

// ChangeStats is the size of a PR in additions/deletions.
// PR size distribution (CLAUDE.md) is computed off of this.
type ChangeStats struct {
	Additions int
	Deletions int
}

func (c ChangeStats) Total() int { return c.Additions + c.Deletions }

func SizeBucket(totalChangedLines int) string {
	switch {
	case totalChangedLines < 50:
		return "XS"
	case totalChangedLines < 200:
		return "S"
	case totalChangedLines < 500:
		return "M"
	case totalChangedLines < 1000:
		return "L"
	default:
		return "XL"
	}
}
