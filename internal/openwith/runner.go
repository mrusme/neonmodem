package openwith

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/exec"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/mrusme/neonmodem/internal/shell"
)

const (
	MaxLines    = 1000
	MaxLineSize = 4096
	GracePeriod = 2 * time.Second
	finalWait   = time.Second
)

var ErrStopped = errors.New("neonmodem is quitting")

type Runner struct {
	logger   *slog.Logger
	lines    int
	lineSize int
	grace    time.Duration

	mu      sync.Mutex
	running map[*run]struct{}
	stopped bool
}

type run struct {
	cmd    *exec.Cmd
	tree   *processTree
	active sync.WaitGroup
	ended  chan struct{}
}

func NewRunner(logger *slog.Logger) *Runner {
	if logger == nil {
		logger = slog.New(slog.DiscardHandler)
	}
	return &Runner{
		logger:   logger,
		lines:    MaxLines,
		lineSize: MaxLineSize,
		grace:    GracePeriod,
		running:  map[*run]struct{}{},
	}
}

func (r *Runner) Start(name string, line string, env []string) error {
	cmd, err := shell.Command(context.Background(), line)
	if err != nil {
		return err
	}
	cmd.Env = append(os.Environ(), env...)
	prepare(cmd)

	stdout, stdoutW, err := os.Pipe()
	if err != nil {
		return err
	}
	stderr, stderrW, err := os.Pipe()
	if err != nil {
		stdout.Close()
		stdoutW.Close()
		return err
	}
	cmd.Stdout = stdoutW
	cmd.Stderr = stderrW

	r.mu.Lock()
	if r.stopped {
		r.mu.Unlock()
		closeAll(stdout, stdoutW, stderr, stderrW)
		return ErrStopped
	}
	err = cmd.Start()
	stdoutW.Close()
	stderrW.Close()
	if err != nil {
		r.mu.Unlock()
		stdout.Close()
		stderr.Close()
		return fmt.Errorf("couldn't be started: %w", err)
	}
	tree, err := attach(cmd)
	if err != nil {
		r.mu.Unlock()
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
		stdout.Close()
		stderr.Close()
		return fmt.Errorf("couldn't be started: %w", err)
	}
	current := &run{cmd: cmd, tree: tree, ended: make(chan struct{})}
	current.active.Add(3)
	r.running[current] = struct{}{}
	r.mu.Unlock()

	logger := r.logger.With("name", name, "pid", cmd.Process.Pid)
	logger.Info("open with started")

	go r.logLines(logger, current, "stdout", stdout)
	go r.logLines(logger, current, "stderr", stderr)
	go r.wait(logger, current, time.Now())
	go func() {
		current.active.Wait()
		current.tree.release()
		r.mu.Lock()
		delete(r.running, current)
		r.mu.Unlock()
		close(current.ended)
	}()

	return nil
}

func (r *Runner) wait(logger *slog.Logger, current *run, started time.Time) {
	defer current.active.Done()
	err := current.cmd.Wait()

	status := ""
	level := slog.LevelInfo
	if state := current.cmd.ProcessState; state != nil {
		status = state.String()
		if !state.Success() {
			level = slog.LevelWarn
		}
	} else if err != nil {
		status = err.Error()
		level = slog.LevelWarn
	}
	logger.Log(context.Background(), level, "open with finished",
		"status", status, "duration", time.Since(started).Round(time.Millisecond))
}

func (r *Runner) logLines(logger *slog.Logger, current *run, stream string, pipe io.ReadCloser) {
	defer current.active.Done()
	defer pipe.Close()

	reader := bufio.NewReaderSize(pipe, r.lineSize)
	logged := 0
	for {
		line, cut, err := readLine(reader)
		if len(line) > 0 {
			switch {
			case logged < r.lines:
				attrs := []any{"stream", stream, "line", string(line)}
				if cut {
					attrs = append(attrs, "cut", true)
				}
				logger.Info("open with output", attrs...)
			case logged == r.lines:
				logger.Info("open with output not logged further", "stream", stream)
			}
			logged++
		}
		if err != nil {
			return
		}
	}
}

func readLine(reader *bufio.Reader) ([]byte, bool, error) {
	line, err := reader.ReadSlice('\n')
	cut := false
	if errors.Is(err, bufio.ErrBufferFull) {
		line = bytes.Clone(line)
		cut = true
		for errors.Is(err, bufio.ErrBufferFull) {
			_, err = reader.ReadSlice('\n')
		}
		for len(line) > 0 && !utf8.Valid(line) {
			line = line[:len(line)-1]
		}
	}
	return bytes.TrimRight(line, "\r\n"), cut, err
}

func (r *Runner) Stop() {
	r.mu.Lock()
	r.stopped = true
	runs := make([]*run, 0, len(r.running))
	for current := range r.running {
		runs = append(runs, current)
	}
	r.mu.Unlock()

	if len(runs) == 0 {
		return
	}
	r.logger.Info("ending open with commands at quit", "count", len(runs))

	for _, current := range runs {
		if err := current.tree.terminate(); err != nil {
			r.logger.Warn("couldn't end an open with command", "pid", current.cmd.Process.Pid, "error", err)
		}
	}

	grace := time.NewTimer(r.grace)
	defer grace.Stop()
	for _, current := range runs {
		select {
		case <-current.ended:
			continue
		case <-grace.C:
		}
		for _, rest := range runs {
			select {
			case <-rest.ended:
			default:
				if err := rest.tree.kill(); err != nil {
					r.logger.Warn("couldn't kill an open with command", "pid", rest.cmd.Process.Pid, "error", err)
				}
			}
		}
		break
	}

	final := time.NewTimer(finalWait)
	defer final.Stop()
	for _, current := range runs {
		select {
		case <-current.ended:
		case <-final.C:
			return
		}
	}
}

func closeAll(files ...*os.File) {
	for _, f := range files {
		f.Close()
	}
}
