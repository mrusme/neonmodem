package cmd

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"strings"
	"testing"

	"github.com/mrusme/neonmodem/internal/config"
	"github.com/mrusme/neonmodem/internal/system"
	"github.com/mrusme/neonmodem/internal/system/credential"
)

type scriptedRunner struct {
	outputs map[string]string
	errs    map[string]error
}

func (r scriptedRunner) Run(_ context.Context, command string) (string, error) {
	if err, ok := r.errs[command]; ok {
		return "", err
	}
	return r.outputs[command], nil
}

func testApp(t *testing.T, systems ...config.SystemConfig) *app {
	t.Helper()

	cfg, err := config.LoadFrom(nil, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	cfg.Systems = systems

	return &app{cfg: cfg, logger: slog.New(slog.DiscardHandler)}
}

func lemmyWith(credentials map[string]string) config.SystemConfig {
	return config.SystemConfig{
		Type: "lemmy",
		Settings: system.Settings{
			URL:         "http://127.0.0.1:9",
			Credentials: credentials,
		},
	}
}

func TestLoadSystemsResolvesCommandsOutsideTheConfiguration(t *testing.T) {
	a := testApp(t, lemmyWith(map[string]string{
		"username":  "vera",
		"token_cmd": "pass show lemmy",
	}))
	runner := scriptedRunner{outputs: map[string]string{"pass show lemmy": "resolved-secret"}}

	systems, errs := a.loadSystems(context.Background(), credential.Resolver{Runner: runner})
	if len(errs) != 0 || len(systems) != 1 {
		t.Fatalf("got %d systems and errors %v", len(systems), errs)
	}
	if !systems[0].Capabilities().Has(system.CapWrite) {
		t.Error("the resolved token didn't reach the system")
	}

	stored := a.cfg.Systems[0].Settings.Credentials
	if stored["token_cmd"] != "pass show lemmy" || stored["token"] != "" {
		t.Errorf("the configuration changed: %v", stored)
	}

	doc, err := a.cfg.Document()
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(doc, []byte("resolved-secret")) {
		t.Error("the resolved token would be written to the configuration file")
	}
	if !bytes.Contains(doc, []byte("token_cmd")) {
		t.Errorf("the command is missing from the configuration file:\n%s", doc)
	}
}

func TestFailingCommandSkipsOnlyItsSystem(t *testing.T) {
	a := testApp(t,
		lemmyWith(map[string]string{"username": "vera", "token_cmd": "pass show lemmy"}),
		config.SystemConfig{Type: "hackernews"},
	)
	failure := &credential.ExitError{Code: 1, State: "exit status 1", Stderr: "not in the password store"}
	runner := scriptedRunner{errs: map[string]error{"pass show lemmy": failure}}

	systems, errs := a.loadSystems(context.Background(), credential.Resolver{Runner: runner})
	if len(systems) != 1 || systems[0].Kind() != "hackernews" {
		t.Fatalf("expected only Hacker News to load, got %d systems", len(systems))
	}
	if len(errs) != 1 {
		t.Fatalf("got errors %v", errs)
	}
	want := "lemmy (http://127.0.0.1:9): token_cmd ended with exit status 1: not in the password store"
	if errs[0].Error() != want {
		t.Errorf("got %q, want %q", errs[0], want)
	}
}

func TestUntrustedConfigurationOnlyStopsSystemsWithCommands(t *testing.T) {
	a := testApp(t,
		lemmyWith(map[string]string{"username": "vera", "token_cmd": "pass show lemmy"}),
		lemmyWith(map[string]string{"username": "vera", "token": "typed"}),
	)
	untrusted := errors.New("the configuration file is writable by every user")
	resolver := credential.Resolver{
		Runner: scriptedRunner{},
		Trust:  func() error { return untrusted },
	}

	systems, errs := a.loadSystems(context.Background(), resolver)
	if len(systems) != 1 || len(errs) != 1 {
		t.Fatalf("got %d systems and errors %v", len(systems), errs)
	}
	if !errors.Is(errs[0], untrusted) || !strings.Contains(errs[0].Error(), "token_cmd not run") {
		t.Errorf("got %v", errs[0])
	}
}

func TestOlderLemmyConnectionsLoadWithoutRunningThePasswordCommand(t *testing.T) {
	a := testApp(t, lemmyWith(map[string]string{
		"username":     "vera",
		"password":     "typed",
		"password_cmd": "pass show lemmy",
	}))
	ran := errors.New("the password command ran")
	runner := scriptedRunner{errs: map[string]error{"pass show lemmy": ran}}

	systems, errs := a.loadSystems(context.Background(), credential.Resolver{Runner: runner})
	if len(errs) != 0 || len(systems) != 1 {
		t.Fatalf("got %d systems and errors %v", len(systems), errs)
	}

	_, err := systems[0].ListPosts(context.Background(), "", system.OrderNew)
	if !errors.Is(err, system.ErrNeedsConnect) ||
		!strings.Contains(err.Error(), "neonmodem connect --type lemmy --url http://127.0.0.1:9") {
		t.Errorf("got %v", err)
	}
}

func TestUnknownSystemTypeIsReported(t *testing.T) {
	a := testApp(t, config.SystemConfig{Type: "usenet"})

	systems, errs := a.loadSystems(context.Background(), credential.Resolver{Runner: scriptedRunner{}})
	if len(systems) != 0 || len(errs) != 1 || !strings.Contains(errs[0].Error(), "unknown system type") {
		t.Errorf("got %d systems and errors %v", len(systems), errs)
	}
}

func TestStartOrderFromTheSortSetting(t *testing.T) {
	a := testApp(t)

	for _, tc := range []struct {
		setting string
		order   system.Order
		notice  bool
	}{
		{"", system.OrderNew, false},
		{"new", system.OrderNew, false},
		{"Top-Week", system.OrderTopWeek, false},
		{"best", system.OrderNew, true},
	} {
		a.cfg.Sort = tc.setting
		order, notice := a.startOrder()
		if order != tc.order || (notice != "") != tc.notice {
			t.Errorf("Sort %q: got %q and notice %q", tc.setting, order, notice)
		}
	}
}

func TestOpenWithCommandsLeaveOutIncompleteEntries(t *testing.T) {
	a := testApp(t)
	a.cfg.OpenWith = []config.OpenWith{
		{Name: "Save page", Cmd: "wget x"},
		{Name: "Broken"},
	}

	commands, notices := a.openWithCommands()
	if len(commands) != 1 || commands[0].Name != "Save page" {
		t.Errorf("got %+v", commands)
	}
	if len(notices) != 1 || !strings.Contains(notices[0], "entry 2") {
		t.Errorf("got %q", notices)
	}
}

func TestNoOpenWithCommandsMeansNoNotices(t *testing.T) {
	commands, notices := testApp(t).openWithCommands()
	if commands != nil || notices != nil {
		t.Errorf("got %+v and %q", commands, notices)
	}
}
