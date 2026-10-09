package prompt

import (
	"context"
	"errors"
	"strings"
	"testing"
)

func TestCodeAsksUntilSixDigitsArrive(t *testing.T) {
	var out strings.Builder
	term := NewTerminal(strings.NewReader("12 34\nabcdef\n1234567\n 12 34 56 \n"), &out, &fakeRunner{})

	code, err := Code(term)
	if err != nil {
		t.Fatal(err)
	}
	if code != "123456" {
		t.Errorf("got %q", code)
	}
	if n := strings.Count(out.String(), "A 2FA code has six digits."); n != 3 {
		t.Errorf("the notice was shown %d times, expected 3:\n%s", n, out.String())
	}
}

func TestCodeReportsTheEndOfTheInput(t *testing.T) {
	term := NewTerminal(strings.NewReader("12\n"), &strings.Builder{}, &fakeRunner{})

	if _, err := Code(term); err == nil || !strings.Contains(err.Error(), "could not read 2FA code") {
		t.Errorf("got %v", err)
	}
}

func TestTheTerminalPrintsPrintableText(t *testing.T) {
	var out strings.Builder
	runner := &fakeRunner{
		outputs: map[string]string{"print key": "key\x1b[2Jvalue"},
		errs:    map[string]error{"broken": errors.New("failed \x1b[2J badly")},
	}
	term := NewTerminal(strings.NewReader("2\nbroken\n2\nprint key\n"), &out, runner)

	term.Notice("a \x1b]8;;https://evil.example\anotice\nsecond line")
	answer, err := term.Generated(context.Background(), apiKey, "key\x1b[2Jvalue")
	if err != nil {
		t.Fatal(err)
	}
	if answer.Value != "key\x1b[2Jvalue" || answer.Command != "print key" {
		t.Errorf("the answer is %+v", answer)
	}
	if strings.ContainsAny(out.String(), "\x1b\a") {
		t.Errorf("a control character reached the terminal:\n%q", out.String())
	}
	for _, want := range []string{"a notice\nsecond line", "The command failed: failed  badly", "\nkeyvalue\n"} {
		if !strings.Contains(out.String(), want) {
			t.Errorf("the output lacks %q:\n%s", want, out.String())
		}
	}
}
