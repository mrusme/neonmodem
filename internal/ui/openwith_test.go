package ui

import (
	"errors"
	"log/slog"
	"reflect"
	"slices"
	"strings"
	"sync"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/mrusme/neonmodem/internal/config"
	"github.com/mrusme/neonmodem/internal/models/post"
	"github.com/mrusme/neonmodem/internal/system"
	"github.com/mrusme/neonmodem/internal/ui/ctx"
	"github.com/mrusme/neonmodem/internal/ui/msgs"
	"github.com/mrusme/neonmodem/internal/ui/windows/popuplist"
	"github.com/mrusme/neonmodem/internal/ui/windows/postshow"
)

type launch struct {
	name string
	line string
	env  []string
}

type fakeLauncher struct {
	mu    sync.Mutex
	calls []launch
	err   error
}

func (f *fakeLauncher) Start(name string, line string, env []string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls = append(f.calls, launch{name: name, line: line, env: env})
	return f.err
}

var testCommands = []config.OpenWith{
	{Name: "Save page", Cmd: `wget "$NM_POST_URL"`},
	{Name: "Bookmark", Cmd: `nb bookmark "$NM_POST_URL"`},
}

func openWithModel(t *testing.T, commands []config.OpenWith, launcher ctx.Launcher) (Model, *ctx.Ctx) {
	t.Helper()

	cfg := config.Defaults("/cache")
	cfg.RenderImages = false
	cfg.RenderShadows = false
	c := ctx.New(nil, &cfg, slog.New(slog.DiscardHandler), []system.System{&countingSystem{}})
	c.OpenWith = commands
	c.Launcher = launcher

	m := NewModel(&c)
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 48})
	m = updated.(Model)
	m.currentView = 1
	return m, &c
}

func settleModel(t *testing.T, m Model, cmd tea.Cmd) (Model, []msgs.Notice) {
	t.Helper()

	var notices []msgs.Notice
	var walk func(cmd tea.Cmd)
	walk = func(cmd tea.Cmd) {
		if cmd == nil {
			return
		}
		msg := cmd()
		if msg == nil || isTick(msg) {
			return
		}
		switch msg := msg.(type) {
		case tea.BatchMsg:
			for _, c := range msg {
				walk(c)
			}
		case msgs.Notice:
			notices = append(notices, msg)
		default:
			updated, next := m.Update(msg)
			m = updated.(Model)
			walk(next)
		}
	}
	walk(cmd)
	return m, notices
}

func isTick(msg tea.Msg) bool {
	pkg := reflect.TypeOf(msg).PkgPath()
	return strings.HasSuffix(pkg, "/spinner") || strings.HasSuffix(pkg, "/cursor")
}

var testPost = post.Post{ID: "p", Subject: "subject", URL: "https://counting.example/p", SysIDX: 0}

func openThePost(t *testing.T, m Model) Model {
	t.Helper()
	updated, cmd := m.Update(msgs.OpenPost{Post: testPost})
	m, _ = settleModel(t, updated.(Model), cmd)
	return m
}

func press(t *testing.T, m Model, key tea.KeyPressMsg) (Model, []msgs.Notice) {
	t.Helper()
	updated, cmd := m.Update(key)
	return settleModel(t, updated.(Model), cmd)
}

var (
	keyO     = tea.KeyPressMsg{Code: 'o', Text: "O", Mod: tea.ModShift}
	keyDown  = tea.KeyPressMsg{Code: tea.KeyDown}
	keyEnter = tea.KeyPressMsg{Code: tea.KeyEnter}
	keyEsc   = tea.KeyPressMsg{Code: tea.KeyEscape}
)

func TestThePostWindowOffersOpenWithOnlyWithCommands(t *testing.T) {
	m, _ := openWithModel(t, testCommands, &fakeLauncher{})
	m = openThePost(t, m)
	if view := m.View().Content; !strings.Contains(view, "o open in browser · O open with") {
		t.Errorf("the bar doesn't show O after o:\n%s", view)
	}

	plain, _ := openWithModel(t, nil, &fakeLauncher{})
	plain = openThePost(t, plain)
	if strings.Contains(plain.View().Content, "open with") {
		t.Error("O is offered without commands")
	}
	plain, _ = press(t, plain, keyO)
	if plain.wm.IsOpen(popuplist.WIN_ID) {
		t.Error("O opened a dialog without commands")
	}
}

