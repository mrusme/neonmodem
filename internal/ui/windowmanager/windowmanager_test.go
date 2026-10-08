package windowmanager

import (
	"log/slog"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/mrusme/neonmodem/internal/config"
	"github.com/mrusme/neonmodem/internal/ui/ctx"
	"github.com/mrusme/neonmodem/internal/ui/msgs"
	"github.com/mrusme/neonmodem/internal/ui/windows"
)

type fakeWindow struct {
	content string
	size    tea.WindowSizeMsg
	focused bool
	seen    []tea.Msg
}

func (w *fakeWindow) Update(msg tea.Msg) (windows.Window, tea.Cmd) {
	w.seen = append(w.seen, msg)
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		w.size = msg
	case msgs.FocusWindow:
		w.focused = true
	case msgs.BlurWindow:
		w.focused = false
	}
	return w, nil
}

func (w *fakeWindow) View() string {
	return w.content
}

func testWM(t *testing.T, width int, height int) *WM {
	t.Helper()

	cfg := config.Defaults("/cache")
	cfg.RenderShadows = false
	c := ctx.New(nil, &cfg, slog.New(slog.DiscardHandler), nil)
	c.Screen = [2]int{width, height + 2}
	c.Content = [2]int{width, height}

	return New(&c)
}

func sent(cmds []tea.Cmd) []tea.Msg {
	var out []tea.Msg
	for _, cmd := range cmds {
		if cmd == nil {
			continue
		}
		if msg := cmd(); msg != nil {
			out = append(out, msg)
		}
	}
	return out
}

type initMsg struct{}

func TestOpenSizesInitializesAndFocusesTheWindow(t *testing.T) {
	wm := testWM(t, 100, 40)
	win := &fakeWindow{}

	out := sent(wm.Open("a", win, Geometry{Left: 4, Top: 1, Right: 6, Bottom: 3}, initMsg{}))

	if !wm.IsOpen("a") || !wm.IsFocused("a") || wm.GetNumberOpen() != 1 || wm.Focused() != "a" {
		t.Fatal("the window should be open and focused")
	}
	if win.size.Width != 90 || win.size.Height != 36 {
		t.Errorf("the window got %dx%d, expected 90x36", win.size.Width, win.size.Height)
	}
	if !win.focused {
		t.Error("the window never got the focus message")
	}

	var sawInit bool
	for _, msg := range win.seen {
		if _, ok := msg.(initMsg); ok {
			sawInit = true
		}
	}
	if !sawInit {
		t.Error("the init message never reached the window")
	}

	var blurredView bool
	for _, msg := range out {
		if _, ok := msg.(msgs.BlurView); ok {
			blurredView = true
		}
	}
	if !blurredView {
		t.Error("opening a window must blur the view behind it")
	}
}

func TestOpeningAnOpenWindowAgainRaisesIt(t *testing.T) {
	wm := testWM(t, 100, 40)
	a, b := &fakeWindow{}, &fakeWindow{}

	wm.Open("a", a, Geometry{}, nil)
	wm.Open("b", b, Geometry{}, nil)
	if a.focused || !b.focused || wm.Focused() != "b" {
		t.Fatal("the last opened window is the focused one")
	}

	wm.Open("a", &fakeWindow{}, Geometry{Left: 10, Right: 10}, initMsg{})
	if wm.GetNumberOpen() != 2 {
		t.Fatalf("opening an open window again must not add one, got %d", wm.GetNumberOpen())
	}
	if !a.focused || b.focused || wm.Focused() != "a" {
		t.Error("opening an open window again raises and focuses it")
	}
	if a.size.Width != 80 {
		t.Errorf("the window keeps the geometry it was opened with, got width %d", a.size.Width)
	}

	var inits int
	for _, msg := range a.seen {
		if _, ok := msg.(initMsg); ok {
			inits++
		}
	}
	if inits != 1 {
		t.Errorf("the init message of a repeated open reaches the window, got %d", inits)
	}
}

