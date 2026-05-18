package logx

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

func TestNew_JSON_AttachesServiceName(t *testing.T) {
	var buf bytes.Buffer
	l := New(Config{Level: "info", Format: FormatJSON, ServiceName: "devpulse", Output: &buf})

	l.Info("hello", "key", "value")

	var rec map[string]any
	if err := json.Unmarshal(buf.Bytes(), &rec); err != nil {
		t.Fatalf("expected json output, got: %s (err=%v)", buf.String(), err)
	}
	if rec["service"] != "devpulse" {
		t.Fatalf("expected service=devpulse, got %v", rec["service"])
	}
	if rec["msg"] != "hello" {
		t.Fatalf("expected msg=hello, got %v", rec["msg"])
	}
	if rec["key"] != "value" {
		t.Fatalf("expected key=value, got %v", rec["key"])
	}
}

func TestNew_RespectsLevel(t *testing.T) {
	var buf bytes.Buffer
	l := New(Config{Level: "warn", Format: FormatJSON, Output: &buf})

	l.Debug("nope")
	l.Info("nope")
	l.Warn("yes")

	out := buf.String()
	if strings.Contains(out, "nope") {
		t.Fatalf("debug/info should be filtered, got: %s", out)
	}
	if !strings.Contains(out, "yes") {
		t.Fatalf("warn should pass, got: %s", out)
	}
}

