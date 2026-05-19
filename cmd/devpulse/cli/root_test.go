package cli

import (
	"bytes"
	"context"
	"strings"
	"testing"
)

func TestRootCmd_PrintsUsage(t *testing.T) {
	cmd := NewRootCmd()
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	cmd.SetArgs([]string{})

	if err := cmd.ExecuteContext(context.Background()); err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	out := buf.String()
	if !strings.Contains(strings.ToLower(out), "devpulse") {
		t.Fatalf("expected output to mention DevPulse, got: %q", out)
	}
}
