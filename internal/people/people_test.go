package people

import (
	"errors"
	"slices"
	"testing"
)

func TestNormalizeAccount(t *testing.T) {
	tests := map[string]string{
		"MilesChou":           "mileschou",
		" dependabot[bot]":    "dependabot",
		"dependabot":          "dependabot",
		"GitHub-Actions[BOT]": "github-actions",
		"":                    "",
	}
	for in, want := range tests {
		if got := NormalizeAccount(in); got != want {
			t.Errorf("NormalizeAccount(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestNormalizeAccounts(t *testing.T) {
	got, err := NormalizeAccounts([]string{"Bob", "alice", "bob", " ", "renovate[bot]", "ACME_emu"})
	if err != nil {
		t.Fatalf("normalize: %v", err)
	}
	want := []string{"acme_emu", "alice", "bob", "renovate"}
	if !slices.Equal(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}

	for _, bad := range []string{"has space", "semi;colon", "-leading", "a/b"} {
		if _, err := NormalizeAccounts([]string{bad}); !errors.Is(err, ErrInvalid) {
			t.Errorf("%q: want ErrInvalid, got %v", bad, err)
		}
	}
}

func TestNormalizeName(t *testing.T) {
	if got, err := NormalizeName("display name", "  Alice Chen "); err != nil || got != "Alice Chen" {
		t.Fatalf("got %q, %v", got, err)
	}
	if _, err := NormalizeName("display name", "  "); !errors.Is(err, ErrInvalid) {
		t.Fatalf("blank: want ErrInvalid, got %v", err)
	}
}
