//go:build unix

package browser

import (
	"bytes"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

type syncBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (b *syncBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Write(p)
}

func (b *syncBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.String()
}

func script(t *testing.T, body string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "browser.sh")
	if err := os.WriteFile(path, []byte("#!/bin/sh\n"+body), 0o700); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestOpenReturnsBeforeTheProgramEnds(t *testing.T) {
	dir := t.TempDir()
	opened := filepath.Join(dir, "opened")
	program := script(t, "printf '%s' \"$1\" > "+opened+"\nsleep 1\n")

	start := time.Now()
	if err := Open("https://example.com/post", program, nil); err != nil {
		t.Fatal(err)
	}
	if took := time.Since(start); took > 500*time.Millisecond {
		t.Errorf("Open waited for the program: %s", took)
	}

	deadline := time.Now().Add(5 * time.Second)
	for {
		data, err := os.ReadFile(opened)
		if err == nil && string(data) == "https://example.com/post" {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("the program didn't receive the URL: %q, %v", data, err)
		}
		time.Sleep(10 * time.Millisecond)
	}
}

func TestOpenLogsAFailedProgram(t *testing.T) {
	logs := &syncBuffer{}
	program := script(t, "exit 3\n")

	if err := Open("https://example.com", program, slog.New(slog.NewTextHandler(logs, nil))); err != nil {
		t.Fatal(err)
	}

	deadline := time.Now().Add(5 * time.Second)
	for !strings.Contains(logs.String(), "exit status 3") {
		if time.Now().After(deadline) {
			t.Fatalf("no warning logged: %q", logs.String())
		}
		time.Sleep(10 * time.Millisecond)
	}
}

func TestOpenReportsAMissingProgram(t *testing.T) {
	if err := Open("https://example.com", filepath.Join(t.TempDir(), "none"), nil); err == nil {
		t.Fatal("expected an error for a missing program")
	}
}
