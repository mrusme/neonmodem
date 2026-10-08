package prompt

import (
	"context"
	"errors"
	"io"
	"strings"
	"testing"
)

type fakeRunner struct {
	outputs map[string]string
	errs    map[string]error
	calls   []string
}

func (f *fakeRunner) Run(_ context.Context, command string) (string, error) {
	f.calls = append(f.calls, command)
	if err, ok := f.errs[command]; ok {
		return "", err
	}
	return f.outputs[command], nil
}

var (
	username = Field{Name: "username", Question: "Please enter your username", NoAccount: true}
	password = Field{Name: "password", Question: "Please enter your password", Secret: true}
	apiKey   = Field{Name: "user API key", Secret: true}
)

func TestCredentialTypedValue(t *testing.T) {
	term, printed := testTerminal("\nvera\n", io.EOF)

	answer, err := term.Credential(context.Background(), username)
	if err != nil {
		t.Fatalf("Credential: %v", err)
	}
	if answer != (Answer{Value: "vera"}) {
		t.Errorf("got %+v", answer)
	}

	want := "How do you want to provide your username?\n" +
		"  1) Enter it now\n" +
		"  2) Read it from a command\n" +
		"  3) No account, read-only access\n" +
		"Please choose (press Enter for 1): Please enter your username: "
	if printed.String() != want {
		t.Errorf("unexpected prompt:\n%q\nwant\n%q", printed.String(), want)
	}
}

func TestCredentialTypedSecret(t *testing.T) {
	term, _ := testTerminal("1\n", io.EOF, "hunter2")

	answer, err := term.Credential(context.Background(), password)
	if err != nil || answer != (Answer{Value: "hunter2"}) {
		t.Errorf("got %+v, %v", answer, err)
	}
}

func TestCredentialFromCommand(t *testing.T) {
	term, printed := testTerminal("2\npass show lemmy/username\n", io.EOF)
	runner := &fakeRunner{outputs: map[string]string{"pass show lemmy/username": "vera"}}
	term.runner = runner

	answer, err := term.Credential(context.Background(), username)
	if err != nil {
		t.Fatalf("Credential: %v", err)
	}
	if answer != (Answer{Value: "vera", Command: "pass show lemmy/username"}) {
		t.Errorf("got %+v", answer)
	}
	if !strings.Contains(printed.String(), `The command printed "vera".`) {
		t.Errorf("a username read from a command should be shown:\n%s", printed)
	}
}

func TestSecretFromCommandIsNotShown(t *testing.T) {
	term, printed := testTerminal("2\npass show lemmy/password\n", io.EOF)
	term.runner = &fakeRunner{outputs: map[string]string{"pass show lemmy/password": "hunter2"}}

	answer, err := term.Credential(context.Background(), password)
	if err != nil || answer.Value != "hunter2" || answer.Command != "pass show lemmy/password" {
		t.Fatalf("got %+v, %v", answer, err)
	}
	if strings.Contains(printed.String(), "hunter2") {
		t.Errorf("the password was printed:\n%s", printed)
	}
}

func TestFailingCommandAsksAgain(t *testing.T) {
	term, printed := testTerminal("2\npass show typo\n1\n", io.EOF, "typed")
	term.runner = &fakeRunner{errs: map[string]error{
		"pass show typo": errors.New("ended with exit status 1: not in the password store"),
	}}

	answer, err := term.Credential(context.Background(), password)
	if err != nil || answer != (Answer{Value: "typed"}) {
		t.Fatalf("got %+v, %v", answer, err)
	}
	out := printed.String()
	if !strings.Contains(out, "The command failed: ended with exit status 1: not in the password store") {
		t.Errorf("the failure wasn't shown:\n%s", out)
	}
	if strings.Count(out, "How do you want to provide your password?") != 2 {
		t.Errorf("the choice wasn't asked again:\n%s", out)
	}
}

