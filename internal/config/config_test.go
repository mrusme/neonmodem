package config

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/mrusme/neonmodem/internal/system"
)

const legacyFile = `Debug = false
Log = '/home/someone/.cache/neonmodem.log'
Proxy = ''
Browser = ''
RenderShadows = true
RenderImages = true
RenderSplash = false
RenderBanner = true

[[Systems]]
Type = 'hackernews'

[Systems.Config]
proxy = ''
url = 'https://news.ycombinator.com'

[Systems.Config.credentials]
password = ''
username = ''

[[Systems]]
Type = 'lobsters'

[Systems.Config]
proxy = ''
url = 'https://lobste.rs'

[Systems.Config.credentials]
[Theme]
[Theme.Header]
[Theme.Header.Selector]
Padding = [0, 1, 0, 1]
Margin = [0, 0, 0, 0]

[Theme.Header.Selector.Foreground]
Light = '#6ca1d0'
Dark = '#6ca1d0'

[Theme.Header.Selector.Border]
Sides = [true, true, true, true]

[Theme.Header.Selector.Border.Border]
Top = '─'
Bottom = '─'
Left = '│'
Right = '│'
TopLeft = '┌'
TopRight = '┐'
BottomLeft = '└'
BottomRight = '┘'
MiddleLeft = '├'
MiddleRight = '┤'
Middle = '┼'
MiddleTop = '┬'
MiddleBottom = '┴'

[Theme.Header.Spinner]
Padding = []
Margin = []

[Theme.Header.Spinner.Foreground]
Light = ''
Dark = ''

[Theme.Reply]
[Theme.Reply.Author]
Padding = [0, 1, 0, 1]
Margin = []

[Theme.Reply.Author.Foreground]
Light = '#000000'
Dark = '#00000'

[Theme.Reply.Author.Background]
Light = '#ffd500'
Dark = '#ffd500'
`

