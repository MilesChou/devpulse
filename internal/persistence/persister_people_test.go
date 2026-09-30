package persistence_test

import (
	"context"
	"errors"
	"slices"
	"testing"

	"github.com/mileschou/devpulse/internal/people"
	"github.com/mileschou/devpulse/internal/persistence"
)

func TestPeoplePersister_Members(t *testing.T) {
	pp := persistence.NewPeoplePersister(setup(t))
	ctx := context.Background()

	alice, err := pp.CreateMember(ctx, "Alice", []string{"alice", "alice-work"})
	if err != nil {
		t.Fatalf("create alice: %v", err)
	}
	if _, err := pp.CreateMember(ctx, "Bob", []string{"bob"}); err != nil {
		t.Fatalf("create bob: %v", err)
	}

	// Name and account clashes are conflicts, and change nothing.
	if _, err := pp.CreateMember(ctx, "Alice", []string{"x"}); !errors.Is(err, people.ErrConflict) {
		t.Fatalf("duplicate name: want ErrConflict, got %v", err)
	}
	if _, err := pp.CreateMember(ctx, "Carol", []string{"carol", "bob"}); !errors.Is(err, people.ErrConflict) {
		t.Fatalf("taken account: want ErrConflict, got %v", err)
	}

	members, err := pp.ListMembers(ctx)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(members) != 2 || members[0].DisplayName != "Alice" || !slices.Equal(members[0].Accounts, []string{"alice", "alice-work"}) {
		t.Fatalf("members: %+v", members)
	}

	// Updating keeps the member's own accounts available to it.
	if err := pp.UpdateMember(ctx, alice.ID, "Alice Chen", []string{"alice", "ac"}); err != nil {
		t.Fatalf("update: %v", err)
	}
	got, err := pp.AccountsOfMember(ctx, alice.ID)
	if err != nil || got.DisplayName != "Alice Chen" || !slices.Equal(got.Accounts, []string{"ac", "alice"}) {
		t.Fatalf("after update: %+v, %v", got, err)
	}
	if err := pp.UpdateMember(ctx, alice.ID, "Bob", nil); !errors.Is(err, people.ErrConflict) {
		t.Fatalf("rename onto Bob: want ErrConflict, got %v", err)
	}
	if err := pp.UpdateMember(ctx, "nope", "X", nil); !errors.Is(err, people.ErrNotFound) {
		t.Fatalf("update unknown: want ErrNotFound, got %v", err)
	}

	if err := pp.DeleteMember(ctx, alice.ID); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if _, err := pp.AccountsOfMember(ctx, alice.ID); !errors.Is(err, people.ErrNotFound) {
		t.Fatalf("deleted member: want ErrNotFound, got %v", err)
	}
	// Its accounts are free again.
	if _, err := pp.CreateMember(ctx, "Dana", []string{"alice"}); err != nil {
		t.Fatalf("reuse freed account: %v", err)
	}
}

func TestPeoplePersister_Teams(t *testing.T) {
	pp := persistence.NewPeoplePersister(setup(t))
	ctx := context.Background()

	alice, _ := pp.CreateMember(ctx, "Alice", []string{"alice"})
	bob, _ := pp.CreateMember(ctx, "Bob", []string{"bob", "bob2"})

	team, err := pp.CreateTeam(ctx, "Web", []string{alice.ID, bob.ID, bob.ID})
	if err != nil {
		t.Fatalf("create team: %v", err)
	}
	if _, err := pp.CreateTeam(ctx, "Web", nil); !errors.Is(err, people.ErrConflict) {
		t.Fatalf("duplicate team: want ErrConflict, got %v", err)
	}
	if _, err := pp.CreateTeam(ctx, "Ghosts", []string{"nope"}); !errors.Is(err, people.ErrInvalid) {
		t.Fatalf("unknown member: want ErrInvalid, got %v", err)
	}

	got, accounts, err := pp.AccountsOfTeam(ctx, team.ID)
	if err != nil || got.Name != "Web" || !slices.Equal(accounts, []string{"alice", "bob", "bob2"}) {
		t.Fatalf("team accounts: %+v %v %v", got, accounts, err)
	}

	members, _ := pp.ListMembers(ctx)
	for _, m := range members {
		if !slices.Equal(m.TeamIDs, []string{team.ID}) {
			t.Fatalf("%s team ids: %v", m.DisplayName, m.TeamIDs)
		}
	}

	if err := pp.UpdateTeam(ctx, team.ID, "Frontend", []string{alice.ID}); err != nil {
		t.Fatalf("update team: %v", err)
	}
	teams, _ := pp.ListTeams(ctx)
	if len(teams) != 1 || teams[0].Name != "Frontend" || !slices.Equal(teams[0].MemberIDs, []string{alice.ID}) {
		t.Fatalf("teams: %+v", teams)
	}

	// Deleting a member drops it from its teams.
	if err := pp.DeleteMember(ctx, alice.ID); err != nil {
		t.Fatalf("delete member: %v", err)
	}
	teams, _ = pp.ListTeams(ctx)
	if len(teams[0].MemberIDs) != 0 {
		t.Fatalf("team still lists deleted member: %+v", teams[0])
	}

	if err := pp.DeleteTeam(ctx, team.ID); err != nil {
		t.Fatalf("delete team: %v", err)
	}
	if _, _, err := pp.AccountsOfTeam(ctx, team.ID); !errors.Is(err, people.ErrNotFound) {
		t.Fatalf("deleted team: want ErrNotFound, got %v", err)
	}
}

func TestPeoplePersister_ExcludedAccounts(t *testing.T) {
	pp := persistence.NewPeoplePersister(setup(t))
	ctx := context.Background()

	got, err := pp.ExcludedAccounts(ctx)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if want := []string{"copilot-pull-request-reviewer", "dependabot", "github-actions"}; !slices.Equal(got, want) {
		t.Fatalf("seeded defaults: got %v, want %v", got, want)
	}

	if err := pp.ReplaceExcludedAccounts(ctx, []string{"renovate"}); err != nil {
		t.Fatalf("replace: %v", err)
	}
	got, _ = pp.ExcludedAccounts(ctx)
	if !slices.Equal(got, []string{"renovate"}) {
		t.Fatalf("after replace: %v", got)
	}
	if err := pp.ReplaceExcludedAccounts(ctx, nil); err != nil {
		t.Fatalf("clear: %v", err)
	}
	if got, _ = pp.ExcludedAccounts(ctx); len(got) != 0 {
		t.Fatalf("after clear: %v", got)
	}
}