func TestNoAccountOption(t *testing.T) {
	term, _ := testTerminal("3\n", io.EOF)
	answer, err := term.Credential(context.Background(), username)
	if err != nil || answer != (Answer{NoAccount: true}) {
		t.Errorf("got %+v, %v", answer, err)
	}

	required := Field{Name: "username", Question: "Please enter your username"}
	term, printed := testTerminal("3\n1\nvera\n", io.EOF)
	answer, err = term.Credential(context.Background(), required)
	if err != nil || answer != (Answer{Value: "vera"}) {
		t.Errorf("got %+v, %v", answer, err)
	}
	if strings.Contains(printed.String(), "No account") || !strings.Contains(printed.String(), "Invalid input") {
		t.Errorf("a system that needs an account must not offer option 3:\n%s", printed)
	}
}

func TestTypedUsernameNeedsAValue(t *testing.T) {
	term, printed := testTerminal("1\n\nvera\n", io.EOF)
	answer, err := term.Credential(context.Background(), username)
	if err != nil || answer != (Answer{Value: "vera"}) {
		t.Errorf("got %+v, %v", answer, err)
	}
	if !strings.Contains(printed.String(), "Invalid input") {
		t.Errorf("an empty username should be rejected:\n%s", printed)
	}
}

func TestCredentialEndOfInput(t *testing.T) {
	term, _ := testTerminal("", io.EOF)
	_, err := term.Credential(context.Background(), username)
	if err == nil || err.Error() != "could not read username: no input" {
		t.Errorf("got %v", err)
	}
}

func TestCanceledCommandEndsTheQuestion(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	term, _ := testTerminal("2\npass show x\n", io.EOF)
	term.runner = &fakeRunner{errs: map[string]error{"pass show x": ctx.Err()}}

	if _, err := term.Credential(ctx, password); !errors.Is(err, context.Canceled) {
		t.Errorf("got %v", err)
	}
}

func TestGeneratedValueStoredInTheFile(t *testing.T) {
	term, printed := testTerminal("\n", io.EOF)
	answer, err := term.Generated(context.Background(), apiKey, "generated-key")
	if err != nil || answer != (Answer{Value: "generated-key"}) {
		t.Errorf("got %+v, %v", answer, err)
	}
	if strings.Contains(printed.String(), "generated-key") {
		t.Errorf("the key was shown without being needed:\n%s", printed)
	}
}

func TestGeneratedValueFromCommand(t *testing.T) {
	term, printed := testTerminal("2\npass show discourse\n", io.EOF)
	term.runner = &fakeRunner{outputs: map[string]string{"pass show discourse": "generated-key"}}

	answer, err := term.Generated(context.Background(), apiKey, "generated-key")
	if err != nil || answer != (Answer{Value: "generated-key", Command: "pass show discourse"}) {
		t.Fatalf("got %+v, %v", answer, err)
	}
	if strings.Count(printed.String(), "generated-key") != 1 {
		t.Errorf("the key should be shown exactly once:\n%s", printed)
	}
}

func TestGeneratedValueMismatchAsksAgain(t *testing.T) {
	term, printed := testTerminal("2\npass show old\n2\npass show new\n", io.EOF)
	term.runner = &fakeRunner{outputs: map[string]string{
		"pass show old": "an-older-key",
		"pass show new": "generated-key",
	}}

	answer, err := term.Generated(context.Background(), apiKey, "generated-key")
	if err != nil || answer.Command != "pass show new" {
		t.Fatalf("got %+v, %v", answer, err)
	}
	out := printed.String()
	if !strings.Contains(out, "The command printed a different user API key.") {
		t.Errorf("the mismatch wasn't reported:\n%s", out)
	}
	if strings.Count(out, "generated-key") != 1 || strings.Contains(out, "an-older-key") {
		t.Errorf("keys were printed more than once or the wrong key was shown:\n%s", out)
	}
}
