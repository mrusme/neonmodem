//go:build !unix && !windows

package openwith

import (
	"errors"
	"os"
	"os/exec"
)

func prepare(*exec.Cmd) {}

func terminate(*exec.Cmd) error {
	return nil
}

func kill(cmd *exec.Cmd) error {
	err := cmd.Process.Kill()
	if errors.Is(err, os.ErrProcessDone) {
		return nil
	}
	return err
}
