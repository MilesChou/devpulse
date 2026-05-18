package cli

import (
	"io"
	"os"
)

// stdoutWriter is overridable from tests via SetStdout.
var stdoutWriter io.Writer = os.Stdout

// stdout is the writer commands print to. Centralizing makes the entire
// CLI testable without forcing each command to thread a writer through.
func stdout() io.Writer { return stdoutWriter }

// SetStdout swaps the writer. Returns the previous writer so tests can
// restore it in t.Cleanup.
func SetStdout(w io.Writer) io.Writer {
	prev := stdoutWriter
	stdoutWriter = w
	return prev
}

