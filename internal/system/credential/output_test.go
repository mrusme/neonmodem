package credential

import (
	"errors"
	"strings"
	"testing"
)

func TestCappedBufferKeepsTheFirstLine(t *testing.T) {
	for _, tc := range []struct {
		writes []string
		want   string
		err    error
	}{
		{[]string{"secret\n", "metadata\n"}, "secret", nil},
		{[]string{"sec", "ret\r\n"}, "secret", nil},
		{[]string{" sec ret \n"}, " sec ret ", nil},
		{[]string{"secret"}, "secret", nil},
		{[]string{""}, "", ErrNoOutput},
		{[]string{"\nsecond\n"}, "", ErrNoOutput},
		{[]string{"\r\n"}, "", ErrNoOutput},
	} {
		b := &cappedBuffer{limit: maxOutput}
		for _, w := range tc.writes {
			if n, err := b.Write([]byte(w)); err != nil || n != len(w) {
				t.Fatalf("Write(%q) = %d, %v", w, n, err)
			}
		}
		got, err := b.firstLine()
		if got != tc.want || !errors.Is(err, tc.err) {
			t.Errorf("%q: got %q, %v; want %q, %v", tc.writes, got, err, tc.want, tc.err)
		}
	}
}

func TestCappedBufferLimit(t *testing.T) {
	b := &cappedBuffer{limit: 8}
	n, err := b.Write([]byte("0123456789"))
	if err != nil || n != 10 {
		t.Fatalf("Write accepted %d bytes, %v", n, err)
	}
	if len(b.data) != 8 || !b.overflow {
		t.Errorf("kept %d bytes, overflow %v", len(b.data), b.overflow)
	}
	if _, err := b.firstLine(); !errors.Is(err, ErrOutputTooLong) {
		t.Errorf("got %v, want ErrOutputTooLong", err)
	}

	b = &cappedBuffer{limit: 8}
	_, _ = b.Write([]byte("ok\n0123456789"))
	if got, err := b.firstLine(); got != "ok" || err != nil {
		t.Errorf("a complete first line before the limit: got %q, %v", got, err)
	}
}

func TestTailBufferLastLine(t *testing.T) {
	for _, tc := range []struct {
		input string
		want  string
	}{
		{"gpg: decryption failed\nError: lemmy/password is not in the password store.\n",
			"Error: lemmy/password is not in the password store."},
		{"\x1b[31mfailed\x1b[0m\n", "failed"},
		{"\x1b]0;title\x07done\n", "done"},
		{"loading...\rnot found\n\n", "not found"},
		{"a\tb\x00c\n", "a bc"},
		{"", ""},
	} {
		b := &tailBuffer{limit: stderrTail}
		_, _ = b.Write([]byte(tc.input))
		if got := b.lastLine(); got != tc.want {
			t.Errorf("%q: got %q, want %q", tc.input, got, tc.want)
		}
	}
}

func TestTailBufferKeepsTheEnd(t *testing.T) {
	b := &tailBuffer{limit: 16}
	for range 100 {
		_, _ = b.Write([]byte("0123456789"))
	}
	_, _ = b.Write([]byte("\nlast"))
	if len(b.data) != 16 {
		t.Errorf("kept %d bytes", len(b.data))
	}
	if got := b.lastLine(); got != "last" {
		t.Errorf("got %q", got)
	}

	long := &tailBuffer{limit: stderrTail}
	_, _ = long.Write([]byte(strings.Repeat("x", 500)))
	if got := long.lastLine(); len(got) != stderrLineMax+3 || !strings.HasSuffix(got, "...") {
		t.Errorf("a long line isn't shortened: %d characters", len(got))
	}
}
