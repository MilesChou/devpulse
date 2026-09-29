package statx

import (
	"math"
	"testing"
)

func TestSummarize(t *testing.T) {
	got := Summarize([]float64{4, 1, 3, 2})
	if got.Count != 4 || got.Avg != 2.5 {
		t.Fatalf("count/avg: %+v", got)
	}
	if got.P50 != 2.5 {
		t.Fatalf("p50: %v", got.P50)
	}
	if math.Abs(got.P90-3.7) > 1e-9 {
		t.Fatalf("p90: %v", got.P90)
	}
}

func TestSummarize_Empty(t *testing.T) {
	if got := Summarize(nil); got != (Summary{}) {
		t.Fatalf("want zero summary, got %+v", got)
	}
}

func TestSummarize_DoesNotMutateInput(t *testing.T) {
	in := []float64{3, 1, 2}
	Summarize(in)
	if in[0] != 3 || in[1] != 1 || in[2] != 2 {
		t.Fatalf("input mutated: %v", in)
	}
}
