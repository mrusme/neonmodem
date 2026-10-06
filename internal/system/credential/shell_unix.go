//go:build unix

package credential

import (
	"context"
	"os"
	"os/exec"
)

func shellCommand(ctx context.Context, command string) (*exec.Cmd, error) {
	shell := os.Getenv("SHELL")
	if shell == "" {
		shell = "/bin/sh"
	}
	return exec.CommandContext(ctx, shell, "-c", command), nil
}
