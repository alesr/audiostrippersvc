package slognoop

import (
	"io"

	"log/slog"
)

// NoopLogger returns a logger that discards all messages.
func NoopLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}
