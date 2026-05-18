package commitsha

import (
	"errors"
	"fmt"
	"strings"
)

// SHA is a Git commit SHA1 (40 lower-case hex chars). Short SHAs are not
// accepted: identity must be exact to avoid cross-build collisions.
type SHA string

var (
	ErrEmpty   = errors.New("commit sha is empty")
	ErrBadLen  = errors.New("commit sha must be 40 hex characters")
	ErrBadHex  = errors.New("commit sha contains non-hex character")
)

// Parse validates and lower-cases the input.
func Parse(s string) (SHA, error) {
	if s == "" {
		return "", ErrEmpty
	}
	if len(s) != 40 {
		return "", fmt.Errorf("%w: %q (len=%d)", ErrBadLen, s, len(s))
	}

	lower := strings.ToLower(s)
	for i := 0; i < len(lower); i++ {
		c := lower[i]
		if !(c >= '0' && c <= '9') && !(c >= 'a' && c <= 'f') {
			return "", fmt.Errorf("%w: %q", ErrBadHex, s)
		}
	}
	return SHA(lower), nil
}

func (s SHA) String() string { return string(s) }

