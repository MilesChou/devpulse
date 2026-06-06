package pullrequest_test

import (
	"testing"

	"github.com/mileschou/devpulse/internal/pullrequest"
)

func TestSizeBucket(t *testing.T) {
	tests := []struct {
		lines int
		want  string
	}{
		{0, "XS"},
		{49, "XS"},
		{50, "S"},
		{199, "S"},
		{200, "M"},
		{499, "M"},
		{500, "L"},
		{999, "L"},
		{1000, "XL"},
	}

	for _, tt := range tests {
		got := pullrequest.SizeBucket(tt.lines)
		if got != tt.want {
			t.Errorf("SizeBucket(%d) = %q, want %q", tt.lines, got, tt.want)
		}
	}
}
