package logging

import (
	"io"
	"log/slog"
	"os"
	"path/filepath"
)

func Open(path string, debug bool) (*slog.Logger, io.Closer, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, nil, err
	}

	f, err := os.OpenFile(path, os.O_WRONLY|os.O_APPEND|os.O_CREATE, 0o644)
	if err != nil {
		return nil, nil, err
	}

	level := slog.LevelInfo
	if debug {
		level = slog.LevelDebug
	}

	handler := slog.NewTextHandler(f, &slog.HandlerOptions{
		Level:     level,
		AddSource: debug,
	})

	return slog.New(handler), f, nil
}

func Discard() *slog.Logger {
	return slog.New(slog.DiscardHandler)
}
