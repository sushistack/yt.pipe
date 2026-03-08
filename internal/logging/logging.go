// Package logging provides structured logging setup using log/slog.
package logging

import (
	"io"
	"log/slog"
	"os"
)

// Setup initializes the global slog logger based on config level and format.
// level: "debug", "info", "warn", "error"
// format: "json" or "text"
func Setup(level, format string) *slog.Logger {
	return SetupWithWriter(level, format, os.Stderr)
}

// SetupWithWriter initializes an slog logger writing to the specified writer.
func SetupWithWriter(level, format string, w io.Writer) *slog.Logger {
	var lvl slog.Level
	switch level {
	case "debug":
		lvl = slog.LevelDebug
	case "warn":
		lvl = slog.LevelWarn
	case "error":
		lvl = slog.LevelError
	default:
		lvl = slog.LevelInfo
	}

	opts := &slog.HandlerOptions{Level: lvl}

	var handler slog.Handler
	if format == "text" {
		handler = slog.NewTextHandler(w, opts)
	} else {
		handler = slog.NewJSONHandler(w, opts)
	}

	logger := slog.New(handler)
	slog.SetDefault(logger)
	return logger
}
