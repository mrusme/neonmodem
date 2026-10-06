package prompt

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"syscall"

	"golang.org/x/term"
)

type Prompter interface {
	Line(question string, field string) (string, error)
	Optional(question string) (string, error)
	Secret(question string, field string) (string, error)
	Notice(text string)
}

type Terminal struct {
	in         io.Reader
	out        io.Writer
	scanner    *bufio.Scanner
	readSecret func() ([]byte, error)
}

func NewTerminal(in io.Reader, out io.Writer) *Terminal {
	return &Terminal{
		in:  in,
		out: out,
		readSecret: func() ([]byte, error) {
			return term.ReadPassword(int(syscall.Stdin))
		},
	}
}

func Stdio() *Terminal {
	return NewTerminal(os.Stdin, os.Stdout)
}

func (t *Terminal) lineScanner() *bufio.Scanner {
	if t.scanner == nil {
		t.scanner = bufio.NewScanner(t.in)
	}
	return t.scanner
}

func (t *Terminal) read(question string) (string, error) {
	fmt.Fprintf(t.out, "%s: ", question)

	s := t.lineScanner()
	if !s.Scan() {
		if err := s.Err(); err != nil {
			return "", err
		}
		return "", io.EOF
	}

	return strings.TrimSpace(s.Text()), nil
}

func (t *Terminal) Line(question string, field string) (string, error) {
	for {
		answer, err := t.read(question)
		if err != nil {
			if errors.Is(err, io.EOF) {
				return "", fmt.Errorf("could not read %s: no input", field)
			}
			return "", fmt.Errorf("could not read %s: %w", field, err)
		}

		if answer != "" {
			return answer, nil
		}

		fmt.Fprintln(t.out, "Invalid input")
	}
}

func (t *Terminal) Optional(question string) (string, error) {
	answer, err := t.read(question)
	if err != nil {
		if errors.Is(err, io.EOF) {
			fmt.Fprintln(t.out, "")
			return "", nil
		}
		return "", err
	}

	return answer, nil
}

func (t *Terminal) Secret(question string, field string) (string, error) {
	for {
		fmt.Fprintf(t.out, "%s (will not echo): ", question)

		answer, err := t.readSecret()
		fmt.Fprintln(t.out, "")
		if err != nil {
			if errors.Is(err, io.EOF) {
				return "", fmt.Errorf("could not read %s: no input", field)
			}
			return "", fmt.Errorf("could not read %s: %w", field, err)
		}

		if len(answer) > 0 {
			return string(answer), nil
		}

		fmt.Fprintln(t.out, "Invalid input")
	}
}

func (t *Terminal) Notice(text string) {
	fmt.Fprintln(t.out, text)
}
