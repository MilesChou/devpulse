package pullrequest

import (
	"regexp"
	"strconv"
	"strings"
)

// revertTitle matches GitHub's revert-button title (`Revert "<title>"`)
// and the conventional-commit spelling (`revert: ...`).
var revertTitle = regexp.MustCompile(`(?i)^\s*revert\b`)

// revertsRef matches GitHub's revert-button body: "Reverts owner/repo#N".
// The owner/repo prefix is optional so a hand-written "Reverts #N" also
// resolves.
var revertsRef = regexp.MustCompile(`(?i)\breverts\s+(?:([\w.-]+/[\w.-]+))?#(\d+)`)

// IsRevertTitle reports whether a PR title marks a revert.
func IsRevertTitle(title string) bool {
	return revertTitle.MatchString(title)
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
