package credential

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os/exec"
	"time"

	"github.com/mrusme/neonmodem/internal/shell"
)

const waitDelay = 2 * time.Second

type ExitError struct {
	Code   int
	State  string
	Stderr string
}

func (e *ExitError) Error() string {
	if e.Stderr == "" {
		return "ended with " + e.State
	}
	return "ended with " + e.State + ": " + e.Stderr
}

type TimeoutError struct {
	After time.Duration
}

func (e *TimeoutError) Error() string {
	return "didn't finish within " + formatDuration(e.After)
}

func formatDuration(d time.Duration) string {
	if d%time.Second == 0 {
		return fmt.Sprintf("%d seconds", d/time.Second)
	}
	return d.String()
}

type Shell struct {
	Stdin   io.Reader
	Stderr  io.Writer
	Timeout time.Duration

	waitDelay time.Duration
}

func (s Shell) Run(ctx context.Context, command string) (string, error) {
	timeout := s.Timeout
	if timeout <= 0 {
		timeout = Timeout
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	cmd, err := shell.Command(ctx, command)
	if err != nil {
		return "", err
	}

	stdout := &cappedBuffer{limit: maxOutput}
	stderr := &tailBuffer{limit: stderrTail}
	cmd.Stdin = s.Stdin
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	if s.Stderr != nil {
		cmd.Stderr = io.MultiWriter(s.Stderr, stderr)
	}
	cmd.WaitDelay = s.waitDelay
	if cmd.WaitDelay == 0 {
		cmd.WaitDelay = waitDelay
	}

	err = cmd.Run()
	switch {
	case err == nil, errors.Is(err, exec.ErrWaitDelay):
	case errors.Is(ctx.Err(), context.DeadlineExceeded):
		return "", &TimeoutError{After: timeout}
	case ctx.Err() != nil:
		return "", fmt.Errorf("was canceled: %w", ctx.Err())
	default:
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			return "", &ExitError{
				Code:   exitErr.ExitCode(),
				State:  exitErr.ProcessState.String(),
				Stderr: stderr.lastLine(),
			}
		}
		return "", fmt.Errorf("couldn't be started: %w", err)
	}

	return stdout.firstLine()
}
