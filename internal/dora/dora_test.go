package dora_test

import (
	"math"
	"testing"
	"time"

	"github.com/mileschou/devpulse/internal/dora"
	"github.com/mileschou/devpulse/internal/incident"
)

var (
	may     = time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC)
	june    = may.AddDate(0, 1, 0)
	baseDay = time.Date(2026, 5, 10, 0, 0, 0, 0, time.UTC)
)

func at(day, hour, minute int) time.Time {
	return time.Date(2026, 5, day, hour, minute, 0, 0, time.UTC)
}

func ptr[T any](v T) *T { return &v }

func approx(t *testing.T, name string, got, want float64) {
	t.Helper()
	if math.Abs(got-want) > 1e-6 {
		t.Fatalf("%s: got %v, want %v", name, got, want)
	}
}

// Spec: "Frequency over one month" — 10 deployments on 6 distinct days
// in 31-day May → 2.26 per week.
func TestCompute_DeploymentFrequency(t *testing.T) {
	var deps []dora.Deployment
	days := []int{1, 1, 2, 3, 3, 3, 10, 11, 31, 31}
	for i, d := range days {
		deps = append(deps, dora.Deployment{Number: i + 1, MergedAt: at(d, 12, 0)})
	}
	// Outside the window on both edges: ignored.
	deps = append(deps,
		dora.Deployment{Number: 98, MergedAt: may.Add(-time.Second)},
		dora.Deployment{Number: 99, MergedAt: june},
	)

	r := dora.Compute(dora.Input{From: may, To: june, Deployments: deps})

	if r.Deployments != 10 {
		t.Fatalf("deployments: %d", r.Deployments)
	}
	if r.DeployDays != 6 {
		t.Fatalf("deploy days: %d", r.DeployDays)
	}
	approx(t, "per week", r.PerWeek, 10.0/31*7)
}

// Spec: "Lead time from first commit" and "Missing first-commit time is
// excluded"; plus negative durations clamp to zero.
func TestCompute_LeadTime(t *testing.T) {
	r := dora.Compute(dora.Input{From: may, To: june, Deployments: []dora.Deployment{
		{Number: 1, MergedAt: at(10, 15, 0), FirstCommitAt: ptr(at(10, 9, 0))},
		// No first commit → excluded.
		{Number: 2, MergedAt: at(11, 15, 0)},
		// Author clock skew → clamped to 0.
		{Number: 3, MergedAt: at(12, 9, 0), FirstCommitAt: ptr(at(12, 10, 0))},
	}})

	if r.LeadTime.Count != 2 {
		t.Fatalf("lead time sample: %d", r.LeadTime.Count)
	}
	approx(t, "avg", r.LeadTime.Avg, 3)
	approx(t, "p90", r.LeadTime.P90, 5.4)
}

// Spec: "Reverts and hotfixes both count" — 20 deployments, one revert,
// one hotfix → 10%.
func TestCompute_ChangeFailureRate(t *testing.T) {
	var deps []dora.Deployment
	for i := range 18 {
		deps = append(deps, dora.Deployment{Number: i + 1, MergedAt: baseDay, Title: "feat: x"})
	}
	deps = append(deps,
		dora.Deployment{Number: 19, MergedAt: baseDay, Title: `Revert "feat: x"`},
		dora.Deployment{Number: 20, MergedAt: baseDay, Title: "fix", Labels: []string{"Hotfix"}},
	)

	r := dora.Compute(dora.Input{From: may, To: june, HotfixLabel: "hotfix", Deployments: deps})

	if r.Reverts != 1 || r.Hotfixes != 1 {
		t.Fatalf("reverts=%d hotfixes=%d", r.Reverts, r.Hotfixes)
	}
	if r.ChangeFailureRate == nil {
		t.Fatal("CFR is nil")
	}
	approx(t, "cfr", *r.ChangeFailureRate, 0.1)
}