func writeFile(t *testing.T, dir string, content string) string {
	t.Helper()
	path := filepath.Join(dir, FileName)
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestLoadLegacyFile(t *testing.T) {
	dir := t.TempDir()
	path := writeFile(t, dir, legacyFile)

	cfg, err := LoadFrom([]string{path}, "/cache")
	if err != nil {
		t.Fatalf("LoadFrom: %v", err)
	}

	if cfg.Path() != path {
		t.Errorf("path %q, want %q", cfg.Path(), path)
	}
	if cfg.RenderSplash {
		t.Error("RenderSplash should come from the file (false)")
	}
	if !cfg.RenderShadows {
		t.Error("RenderShadows should stay true")
	}
	if cfg.Log != "/home/someone/.cache/neonmodem.log" {
		t.Errorf("Log %q was not read from the file", cfg.Log)
	}

	if len(cfg.Systems) != 2 {
		t.Fatalf("expected 2 systems, got %d", len(cfg.Systems))
	}
	if cfg.Systems[0].Type != "hackernews" ||
		cfg.Systems[0].Settings.URL != "https://news.ycombinator.com" {
		t.Errorf("unexpected first system: %+v", cfg.Systems[0])
	}
	if cfg.Systems[1].Settings.URL != "https://lobste.rs" {
		t.Errorf("unexpected second system: %+v", cfg.Systems[1])
	}
	if cfg.Systems[0].Settings.Credential("password") != "" {
		t.Error("empty credential should read as empty")
	}

	if cfg.Theme.Header.Selector.Border.Border != NormalBorder {
		t.Errorf("border table was not decoded: %+v", cfg.Theme.Header.Selector.Border.Border)
	}
	if cfg.Theme.Reply.Author.Foreground.Dark != "#00000" {
		t.Errorf("file value should win over the default: %q", cfg.Theme.Reply.Author.Foreground.Dark)
	}
	if cfg.Theme.Post.Subject.Background.Light != "#f119a0" {
		t.Errorf("untouched theme values should keep their defaults")
	}
	if cfg.Theme.Header.Spinner.Foreground.Light != "" {
		t.Errorf("legacy empty spinner color should be kept as written")
	}
}

func TestSaveWritesOnlyDifferences(t *testing.T) {
	path := writeFile(t, t.TempDir(), "RenderSplash = false\n\n"+
		"[Theme.Post.Author.Foreground]\nLight = '#ff0000'\nDark = '#ff0000'\n")
	cfg, err := LoadFrom([]string{path}, "/cache")
	if err != nil {
		t.Fatal(err)
	}
	cfg.Systems = append(cfg.Systems, SystemConfig{
		Type: "lobsters",
		Settings: system.Settings{
			URL:         "https://lobste.rs",
			Credentials: map[string]string{"username": "vera"},
		},
	})

	if err := cfg.Save(); err != nil {
		t.Fatalf("Save: %v", err)
	}

	data, err := os.ReadFile(cfg.path)
	if err != nil {
		t.Fatal(err)
	}
	doc := string(data)

	for _, want := range []string{
		"RenderSplash = false",
		"[[Systems]]",
		"Type = 'lobsters'",
		"url = 'https://lobste.rs'",
		"username = 'vera'",
		"[Theme.Post.Author.Foreground]",
		"Light = '#ff0000'",
	} {
		if !strings.Contains(doc, want) {
			t.Errorf("saved file is missing %q:\n%s", want, doc)
		}
	}
	for _, unwanted := range []string{
		"RenderShadows", "Log =", "Debug", "Theme.Header", "Theme.PostsList", "Border",
		"[Theme.Post.Author.Background]", "Padding",
	} {
		if strings.Contains(doc, unwanted) {
			t.Errorf("saved file should not contain default %q:\n%s", unwanted, doc)
		}
	}
	if lines := strings.Count(doc, "\n"); lines > 25 {
		t.Errorf("saved file is too long (%d lines):\n%s", lines, doc)
	}

	info, err := os.Stat(cfg.path)
	if err != nil {
		t.Fatal(err)
	}
	if mode := info.Mode().Perm(); mode != 0o600 && runtime.GOOS != "windows" {
		t.Errorf("file mode %o, want 600", mode)
	}

	reloaded, err := LoadFrom([]string{cfg.path}, "/cache")
	if err != nil {
		t.Fatalf("reload: %v", err)
	}
	if reloaded.RenderSplash || !reloaded.RenderShadows {
		t.Error("reloaded flags differ")
	}
	if len(reloaded.Systems) != 1 || reloaded.Systems[0].Settings.Credential("username") != "vera" {
		t.Errorf("reloaded systems differ: %+v", reloaded.Systems)
	}
	if reloaded.Theme.Post.Author.Foreground.Light != "#ff0000" {
		t.Error("reloaded theme change lost")
	}
	if reloaded.Theme.Post.Subject.Background.Light != "#f119a0" {
		t.Error("reloaded default was not restored for the pruned sibling value")
	}
}

func TestSaveReplacesLongerFile(t *testing.T) {
	dir := t.TempDir()
	path := writeFile(t, dir, legacyFile)

	cfg, err := LoadFrom([]string{path}, "/cache")
	if err != nil {
		t.Fatal(err)
	}
	cfg.Systems = cfg.Systems[:1]

	if err := cfg.Save(); err != nil {
		t.Fatalf("Save: %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Count(string(data), "[[Systems]]") != 1 {
		t.Errorf("expected exactly one system in the rewritten file:\n%s", data)
	}
	if len(data) >= len(legacyFile) {
		t.Errorf("rewritten file (%d bytes) should be shorter than the legacy file (%d bytes)", len(data), len(legacyFile))
	}

	reloaded, err := LoadFrom([]string{path}, "/cache")
	if err != nil {
		t.Fatalf("rewritten file does not parse: %v", err)
	}
	if len(reloaded.Systems) != 1 {
		t.Errorf("expected 1 system after reload, got %d", len(reloaded.Systems))
	}
	if reloaded.Log != "/home/someone/.cache/neonmodem.log" {
		t.Error("a Log path that differs from the default must survive a save")
	}
}

func TestEnvOverrides(t *testing.T) {
	t.Setenv("NEONMODEM_RENDERSPLASH", "false")
	t.Setenv("NEONMODEM_PROXY", "http://proxy.local:8080")
	t.Setenv("NEONMODEM_DEBUG", "1")
	t.Setenv("NEONMODEM_SORT", "top-week")

	cfg, err := LoadFrom(nil, "/cache")
	if err != nil {
		t.Fatal(err)
	}
	cfg.applyEnv()

	if cfg.RenderSplash {
		t.Error("NEONMODEM_RENDERSPLASH=false was ignored")
	}
	if cfg.Proxy != "http://proxy.local:8080" {
		t.Errorf("NEONMODEM_PROXY was ignored: %q", cfg.Proxy)
	}
	if !cfg.Debug {
		t.Error("NEONMODEM_DEBUG=1 was ignored")
	}
	if cfg.Sort != "top-week" {
		t.Errorf("NEONMODEM_SORT was ignored: %q", cfg.Sort)
	}
}

func TestDefaultsAreWellFormed(t *testing.T) {
	cfg := Defaults("/cache")

	if cfg.Theme.Reply.Author.Foreground.Dark != "#000000" {
		t.Errorf("reply author color default is %q", cfg.Theme.Reply.Author.Foreground.Dark)
	}
	if cfg.Theme.Header.Spinner.Foreground.Dark != "#FF5FAF" {
		t.Errorf("spinner color default is %q", cfg.Theme.Header.Spinner.Foreground.Dark)
	}
	if cfg.Log != filepath.Join("/cache", "neonmodem.log") {
		t.Errorf("log default is %q", cfg.Log)
	}
}

func TestCommandsAllowedWithoutAFile(t *testing.T) {
	cfg, err := LoadFrom(nil, "/cache")
	if err != nil {
		t.Fatalf("LoadFrom: %v", err)
	}
	if err := cfg.CommandsAllowed(); err != nil {
		t.Errorf("a configuration without a file should allow commands: %v", err)
	}
}

func TestTimeoutsDefaultAndOverride(t *testing.T) {
	cfg, err := LoadFrom(nil, "/cache")
	if err != nil {
		t.Fatal(err)
	}
	if cfg.ReadTimeout != 20 || cfg.WriteTimeout != 60 {
		t.Fatalf("defaults are %d and %d", cfg.ReadTimeout, cfg.WriteTimeout)
	}
	if cfg.ReadDeadline() != 20*time.Second || cfg.WriteDeadline() != 60*time.Second {
		t.Error("the deadlines don't match the settings")
	}

	t.Setenv("NEONMODEM_READTIMEOUT", " 5 ")
	t.Setenv("NEONMODEM_WRITETIMEOUT", "0")
	cfg.applyEnv()
	cfg.normalize()
	if cfg.ReadTimeout != 5 || cfg.WriteTimeout != 0 || cfg.WriteDeadline() != 0 {
		t.Errorf("the environment was ignored: %d and %d", cfg.ReadTimeout, cfg.WriteTimeout)
	}
	if len(cfg.Notices()) != 0 {
		t.Errorf("unexpected notices: %q", cfg.Notices())
	}
}

func TestBadTimeoutsFallBackWithANotice(t *testing.T) {
	path := filepath.Join(t.TempDir(), FileName)
	if err := os.WriteFile(path, []byte("ReadTimeout = -3\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	cfg, err := LoadFrom([]string{path}, "/cache")
	if err != nil {
		t.Fatal(err)
	}
	if cfg.ReadTimeout != 20 {
		t.Errorf("a negative value should fall back to the default, got %d", cfg.ReadTimeout)
	}
	if n := cfg.Notices(); len(n) != 1 || !strings.Contains(n[0], "ReadTimeout setting -3") {
		t.Errorf("unexpected notices: %q", n)
	}

	t.Setenv("NEONMODEM_WRITETIMEOUT", "soon")
	cfg.applyEnv()
	cfg.normalize()
	if cfg.WriteTimeout != 60 {
		t.Errorf("a value that isn't a number should keep the default, got %d", cfg.WriteTimeout)
	}
	if n := cfg.Notices(); len(n) != 2 || !strings.Contains(n[1], `NEONMODEM_WRITETIMEOUT="soon"`) {
		t.Errorf("unexpected notices: %q", n)
	}
}

func TestSaveKeepsChangedTimeouts(t *testing.T) {
	path := filepath.Join(t.TempDir(), FileName)
	if err := os.WriteFile(path, []byte("ReadTimeout = 30\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	cfg, err := LoadFrom([]string{path}, "/cache")
	if err != nil {
		t.Fatal(err)
	}
	if err := cfg.Save(); err != nil {
		t.Fatal(err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "ReadTimeout = 30") || strings.Contains(string(data), "WriteTimeout") {
		t.Errorf("unexpected file:\n%s", data)
	}
}

func TestSaveLeavesInMemoryChangesOut(t *testing.T) {
	path := writeFile(t, t.TempDir(), "Sort = 'hot'\n")
	cfg, err := LoadFrom([]string{path}, "/cache")
	if err != nil {
		t.Fatal(err)
	}

	t.Setenv("NEONMODEM_RENDERSPLASH", "false")
	cfg.applyEnv()
	cfg.Debug = true
	cfg.Systems = append(cfg.Systems, SystemConfig{Type: "lobsters", Settings: system.Settings{URL: "https://lobste.rs"}})
	if err := cfg.Save(); err != nil {
		t.Fatal(err)
	}

	doc := string(content(t, path))
	for _, want := range []string{"Sort = 'hot'", "[[Systems]]", "url = 'https://lobste.rs'"} {
		if !strings.Contains(doc, want) {
			t.Errorf("the saved file lacks %q:\n%s", want, doc)
		}
	}
	for _, unwanted := range []string{"RenderSplash", "Debug"} {
		if strings.Contains(doc, unwanted) {
			t.Errorf("the saved file holds %q, which was only changed in memory:\n%s", unwanted, doc)
		}
	}
}

func TestSaveWritesThroughALinkAndKeepsTheFile(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlinks need privileges on Windows")
	}
	dir := t.TempDir()
	target := filepath.Join(dir, "real.toml")
	if err := os.WriteFile(target, []byte("Sort = 'hot'\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(dir, FileName)
	if err := os.Symlink(target, link); err != nil {
		t.Fatal(err)
	}
	hard := filepath.Join(dir, "hard.toml")
	if err := os.Link(target, hard); err != nil {
		t.Fatal(err)
	}

	cfg, err := LoadFrom([]string{link}, "/cache")
	if err != nil {
		t.Fatal(err)
	}
	cfg.Systems = append(cfg.Systems, SystemConfig{Type: "lobsters", Settings: system.Settings{URL: "https://lobste.rs"}})
	if err := cfg.Save(); err != nil {
		t.Fatal(err)
	}

	if info, err := os.Lstat(link); err != nil || info.Mode()&os.ModeSymlink == 0 {
		t.Errorf("the link was replaced: %v, %v", info, err)
	}
	doc := string(content(t, target))
	if !strings.Contains(doc, "url = 'https://lobste.rs'") || !strings.Contains(doc, "Sort = 'hot'") {
		t.Errorf("the target didn't get the content:\n%s", doc)
	}
	if string(content(t, hard)) != doc {
		t.Error("the hard link no longer shares the file, so the inode changed")
	}
	if info, err := os.Stat(target); err != nil || info.Mode().Perm() != 0o600 {
		t.Errorf("the target's mode is %v, expected 0600", info.Mode())
	}
}

func TestSnippetShowsTheSystemTables(t *testing.T) {
	snippet, err := Snippet(SystemConfig{Type: "lemmy", Settings: system.Settings{
		URL:         "https://lemmy.ml",
		Credentials: map[string]string{"username": "vera", "token_cmd": "pass show lemmy"},
		Options:     map[string]string{"listing": "local"},
	}})
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"[[Systems]]", "Type = 'lemmy'", "[Systems.Config]", "url = 'https://lemmy.ml'",
		"[Systems.Config.credentials]", "token_cmd = 'pass show lemmy'", "[Systems.Config.options]", "listing = 'local'"} {
		if !strings.Contains(snippet, want) {
			t.Errorf("the snippet lacks %q:\n%s", want, snippet)
		}
	}

	plain, err := Snippet(SystemConfig{Type: "hackernews", Settings: system.Settings{URL: "https://news.ycombinator.com"}})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(plain, "credentials") || strings.Contains(plain, "options") {
		t.Errorf("empty tables must be left out:\n%s", plain)
	}
}

func content(t *testing.T, path string) []byte {
	t.Helper()

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return data
}
