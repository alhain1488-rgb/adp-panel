// Package logging provides a structured slog logger for the panel.
package logging

import (
	"log/slog"
	"os"
	"strings"
)

// New returns a JSON structured logger writing to stdout. The level is read
// from the LOG_LEVEL env (debug|info|warn|error), defaulting to info.
func New() *slog.Logger {
	level := slog.LevelInfo
	switch strings.ToLower(os.Getenv("LOG_LEVEL")) {
	case "debug":
		level = slog.LevelDebug
	case "warn":
		level = slog.LevelWarn
	case "error":
		level = slog.LevelError
	}
	handler := slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: level})
	return slog.New(handler)
}
