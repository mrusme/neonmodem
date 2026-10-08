//go:build unix

package shell

import (
	"context"
	"slices"
	"testing"
)

func TestCommandUsesTheUserShell(t *testing.T) {
	t.Setenv("SHELL", "/usr/bin/zsh")

	cmd, err := Command(context.Background(), "printf ok")
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{"/usr/bin/zsh", "-c", "printf ok"}; !slices.Equal(cmd.Args, want) {
		t.Errorf("got %q, want %q", cmd.Args, want)
	}
}

func TestCommandFallsBackToBinSh(t *testing.T) {
	t.Setenv("SHELL", "")

	cmd, err := Command(context.Background(), "printf ok")
	if err != nil {
		t.Fatal(err)
	}
	if cmd.Args[0] != "/bin/sh" {
		t.Errorf("got %q", cmd.Args)
	}
}
