package persistence

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"slices"

	"github.com/mileschou/devpulse/internal/people"
)

// PeoplePersister stores members, their accounts, teams, and excluded
// accounts. Inputs are expected to be normalized (people.Normalize*);
// uniqueness is checked inside each write transaction so a clash comes
// back as people.ErrConflict on every dialect instead of a
// driver-specific constraint error.
type PeoplePersister struct{ *Persister }

func NewPeoplePersister(p *Persister) *PeoplePersister { return &PeoplePersister{Persister: p} }

// ListMembers returns every member with accounts and team ids, ordered
// by display name.
func (r *PeoplePersister) ListMembers(ctx context.Context) ([]people.Member, error) {
	rows, err := r.QueryCtx(ctx, `SELECT id, display_name FROM members ORDER BY display_name`)
	if err != nil {
		return nil, fmt.Errorf("list members: %w", err)
	}
	var out []people.Member
	for rows.Next() {
		var m people.Member
		if err := rows.Scan(&m.ID, &m.DisplayName); err != nil {
			rows.Close()
			return nil, fmt.Errorf("list members scan: %w", err)
		}
		out = append(out, m)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list members rows: %w", err)
	}

	accounts, err := r.pairs(ctx, `SELECT member_id, account FROM member_accounts ORDER BY account`)
	if err != nil {
		return nil, fmt.Errorf("list member accounts: %w", err)
	}
	teams, err := r.pairs(ctx, `SELECT member_id, team_id FROM team_members ORDER BY team_id`)
	if err != nil {
		return nil, fmt.Errorf("list member teams: %w", err)
	}
	for i := range out {
		out[i].Accounts = nonNil(accounts[out[i].ID])
		out[i].TeamIDs = nonNil(teams[out[i].ID])
	}
	return nonNil(out), nil
}

// CreateMember inserts a member with its accounts.
func (r *PeoplePersister) CreateMember(ctx context.Context, name string, accounts []string) (people.Member, error) {
	m := people.Member{ID: r.NewID(), DisplayName: name, Accounts: nonNil(accounts), TeamIDs: []string{}}
	err := r.inTx(ctx, "create member", func(tx *sql.Tx) error {
		if err := r.checkMemberConflicts(ctx, tx, m.ID, name, accounts); err != nil {
			return err
		}
		now := r.Now()
		if err := r.exec(ctx, tx, `INSERT INTO members (id, display_name, created_at, updated_at) VALUES (?, ?, ?, ?)`,
			m.ID, name, now, now); err != nil {
			return err
		}
		return r.insertAccounts(ctx, tx, m.ID, accounts)
	})
	return m, err
}

// UpdateMember replaces a member's display name and accounts.
func (r *PeoplePersister) UpdateMember(ctx context.Context, id, name string, accounts []string) error {
	return r.inTx(ctx, "update member", func(tx *sql.Tx) error {
		if err := r.mustExist(ctx, tx, `SELECT COUNT(*) FROM members WHERE id = ?`, id); err != nil {
			return err
		}
		if err := r.checkMemberConflicts(ctx, tx, id, name, accounts); err != nil {
			return err
		}
		if err := r.exec(ctx, tx, `UPDATE members SET display_name = ?, updated_at = ? WHERE id = ?`, name, r.Now(), id); err != nil {
			return err
		}
		if err := r.exec(ctx, tx, `DELETE FROM member_accounts WHERE member_id = ?`, id); err != nil {
			return err
		}
		return r.insertAccounts(ctx, tx, id, accounts)
	})
}