// A PR that is both a revert and a hotfix counts once, as a revert.
func TestCompute_ChangeFailureRate_CountsOnce(t *testing.T) {
	r := dora.Compute(dora.Input{From: may, To: june, HotfixLabel: "hotfix", Deployments: []dora.Deployment{
		{Number: 1, MergedAt: baseDay, Title: `Revert "x"`, HeadRef: "hotfix/x"},
	}})
	if r.Reverts != 1 || r.Hotfixes != 0 {
		t.Fatalf("reverts=%d hotfixes=%d", r.Reverts, r.Hotfixes)
	}
	approx(t, "cfr", *r.ChangeFailureRate, 1)
}

// Spec: "No deployments" → CFR not applicable.
func TestCompute_NoDeployments(t *testing.T) {
	r := dora.Compute(dora.Input{From: may, To: june})
	if r.ChangeFailureRate != nil {
		t.Fatalf("CFR should be nil, got %v", *r.ChangeFailureRate)
	}
	if r.Deployments != 0 || r.PerWeek != 0 || r.LeadTime.Count != 0 || r.Recovery.Count != 0 {
		t.Fatalf("non-zero report: %+v", r)
	}
}

// Spec: "Recovery via revert", "Recovery via incident issue", and "Open
// incident is not a sample".
func TestCompute_RecoveryTime(t *testing.T) {
	r := dora.Compute(dora.Input{
		From: may, To: june,
		Deployments: []dora.Deployment{
			{
				Number: 11, MergedAt: at(10, 12, 30), Title: `Revert "feat"`,
				RevertsNumber: ptr(10), RevertedMergedAt: ptr(at(10, 10, 0)),
			},
			// Revert whose target is unknown: counts for CFR, no sample.
			{Number: 12, MergedAt: at(11, 12, 0), Title: "revert: y"},
		},
		Incidents: []incident.Incident{
			{Number: 1, OpenedAt: at(12, 8, 0), ResolvedAt: ptr(at(12, 11, 0))},
			{Number: 2, OpenedAt: at(13, 8, 0)}, // still open
			// Resolved after the window: attributed to June.
			{Number: 3, OpenedAt: at(31, 8, 0), ResolvedAt: ptr(june.Add(time.Hour))},
		},
	})

	if r.Reverts != 2 {
		t.Fatalf("reverts: %d", r.Reverts)
	}
	if r.RecoveryFromReverts != 1 || r.RecoveryFromIncidents != 1 {
		t.Fatalf("sources: reverts=%d incidents=%d", r.RecoveryFromReverts, r.RecoveryFromIncidents)
	}
	if r.Recovery.Count != 2 {
		t.Fatalf("recovery sample: %d", r.Recovery.Count)
	}
	approx(t, "avg", r.Recovery.Avg, (2.5+3)/2)
}

// Spec: "Re-landing a reverted change is not a remediation" — reverting
// the revert puts the change back. It must neither count as a revert
// nor yield a recovery sample (revert merge → re-land merge measures
// the fix, not the recovery).
func TestCompute_RelandIsNotARevert(t *testing.T) {
	r := dora.Compute(dora.Input{From: may, To: june, Deployments: []dora.Deployment{
		{Number: 1, MergedAt: at(10, 10, 0), Title: "feat: x"},
		{Number: 2, MergedAt: at(10, 12, 0), Title: `Revert "feat: x"`,
			RevertsNumber: ptr(1), RevertedMergedAt: ptr(at(10, 10, 0))},
		{Number: 3, MergedAt: at(12, 9, 0), Title: `Revert "Revert "feat: x""`,
			RevertsNumber: ptr(2), RevertedMergedAt: ptr(at(10, 12, 0))},
	}})

	if r.Reverts != 1 || r.RecoveryFromReverts != 1 {
		t.Fatalf("reverts=%d recovery samples=%d, want 1 and 1", r.Reverts, r.RecoveryFromReverts)
	}
	approx(t, "recovery avg", r.Recovery.Avg, 2)
	approx(t, "cfr", *r.ChangeFailureRate, 1.0/3)
}
