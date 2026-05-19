package jobs_test

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/mileschou/devpulse/internal/jobs"
	"github.com/mileschou/devpulse/internal/persistence"
	"github.com/mileschou/devpulse/internal/persistence/persistencetest"
)

func setup(t *testing.T) *persistence.Persister {
	return persistencetest.NewMemoryPersister(t)
}

func TestEnqueueAndLease(t *testing.T) {
	ctx := context.Background()
	p := setup(t)
	q := jobs.NewQueue(p)

	payload := map[string]any{"key": "value"}
	id, err := q.Enqueue(ctx, "test_kind", payload, time.Time{}, 3)
	if err != nil {
		t.Fatalf("enqueue: %v", err)
	}
	if id == "" {
		t.Fatal("expected non-empty id")
	}

	got, err := q.Lease(ctx, "test_kind", 60*time.Second)
	if err != nil {
		t.Fatalf("lease: %v", err)
	}
	if got == nil {
		t.Fatal("expected to lease a job")
	}
	if got.ID != id {
		t.Fatalf("id mismatch: %s vs %s", got.ID, id)
	}
	if got.Kind != "test_kind" {
		t.Fatalf("kind: %q", got.Kind)
	}
	if got.Attempts != 1 {
		t.Fatalf("attempts should be 1 after lease, got %d", got.Attempts)
	}

	var decoded map[string]any
	if err := json.Unmarshal(got.Payload, &decoded); err != nil {
		t.Fatalf("decode payload: %v", err)
	}
	if decoded["key"] != "value" {
		t.Fatalf("payload: %v", decoded)
	}
}

func TestLease_ReturnsNilWhenEmpty(t *testing.T) {
	ctx := context.Background()
	p := setup(t)
	q := jobs.NewQueue(p)

	got, err := q.Lease(ctx, "nothing", 60*time.Second)
	if err != nil {
		t.Fatalf("lease: %v", err)
	}
	if got != nil {
		t.Fatalf("expected nil, got %+v", got)
	}
}

func TestLease_RespectsAvailableAt(t *testing.T) {
	ctx := context.Background()
	p := setup(t)
	q := jobs.NewQueue(p)

	future := time.Now().UTC().Add(time.Hour)
	if _, err := q.Enqueue(ctx, "k", "payload", future, 3); err != nil {
		t.Fatalf("enqueue: %v", err)
	}

	got, err := q.Lease(ctx, "k", 60*time.Second)
	if err != nil {
		t.Fatalf("lease: %v", err)
	}
	if got != nil {
		t.Fatalf("job should not be available yet, got %+v", got)
	}
}

func TestMarkDone(t *testing.T) {
	ctx := context.Background()
	p := setup(t)
	q := jobs.NewQueue(p)

	id, _ := q.Enqueue(ctx, "k", "x", time.Time{}, 3)
	_, _ = q.Lease(ctx, "k", time.Minute)
	if err := q.MarkDone(ctx, id); err != nil {
		t.Fatalf("mark done: %v", err)
	}

	// Subsequent lease must not pick the done job.
	if got, _ := q.Lease(ctx, "k", time.Minute); got != nil {
		t.Fatalf("done job should not be leased again, got %+v", got)
	}
}

func TestMarkFailed_RescheduleWithBackoff(t *testing.T) {
	ctx := context.Background()
	p := setup(t)
	q := jobs.NewQueue(p)

	id, _ := q.Enqueue(ctx, "k", "x", time.Time{}, 5)
	_, _ = q.Lease(ctx, "k", time.Minute) // attempts = 1

	if err := q.MarkFailed(ctx, id, errors.New("boom")); err != nil {
		t.Fatalf("mark failed: %v", err)
	}

	// available_at is now in the future (2^1 = 2 seconds). Immediate lease
	// should yield nothing.
	if got, _ := q.Lease(ctx, "k", time.Minute); got != nil {
		t.Fatalf("requeued job should not be available immediately, got %+v", got)
	}
}

func TestMarkFailed_BurnsOutAtMaxAttempts(t *testing.T) {
	ctx := context.Background()
	p := setup(t)
	q := jobs.NewQueue(p)

	id, _ := q.Enqueue(ctx, "k", "x", time.Time{}, 1) // single attempt allowed
	_, _ = q.Lease(ctx, "k", time.Minute)             // attempts -> 1

	if err := q.MarkFailed(ctx, id, errors.New("dead")); err != nil {
		t.Fatalf("mark failed: %v", err)
	}

	// Even with available_at in the past, status=failed prevents lease.
	if got, _ := q.Lease(ctx, "k", time.Minute); got != nil {
		t.Fatalf("burned-out job must not be leased, got %+v", got)
	}
}