// DeleteMember removes a member, its accounts, and its team memberships.
func (r *PeoplePersister) DeleteMember(ctx context.Context, id string) error {
	return r.inTx(ctx, "delete member", func(tx *sql.Tx) error {
		if err := r.mustExist(ctx, tx, `SELECT COUNT(*) FROM members WHERE id = ?`, id); err != nil {
			return err
		}
		for _, q := range []string{
			`DELETE FROM member_accounts WHERE member_id = ?`,
			`DELETE FROM team_members WHERE member_id = ?`,
			`DELETE FROM members WHERE id = ?`,
		} {
			if err := r.exec(ctx, tx, q, id); err != nil {
				return err
			}
		}
		return nil
	})
}

// ListTeams returns every team with its member ids, ordered by name.
func (r *PeoplePersister) ListTeams(ctx context.Context) ([]people.Team, error) {
	rows, err := r.QueryCtx(ctx, `SELECT id, name FROM teams ORDER BY name`)
	if err != nil {
		return nil, fmt.Errorf("list teams: %w", err)
	}
	var out []people.Team
	for rows.Next() {
		var t people.Team
		if err := rows.Scan(&t.ID, &t.Name); err != nil {
			rows.Close()
			return nil, fmt.Errorf("list teams scan: %w", err)
		}
		out = append(out, t)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list teams rows: %w", err)
	}

	members, err := r.pairs(ctx, `SELECT team_id, member_id FROM team_members ORDER BY member_id`)
	if err != nil {
		return nil, fmt.Errorf("list team members: %w", err)
	}
	for i := range out {
		out[i].MemberIDs = nonNil(members[out[i].ID])
	}
	return nonNil(out), nil
}

// CreateTeam inserts a team with its members.
func (r *PeoplePersister) CreateTeam(ctx context.Context, name string, memberIDs []string) (people.Team, error) {
	t := people.Team{ID: r.NewID(), Name: name, MemberIDs: nonNil(memberIDs)}
	err := r.inTx(ctx, "create team", func(tx *sql.Tx) error {
		if err := r.checkTeam(ctx, tx, t.ID, name, memberIDs); err != nil {
			return err
		}
		now := r.Now()
		if err := r.exec(ctx, tx, `INSERT INTO teams (id, name, created_at, updated_at) VALUES (?, ?, ?, ?)`,
			t.ID, name, now, now); err != nil {
			return err
		}
		return r.insertTeamMembers(ctx, tx, t.ID, memberIDs)
	})
	return t, err
}

// UpdateTeam replaces a team's name and members.
func (r *PeoplePersister) UpdateTeam(ctx context.Context, id, name string, memberIDs []string) error {
	return r.inTx(ctx, "update team", func(tx *sql.Tx) error {
		if err := r.mustExist(ctx, tx, `SELECT COUNT(*) FROM teams WHERE id = ?`, id); err != nil {
			return err
		}
		if err := r.checkTeam(ctx, tx, id, name, memberIDs); err != nil {
			return err
		}
		if err := r.exec(ctx, tx, `UPDATE teams SET name = ?, updated_at = ? WHERE id = ?`, name, r.Now(), id); err != nil {
			return err
		}
		if err := r.exec(ctx, tx, `DELETE FROM team_members WHERE team_id = ?`, id); err != nil {
			return err
		}
		return r.insertTeamMembers(ctx, tx, id, memberIDs)
	})
}

// DeleteTeam removes a team; its members stay.
func (r *PeoplePersister) DeleteTeam(ctx context.Context, id string) error {
	return r.inTx(ctx, "delete team", func(tx *sql.Tx) error {
		if err := r.mustExist(ctx, tx, `SELECT COUNT(*) FROM teams WHERE id = ?`, id); err != nil {
			return err
		}
		if err := r.exec(ctx, tx, `DELETE FROM team_members WHERE team_id = ?`, id); err != nil {
			return err
		}
		return r.exec(ctx, tx, `DELETE FROM teams WHERE id = ?`, id)
	})
}

