package logging

import (
	"log/slog"
	"os"
)

// SetupJSON sets slog's default logger to use JSON output at the given level.
func SetupJSON(level slog.Level) {
	logger := slog.New(
		slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: level}),
	)
	slog.SetDefault(logger)
}
