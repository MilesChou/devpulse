package logx

import (
	"io"
	"log/slog"
	"os"
	"strings"
)

// Format selects the slog handler shape.
type Format int

const (
	FormatJSON Format = iota
	FormatText
)

// Config controls logger construction. ServiceName is attached as a
// default attribute on every record.
type Config struct {
	Level       string // "debug" | "info" | "warn" | "error"
	Format      Format
	ServiceName string
	Output      io.Writer // defaults to os.Stderr when nil
}

// New returns a structured slog logger. The returned logger is also
// installed as the package-level slog default so callers can `slog.Info`
// without explicit logger plumbing.
func New(cfg Config) *slog.Logger {
	out := cfg.Output
	if out == nil {
		out = os.Stderr
	}

	opts := &slog.HandlerOptions{Level: parseLevel(cfg.Level)}

	var handler slog.Handler
	if cfg.Format == FormatText {
		handler = slog.NewTextHandler(out, opts)
	} else {
		handler = slog.NewJSONHandler(out, opts)
	}

	logger := slog.New(handler)
	if cfg.ServiceName != "" {
		logger = logger.With(slog.String("service", cfg.ServiceName))
	}
	slog.SetDefault(logger)
	return logger
}

func parseLevel(s string) slog.Level {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "debug":
		return slog.LevelDebug
	case "warn":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}
