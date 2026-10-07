//go:build unix

package shell

import (
	"context"
	"os"
	"os/exec"
)

func Command(ctx context.Context, line string) (*exec.Cmd, error) {
	shell := os.Getenv("SHELL")
	if shell == "" {
		shell = "/bin/sh"
	}
	return exec.CommandContext(ctx, shell, "-c", line), nil
}
