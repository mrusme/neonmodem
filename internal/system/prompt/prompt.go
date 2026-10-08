package prompt

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"

	"github.com/mrusme/neonmodem/internal/system/credential"
	"golang.org/x/term"
)

type Field struct {
	Name      string
	Question  string
	Secret    bool
	NoAccount bool
}

type Answer struct {
	Value     string
	Command   string
	NoAccount bool
}

type Prompter interface {
	Line(question string, field string) (string, error)
	Optional(question string) (string, error)
	Secret(question string, field string) (string, error)
	Credential(ctx context.Context, f Field) (Answer, error)
	Generated(ctx context.Context, f Field, value string) (Answer, error)
	Choose(question string, options []string) (int, error)
	Notice(text string)
}

const (
	optionEnter     = 0
	optionNoAccount = 2
	optionStore     = 0
)

type Terminal struct {
	in         io.Reader
	out        io.Writer
	scanner    *bufio.Scanner
	readSecret func() ([]byte, error)
	runner     credential.Runner
}

func NewTerminal(in io.Reader, out io.Writer, runner credential.Runner) *Terminal {
	return &Terminal{
		in:  in,
		out: out,
		readSecret: func() ([]byte, error) {
			return term.ReadPassword(int(os.Stdin.Fd()))
		},
		runner: runner,
	}
}

func Stdio(runner credential.Runner) *Terminal {
	return NewTerminal(os.Stdin, os.Stdout, runner)
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

func readError(field string, err error) error {
	if errors.Is(err, io.EOF) {
		return fmt.Errorf("could not read %s: no input", field)
	}
	return fmt.Errorf("could not read %s: %w", field, err)
}

func (t *Terminal) Line(question string, field string) (string, error) {
	for {
		answer, err := t.read(question)
		if err != nil {
			return "", readError(field, err)
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
			return "", readError(field, err)
		}

		if len(answer) > 0 {
			return string(answer), nil
		}

		fmt.Fprintln(t.out, "Invalid input")
	}
}

func (t *Terminal) Credential(ctx context.Context, f Field) (Answer, error) {
	options := []string{"Enter it now", "Read it from a command"}
	if f.NoAccount {
		options = append(options, "No account, read-only access")
	}
	question := fmt.Sprintf("How do you want to provide your %s?", f.Name)

	for {
		choice, err := t.choose(question, options, f.Name)
		if err != nil {
			return Answer{}, err
		}

		switch choice {
		case optionEnter:
			value, err := t.enter(f)
			return Answer{Value: value}, err
		case optionNoAccount:
			return Answer{NoAccount: true}, nil
		}

		command, err := t.Line(commandQuestion(f), f.Name+" command")
		if err != nil {
			return Answer{}, err
		}
		value, ok, err := t.run(ctx, command)
		if err != nil {
			return Answer{}, err
		}
		if !ok {
			continue
		}

		if !f.Secret {
			fmt.Fprintf(t.out, "The command printed %q.\n", value)
		}
		return Answer{Value: value, Command: command}, nil
	}
}

func (t *Terminal) Generated(ctx context.Context, f Field, value string) (Answer, error) {
	options := []string{"Store it in the configuration file", "Read it from a command"}
	question := fmt.Sprintf("How do you want to store your %s?", f.Name)
	shown := false

	for {
		choice, err := t.choose(question, options, f.Name)
		if err != nil {
			return Answer{}, err
		}
		if choice == optionStore {
			return Answer{Value: value}, nil
		}

		if !shown {
			fmt.Fprintf(t.out,
				"Save this %s in your password manager now. It won't be shown again.\n%s\n",
				f.Name, value)
			shown = true
		}

		command, err := t.Line(commandQuestion(f), f.Name+" command")
		if err != nil {
			return Answer{}, err
		}
		printed, ok, err := t.run(ctx, command)
		if err != nil {
			return Answer{}, err
		}
		if !ok {
			continue
		}
		if printed != value {
			fmt.Fprintf(t.out, "The command printed a different %s.\n", f.Name)
			continue
		}

		return Answer{Value: value, Command: command}, nil
	}
}

func commandQuestion(f Field) string {
	return "Please enter the command that prints your " + f.Name
}

func (t *Terminal) enter(f Field) (string, error) {
	if f.Secret {
		return t.Secret(f.Question, f.Name)
	}
	return t.Line(f.Question, f.Name)
}

func (t *Terminal) run(ctx context.Context, command string) (string, bool, error) {
	value, err := t.runner.Run(ctx, command)
	if err == nil {
		return value, true, nil
	}
	if ctx.Err() != nil {
		return "", false, ctx.Err()
	}

	fmt.Fprintf(t.out, "The command failed: %v\n", err)
	return "", false, nil
}

func (t *Terminal) Choose(question string, options []string) (int, error) {
	return t.choose(question, options, "choice")
}

func (t *Terminal) choose(question string, options []string, field string) (int, error) {
	fmt.Fprintln(t.out, question)
	for i, option := range options {
		fmt.Fprintf(t.out, "  %d) %s\n", i+1, option)
	}

	for {
		answer, err := t.read("Please choose (press Enter for 1)")
		if err != nil {
			return 0, readError(field, err)
		}
		if answer == "" {
			return 0, nil
		}
		if n, err := strconv.Atoi(answer); err == nil && n >= 1 && n <= len(options) {
			return n - 1, nil
		}

		fmt.Fprintln(t.out, "Invalid input")
	}
}

func (t *Terminal) Notice(text string) {
	fmt.Fprintln(t.out, text)
}
