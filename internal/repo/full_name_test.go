package repo

import (
	"errors"
	"testing"
)

func TestParseFullName(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		wantOwner string
		wantName  string
		wantErr   error
	}{
		{"normal", "MilesChou/devpulse", "MilesChou", "devpulse", nil},
		{"with-dash", "ory/hydra-client-go", "ory", "hydra-client-go", nil},
		{"empty", "", "", "", ErrEmptyFullName},
		{"whitespace only", "   ", "", "", ErrEmptyFullName},
		{"no slash", "foo", "", "", ErrMalformedFullName},
		{"too many slashes", "a/b/c", "", "", ErrMalformedFullName},
		{"empty owner", "/bar", "", "", ErrMalformedFullName},
		{"empty name", "foo/", "", "", ErrMalformedFullName},
		{"space inside", "foo bar/baz", "", "", ErrMalformedFullName},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseFullName(tt.input)

			if tt.wantErr != nil {
				if !errors.Is(err, tt.wantErr) {
					t.Fatalf("expected error %v, got %v", tt.wantErr, err)
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got.Owner != tt.wantOwner || got.Name != tt.wantName {
				t.Fatalf("got %+v, want owner=%q name=%q", got, tt.wantOwner, tt.wantName)
			}
		})
	}
}

func TestFullName_String_RoundTrip(t *testing.T) {
	in := "MilesChou/devpulse"
	got, err := ParseFullName(in)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if got.String() != in {
		t.Fatalf("round-trip failed: got %q want %q", got.String(), in)
	}
}

