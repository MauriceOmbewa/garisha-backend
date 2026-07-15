// Package logger provides a structured, levelled logger built on top of
// the standard library's log/slog package.  A single *slog.Logger is
// constructed once at startup and injected wherever it is needed; there is
// no package-level global so every call site receives a logger through its
// constructor, making dependencies explicit and tests straightforward.
package logger

import (
	"io"
	"log/slog"
	"os"
	"strings"
)

// New creates and returns a *slog.Logger configured for the given
// environment.
//
//   - "production"  → JSON handler, Info level
//   - anything else → Text handler, Debug level (local/development friendly)
//
// Output is always written to w; pass os.Stdout for normal use.
func New(env string, w io.Writer) *slog.Logger {
	if w == nil {
		w = os.Stdout
	}

	var (
		level   slog.Level
		handler slog.Handler
	)

	opts := &slog.HandlerOptions{
		AddSource: false,
	}

	switch strings.ToLower(env) {
	case "production", "prod":
		level = slog.LevelInfo
		opts.Level = level
		handler = slog.NewJSONHandler(w, opts)
	default:
		level = slog.LevelDebug
		opts.Level = level
		opts.AddSource = true
		handler = slog.NewTextHandler(w, opts)
	}

	return slog.New(handler)
}