func TestCloseFocusesTheWindowBelowOrTheView(t *testing.T) {
	wm := testWM(t, 100, 40)
	a, b := &fakeWindow{}, &fakeWindow{}
	wm.Open("a", a, Geometry{}, nil)
	wm.Open("b", b, Geometry{}, nil)

	closed, cmds := wm.CloseFocused()
	if !closed || wm.IsOpen("b") || wm.Focused() != "a" || !a.focused {
		t.Fatal("closing the focused window focuses the one below")
	}
	if _, ok := sent(cmds)[0].(msgs.WindowClosed); !ok {
		t.Error("closing sends WindowClosed first")
	}

	closed, cmds = wm.Close("a")
	if !closed || wm.GetNumberOpen() != 0 || wm.Focused() != "" {
		t.Fatal("closing the last window empties the stack")
	}
	var focusedView bool
	for _, msg := range sent(cmds) {
		if _, ok := msg.(msgs.FocusView); ok {
			focusedView = true
		}
	}
	if !focusedView {
		t.Error("closing the last window focuses the view")
	}

	if closed, cmds := wm.Close("missing"); closed || cmds != nil {
		t.Error("closing an unknown window reports false")
	}
}

func TestResizeAllAppliesEachGeometry(t *testing.T) {
	wm := testWM(t, 100, 40)
	a, b := &fakeWindow{}, &fakeWindow{}
	wm.Open("a", a, Geometry{Left: 1, Top: 1, Right: 1, Bottom: 1}, nil)
	wm.Open("b", b, Geometry{Left: 10, Top: 5, Right: 10, Bottom: 5}, nil)

	wm.ResizeAll(60, 30)
	if a.size.Width != 58 || a.size.Height != 28 {
		t.Errorf("a got %dx%d, expected 58x28", a.size.Width, a.size.Height)
	}
	if b.size.Width != 40 || b.size.Height != 20 {
		t.Errorf("b got %dx%d, expected 40x20", b.size.Width, b.size.Height)
	}
}

func TestCenteredClampsToTheContent(t *testing.T) {
	if g := Centered(100, 40, 50, 20); g != (Geometry{Left: 25, Top: 10, Right: 25, Bottom: 10}) {
		t.Errorf("unexpected geometry %+v", g)
	}
	if g := Centered(100, 40, 51, 21); g != (Geometry{Left: 24, Top: 9, Right: 25, Bottom: 10}) {
		t.Errorf("an odd remainder goes to the right and the bottom, got %+v", g)
	}
	if g := Centered(30, 10, 50, 20); g != (Geometry{}) {
		t.Errorf("a window larger than the content fills it, got %+v", g)
	}
}

func TestViewPlacesWindowsBelowTheHeader(t *testing.T) {
	wm := testWM(t, 20, 5)
	base := strings.TrimSuffix(strings.Repeat(strings.Repeat(".", 20)+"\n", 7), "\n")

	wm.Open("a", &fakeWindow{content: "AB\nCD"}, Geometry{Left: 2, Top: 1, Right: 16, Bottom: 2}, nil)
	wm.Open("b", &fakeWindow{content: "X"}, Geometry{Left: 3, Top: 1, Right: 16, Bottom: 3}, nil)

	lines := strings.Split(wm.View(base), "\n")
	if len(lines) != 7 {
		t.Fatalf("the composed screen has %d lines, expected 7", len(lines))
	}
	if lines[3] != "..AX................" || lines[4] != "..CD................" {
		t.Errorf("windows are misplaced:\n%s", strings.Join(lines, "\n"))
	}
	if lines[0] != strings.Repeat(".", 20) || lines[6] != strings.Repeat(".", 20) {
		t.Error("the rows outside the windows show the base")
	}
}

func TestViewWithoutWindowsIsTheBase(t *testing.T) {
	wm := testWM(t, 20, 5)
	if wm.View("base") != "base" {
		t.Error("without windows the view is the base")
	}
}
