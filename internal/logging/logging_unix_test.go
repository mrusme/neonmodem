//go:build unix

package logging

import (
	"io/fs"
	"os"
	"path/filepath"
	"testing"
)

func modeOf(t *testing.T, path string) fs.FileMode {
	t.Helper()
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	return info.Mode().Perm()
}

func TestOpenCreatesAPrivateFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "logs", "neonmodem.log")

	_, closer, err := Open(path, false)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer closer.Close()

	if mode := modeOf(t, path); mode != 0o600 {
		t.Errorf("new log file has mode %o, want 600", mode)
	}
}

func TestOpenMakesAnExistingFilePrivate(t *testing.T) {
	path := filepath.Join(t.TempDir(), "neonmodem.log")
	if err := os.WriteFile(path, []byte("earlier\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(path, 0o644); err != nil {
		t.Fatal(err)
	}

	logger, closer, err := Open(path, false)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	logger.Info("appended")
	closer.Close()

	if mode := modeOf(t, path); mode != 0o600 {
		t.Errorf("existing log file has mode %o, want 600", mode)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(data[:8]) != "earlier\n" || len(data) <= 8 {
		t.Errorf("the log wasn't appended to: %q", data)
	}
}
