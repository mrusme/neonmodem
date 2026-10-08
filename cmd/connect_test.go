package cmd

import (
	"bytes"
	"context"
	"log/slog"
	"maps"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mrusme/neonmodem/internal/config"
	"github.com/mrusme/neonmodem/internal/system"
	"github.com/mrusme/neonmodem/internal/system/prompt"
	"github.com/mrusme/neonmodem/internal/system/systemtest"
)

func TestValidateSysURL(t *testing.T) {
	valid := []string{
		"",
		"http://localhost:3001",
		"https://www.keebtalk.com",
		"https://example.com/forum",
	}
	for _, sysURL := range valid {
		if err := validateSysURL(sysURL); err != nil {
			t.Errorf("expected %q to be valid, got: %v", sysURL, err)
		}
	}

	invalid := []string{
		"localhost:3001",
		"example.com",
		"ftp://example.com",
		"://example.com",
		"https://",
	}
	for _, sysURL := range invalid {
		if err := validateSysURL(sysURL); err == nil {
			t.Errorf("expected %q to be rejected, got no error", sysURL)
		}
	}
}

func connectApp(t *testing.T, systems ...config.SystemConfig) (*app, string) {
	t.Helper()

	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	dir := t.TempDir()
	path := filepath.Join(dir, config.FileName)

	initial, err := config.LoadFrom(nil, dir)
	if err != nil {
		t.Fatal(err)
	}
	initial.Systems = systems
	doc, err := initial.Document()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, doc, 0o600); err != nil {
		t.Fatal(err)
	}

	cfg, err := config.LoadFrom([]string{path}, dir)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Path() != path {
		t.Fatalf("the configuration would be saved to %q", cfg.Path())
	}

	return &app{cfg: cfg, logger: slog.New(slog.DiscardHandler)}, path
}

func runConnect(t *testing.T, a *app, input string, sysType string, sysURL string) (string, error) {
	t.Helper()

	var out bytes.Buffer
	term := prompt.NewTerminal(strings.NewReader(input), &out, systemtest.Commands{})
	err := a.connect(context.Background(), term, &out, sysType, sysURL)
	return out.String(), err
}