func TestOpenWithRunsThePickedCommand(t *testing.T) {
	launcher := &fakeLauncher{}
	m, _ := openWithModel(t, testCommands, launcher)
	m = openThePost(t, m)

	m, _ = press(t, m, keyO)
	if m.wm.Focused() != popuplist.WIN_ID {
		t.Fatalf("O didn't open the dialog, focused: %q", m.wm.Focused())
	}
	view := m.View().Content
	for _, want := range []string{"Open with", "Save page", `nb bookmark "$NM_POST_URL"`} {
		if !strings.Contains(view, want) {
			t.Errorf("the dialog doesn't show %q:\n%s", want, view)
		}
	}

	m, _ = press(t, m, keyDown)
	m, notices := press(t, m, keyEnter)

	if len(launcher.calls) != 1 {
		t.Fatalf("got %d starts", len(launcher.calls))
	}
	call := launcher.calls[0]
	if call.name != "Bookmark" || call.line != `nb bookmark "$NM_POST_URL"` {
		t.Errorf("started %q with %q", call.name, call.line)
	}
	for _, want := range []string{"NM_SYSTEM_TYPE=counting", "NM_POST_ID=p", "NM_POST_URL=https://counting.example/p"} {
		if !slices.Contains(call.env, want) {
			t.Errorf("missing %q in %q", want, call.env)
		}
	}
	if len(notices) != 1 || notices[0].Text != "Running Bookmark" || notices[0].IsError {
		t.Errorf("got notices %+v", notices)
	}
	if m.wm.IsOpen(popuplist.WIN_ID) || m.wm.Focused() != postshow.WIN_ID {
		t.Errorf("the dialog should close and leave the post focused, focused: %q", m.wm.Focused())
	}
}

func TestEscClosesTheOpenWithDialog(t *testing.T) {
	launcher := &fakeLauncher{}
	m, _ := openWithModel(t, testCommands, launcher)
	m = openThePost(t, m)

	m, _ = press(t, m, keyO)
	m, _ = press(t, m, keyEsc)

	if m.wm.IsOpen(popuplist.WIN_ID) || m.wm.Focused() != postshow.WIN_ID {
		t.Errorf("esc should return to the post, focused: %q", m.wm.Focused())
	}
	if len(launcher.calls) != 0 {
		t.Errorf("esc started %+v", launcher.calls)
	}
}

func TestAFailedStartShowsTheError(t *testing.T) {
	launcher := &fakeLauncher{err: errors.New("couldn't be started: no shell")}
	m, _ := openWithModel(t, testCommands, launcher)
	m = openThePost(t, m)

	m, _ = press(t, m, keyO)
	_, notices := press(t, m, keyEnter)

	if len(notices) != 1 || !notices[0].IsError || notices[0].Text != "Save page: couldn't be started: no shell" {
		t.Errorf("got notices %+v", notices)
	}
}

func TestPickingACommandKeepsAPostLoading(t *testing.T) {
	m, c := openWithModel(t, testCommands, &fakeLauncher{})

	updated, _ := m.Update(msgs.OpenPost{Post: testPost})
	m = updated.(Model)
	if !c.IsLoading() {
		t.Fatal("the post should be loading")
	}

	updated, cmd := m.Update(msgs.OpenWithMenu{Post: testPost})
	m, _ = settleModel(t, updated.(Model), cmd)
	updated, cmd = m.Update(msgs.Picked{Kind: msgs.PickOpenWith, Item: OpenWithItem{Name: "Save page", Line: "true"}})
	settleModel(t, updated.(Model), cmd)

	if !c.IsLoading() {
		t.Error("picking a command stopped the post's spinner")
	}
}
