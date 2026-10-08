package prompt

import (
	"bytes"
	"errors"
	"io"
	"strings"
	"testing"
)

func testTerminal(input string, secretErr error, secrets ...string) (*Terminal, *bytes.Buffer) {
	printed := new(bytes.Buffer)
	t := NewTerminal(strings.NewReader(input), printed, nil)

	i := 0
	t.readSecret = func() ([]byte, error) {
		if i >= len(secrets) {
			return nil, secretErr
		}
		answer := secrets[i]
		i++
		return []byte(answer), nil
	}

	return t, printed
}

func TestLine(t *testing.T) {
	term, _ := testTerminal("vera\n", io.EOF)

	answer, err := term.Line("Please enter your username", "username")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if answer != "vera" {
		t.Fatalf("expected vera, got: %q", answer)
	}
}

func TestLineTrimsSurroundingSpaces(t *testing.T) {
	term, _ := testTerminal("  ver a \n", io.EOF)

	answer, err := term.Line("key", "key")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if answer != "ver a" {
		t.Fatalf("expected surrounding spaces to be trimmed, got: %q", answer)
	}
}

func TestLineRepeatsOnEmptyInput(t *testing.T) {
	term, printed := testTerminal("\n   \nvera\n", io.EOF)

	answer, err := term.Line("username", "username")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if answer != "vera" {
		t.Fatalf("expected vera, got: %q", answer)
	}
	if got := strings.Count(printed.String(), "Invalid input"); got != 2 {
		t.Fatalf("expected 2 rejections, got %d", got)
	}
}

// An empty stdin used to keep the prompt going until the disk filled up.
func TestLineOnEOF(t *testing.T) {
	term, printed := testTerminal("", io.EOF)

	if _, err := term.Line("username", "username"); err == nil {
		t.Fatal("expected an error at end of input, got none")
	}
	if got := strings.Count(printed.String(), "Invalid input"); got != 0 {
		t.Fatalf("expected no rejections before giving up, got %d", got)
	}
}

// Blank lines followed by end of input must terminate, not loop.
func TestLineOnEmptyLinesThenEOF(t *testing.T) {
	term, _ := testTerminal("\n\n", io.EOF)

	if _, err := term.Line("username", "username"); err == nil {
		t.Fatal("expected an error at end of input, got none")
	}
}

func TestLineKeepsOneScanner(t *testing.T) {
	term, _ := testTerminal("vera\nhup_token\n", io.EOF)

	first, err := term.Line("username", "username")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	second, err := term.Line("token", "token")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if first != "vera" || second != "hup_token" {
		t.Fatalf("expected both lines to be read, got %q and %q", first, second)
	}
}

func TestOptional(t *testing.T) {
	term, _ := testTerminal("vera\n\n", io.EOF)

	answer, err := term.Optional("username")
	if err != nil || answer != "vera" {
		t.Fatalf("expected vera, got %q (%v)", answer, err)
	}

	answer, err = term.Optional("username")
	if err != nil || answer != "" {
		t.Fatalf("expected an empty answer for an empty line, got %q (%v)", answer, err)
	}

	answer, err = term.Optional("username")
	if err != nil || answer != "" {
		t.Fatalf("expected an empty answer at end of input, got %q (%v)", answer, err)
	}
}

func TestSecret(t *testing.T) {
	term, _ := testTerminal("", io.EOF, "hunter2 ")

	answer, err := term.Secret("Please enter your password", "password")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if answer != "hunter2 " {
		t.Fatalf("expected the password verbatim, got: %q", answer)
	}
}

func TestSecretRepeatsOnEmptyInput(t *testing.T) {
	term, printed := testTerminal("", io.EOF, "", "hunter2")

	answer, err := term.Secret("password", "password")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if answer != "hunter2" {
		t.Fatalf("expected hunter2, got: %q", answer)
	}
	if got := strings.Count(printed.String(), "Invalid input"); got != 1 {
		t.Fatalf("expected 1 rejection, got %d", got)
	}
}

// term.ReadPassword fails outright when stdin isn't a terminal, which used to
// leave the prompt repeating without end.
func TestSecretOnReadError(t *testing.T) {
	term, printed := testTerminal("", errors.New("inappropriate ioctl for device"))

	if _, err := term.Secret("password", "password"); err == nil {
		t.Fatal("expected an error when the secret cannot be read, got none")
	}
	if got := strings.Count(printed.String(), "Invalid input"); got != 0 {
		t.Fatalf("expected no rejections before giving up, got %d", got)
	}
}

func TestNotice(t *testing.T) {
	term, printed := testTerminal("", io.EOF)

	term.Notice("hello")
	if printed.String() != "hello\n" {
		t.Fatalf("unexpected output: %q", printed.String())
	}
}

func TestChoose(t *testing.T) {
	term, printed := testTerminal("x\n2\n", io.EOF)

	choice, err := term.Choose("What do you want to do?", []string{"Keep it", "Replace it"})
	if err != nil || choice != 1 {
		t.Fatalf("got %d, %v", choice, err)
	}
	for _, want := range []string{"What do you want to do?\n", "  1) Keep it\n", "  2) Replace it\n", "Invalid input"} {
		if !strings.Contains(printed.String(), want) {
			t.Errorf("missing %q in %q", want, printed.String())
		}
	}
}

func TestChooseDefaultsToTheFirstOption(t *testing.T) {
	term, _ := testTerminal("\n", io.EOF)

	if choice, err := term.Choose("Pick", []string{"a", "b"}); err != nil || choice != 0 {
		t.Fatalf("got %d, %v", choice, err)
	}
}

func TestChooseOnEOF(t *testing.T) {
	term, _ := testTerminal("", io.EOF)

	_, err := term.Choose("Pick", []string{"a", "b"})
	if err == nil || err.Error() != "could not read choice: no input" {
		t.Fatalf("got %v", err)
	}
}