func readFile(t *testing.T, path string) []byte {
	t.Helper()

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func lemmyAt(sysURL string, credentials map[string]string) config.SystemConfig {
	return config.SystemConfig{
		Type:     "lemmy",
		Settings: system.Settings{URL: sysURL, Credentials: credentials},
	}
}

func TestConnectKeepsAnExistingConnection(t *testing.T) {
	existing := lemmyAt("http://lemmy.example", map[string]string{"username": "vera", "password": "old"})
	existing.Type = "Lemmy"
	a, path := connectApp(t, existing)
	before := readFile(t, path)

	out, err := runConnect(t, a, "1\n", "lemmy", "http://lemmy.example/")
	if err != nil {
		t.Fatalf("connect: %v\n%s", err, out)
	}

	if !strings.Contains(out, "http://lemmy.example is already connected.") ||
		!strings.Contains(out, "Nothing changed.") {
		t.Errorf("unexpected output:\n%s", out)
	}
	if !bytes.Equal(readFile(t, path), before) {
		t.Error("keeping the connection changed the file")
	}
}

func TestConnectReplacesAConnectionInPlace(t *testing.T) {
	news := config.SystemConfig{Type: "hackernews", Settings: system.Settings{URL: "https://news.ycombinator.com"}}
	other := lemmyAt("https://other.example", map[string]string{"username": "vera", "token_cmd": "pass show other"})
	a, path := connectApp(t,
		news,
		lemmyAt("http://lemmy.example", map[string]string{"username": "vera", "password_cmd": "pass show lemmy"}),
		other,
	)

	out, err := runConnect(t, a, "2\n3\nlocal\n", "lemmy", "http://lemmy.example")
	if err != nil {
		t.Fatalf("connect: %v\n%s", err, out)
	}
	if !strings.Contains(out, "Successfully replaced the connection!") ||
		!strings.Contains(out, "The previous credentials stay valid until you revoke them on lemmy.example.") {
		t.Errorf("unexpected output:\n%s", out)
	}

	saved, err := config.LoadFrom([]string{path}, filepath.Dir(path))
	if err != nil {
		t.Fatal(err)
	}
	if len(saved.Systems) != 3 {
		t.Fatalf("got %d systems", len(saved.Systems))
	}
	replaced := saved.Systems[1]
	if replaced.Type != "lemmy" || replaced.Settings.URL != "http://lemmy.example" ||
		len(replaced.Settings.Credentials) != 0 || replaced.Settings.Options["listing"] != "local" {
		t.Errorf("the replaced entry is %+v", replaced)
	}
	if saved.Systems[0].Type != "hackernews" ||
		!maps.Equal(saved.Systems[2].Settings.Credentials, other.Settings.Credentials) {
		t.Errorf("the other systems changed: %+v", saved.Systems)
	}
}

func TestConnectReplacesTheOnlyHackerNewsConnection(t *testing.T) {
	a, _ := connectApp(t, config.SystemConfig{
		Type: "hackernews",
		Settings: system.Settings{
			URL:         "https://news.ycombinator.com",
			Credentials: map[string]string{"username": "vera", "password": "old"},
		},
	})

	out, err := runConnect(t, a, "2\n3\n", "hackernews", "")
	if err != nil {
		t.Fatalf("connect: %v\n%s", err, out)
	}
	if !strings.Contains(out, "Hacker News is already connected.") {
		t.Errorf("unexpected output:\n%s", out)
	}
	if len(a.cfg.Systems) != 1 || len(a.cfg.Systems[0].Settings.Credentials) != 0 {
		t.Errorf("got systems %+v", a.cfg.Systems)
	}
}

func TestAFailedReplaceLeavesTheFileUnchanged(t *testing.T) {
	a, path := connectApp(t, lemmyAt("http://lemmy.example", map[string]string{"username": "vera", "password": "old"}))
	before := readFile(t, path)

	_, err := runConnect(t, a, "2\n3\nnonsense\n", "lemmy", "http://lemmy.example")
	if err == nil || !strings.Contains(err.Error(), "is not a valid feed") {
		t.Fatalf("got %v", err)
	}
	if !bytes.Equal(readFile(t, path), before) {
		t.Error("the failed connect changed the file")
	}
	if a.cfg.Systems[0].Settings.Credential("password") != "old" {
		t.Errorf("the failed connect changed the entry in memory: %+v", a.cfg.Systems[0])
	}
}

func TestConnectAddsANewConnection(t *testing.T) {
	a, path := connectApp(t, lemmyAt("http://lemmy.example", nil))

	out, err := runConnect(t, a, "3\n\n", "lemmy", "https://other.example")
	if err != nil {
		t.Fatalf("connect: %v\n%s", err, out)
	}
	if strings.Contains(out, "already connected") ||
		!strings.Contains(out, "Successfully added new connection!") ||
		strings.Contains(out, "previous credentials") {
		t.Errorf("unexpected output:\n%s", out)
	}

	saved, err := config.LoadFrom([]string{path}, filepath.Dir(path))
	if err != nil {
		t.Fatal(err)
	}
	if len(saved.Systems) != 2 || saved.Systems[1].Settings.URL != "https://other.example" {
		t.Errorf("got systems %+v", saved.Systems)
	}
}

func TestConnectLeavesInMemoryChangesOutOfTheFile(t *testing.T) {
	a, path := connectApp(t, lemmyAt("http://lemmy.example", nil))
	a.cfg.Debug = true
	a.cfg.RenderSplash = false

	out, err := runConnect(t, a, "3\n\n", "lemmy", "https://other.example")
	if err != nil {
		t.Fatalf("connect: %v\n%s", err, out)
	}

	doc := string(readFile(t, path))
	if strings.Contains(doc, "Debug") || strings.Contains(doc, "RenderSplash") {
		t.Errorf("the flag and the override reached the file:\n%s", doc)
	}
	if !strings.Contains(doc, "url = 'https://other.example'") {
		t.Errorf("the new connection is missing:\n%s", doc)
	}
}

func TestConnectPrintsTheSnippetWhenTheFileCantBeWritten(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root can write a read-only file")
	}
	a, path := connectApp(t, lemmyAt("http://lemmy.example", nil))
	before := readFile(t, path)
	if err := os.Chmod(path, 0o444); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(path, 0o600) })

	out, err := runConnect(t, a, "3\n\n", "lemmy", "https://other.example")
	if err == nil || !strings.Contains(err.Error(), "wasn't saved") {
		t.Fatalf("got %v\n%s", err, out)
	}
	for _, want := range []string{"couldn't be written", "Add this to it yourself", "[[Systems]]", "url = 'https://other.example'"} {
		if !strings.Contains(out, want) {
			t.Errorf("the output lacks %q:\n%s", want, out)
		}
	}
	if !bytes.Equal(readFile(t, path), before) {
		t.Error("the file changed")
	}
}
