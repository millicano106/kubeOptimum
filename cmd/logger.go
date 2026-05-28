package cmd

import (
	"log/slog"
	"os"
)

// initLogger configures the global slog logger. In verbose mode a text handler
// at Debug level is written to stderr. Otherwise the default logger is left
// untouched (Info level, which silences all Debug calls in this codebase).
func initLogger(verbose bool) {
	if !verbose {
		return
	}
	h := slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelDebug})
	slog.SetDefault(slog.New(h))
}
