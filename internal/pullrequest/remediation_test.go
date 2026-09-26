package pullrequest

import "testing"

func TestIsRevertTitle(t *testing.T) {
	cases := []struct {
		title string
		want  bool
	}{
		{`Revert "feat: add x"`, true},
		{"revert: drop y", true},
		{"  REVERT the thing", true},
		{"revert(api): drop y", true},
		{"revert!: drop y", true},
		{"Revert", true},
		{"Revert-safe migration helper", false},
		{"revert/cleanup: tidy", false},
		{"Reverting is hard", false},
		{`Revert "Revert "feat: x""`, false},
		{`Revert "Revert "Revert "feat: x"""`, true},
		{`Revert "revert: drop y"`, false},
		{`Revert "Revert-safe migration helper"`, true},
		{"Reverted migration test", false},
		{"feat: support revert detection", false},
		{"", false},
	}
	for _, c := range cases {
		if got := IsRevertTitle(c.title); got != c.want {
			t.Errorf("IsRevertTitle(%q) = %v, want %v", c.title, got, c.want)
		}
	}
}

func TestParseRevertedNumber(t *testing.T) {
	cases := []struct {
		name string
		body string
		want int // 0 = nil
	}{
		{"github revert button", "Reverts MilesChou/devpulse#42", 42},
		{"case-insensitive repo", "reverts mileschou/DevPulse#7\n\nbecause", 7},
		{"bare number", "This reverts #13.", 13},
		{"cross repo ignored", "Reverts other/repo#42", 0},
		{"no reference", "Fixes #42", 0},
		{"empty", "", 0},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := ParseRevertedNumber(c.body, "MilesChou/devpulse")
			switch {
			case c.want == 0 && got != nil:
				t.Fatalf("want nil, got %d", *got)
			case c.want != 0 && (got == nil || *got != c.want):
				t.Fatalf("want %d, got %v", c.want, got)
			}
		})
	}
}

func TestIsHotfix(t *testing.T) {
	cases := []struct {
		name    string
		labels  []string
		headRef string
		label   string
		want    bool
	}{
		{"label match", []string{"bug", "hotfix"}, "fix/x", "hotfix", true},
		{"label case-insensitive", []string{"Urgent-Fix"}, "x", "urgent-fix", true},
		{"branch prefix", nil, "hotfix/login", "hotfix", true},
		{"branch prefix upper", nil, "HotFix/login", "hotfix", true},
		{"neither", []string{"bug"}, "fix/hotfix", "hotfix", false},
		{"empty label never matches", []string{""}, "x", "", false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := IsHotfix(c.labels, c.headRef, c.label); got != c.want {
				t.Fatalf("got %v, want %v", got, c.want)
			}
		})
	}
}
