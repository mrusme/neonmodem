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

	f, err := os.OpenFile(path, os.O_WRONLY|os.O_APPEND|os.O_CREATE, 0o600)
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
	logger := slog.New(handler)

	if err := restrict(f); err != nil {
		logger.Warn("could not make the log file private", "path", path, "error", err)
	}

	return logger, f, nil
}

func restrict(f *os.File) error {
	info, err := f.Stat()
	if err != nil {
		return err
	}
	if info.Mode().Perm()&0o077 == 0 {
		return nil
	}
	return f.Chmod(0o600)
}

func Discard() *slog.Logger {
	return slog.New(slog.DiscardHandler)
}
