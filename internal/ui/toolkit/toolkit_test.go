package toolkit

import (
	"log/slog"
	"slices"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/mrusme/neonmodem/internal/config"
	"github.com/mrusme/neonmodem/internal/ui/ctx"
	"github.com/mrusme/neonmodem/internal/ui/msgs"
)

func testToolKit(t *testing.T) *ToolKit {
	t.Helper()

	cfg := config.Defaults("/cache")
	c := ctx.New(nil, &cfg, slog.New(slog.DiscardHandler), nil)
	return New("win", &c)
}

func TestDialogsRenderExactlyTheWindowSize(t *testing.T) {
	content := strings.Repeat("a line of content that goes on\n", 40)

	for _, size := range [][2]int{{30, 8}, {44, 12}, {80, 24}, {120, 40}} {
		for _, isError := range []bool{false, true} {
			tk := testToolKit(t)
			tk.SetErrorDialog(isError)
			tk.HandleMsg(tea.WindowSizeMsg{Width: size[0], Height: size[1]})

			for _, focused := range []bool{true, false} {
				if focused {
					tk.Focus()
				} else {
					tk.Blur()
				}
				for _, view := range []string{tk.Dialog("Title", content), tk.DialogWithStatus("Title", content, "status")} {
					if w, h := lipgloss.Width(view), lipgloss.Height(view); w != size[0] || h != size[1] {
						t.Errorf("error=%v focused=%v at %dx%d renders %dx%d",
							isError, focused, size[0], size[1], w, h)
					}
				}
			}
		}
	}
}

func TestInnerSizesNeverDropBelowOne(t *testing.T) {
	tk := testToolKit(t)
	tk.HandleMsg(tea.WindowSizeMsg{Width: 3, Height: 2})
	if tk.InnerWidth() < 1 || tk.InnerHeight() < 1 {
		t.Errorf("inner size %dx%d", tk.InnerWidth(), tk.InnerHeight())
	}
	if tk.Dialog("Title", "content") == "" {
		t.Error("a tiny window still renders")
	}
}

func TestKeyBarOrderIsCaseInsensitiveWithLowercaseFirst(t *testing.T) {
	tk := testToolKit(t)
	tk.KeymapAdd("reply", "reply", "r")
	tk.KeymapAdd("openwith", "open with", "O")
	tk.KeymapAdd("older", "older replies", "z")
	tk.KeymapAdd("open", "open in browser", "o")

	want := []string{"o open in browser", "O open with", "r reply", "z older replies", "esc close"}
	if got := tk.KeymapHelpStrings(); !slices.Equal(got, want) {
		t.Errorf("key bar order is %q, expected %q", got, want)
	}
}

func TestBlurredViewsComeFromTheCache(t *testing.T) {
	tk := testToolKit(t)
	renders := 0
	tk.SetViewFunc(func(cached bool) string {
		if v := tk.DefaultCaching(cached); v != "" {
			return v
		}
		renders++
		return "view"
	})

	tk.Focus()
	tk.View(true)
	tk.View(true)
	if renders != 3 {
		t.Fatalf("a focused window renders every time, got %d renders", renders)
	}

	tk.Blur()
	tk.View(true)
	tk.View(true)
	if renders != 4 {
		t.Errorf("a blurred window serves its cache, got %d renders", renders)
	}

	tk.InvalidateCache()
	tk.View(true)
	if renders != 5 {
		t.Errorf("an invalidated cache renders again, got %d renders", renders)
	}
	tk.View(false)
	if renders != 6 {
		t.Errorf("an uncached view always renders, got %d renders", renders)
	}
}

func TestHandleMsgDispatchesKeysAndFocus(t *testing.T) {
	tk := testToolKit(t)
	tk.KeymapAdd("reply", "reply", "r")

	var calls []string
	tk.SetMsgHandling(MsgHandling{
		OnKeymapKey: []MsgHandlingKeymapKey{{ID: "reply", Handler: func() (bool, []tea.Cmd) {
			calls = append(calls, "reply")
			return true, nil
		}}},
		OnAnyNumberKey: func(n int8) (bool, []tea.Cmd) {
			calls = append(calls, "number")
			return false, nil
		},
		OnAnyUncaughtKey: func(k tea.KeyPressMsg) (bool, []tea.Cmd) {
			calls = append(calls, "uncaught "+k.String())
			return false, nil
		},
		OnViewResize: func() (bool, []tea.Cmd) {
			calls = append(calls, "resize")
			return false, nil
		},
	})

	if handled, _ := tk.HandleMsg(tea.KeyPressMsg{Code: 'r', Text: "r"}); !handled {
		t.Error("a keymap key is handled")
	}
	tk.HandleMsg(tea.KeyPressMsg{Code: '2', Text: "2"})
	tk.HandleMsg(tea.KeyPressMsg{Code: 'x', Text: "x"})
	tk.HandleMsg(tea.WindowSizeMsg{Width: 80, Height: 24})
	tk.HandleMsg(msgs.FocusWindow{ID: "other"})
	tk.HandleMsg(msgs.FocusWindow{ID: "win"})

	want := []string{"reply", "number", "uncaught x", "resize"}
	if !slices.Equal(calls, want) {
		t.Errorf("handlers ran as %q, expected %q", calls, want)
	}
	if !tk.IsFocused() || tk.ViewWidth() != 80 || tk.ViewHeight() != 24 {
		t.Error("the toolkit tracks its focus and size")
	}

	tk.HandleMsg(msgs.BlurWindow{ID: "*"})
	if tk.IsFocused() {
		t.Error("a blur for every window blurs this one")
	}
}
