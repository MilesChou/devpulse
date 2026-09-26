// Package statx holds the small descriptive-statistics helpers shared by
// the metrics code paths.
package statx

import (
	"math"
	"sort"
)

// Summary is the avg / p50 / p90 of a sample, in the sample's unit.
type Summary struct {
	Count int
	Avg   float64
	P50   float64
	P90   float64
}

// Summarize sorts a copy of values and returns its Summary. An empty
// sample yields the zero Summary.
func Summarize(values []float64) Summary {
	if len(values) == 0 {
		return Summary{}
	}
	sorted := append([]float64(nil), values...)
	sort.Float64s(sorted)

	var sum float64
	for _, v := range sorted {
		sum += v
	}
	return Summary{
		Count: len(sorted),
		Avg:   sum / float64(len(sorted)),
		P50:   Percentile(sorted, 0.5),
		P90:   Percentile(sorted, 0.9),
	}
}

// Percentile returns the p-th percentile (0..1) of an ascending-sorted
// sample using linear interpolation between the closest ranks.
func Percentile(sorted []float64, p float64) float64 {
	n := len(sorted)
	if n == 0 {
		return 0
	}
	if n == 1 {
		return sorted[0]
	}
	idx := p * float64(n-1)
	lower := int(math.Floor(idx))
	upper := int(math.Ceil(idx))
	if lower == upper {
		return sorted[lower]
	}
	frac := idx - float64(lower)
	return sorted[lower]*(1-frac) + sorted[upper]*frac
}