// ExcludedAccounts returns the excluded accounts, sorted.
func (r *PeoplePersister) ExcludedAccounts(ctx context.Context) ([]string, error) {
	rows, err := r.QueryCtx(ctx, `SELECT account FROM excluded_accounts ORDER BY account`)
	if err != nil {
		return nil, fmt.Errorf("list excluded accounts: %w", err)
	}
	defer rows.Close()
	out := []string{}
	for rows.Next() {
		var a string
		if err := rows.Scan(&a); err != nil {
			return nil, fmt.Errorf("list excluded accounts scan: %w", err)
		}
		out = append(out, a)
	}
	return out, rows.Err()
}

// ReplaceExcludedAccounts makes the excluded set equal to accounts.
func (r *PeoplePersister) ReplaceExcludedAccounts(ctx context.Context, accounts []string) error {
	return r.inTx(ctx, "replace excluded accounts", func(tx *sql.Tx) error {
		if err := r.exec(ctx, tx, `DELETE FROM excluded_accounts`); err != nil {
			return err
		}
		now := r.Now()
		for _, a := range accounts {
			if err := r.exec(ctx, tx, `INSERT INTO excluded_accounts (account, created_at) VALUES (?, ?)`, a, now); err != nil {
				return err
			}
		}
		return nil
	})
}

// AccountsOfMember returns the member's accounts, or ErrNotFound.
func (r *PeoplePersister) AccountsOfMember(ctx context.Context, id string) (people.Member, error) {
	var m people.Member
	err := r.QueryRowCtx(ctx, `SELECT id, display_name FROM members WHERE id = ?`, id).Scan(&m.ID, &m.DisplayName)
	if errors.Is(err, sql.ErrNoRows) {
		return m, people.ErrNotFound
	}
	if err != nil {
		return m, fmt.Errorf("find member: %w", err)
	}
	m.Accounts, err = r.strings(ctx, `SELECT account FROM member_accounts WHERE member_id = ? ORDER BY account`, id)
	return m, err
}

// AccountsOfTeam returns the team and the accounts of all its members,
// or ErrNotFound.
func (r *PeoplePersister) AccountsOfTeam(ctx context.Context, id string) (people.Team, []string, error) {
	var t people.Team
	err := r.QueryRowCtx(ctx, `SELECT id, name FROM teams WHERE id = ?`, id).Scan(&t.ID, &t.Name)
	if errors.Is(err, sql.ErrNoRows) {
		return t, nil, people.ErrNotFound
	}
	if err != nil {
		return t, nil, fmt.Errorf("find team: %w", err)
	}
	accounts, err := r.strings(ctx, `SELECT a.account FROM member_accounts a
	                                   JOIN team_members tm ON tm.member_id = a.member_id
	                                  WHERE tm.team_id = ? ORDER BY a.account`, id)
	return t, accounts, err
}

// ----- helpers -----

func (r *PeoplePersister) inTx(ctx context.Context, op string, fn func(*sql.Tx) error) error {
	tx, err := r.DB.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("%s begin: %w", op, err)
	}
	defer func() { _ = tx.Rollback() }()
	if err := fn(tx); err != nil {
		if errors.Is(err, people.ErrConflict) || errors.Is(err, people.ErrNotFound) || errors.Is(err, people.ErrInvalid) {
			return err
		}
		return fmt.Errorf("%s: %w", op, err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("%s commit: %w", op, err)
	}
	return nil
}

func (r *PeoplePersister) exec(ctx context.Context, tx *sql.Tx, q string, args ...any) error {
	r.Logger.Debug("sql.exec", slog.String("query", q), slog.Any("args", args))
	_, err := tx.ExecContext(ctx, r.Rebind(q), args...)
	return err
}

func (r *PeoplePersister) count(ctx context.Context, tx *sql.Tx, q string, args ...any) (int, error) {
	r.Logger.Debug("sql.query_row", slog.String("query", q), slog.Any("args", args))
	var n int
	err := tx.QueryRowContext(ctx, r.Rebind(q), args...).Scan(&n)
	return n, err
}

