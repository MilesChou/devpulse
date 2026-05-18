package http

import (
	"context"
	"testing"
	"time"
)

func TestStart_NoAddrIsNoOp(t *testing.T) {
	s := New(Config{})
	ctx, cancel := context.WithCancel(context.Background())

	done := make(chan error, 1)
	go func() { done <- s.Start(ctx) }()

	cancel()
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("v1 server should return nil after cancel, got: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("server did not return on cancel")
	}
}

