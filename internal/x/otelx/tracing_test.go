package otelx

import (
	"context"
	"testing"
)

func TestNewProvider_NoEndpoint_ReturnsNoop(t *testing.T) {
	p, err := NewProvider(context.Background(), Config{ServiceName: "test"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if p == nil || p.TracerProvider == nil {
		t.Fatalf("expected provider")
	}
	if err := p.Shutdown(context.Background()); err != nil {
		t.Fatalf("noop shutdown should not error, got: %v", err)
	}
}

func TestShutdown_NilProvider_NoPanic(t *testing.T) {
	var p *Provider
	if err := p.Shutdown(context.Background()); err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
}

