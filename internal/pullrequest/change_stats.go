package pullrequest

import "math"

// ChangeStats is the size of a PR in additions/deletions.
// PR size distribution (CLAUDE.md) is computed off of this.
type ChangeStats struct {
	Additions int
	Deletions int
}

func (c ChangeStats) Total() int { return c.Additions + c.Deletions }

// sizeBuckets is the single source of truth for the PR size taxonomy:
// each label paired with its exclusive upper bound on total changed
// lines, in ascending order. The last bucket is open-topped. Both
// SizeBucket (classification) and SizeBuckets (display ordering) derive
// from this, so adding a tier can't drift from how it's rendered.
var sizeBuckets = []struct {
	Label string
	Max   int
}{
	{"XS", 50},
	{"S", 200},
	{"M", 500},
	{"L", 1000},
	{"XL", math.MaxInt},
}

// SizeBuckets returns the bucket labels in ascending size order, so
// consumers rendering a distribution don't re-encode the taxonomy.
func SizeBuckets() []string {
	labels := make([]string, len(sizeBuckets))
	for i, b := range sizeBuckets {
		labels[i] = b.Label
	}
	return labels
}

// SizeBucket classifies a PR by its total changed lines.
func SizeBucket(totalChangedLines int) string {
	for _, b := range sizeBuckets {
		if totalChangedLines < b.Max {
			return b.Label
		}
	}
	return sizeBuckets[len(sizeBuckets)-1].Label
}
