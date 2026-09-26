package pullrequest

import (
	"regexp"
	"strconv"
	"strings"
)

// revertTitle matches GitHub's revert-button title (`Revert "<title>"`),
// the conventional-commit spellings (`revert: ...`, `revert(scope): ...`,
// `revert!: ...`) and hand-written titles (`Revert the login change`).
// The word must be followed by whitespace, one of `:("!`, or the end:
// `\b` alone would also accept `Revert-safe helper` and `revert/cleanup`,
// which are ordinary changes.
var revertTitle = regexp.MustCompile(`(?i)^\s*revert(?:[\s:("!]|$)`)

// revertsRef matches GitHub's revert-button body: "Reverts owner/repo#N".
// The owner/repo prefix is optional so a hand-written "Reverts #N" also
// resolves.
var revertsRef = regexp.MustCompile(`(?i)\breverts\s+(?:([\w.-]+/[\w.-]+))?#(\d+)`)

// revertQuoted matches the quoted prefix GitHub's revert button puts in
// front of the original title: `Revert "`.
var revertQuoted = regexp.MustCompile(`(?i)^\s*revert\s+"`)

// IsRevertTitle reports whether a PR title marks a revert.
//
// Reverting a revert re-lands the original change, and GitHub's revert
// button nests the titles: `Revert "Revert "X""` puts X back. The quoted
// prefixes are peeled one at a time, and the title is a revert only
// when the nesting depth is odd. The innermost level may use any revert
// spelling, so `Revert "revert: x"` is a re-land too.
func IsRevertTitle(title string) bool {
	depth := 0
	for rest := title; revertTitle.MatchString(rest); {
		depth++
		loc := revertQuoted.FindStringIndex(rest)
		if loc == nil {
			break
		}
		rest = rest[loc[1]:]
	}
	return depth%2 == 1
}

// ParseRevertedNumber extracts the PR number a revert PR body points at.
// A reference to another repository (case-insensitive compare against
// repoFullName) is ignored: the reverted PR would not be in this repo's
// store. Returns nil when the body carries no usable reference.
func ParseRevertedNumber(body, repoFullName string) *int {
	m := revertsRef.FindStringSubmatch(body)
	if m == nil {
		return nil
	}
	if m[1] != "" && !strings.EqualFold(m[1], repoFullName) {
		return nil
	}
	n, err := strconv.Atoi(m[2])
	if err != nil || n <= 0 {
		return nil
	}
	return &n
}

// HotfixBranchPrefix marks a hotfix PR by its head branch, the other
// widespread convention next to a label.
const HotfixBranchPrefix = "hotfix/"

// IsHotfix reports whether a PR is a hotfix: it carries hotfixLabel
// (case-insensitive) or its head branch starts with HotfixBranchPrefix.
func IsHotfix(labels []string, headRef, hotfixLabel string) bool {
	if strings.HasPrefix(strings.ToLower(headRef), HotfixBranchPrefix) {
		return true
	}
	return HasLabel(labels, hotfixLabel)
}

// HasLabel reports whether labels contains want, ignoring case and
// surrounding whitespace. An empty want never matches.
func HasLabel(labels []string, want string) bool {
	want = strings.TrimSpace(want)
	if want == "" {
		return false
	}
	for _, l := range labels {
		if strings.EqualFold(strings.TrimSpace(l), want) {
			return true
		}
	}
	return false
}
