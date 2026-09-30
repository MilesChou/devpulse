package repoadmin_test

import (
	"context"
	"errors"
	"testing"

	"github.com/mileschou/devpulse/internal/persistence"
	"github.com/mileschou/devpulse/internal/persistence/persistencetest"
	"github.com/mileschou/devpulse/internal/repo"
	"github.com/mileschou/devpulse/internal/repoadmin"
)

type fakeMeta struct {
	err  error
	desc string
}

func (f fakeMeta) GetRepo(_ context.Context, name repo.FullName) (repo.Repo, error) {
	if f.err != nil {
		return repo.Repo{}, f.err
	}
	return repo.Repo{Name: name, Description: &f.desc, DefaultBranch: "main"}, nil
}

func newService(t *testing.T, meta repoadmin.MetadataFetcher) *repoadmin.Service {
	t.Helper()
	p := persistencetest.NewMemoryPersister(t)
	return repoadmin.New(persistence.NewRepoPersister(p), meta)
}

func name(t *testing.T, s string) repo.FullName {
	t.Helper()
	n, err := repo.ParseFullName(s)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	return n
}

func TestRegister(t *testing.T) {
	ctx := context.Background()
	s := newService(t, fakeMeta{desc: "web app"})

	reg, err := s.Register(ctx, name(t, "acme/web"))
	if err != nil {
		t.Fatalf("register: %v", err)
	}
	if !reg.Created || reg.MetadataErr != nil || reg.Repo.DefaultBranch != "main" || *reg.Repo.Description != "web app" {
		t.Fatalf("first register: %+v", reg)
	}

	again, err := s.Register(ctx, name(t, "acme/web"))
	if err != nil {
		t.Fatalf("register again: %v", err)
	}
	if again.Created || again.Repo.ID != reg.Repo.ID {
		t.Fatalf("second register must reuse the row: %+v", again)
	}
}

func TestRegister_MetadataFailureStillRegisters(t *testing.T) {
	ctx := context.Background()
	s := newService(t, fakeMeta{err: errors.New("404")})

	reg, err := s.Register(ctx, name(t, "acme/private"))
	if err != nil {
		t.Fatalf("register: %v", err)
	}
	if reg.MetadataErr == nil || !reg.Created || reg.Repo.ID == "" {
		t.Fatalf("want a registered repo with a metadata error: %+v", reg)
	}
	if _, err := s.Find(ctx, name(t, "acme/private")); err != nil {
		t.Fatalf("repo must be stored: %v", err)
	}
}

func TestUpdateConfig(t *testing.T) {
	ctx := context.Background()
	s := newService(t, fakeMeta{})
	reg, _ := s.Register(ctx, name(t, "acme/web"))

	pr, incident := 500, "  sev1 "
	got, err := s.UpdateConfig(ctx, reg.Repo, repoadmin.Config{PRStart: &pr, IncidentLabel: &incident})
	if err != nil {
		t.Fatalf("update: %v", err)
	}
	if got.PRSyncStartNumber != 500 || got.IncidentLabel != "sev1" || got.HotfixLabel != "hotfix" {
		t.Fatalf("updated repo: %+v", got)
	}
	stored, _ := s.Find(ctx, name(t, "acme/web"))
	if stored.PRSyncStartNumber != 500 || stored.IncidentLabel != "sev1" || stored.HotfixLabel != "hotfix" {
		t.Fatalf("stored repo: %+v", stored)
	}

	// One invalid field rejects the whole update.
	zero, hotfix := 0, "urgent"
	if _, err := s.UpdateConfig(ctx, stored, repoadmin.Config{PRStart: &zero, HotfixLabel: &hotfix}); !errors.Is(err, repoadmin.ErrInvalidConfig) {
		t.Fatalf("pr-start 0: want ErrInvalidConfig, got %v", err)
	}
	blank := " "
	if _, err := s.UpdateConfig(ctx, stored, repoadmin.Config{HotfixLabel: &blank}); !errors.Is(err, repoadmin.ErrInvalidConfig) {
		t.Fatalf("blank label: want ErrInvalidConfig, got %v", err)
	}
	after, _ := s.Find(ctx, name(t, "acme/web"))
	if after.HotfixLabel != "hotfix" || after.PRSyncStartNumber != 500 {
		t.Fatalf("rejected update must change nothing: %+v", after)
	}
}

func TestRemove(t *testing.T) {
	ctx := context.Background()
	s := newService(t, fakeMeta{})
	_, _ = s.Register(ctx, name(t, "acme/web"))

	if err := s.Remove(ctx, name(t, "acme/web")); err != nil {
		t.Fatalf("remove: %v", err)
	}
	if _, err := s.Find(ctx, name(t, "acme/web")); !errors.Is(err, repoadmin.ErrNotFound) {
		t.Fatalf("after remove: want ErrNotFound, got %v", err)
	}
	if err := s.Remove(ctx, name(t, "acme/web")); !errors.Is(err, repoadmin.ErrNotFound) {
		t.Fatalf("remove twice: want ErrNotFound, got %v", err)
	}
}
