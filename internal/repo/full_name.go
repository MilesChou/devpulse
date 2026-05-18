package repo

import (
	"errors"
	"fmt"
	"strings"
)

// FullName is the canonical "owner/name" identifier of a repository.
// Both parts are non-empty, contain no whitespace, and the input is
// case-preserved so it round-trips with the original platform identity.
type FullName struct {
	Owner string
	Name  string
}

var (
	ErrEmptyFullName     = errors.New("repo full name is empty")
	ErrMalformedFullName = errors.New("repo full name must be owner/name")
)

// ParseFullName parses "owner/name" and rejects empty or malformed input.
func ParseFullName(s string) (FullName, error) {
	if strings.TrimSpace(s) == "" {
		return FullName{}, ErrEmptyFullName
	}

	parts := strings.Split(s, "/")
	if len(parts) != 2 {
		return FullName{}, fmt.Errorf("%w: %q", ErrMalformedFullName, s)
	}

	owner, name := strings.TrimSpace(parts[0]), strings.TrimSpace(parts[1])
	if owner == "" || name == "" {
		return FullName{}, fmt.Errorf("%w: %q", ErrMalformedFullName, s)
	}
	if strings.ContainsAny(owner, " \t") || strings.ContainsAny(name, " \t") {
		return FullName{}, fmt.Errorf("%w: %q", ErrMalformedFullName, s)
	}

	return FullName{Owner: owner, Name: name}, nil
}

func (f FullName) String() string {
	return f.Owner + "/" + f.Name
}
