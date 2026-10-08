package credential

import (
	"context"
	"errors"
	"log/slog"
	"maps"
	"slices"
	"strings"
	"sync"
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

type recorder struct {
	mu      sync.Mutex
	records []slog.Record
}

func (r *recorder) Enabled(context.Context, slog.Level) bool { return true }

func (r *recorder) Handle(_ context.Context, rec slog.Record) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.records = append(r.records, rec.Clone())
	return nil
}

func (r *recorder) WithAttrs([]slog.Attr) slog.Handler { return r }
func (r *recorder) WithGroup(string) slog.Handler      { return r }

func (r *recorder) warnings() []string {
	r.mu.Lock()
	defer r.mu.Unlock()

	var out []string
	for _, rec := range r.records {
		if rec.Level != slog.LevelWarn {
			continue
		}
		line := rec.Message
		rec.Attrs(func(a slog.Attr) bool {
			line += " " + a.Key + "=" + a.Value.String()
			return true
		})
		out = append(out, line)
	}
	return out
}

var userAndPassword = []string{"username", "password"}

func TestResolveRunsCommandsAndKeepsPlainValues(t *testing.T) {
	runner := &fakeRunner{outputs: map[string]string{"pass show lemmy": "hunter2"}}
	input := map[string]string{"username": "vera", "password_cmd": "pass show lemmy"}
	before := maps.Clone(input)

	got, err := Resolver{Runner: runner}.Resolve(context.Background(), nil, input, userAndPassword)
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}

	want := map[string]string{"username": "vera", "password": "hunter2"}
	if !maps.Equal(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
	if !maps.Equal(input, before) {
		t.Errorf("the input was changed: %v", input)
	}
}

func TestCommandReplacesThePlainValue(t *testing.T) {
	runner := &fakeRunner{outputs: map[string]string{"pass": "new"}}
	log := &recorder{}

	got, err := Resolver{Runner: runner}.Resolve(context.Background(), slog.New(log),
		map[string]string{"password": "old", "password_cmd": "pass"}, userAndPassword)
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if got["password"] != "new" {
		t.Errorf("the command should win, got %q", got["password"])
	}

	warnings := log.warnings()
	if len(warnings) != 1 || !strings.Contains(warnings[0], "key=password") {
		t.Errorf("expected one warning about the stored password, got %v", warnings)
	}
	for _, w := range warnings {
		if strings.Contains(w, "old") || strings.Contains(w, "new") {
			t.Errorf("a warning contains a credential value: %q", w)
		}
	}
}

func TestResolveFollowsTheDeclaredOrder(t *testing.T) {
	runner := &fakeRunner{outputs: map[string]string{"user": "vera", "pass": "pw"}}

	_, err := Resolver{Runner: runner}.Resolve(context.Background(), nil,
		map[string]string{"password_cmd": "pass", "username_cmd": "user"}, userAndPassword)
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if !slices.Equal(runner.calls, []string{"user", "pass"}) {
		t.Errorf("commands ran in the order %v", runner.calls)
	}
}

func TestUnknownKeysAreIgnoredWithAWarning(t *testing.T) {
	runner := &fakeRunner{}
	log := &recorder{}

	got, err := Resolver{Runner: runner}.Resolve(context.Background(), slog.New(log),
		map[string]string{"username": "vera", "pasword_cmd": "rm -rf /tmp/x", "nickname": "v"},
		userAndPassword)
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if len(runner.calls) != 0 {
		t.Errorf("a command for an unknown key ran: %v", runner.calls)
	}
	if !maps.Equal(got, map[string]string{"username": "vera"}) {
		t.Errorf("got %v", got)
	}

	warnings := log.warnings()
	if len(warnings) != 2 ||
		!strings.Contains(warnings[0], "key=nickname") ||
		!strings.Contains(warnings[1], "key=pasword_cmd") {
		t.Errorf("expected sorted warnings for both unknown keys, got %v", warnings)
	}
}

func TestAnEmptyCommandCountsAsUnset(t *testing.T) {
	runner := &fakeRunner{}
	got, err := Resolver{Runner: runner}.Resolve(context.Background(), nil,
		map[string]string{"password": "pw", "password_cmd": "   "}, userAndPassword)
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if got["password"] != "pw" || len(runner.calls) != 0 {
		t.Errorf("got %v after %v", got, runner.calls)
	}
}

func TestRunnerErrorsNameTheKey(t *testing.T) {
	failure := &ExitError{Code: 1, State: "exit status 1", Stderr: "not in the password store"}
	runner := &fakeRunner{errs: map[string]error{"pass": failure}}

	_, err := Resolver{Runner: runner}.Resolve(context.Background(), nil,
		map[string]string{"password_cmd": "pass"}, userAndPassword)
	if err == nil {
		t.Fatal("expected an error")
	}
	want := "password_cmd ended with exit status 1: not in the password store"
	if err.Error() != want {
		t.Errorf("got %q, want %q", err, want)
	}
	if _, ok := errors.AsType[*ExitError](err); !ok {
		t.Errorf("the runner's error isn't wrapped: %v", err)
	}
}

func TestUntrustedConfigurationBlocksCommands(t *testing.T) {
	untrusted := errors.New("/cfg is writable by every user")
	checks := 0
	resolver := Resolver{
		Runner: &fakeRunner{},
		Trust: func() error {
			checks++
			return untrusted
		},
	}

	_, err := resolver.Resolve(context.Background(), nil,
		map[string]string{"password_cmd": "pass"}, userAndPassword)
	if !errors.Is(err, untrusted) || !strings.HasPrefix(err.Error(), "password_cmd not run: ") {
		t.Errorf("got %v", err)
	}

	got, err := resolver.Resolve(context.Background(), nil,
		map[string]string{"username": "vera", "password": "pw"}, userAndPassword)
	if err != nil || got["password"] != "pw" {
		t.Errorf("plain values must not depend on the check: %v %v", got, err)
	}
	if checks != 1 {
		t.Errorf("the check ran %d times, want 1", checks)
	}
}

func TestTrustIsCheckedOncePerSystem(t *testing.T) {
	checks := 0
	resolver := Resolver{
		Runner: &fakeRunner{outputs: map[string]string{"u": "vera", "p": "pw"}},
		Trust: func() error {
			checks++
			return nil
		},
	}
	if _, err := resolver.Resolve(context.Background(), nil,
		map[string]string{"username_cmd": "u", "password_cmd": "p"}, userAndPassword); err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if checks != 1 {
		t.Errorf("the check ran %d times, want 1", checks)
	}
}
