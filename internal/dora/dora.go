package dora

import (
	"time"

	"github.com/mileschou/devpulse/internal/incident"
	"github.com/mileschou/devpulse/internal/pullrequest"
	"github.com/mileschou/devpulse/internal/x/statx"
)

// Deployment is one PR merged into the default branch, with the facts
// Compute needs. RevertedMergedAt is the merge time of the PR this one
// reverts (nil when unknown or never merged); the caller resolves it
// because the reverted PR may have merged before the window.
type Deployment struct {
	Number           int
	MergedAt         time.Time
	FirstCommitAt    *time.Time
	Title            string
	Labels           []string
	HeadRef          string
	RevertsNumber    *int
	RevertedMergedAt *time.Time
}

// Input is everything Compute reads. The window is [From, To). Rows
// outside the window are ignored, so callers may over-fetch. PerWeek
// divides by the window length, so a caller reporting a window that is
// still in progress passes now as To.
type Input struct {
	From        time.Time
	To          time.Time
	HotfixLabel string
	Deployments []Deployment
	Incidents   []incident.Incident
}

// Report is the DORA result for one window. Durations are in hours.
type Report struct {
	Deployments int
	PerWeek     float64
	DeployDays  int

	LeadTime statx.Summary

	Reverts  int
	Hotfixes int
	// ChangeFailureRate is nil when there were no deployments: 0/0 is
	// "not applicable", not a perfect score.
	ChangeFailureRate *float64

	Recovery              statx.Summary
	RecoveryFromReverts   int
	RecoveryFromIncidents int
}

// Compute derives the Report for in's window.
func Compute(in Input) Report {
	var r Report

	days := map[string]struct{}{}
	var leadHours, recoveryHours []float64

	for _, d := range in.Deployments {
		if !inWindow(d.MergedAt, in.From, in.To) {
			continue
		}
		r.Deployments++
		days[d.MergedAt.UTC().Format(time.DateOnly)] = struct{}{}

		if d.FirstCommitAt != nil {
			leadHours = append(leadHours, nonNegHours(d.MergedAt.Sub(*d.FirstCommitAt)))
		}

		switch {
		case pullrequest.IsRevertTitle(d.Title):
			r.Reverts++
			if d.RevertedMergedAt != nil {
				recoveryHours = append(recoveryHours, nonNegHours(d.MergedAt.Sub(*d.RevertedMergedAt)))
				r.RecoveryFromReverts++
			}
		case pullrequest.IsHotfix(d.Labels, d.HeadRef, in.HotfixLabel):
			r.Hotfixes++
		}
	}

	for _, inc := range in.Incidents {
		if inc.ResolvedAt == nil || !inWindow(*inc.ResolvedAt, in.From, in.To) {
			continue
		}
		recoveryHours = append(recoveryHours, nonNegHours(inc.ResolvedAt.Sub(inc.OpenedAt)))
		r.RecoveryFromIncidents++
	}

	r.DeployDays = len(days)
	if windowDays := in.To.Sub(in.From).Hours() / 24; windowDays > 0 {
		r.PerWeek = float64(r.Deployments) / windowDays * 7
	}
	if r.Deployments > 0 {
		rate := float64(r.Reverts+r.Hotfixes) / float64(r.Deployments)
		r.ChangeFailureRate = &rate
	}
	r.LeadTime = statx.Summarize(leadHours)
	r.Recovery = statx.Summarize(recoveryHours)
	return r
}

func inWindow(t, from, to time.Time) bool {
	return !t.Before(from) && t.Before(to)
}

// nonNegHours clamps negative durations (author clock skew, rewritten
// history) to zero rather than letting them pull averages down.
func nonNegHours(d time.Duration) float64 {
	if d < 0 {
		return 0
	}
	return d.Hours()
}
