//go:build !unix && !windows

package openwith

import (
	"errors"
	"os"
	"os/exec"
)

type processTree struct {
	process *os.Process
}

func prepare(*exec.Cmd) {}

func attach(cmd *exec.Cmd) (*processTree, error) {
	if cmd.Process == nil {
		return nil, errors.New("the shell has no process")
	}
	return &processTree{process: cmd.Process}, nil
}

func (t *processTree) terminate() error {
	return nil
}

func (t *processTree) kill() error {
	err := t.process.Kill()
	if errors.Is(err, os.ErrProcessDone) {
		return nil
	}
	return err
}

func (t *processTree) release() {}
