//go:build unix

package openwith

import (
	"errors"
	"os/exec"
	"syscall"
)

type processTree struct {
	group int
}

func prepare(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
}

func attach(cmd *exec.Cmd) (*processTree, error) {
	if cmd.Process.Pid <= 0 {
		return nil, errors.New("the shell has no process ID")
	}
	return &processTree{group: cmd.Process.Pid}, nil
}

func (t *processTree) terminate() error {
	return t.signal(syscall.SIGTERM)
}

func (t *processTree) kill() error {
	return t.signal(syscall.SIGKILL)
}

func (t *processTree) release() {}

func (t *processTree) signal(sig syscall.Signal) error {
	err := syscall.Kill(-t.group, sig)
	if errors.Is(err, syscall.ESRCH) {
		return nil
	}
	return err
}
