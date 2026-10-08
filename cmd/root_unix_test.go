//go:build unix

package cmd

import (
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mrusme/neonmodem/internal/config"
)

func TestOpenWithCommandsNeedATrustedFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, config.FileName)
	if err := os.WriteFile(path, []byte("[[OpenWith]]\nname = 'Save page'\ncmd = 'wget x'\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(path, 0o666); err != nil {
		t.Fatal(err)
	}
	cfg, err := config.LoadFrom([]string{path}, dir)
	if err != nil {
		t.Fatal(err)
	}
	a := &app{cfg: cfg, logger: slog.New(slog.DiscardHandler)}

	commands, notices := a.openWithCommands()
	if commands != nil {
		t.Errorf("commands from a world-writable file: %+v", commands)
	}
	if len(notices) != 1 || !strings.Contains(notices[0], "Open with commands are turned off") {
		t.Errorf("got %q", notices)
	}
}
