package logger

import (
	"fmt"
	"io"
	"log/slog"
	"strings"
)

func New(environment, level string, output io.Writer) (*slog.Logger, error) {
	var parsed slog.Level
	if err := parsed.UnmarshalText([]byte(strings.ToLower(level))); err != nil {
		return nil, fmt.Errorf("configuration LOG_LEVEL: %w", err)
	}
	opts := &slog.HandlerOptions{Level: parsed}
	if environment == "development" {
		return slog.New(slog.NewTextHandler(output, opts)), nil
	}
	return slog.New(slog.NewJSONHandler(output, opts)), nil
}
