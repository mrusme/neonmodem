//go:build windows

package openwith

import (
	"log/slog"
	"os/exec"
	"strconv"
	"strings"
	"testing"
	"time"
)

func newTestRunner(t *testing.T) (*Runner, *logBuffer) {
	t.Helper()

	logs := &logBuffer{}
	r := NewRunner(slog.New(slog.NewJSONHandler(logs, nil)))
	t.Cleanup(r.Stop)
	return r, logs
}

func processes(t *testing.T, filter string) []int {
	t.Helper()

	out, err := exec.Command("powershell", "-NoProfile", "-Command",
		`Get-CimInstance Win32_Process -Filter "`+filter+`" | ForEach-Object { $_.ProcessId }`).Output()
	if err != nil {
		t.Fatalf("listing processes: %v", err)
	}

	var pids []int
	for field := range strings.FieldsSeq(string(out)) {
		pid, err := strconv.Atoi(field)
		if err != nil {
			t.Fatalf("unexpected process list %q", out)
		}
		pids = append(pids, pid)
	}
	return pids
}

func children(t *testing.T, pid int) []int {
	return processes(t, "ParentProcessId="+strconv.Itoa(pid))
}

func alive(t *testing.T, pid int) bool {
	return len(processes(t, "ProcessId="+strconv.Itoa(pid))) > 0
}

func TestRunnerLogsBothStreamsAndTheStatus(t *testing.T) {
	r, logs := newTestRunner(t)

	if err := r.Start("Both", "echo out & echo err 1>&2 & exit 3", nil); err != nil {
		t.Fatal(err)
	}
	waitFor(t, "the end record", func() bool { return len(logs.find(t, "open with finished")) == 1 })
	waitFor(t, "both lines", func() bool { return len(logs.find(t, "open with output")) == 2 })

	finished := logs.find(t, "open with finished")[0]
	if finished["status"] != "exit status 3" || finished["level"] != "WARN" || finished["name"] != "Both" {
		t.Errorf("got %v", finished)
	}
	output := logs.find(t, "open with output")
	if out, errs := lines(output, "stdout"), lines(output, "stderr"); len(out) != 1 || strings.TrimSpace(out[0]) != "out" ||
		len(errs) != 1 || strings.TrimSpace(errs[0]) != "err" {
		t.Errorf("got %v", output)
	}
}

func TestStopEndsTheProcessTree(t *testing.T) {
	r, logs := newTestRunner(t)

	if err := r.Start("Ping", "ping -n 30 127.0.0.1 > NUL", nil); err != nil {
		t.Fatal(err)
	}
	waitFor(t, "the start", func() bool { return len(logs.find(t, "open with started")) == 1 })
	pid := int(logs.find(t, "open with started")[0]["pid"].(float64))
	waitFor(t, "the ping process", func() bool { return len(children(t, pid)) > 0 })

	start := time.Now()
	r.Stop()
	if took := time.Since(start); took > GracePeriod+finalWait+time.Second {
		t.Errorf("stopping took %s", took)
	}

	waitFor(t, "the shell to end", func() bool { return !alive(t, pid) })
	waitFor(t, "the ping process to end", func() bool { return len(children(t, pid)) == 0 })
	if finished := logs.find(t, "open with finished"); len(finished) != 1 {
		t.Errorf("got %v", finished)
	}
}
