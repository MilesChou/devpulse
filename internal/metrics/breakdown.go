package metrics

import (
	"context"
	"slices"
	"time"

	"github.com/mileschou/devpulse/internal/people"
	"github.com/mileschou/devpulse/internal/repo"
)

// AuthorLister lists the normalized accounts active in a window (see
// persistence.MetricsPersister.Authors).
type AuthorLister interface {
	Authors(ctx context.Context, repoID string, from, to time.Time) ([]string, error)
}

// ScopedSource returns a Source limited to work owned by accounts.
type ScopedSource func(accounts []string) Source

// Row is one line of a per-member breakdown: a mapped member, or an
// active account no member claims yet (MemberID nil, Name = account).
type Row struct {
	MemberID *string  `json:"member_id"`
	Name     string   `json:"name"`
	Accounts []string `json:"accounts"`
	Report   Report   `json:"report"`
}

// ComputeByMember breaks the window down by person. Members with no
// activity in the window are left out, so the table lists who actually
// worked on the repo; unmapped active accounts get their own rows so an
// operator can see who still needs mapping. Rows are ordered members
// first (by name), then unmapped accounts.
func ComputeByMember(
	ctx context.Context,
	authors AuthorLister,
	scoped ScopedSource,
	members []people.Member,
	rp repo.Repo,
	w Window,
	now time.Time,
) ([]Row, error) {
	active, err := authors.Authors(ctx, rp.ID, w.From, w.To)
	if err != nil {
		return nil, err
	}

	rows := []Row{}
	claimed := map[string]bool{}
	for _, m := range members {
		for _, a := range m.Accounts {
			claimed[a] = true
		}
		if !slices.ContainsFunc(m.Accounts, func(a string) bool { return slices.Contains(active, a) }) {
			continue
		}
		id := m.ID
		scope := &Scope{Kind: "member", ID: m.ID, Name: m.DisplayName, Accounts: m.Accounts}
		r, err := Compute(ctx, scoped(m.Accounts), rp, w, now, scope)
		if err != nil {
			return nil, err
		}
		rows = append(rows, Row{MemberID: &id, Name: m.DisplayName, Accounts: m.Accounts, Report: r})
	}

	for _, a := range active {
		if claimed[a] {
			continue
		}
		accounts := []string{a}
		scope := &Scope{Kind: "account", ID: a, Name: a, Accounts: accounts}
		r, err := Compute(ctx, scoped(accounts), rp, w, now, scope)
		if err != nil {
			return nil, err
		}
		rows = append(rows, Row{Name: a, Accounts: accounts, Report: r})
	}
	return rows, nil
}
