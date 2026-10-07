package shell

import (
	"context"
	"os"
	"os/exec"
	"syscall"
)

func Command(ctx context.Context, line string) (*exec.Cmd, error) {
	comspec := os.Getenv("ComSpec")
	if comspec == "" {
		comspec = "cmd.exe"
	}

	cmd := exec.CommandContext(ctx, comspec)
	cmd.SysProcAttr = &syscall.SysProcAttr{
		CmdLine: syscall.EscapeArg(comspec) + ` /d /s /c "` + line + `"`,
	}
	return cmd, nil
}
