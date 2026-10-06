package credential

import (
	"context"
	"os"
	"os/exec"
	"syscall"
)

func shellCommand(ctx context.Context, command string) (*exec.Cmd, error) {
	comspec := os.Getenv("ComSpec")
	if comspec == "" {
		comspec = "cmd.exe"
	}

	cmd := exec.CommandContext(ctx, comspec)
	cmd.SysProcAttr = &syscall.SysProcAttr{
		CmdLine: syscall.EscapeArg(comspec) + ` /d /s /c "` + command + `"`,
	}
	return cmd, nil
}
