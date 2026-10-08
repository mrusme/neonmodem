//go:build unix

package openwith

import (
	"errors"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"
)

func newTestRunner(t *testing.T) (*Runner, *logBuffer) {
	t.Helper()
	t.Setenv("SHELL", "/bin/sh")

	logs := &logBuffer{}
	r := NewRunner(slog.New(slog.NewJSONHandler(logs, nil)))
	t.Cleanup(r.Stop)
	return r, logs
}

func TestRunnerLogsBothStreamsAndTheStatus(t *testing.T) {
	r, logs := newTestRunner(t)

	if err := r.Start("Both", "printf 'out\\n'; printf 'err\\n' >&2; exit 3", nil); err != nil {
		t.Fatal(err)
	}
	waitFor(t, "the end record", func() bool { return len(logs.find(t, "open with finished")) == 1 })
	waitFor(t, "both lines", func() bool { return len(logs.find(t, "open with output")) == 2 })

	finished := logs.find(t, "open with finished")[0]
	if finished["status"] != "exit status 3" || finished["level"] != "WARN" || finished["name"] != "Both" {
		t.Errorf("got %v", finished)
	}
	output := logs.find(t, "open with output")
	if out, errs := lines(output, "stdout"), lines(output, "stderr"); len(out) != 1 || out[0] != "out" ||
		len(errs) != 1 || errs[0] != "err" {
		t.Errorf("got %v", output)
	}
	started := logs.find(t, "open with started")
	if len(started) != 1 || started[0]["pid"] != finished["pid"] {
		t.Errorf("got %v", started)
	}
	for _, record := range logs.records(t) {
		if strings.Contains(record["msg"].(string), "printf") {
			t.Errorf("the command line was logged: %v", record)
		}
	}
}

func TestRunnerPassesTheVariablesWithoutRunningThem(t *testing.T) {
	r, logs := newTestRunner(t)
	dir := t.TempDir()
	marker := filepath.Join(dir, "pwned")
	subject := `quote " and $(touch ` + marker + `); touch ` + marker

	env := []string{"NM_POST_SUBJECT=" + subject, "NM_POST_ID=42"}
	if err := r.Start("Env", `printf '%s|%s\n' "$NM_POST_ID" "$NM_POST_SUBJECT"`, env); err != nil {
		t.Fatal(err)
	}
	waitFor(t, "the output", func() bool { return len(logs.find(t, "open with output")) == 1 })

	if got := logs.find(t, "open with output")[0]["line"]; got != "42|"+subject {
		t.Errorf("got %q", got)
	}
	if _, err := os.Stat(marker); !errors.Is(err, os.ErrNotExist) {
		t.Error("a value was run as code")
	}
}

func TestRunnerHasNoTerminal(t *testing.T) {
	r, logs := newTestRunner(t)

	if err := r.Start("Terminal", "test -t 0 || echo no-stdin-tty; (: > /dev/tty) 2>/dev/null || echo no-tty", nil); err != nil {
		t.Fatal(err)
	}
	waitFor(t, "the output", func() bool { return len(logs.find(t, "open with output")) == 2 })

	if got := lines(logs.find(t, "open with output"), "stdout"); strings.Join(got, ",") != "no-stdin-tty,no-tty" {
		t.Errorf("got %v", got)
	}
}

func TestRunnerLogsWhatProgramsWriteAfterTheShellEnded(t *testing.T) {
	r, logs := newTestRunner(t)

	if err := r.Start("Late", "(sleep 0.3; echo late) &", nil); err != nil {
		t.Fatal(err)
	}
	waitFor(t, "the late line", func() bool { return len(logs.find(t, "open with output")) == 1 })

	records := logs.records(t)
	finishedAt, lateAt := -1, -1
	for i, record := range records {
		switch record["msg"] {
		case "open with finished":
			finishedAt = i
		case "open with output":
			lateAt = i
		}
	}
	if finishedAt < 0 || lateAt < finishedAt {
		t.Errorf("the late line should follow the end record: %v", records)
	}
}

func TestRunnerLimitsTheOutput(t *testing.T) {
	r, logs := newTestRunner(t)
	r.lines = 3
	r.lineSize = 16

	if err := r.Start("Chatty", "for i in 1 2 3 4 5; do echo line$i; done; printf '%040d\\n' 0 >&2; echo after >&2", nil); err != nil {
		t.Fatal(err)
	}
	waitFor(t, "the end", func() bool { return len(logs.find(t, "open with finished")) == 1 })
	waitFor(t, "the stderr lines", func() bool { return len(lines(logs.find(t, "open with output"), "stderr")) == 2 })

	output := logs.find(t, "open with output")
	if got := lines(output, "stdout"); strings.Join(got, ",") != "line1,line2,line3" {
		t.Errorf("stdout: %v", got)
	}
	if limit := logs.find(t, "open with output not logged further"); len(limit) != 1 || limit[0]["stream"] != "stdout" {
		t.Errorf("limit records: %v", limit)
	}
	errs := lines(output, "stderr")
	if len(errs) != 2 || len(errs[0]) != 16 || errs[1] != "after" {
		t.Errorf("stderr: %v", errs)
	}
	for _, record := range output {
		if record["stream"] == "stderr" && strings.HasPrefix(record["line"].(string), "0000") && record["cut"] != true {
			t.Errorf("the long line isn't marked as cut: %v", record)
		}
	}
}

func alive(pid int) bool {
	return syscall.Kill(pid, 0) == nil
}

func TestStopEndsRunningCommands(t *testing.T) {
	r, logs := newTestRunner(t)

	if err := r.Start("Sleep", "sleep 30", nil); err != nil {
		t.Fatal(err)
	}
	waitFor(t, "the start", func() bool { return len(logs.find(t, "open with started")) == 1 })
	pid := int(logs.find(t, "open with started")[0]["pid"].(float64))

	start := time.Now()
	r.Stop()
	if took := time.Since(start); took > GracePeriod {
		t.Errorf("stopping took %s", took)
	}
	if alive(pid) {
		t.Error("the command still runs")
	}
	if finished := logs.find(t, "open with finished"); len(finished) != 1 || finished[0]["status"] != "signal: terminated" {
		t.Errorf("got %v", finished)
	}
	if err := r.Start("Late", "true", nil); !errors.Is(err, ErrStopped) {
		t.Errorf("a start after Stop: %v", err)
	}
}

func TestStopKillsCommandsThatIgnoreTerm(t *testing.T) {
	r, logs := newTestRunner(t)
	r.grace = 200 * time.Millisecond

	if err := r.Start("Stubborn", "trap '' TERM; sleep 30 & wait", nil); err != nil {
		t.Fatal(err)
	}
	waitFor(t, "the start", func() bool { return len(logs.find(t, "open with started")) == 1 })
	time.Sleep(100 * time.Millisecond)

	r.Stop()
	if finished := logs.find(t, "open with finished"); len(finished) != 1 || finished[0]["status"] != "signal: killed" {
		t.Errorf("got %v", finished)
	}
}

func TestStopEndsProgramsLeftByAFinishedShell(t *testing.T) {
	r, logs := newTestRunner(t)

	if err := r.Start("Background", "sleep 30 &", nil); err != nil {
		t.Fatal(err)
	}
	waitFor(t, "the shell's end", func() bool { return len(logs.find(t, "open with finished")) == 1 })
	pid := int(logs.find(t, "open with started")[0]["pid"].(float64))

	r.Stop()
	waitFor(t, "an empty process group", func() bool {
		return errors.Is(syscall.Kill(-pid, 0), syscall.ESRCH)
	})
}
