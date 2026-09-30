package syncrun

import (
	"context"
	"errors"
	"testing"

	"github.com/mileschou/devpulse/internal/repo"
)

func rp(s string) repo.Repo {
	n, _ := repo.ParseFullName(s)
	return repo.Repo{Name: n}
}

func TestRunner_SerializesAndRecordsOutcome(t *testing.T) {
	release := make(chan struct{})
	var got []string
	rn := New(context.Background(), func(_ context.Context, r repo.Repo) error {
		got = append(got, r.Name.String())
		<-release
		if r.Name.Name == "bad" {
			return errors.New("boom")
		}
		return nil
	}, nil)

	if err := rn.Start(rp("acme/web")); err != nil {
		t.Fatalf("start: %v", err)
	}
	if s := rn.Status(); s.Running != "acme/web" || s.StartedAt == nil {
		t.Fatalf("running status: %+v", s)
	}
	if err := rn.Start(rp("acme/api")); !errors.Is(err, ErrBusy) {
		t.Fatalf("second start while running: want ErrBusy, got %v", err)
	}

	close(release)
	rn.Wait()
	s := rn.Status()
	if s.Running != "" || s.LastRepo != "acme/web" || s.LastError != "" || s.LastFinishedAt == nil {
		t.Fatalf("finished status: %+v", s)
	}

	if err := rn.Start(rp("acme/bad")); err != nil {
		t.Fatalf("start after finish: %v", err)
	}
	rn.Wait()
	if s := rn.Status(); s.LastRepo != "acme/bad" || s.LastError != "boom" {
		t.Fatalf("failed status: %+v", s)
	}
	if len(got) != 2 {
		t.Fatalf("synced %v, want exactly two runs", got)
	}
}

func TestRunner_UsesItsOwnContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	rn := New(ctx, func(ctx context.Context, _ repo.Repo) error {
		<-ctx.Done()
		return ctx.Err()
	}, nil)
	if err := rn.Start(rp("acme/web")); err != nil {
		t.Fatalf("start: %v", err)
	}
	cancel()
	rn.Wait()
	if s := rn.Status(); s.LastError != context.Canceled.Error() {
		t.Fatalf("cancelled sync: %+v", s)
	}
}
