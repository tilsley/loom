// Package logging provides a slog.Logger factory used by all loom apps.
//
// Log format is controlled by the LOG_FORMAT environment variable:
//
//	LOG_FORMAT=json    structured JSON, suitable for log aggregators (default)
//	LOG_FORMAT=text    human-readable key=value pairs, for local development
//
// Log level is controlled by LOG_LEVEL (debug, info, warn, error; default info).
package logging

import (
	"log/slog"
	"os"
	"strings"
)

// New returns a logger configured from environment variables.
func New() *slog.Logger {
	level := parseLevel(os.Getenv("LOG_LEVEL"))
	opts := &slog.HandlerOptions{Level: level}

	var handler slog.Handler
	switch strings.ToLower(os.Getenv("LOG_FORMAT")) {
	case "text", "console":
		handler = slog.NewTextHandler(os.Stdout, opts)
	default:
		handler = slog.NewJSONHandler(os.Stdout, opts)
	}

	return slog.New(handler)
}

func parseLevel(s string) slog.Level {
	switch strings.ToLower(s) {
	case "debug":
		return slog.LevelDebug
	case "warn", "warning":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}
