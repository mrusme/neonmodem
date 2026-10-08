//go:build unix

package config

import (
	"io/fs"
	"os"
	"strings"
	"syscall"
	"testing"
	"time"
)

type fakeInfo struct {
	mode fs.FileMode
	sys  any
}

func (f fakeInfo) Name() string       { return FileName }
func (f fakeInfo) Size() int64        { return 0 }
func (f fakeInfo) Mode() fs.FileMode  { return f.mode }
func (f fakeInfo) ModTime() time.Time { return time.Time{} }
func (f fakeInfo) IsDir() bool        { return false }
func (f fakeInfo) Sys() any           { return f.sys }

func TestCommandsAllowedChecksTheLoadedFile(t *testing.T) {
	path := writeFile(t, t.TempDir(), "Debug = true\n")

	for _, tc := range []struct {
		mode    fs.FileMode
		allowed bool
	}{
		{0o600, true},
		{0o644, true},
		{0o660, true},
		{0o666, false},
		{0o602, false},
	} {
		if err := os.Chmod(path, tc.mode); err != nil {
			t.Fatal(err)
		}
		cfg, err := LoadFrom([]string{path}, "/cache")
		if err != nil {
			t.Fatalf("LoadFrom: %v", err)
		}

		err = cfg.CommandsAllowed()
		if tc.allowed && err != nil {
			t.Errorf("mode %o: unexpected error: %v", tc.mode, err)
		}
		if !tc.allowed && (err == nil || !strings.Contains(err.Error(), "writable by every user")) {
			t.Errorf("mode %o: expected the file to be rejected, got %v", tc.mode, err)
		}
	}
}

func TestCheckOwnershipRejectsAnotherOwner(t *testing.T) {
	foreign := fakeInfo{mode: 0o600, sys: &syscall.Stat_t{Uid: 4242}}
	err := checkOwnership("/cfg", foreign, 1000)
	if err == nil || !strings.Contains(err.Error(), "user ID 4242") {
		t.Errorf("expected a foreign owner to be rejected, got %v", err)
	}

	own := fakeInfo{mode: 0o600, sys: &syscall.Stat_t{Uid: 1000}}
	if err := checkOwnership("/cfg", own, 1000); err != nil {
		t.Errorf("own file rejected: %v", err)
	}

	unknown := fakeInfo{mode: 0o600}
	if err := checkOwnership("/cfg", unknown, 1000); err != nil {
		t.Errorf("file without owner information rejected: %v", err)
	}
}

func TestCheckOwnershipAcceptsRoot(t *testing.T) {
	store := fakeInfo{mode: 0o444, sys: &syscall.Stat_t{Uid: 0}}
	if err := checkOwnership("/nix/store/abc-neonmodem.toml", store, 1000); err != nil {
		t.Errorf("a read-only file owned by root was rejected: %v", err)
	}

	writable := fakeInfo{mode: 0o666, sys: &syscall.Stat_t{Uid: 0}}
	if err := checkOwnership("/cfg", writable, 1000); err == nil {
		t.Error("a root-owned file that every user can write to was accepted")
	}

	foreign := fakeInfo{mode: 0o600, sys: &syscall.Stat_t{Uid: 1000}}
	if err := checkOwnership("/cfg", foreign, 0); err == nil {
		t.Error("root running with another user's file should be rejected")
	}
}
