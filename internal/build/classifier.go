package build

import "strings"

// HumanSignal labels a build outcome with a coarse intent: whether it
// points at a human-driven CI issue we want surfaced in dashboards.
//
// Mirrors the PHP-side classifier in src/DevPulse/Ci/Classification.
type HumanSignal int

const (
	HumanSignalNone           HumanSignal = iota
	HumanSignalTestFailure                // a real test failed
	HumanSignalInfraFailure               // CI infra / runner / network noise
	HumanSignalCancelation                // human or system cancellation
	HumanSignalConfiguration              // syntax / config error in pipeline
)

// ClassifyFromLog scans a build log excerpt and returns the human signal.
// Cheap heuristic; intentional false-positives are preferable to silent
// misses because the signal is reviewed by humans.
func ClassifyFromLog(log string) HumanSignal {
	lc := strings.ToLower(log)

	switch {
	case containsAny(lc, "no space left", "i/o timeout", "connection refused", "could not resolve host"):
		return HumanSignalInfraFailure
	case containsAny(lc, "yaml syntax error", "invalid configuration", "unknown step"):
		return HumanSignalConfiguration
	case containsAny(lc, "canceled", "cancelled"):
		return HumanSignalCancelation
	case containsAny(lc, "test failed", "assertion failed", "expected", "fail:"):
		return HumanSignalTestFailure
	}
	return HumanSignalNone
}

func containsAny(haystack string, needles ...string) bool {
	for _, n := range needles {
		if strings.Contains(haystack, n) {
			return true
		}
	}
	return false
}

