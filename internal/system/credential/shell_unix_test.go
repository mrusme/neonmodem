//go:build unix

package credential

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func runShell(t *testing.T, s Shell, command string) (string, error) {
	t.Helper()
	return s.Run(context.Background(), command)
}

func useShell(t *testing.T, shell string) {
	t.Helper()
	t.Setenv("SHELL", shell)
}

func TestShellReadsTheFirstLine(t *testing.T) {
	useShell(t, "/bin/sh")
	for command, want := range map[string]string{
		`printf 'secret\nmetadata: x\n'`: "secret",
		`printf 'secret\r\n'`:            "secret",
		`printf ' sec ret \n'`:           " sec ret ",
		`printf 'secret'`:                "secret",
	} {
		got, err := runShell(t, Shell{}, command)
		if err != nil || got != want {
			t.Errorf("%s: got %q, %v; want %q", command, got, err, want)
		}
	}
}

func TestShellRejectsEmptyOutput(t *testing.T) {
	useShell(t, "/bin/sh")
	for _, command := range []string{"true", `printf '\nsecond\n'`} {
		if _, err := runShell(t, Shell{}, command); !errors.Is(err, ErrNoOutput) {
			t.Errorf("%s: got %v, want ErrNoOutput", command, err)
		}
	}
}

func TestShellReportsTheExitStatus(t *testing.T) {
	useShell(t, "/bin/sh")
	var terminal bytes.Buffer

	_, err := runShell(t, Shell{Stderr: &terminal},
		`printf 'secret\n'; echo 'Error: entry not found.' >&2; exit 3`)

	var exitErr *ExitError
	if !errors.As(err, &exitErr) {
		t.Fatalf("got %v, want an ExitError", err)
	}
	if exitErr.Code != 3 || exitErr.Stderr != "Error: entry not found." {
		t.Errorf("got %+v", exitErr)
	}
	if strings.Contains(err.Error(), "secret") {
		t.Errorf("the error contains stdout: %v", err)
	}
	if !strings.Contains(terminal.String(), "Error: entry not found.") {
		t.Errorf("stderr didn't reach the terminal: %q", terminal.String())
	}
}

func TestShellReportsAMissingShell(t *testing.T) {
	useShell(t, filepath.Join(t.TempDir(), "missing-shell"))
	_, err := runShell(t, Shell{}, "printf ok")
	if err == nil || !strings.HasPrefix(err.Error(), "couldn't be started: ") {
		t.Errorf("got %v", err)
	}
}

func TestShellFallsBackToBinSh(t *testing.T) {
	useShell(t, "")
	if got, err := runShell(t, Shell{}, "printf 'ok\n'"); err != nil || got != "ok" {
		t.Errorf("got %q, %v", got, err)
	}
}

func TestShellUsesTheUserShell(t *testing.T) {
	fake := filepath.Join(t.TempDir(), "fake-shell")
	script := "#!/bin/sh\nprintf 'ran by the user shell: %s\\n' \"$2\"\n"
	if err := os.WriteFile(fake, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	useShell(t, fake)

	got, err := runShell(t, Shell{}, "pass show x")
	if err != nil || got != "ran by the user shell: pass show x" {
		t.Errorf("got %q, %v", got, err)
	}
}

func TestShellPassesStdinAndEnvironment(t *testing.T) {
	useShell(t, "/bin/sh")
	t.Setenv("CREDENTIAL_TEST_VALUE", "from the environment")

	got, err := runShell(t, Shell{Stdin: strings.NewReader("typed\n")},
		`read -r line; printf '%s, %s\n' "$line" "$CREDENTIAL_TEST_VALUE"`)
	if err != nil || got != "typed, from the environment" {
		t.Errorf("got %q, %v", got, err)
	}
}

func TestShellLimitsTheOutput(t *testing.T) {
	useShell(t, "/bin/sh")
	if _, err := runShell(t, Shell{}, "head -c 70000 /dev/zero | tr '\\0' a"); !errors.Is(err, ErrOutputTooLong) {
		t.Errorf("got %v, want ErrOutputTooLong", err)
	}
	got, err := runShell(t, Shell{}, "printf 'ok\\n'; head -c 70000 /dev/zero | tr '\\0' a")
	if err != nil || got != "ok" {
		t.Errorf("got %q, %v", got, err)
	}
}

func TestShellStopsAfterTheTimeout(t *testing.T) {
	useShell(t, "/bin/sh")
	start := time.Now()

	_, err := runShell(t, Shell{Timeout: 200 * time.Millisecond}, "sleep 5")

	if _, ok := errors.AsType[*TimeoutError](err); !ok {
		t.Fatalf("got %v, want a TimeoutError", err)
	}
	if err.Error() != "didn't finish within 200ms" {
		t.Errorf("got %q", err)
	}
	if elapsed := time.Since(start); elapsed > 3*time.Second {
		t.Errorf("the command ran for %s", elapsed)
	}
}

func TestShellDoesNotWaitForBackgroundProcesses(t *testing.T) {
	useShell(t, "/bin/sh")
	start := time.Now()

	got, err := runShell(t, Shell{waitDelay: 200 * time.Millisecond}, "sleep 3 & printf 'secret\\n'")
	if err != nil || got != "secret" {
		t.Errorf("got %q, %v", got, err)
	}
	if elapsed := time.Since(start); elapsed > 2*time.Second {
		t.Errorf("waited %s for a background process", elapsed)
	}
}

func TestShellReportsCancellation(t *testing.T) {
	useShell(t, "/bin/sh")
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := Shell{}.Run(ctx, "sleep 5")
	if !errors.Is(err, context.Canceled) {
		t.Errorf("got %v", err)
	}
}

func TestTimeoutMessage(t *testing.T) {
	if got := (&TimeoutError{After: Timeout}).Error(); got != "didn't finish within 60 seconds" {
		t.Errorf("got %q", got)
	}
}
