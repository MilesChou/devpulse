// Package people models who is behind the GitHub accounts DevPulse
// sees: members with a display name and one or more accounts, teams of
// members, and accounts excluded from every metric (bots).
package people

import (
	"errors"
	"fmt"
	"regexp"
	"slices"
	"strings"
)

// Member is a person, identified in reports by DisplayName, who acts
// through one or more GitHub accounts.
type Member struct {
	ID          string
	DisplayName string
	Accounts    []string // normalized, sorted
	TeamIDs     []string
}

// Team is a named set of members.
type Team struct {
	ID        string
	Name      string
	MemberIDs []string
}

// ErrInvalid classifies rejected input (blank name, malformed account)
// so callers can answer 400.
var ErrInvalid = errors.New("people: invalid input")

// ErrConflict classifies input that clashes with stored data (a
// display name or account already taken) so callers can answer 409.
var ErrConflict = errors.New("people: conflict")

// ErrNotFound is returned for an unknown member or team.
var ErrNotFound = errors.New("people: not found")

// botSuffix is how GitHub's REST API names app accounts
// ("dependabot[bot]"); its GraphQL API omits it ("dependabot").
const botSuffix = "[bot]"

// NormalizeAccount lower-cases a GitHub login and drops a trailing
// "[bot]", so REST and GraphQL spellings of the same account compare
// equal. The SQL side applies the same rule; see persistence.
func NormalizeAccount(s string) string {
	return strings.TrimSuffix(strings.ToLower(strings.TrimSpace(s)), botSuffix)
}

// accountPattern accepts GitHub logins (letters, digits, hyphens) and
// the underscores Enterprise Managed Users add, after normalization.
var accountPattern = regexp.MustCompile(`^[a-z0-9_][a-z0-9_.-]{0,63}$`)

// NormalizeAccounts normalizes, validates, de-duplicates and sorts a
// list of accounts.
func NormalizeAccounts(in []string) ([]string, error) {
	out := make([]string, 0, len(in))
	for _, raw := range in {
		a := NormalizeAccount(raw)
		if a == "" {
			continue
		}
		if !accountPattern.MatchString(a) {
			return nil, fmt.Errorf("%w: %q is not a GitHub account", ErrInvalid, raw)
		}
		out = append(out, a)
	}
	slices.Sort(out)
	return slices.Compact(out), nil
}

// NormalizeName trims a display or team name and rejects a blank one.
func NormalizeName(kind, s string) (string, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return "", fmt.Errorf("%w: %s must not be blank", ErrInvalid, kind)
	}
	if len(s) > 255 {
		return "", fmt.Errorf("%w: %s is longer than 255 bytes", ErrInvalid, kind)
	}
	return s, nil
}
