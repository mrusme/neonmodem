package ui

import (
	"fmt"
	"log/slog"
	"strings"
	"testing"

	"charm.land/bubbles/v2/list"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/mrusme/neonmodem/internal/config"
	"github.com/mrusme/neonmodem/internal/models/forum"
	"github.com/mrusme/neonmodem/internal/models/post"
	"github.com/mrusme/neonmodem/internal/system"
	"github.com/mrusme/neonmodem/internal/ui/ctx"
	"github.com/mrusme/neonmodem/internal/ui/msgs"
	"github.com/mrusme/neonmodem/internal/ui/windows"
	"github.com/mrusme/neonmodem/internal/ui/windows/msgerror"
	"github.com/mrusme/neonmodem/internal/ui/windows/popuplist"
	"github.com/mrusme/neonmodem/internal/ui/windows/postcreate"
	"github.com/mrusme/neonmodem/internal/ui/windows/postshow"
)

func settle(win windows.Window, cmd tea.Cmd) windows.Window {
	if cmd == nil {
		return win
	}
	msg := cmd()
	if batch, ok := msg.(tea.BatchMsg); ok {
		for _, c := range batch {
			win = settle(win, c)
		}
		return win
	}
	switch msg.(type) {
	case nil, msgs.Notice, msgs.ShowError:
		return win
	}
	win, next := win.Update(msg)
	return settle(win, next)
}

func TestWindowsRenderExactlyTheirSize(t *testing.T) {
	cfg := config.Defaults("/cache")
	cfg.RenderImages = false
	c := ctx.New(nil, &cfg, slog.New(slog.DiscardHandler), []system.System{&countingSystem{}})

	longBody := strings.Repeat("A long line of post text that has to wrap. ", 40)
	p := post.Post{ID: "1", Subject: "A subject", Body: longBody, SysIDX: 0}

	var items []list.Item
	for i := range 12 {
		items = append(items, forum.Forum{Name: fmt.Sprintf("forum %d", i), Info: "about it"})
	}

	cases := []struct {
		name string
		open func() windows.Window
		id   string
	}{
		{"picker", func() windows.Window {
			win, _ := popuplist.NewModel(&c).Update(msgs.OpenPicker{Kind: msgs.PickSystem, Items: items})
			return win
		}, popuplist.WIN_ID},
		{"loading picker", func() windows.Window {
			win, _ := popuplist.NewModel(&c).Update(msgs.OpenPicker{Kind: msgs.PickForum, Items: items[:1]})
			return win
		}, popuplist.WIN_ID},
		{"loading post", func() windows.Window {
			win, _ := postshow.NewModel(&c).Update(msgs.OpenPost{Post: p})
			return win
		}, postshow.WIN_ID},
		{"post", func() windows.Window {
			win, cmd := postshow.NewModel(&c).Update(msgs.OpenPost{Post: p})
			return settle(win, cmd)
		}, postshow.WIN_ID},
		{"reply", func() windows.Window {
			win, _ := postcreate.NewModel(&c).Update(msgs.Compose{Action: msgs.ComposeReply, Post: p})
			return win
		}, postcreate.WIN_ID},
		{"new post", func() windows.Window {
			win, _ := postcreate.NewModel(&c).Update(msgs.Compose{Action: msgs.ComposePost, Post: p})
			return win
		}, postcreate.WIN_ID},
		{"error", func() windows.Window {
			win, _ := msgerror.NewModel(&c).Update(msgs.Message(longBody))
			return win
		}, msgerror.WIN_ID},
		{"notices", func() windows.Window {
			var entries []msgs.NoticeEntry
			for i := range 8 {
				entries = append(entries, msgs.NoticeEntry{Text: fmt.Sprintf("notice %d: %s", i, longBody[:120]), IsError: i%2 == 0})
			}
			win, _ := msgerror.NewModel(&c).Update(msgs.ShowNotices{Entries: entries})
			return win
		}, msgerror.WIN_ID},
	}

	for _, size := range [][2]int{{44, 12}, {60, 14}, {80, 24}, {110, 28}} {
		for _, tc := range cases {
			win := tc.open()
			win, _ = win.Update(tea.WindowSizeMsg{Width: size[0], Height: size[1]})
			for _, focus := range []tea.Msg{msgs.FocusWindow{ID: tc.id}, msgs.BlurWindow{ID: tc.id}} {
				win, _ = win.Update(focus)
				view := win.View()
				if w, h := lipgloss.Width(view), lipgloss.Height(view); w != size[0] || h != size[1] {
					t.Errorf("%s at %dx%d after %T renders %dx%d", tc.name, size[0], size[1], focus, w, h)
				}
			}
		}
	}
}
