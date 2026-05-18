package commitsha

import (
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
)

// SHA is a Git commit SHA1 (40 lower-case hex chars). Short SHAs are not
// accepted: identity must be exact to avoid cross-build collisions.
type SHA string

var (
	ErrEmpty  = errors.New("commit sha is empty")
	ErrBadLen = errors.New("commit sha must be 40 hex characters")
	ErrBadHex = errors.New("commit sha contains non-hex character")
)

// Parse validates and lower-cases the input.
func Parse(s string) (SHA, error) {
	if s == "" {
		return "", ErrEmpty
	}
	if len(s) != 40 {
		return "", fmt.Errorf("%w: %q (len=%d)", ErrBadLen, s, len(s))
	}
	if _, err := hex.DecodeString(s); err != nil {
		return "", fmt.Errorf("%w: %q", ErrBadHex, s)
	}
	return SHA(strings.ToLower(s)), nil
}

func (s SHA) String() string { return string(s) }
