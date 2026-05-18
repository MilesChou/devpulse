package commitsha

import (
	"errors"
	"testing"
)

func TestParse(t *testing.T) {
	tests := []struct {
		name    string
		in      string
		want    string
		wantErr error
	}{
		{"happy lowercase", "abc1234567890abcdef1234567890abcdef12345", "abc1234567890abcdef1234567890abcdef12345", nil},
		{"normalizes to lowercase", "ABC1234567890ABCDEF1234567890ABCDEF12345", "abc1234567890abcdef1234567890abcdef12345", nil},
		{"empty", "", "", ErrEmpty},
		{"short", "abc123", "", ErrBadLen},
		{"non-hex", "ZZZ1234567890abcdef1234567890abcdef12345", "", ErrBadHex},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Parse(tt.in)

			if tt.wantErr != nil {
				if !errors.Is(err, tt.wantErr) {
					t.Fatalf("expected %v, got %v", tt.wantErr, err)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if string(got) != tt.want {
				t.Fatalf("got %q want %q", got, tt.want)
			}
		})
	}
}