func (r *PeoplePersister) mustExist(ctx context.Context, tx *sql.Tx, q, id string) error {
	n, err := r.count(ctx, tx, q, id)
	if err != nil {
		return err
	}
	if n == 0 {
		return people.ErrNotFound
	}
	return nil
}

// checkMemberConflicts rejects a display name or account that another
// member (not selfID) already uses.
func (r *PeoplePersister) checkMemberConflicts(ctx context.Context, tx *sql.Tx, selfID, name string, accounts []string) error {
	n, err := r.count(ctx, tx, `SELECT COUNT(*) FROM members WHERE display_name = ? AND id <> ?`, name, selfID)
	if err != nil {
		return err
	}
	if n > 0 {
		return fmt.Errorf("%w: display name %q is already used", people.ErrConflict, name)
	}
	for _, a := range accounts {
		var owner string
		q := `SELECT m.display_name FROM member_accounts a JOIN members m ON m.id = a.member_id
		       WHERE a.account = ? AND a.member_id <> ?`
		err := tx.QueryRowContext(ctx, r.Rebind(q), a, selfID).Scan(&owner)
		if errors.Is(err, sql.ErrNoRows) {
			continue
		}
		if err != nil {
			return err
		}
		return fmt.Errorf("%w: account %q already belongs to %s", people.ErrConflict, a, owner)
	}
	return nil
}

// checkTeam rejects a taken team name or an unknown member id.
func (r *PeoplePersister) checkTeam(ctx context.Context, tx *sql.Tx, selfID, name string, memberIDs []string) error {
	n, err := r.count(ctx, tx, `SELECT COUNT(*) FROM teams WHERE name = ? AND id <> ?`, name, selfID)
	if err != nil {
		return err
	}
	if n > 0 {
		return fmt.Errorf("%w: team name %q is already used", people.ErrConflict, name)
	}
	for _, id := range memberIDs {
		n, err := r.count(ctx, tx, `SELECT COUNT(*) FROM members WHERE id = ?`, id)
		if err != nil {
			return err
		}
		if n == 0 {
			return fmt.Errorf("%w: unknown member %q", people.ErrInvalid, id)
		}
	}
	return nil
}

func (r *PeoplePersister) insertAccounts(ctx context.Context, tx *sql.Tx, memberID string, accounts []string) error {
	now := r.Now()
	for _, a := range accounts {
		if err := r.exec(ctx, tx, `INSERT INTO member_accounts (account, member_id, created_at) VALUES (?, ?, ?)`,
			a, memberID, now); err != nil {
			return err
		}
	}
	return nil
}

func (r *PeoplePersister) insertTeamMembers(ctx context.Context, tx *sql.Tx, teamID string, memberIDs []string) error {
	ids := slices.Clone(memberIDs)
	slices.Sort(ids)
	for _, id := range slices.Compact(ids) {
		if err := r.exec(ctx, tx, `INSERT INTO team_members (team_id, member_id) VALUES (?, ?)`, teamID, id); err != nil {
			return err
		}
	}
	return nil
}

// pairs runs a two-column query and groups the second column by the first.
func (r *PeoplePersister) pairs(ctx context.Context, q string) (map[string][]string, error) {
	rows, err := r.QueryCtx(ctx, q)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string][]string{}
	for rows.Next() {
		var k, v string
		if err := rows.Scan(&k, &v); err != nil {
			return nil, err
		}
		out[k] = append(out[k], v)
	}
	return out, rows.Err()
}

func (r *PeoplePersister) strings(ctx context.Context, q string, args ...any) ([]string, error) {
	rows, err := r.QueryCtx(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []string{}
	for rows.Next() {
		var s string
		if err := rows.Scan(&s); err != nil {
			return nil, err
		}
		out = append(out, s)
	}
	return out, rows.Err()
}

func nonNil[T any](s []T) []T {
	if s == nil {
		return []T{}
	}
	return s
}
