package logging

import (
	"log/slog"
	"os"
)

// Init configures the default slog logger with JSON output to stdout.
// CloudWatch Logs parses JSON lines automatically for Insights queries.
func Init() {
	h := slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	})
	slog.SetDefault(slog.New(h))
}
