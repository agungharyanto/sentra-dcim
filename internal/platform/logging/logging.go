// Package logging builds the application's structured JSON logger.
package logging

import (
	"io"
	"log/slog"
)

// New returns a JSON slog logger writing to w at the given level ("debug", "info", "warn", "error").
// Timestamps are always emitted in UTC.
func New(w io.Writer, level string) *slog.Logger {
	var lvl slog.Level
	_ = lvl.UnmarshalText([]byte(level))
	h := slog.NewJSONHandler(w, &slog.HandlerOptions{
		Level: lvl,
		ReplaceAttr: func(_ []string, a slog.Attr) slog.Attr {
			if a.Key == slog.TimeKey && a.Value.Kind() == slog.KindTime {
				a.Value = slog.TimeValue(a.Value.Time().UTC())
			}
			return a
		},
	})
	return slog.New(h).With("service", "sentra")
}

// Discard returns a logger that drops everything (for tests).
func Discard() *slog.Logger { return slog.New(slog.NewTextHandler(io.Discard, nil)) }
