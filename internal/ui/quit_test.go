package ui

import (
	"log/slog"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/mrusme/neonmodem/internal/config"
	"github.com/mrusme/neonmodem/internal/models/post"
	"github.com/mrusme/neonmodem/internal/system"
	"github.com/mrusme/neonmodem/internal/ui/ctx"
	"github.com/mrusme/neonmodem/internal/ui/msgs"
	"github.com/mrusme/neonmodem/internal/ui/windows/popuplist"
)

var (
	keyCtrlC = tea.KeyPressMsg{Code: 'c', Mod: tea.ModCtrl}
	keyCtrlQ = tea.KeyPressMsg{Code: 'q', Mod: tea.ModCtrl}
	keyCtrlT = tea.KeyPressMsg{Code: 't', Mod: tea.ModCtrl}
	keyCtrlO = tea.KeyPressMsg{Code: 'o', Mod: tea.ModCtrl}
)

func rootModel(t *testing.T) (Model, *ctx.Ctx) {
	t.Helper()

	cfg := config.Defaults("/cache")
	cfg.RenderImages = false
	cfg.RenderShadows = false
	c := ctx.New(nil, &cfg, slog.New(slog.DiscardHandler), []system.System{&countingSystem{}})

	m := NewModel(&c)
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	m = updated.(Model)
	m.currentView = 1
	return m, &c
}

func quits(cmd tea.Cmd) bool {
	if cmd == nil {
		return false
	}
	_, ok := cmd().(tea.QuitMsg)
	return ok
}

func everyState(t *testing.T, check func(state string, m Model)) {
	t.Helper()

	m, _ := rootModel(t)
	check("the posts list", m)

	updated, cmd := m.Update(msgs.OpenPost{Post: post.Post{ID: "p", Subject: "s", SysIDX: 0}})
	m, _ = settleModel(t, updated.(Model), cmd)
	check("the post window", m)

	updated, cmd = m.Update(msgs.Compose{Action: msgs.ComposeReply, Post: post.Post{ID: "p", SysIDX: 0}})
	m, _ = settleModel(t, updated.(Model), cmd)
	check("the compose window", m)

	updated, cmd = m.Update(keyCtrlO)
	m, _ = settleModel(t, updated.(Model), cmd)
	check("a picker", m)
}

func TestCtrlQQuitsFromEveryState(t *testing.T) {
	everyState(t, func(state string, m Model) {
		if _, cmd := m.Update(keyCtrlQ); !quits(cmd) {
			t.Errorf("ctrl+q over %s doesn't quit", state)
		}
	})
}

func TestCtrlCOnlyNamesTheQuitKey(t *testing.T) {
	everyState(t, func(state string, m Model) {
		open := m.wm.Focused()
		updated, cmd := m.Update(keyCtrlC)
		if quits(cmd) {
			t.Errorf("ctrl+c over %s quits", state)
			return
		}
		m, notices := settleModel(t, updated.(Model), cmd)
		if len(notices) != 1 || notices[0].Text != "Press ctrl+q to quit" {
			t.Errorf("ctrl+c over %s gives the notices %+v", state, notices)
		}
		if m.wm.Focused() != open {
			t.Errorf("ctrl+c over %s moved the focus from %q to %q", state, open, m.wm.Focused())
		}
	})
}

func TestAForumListArrivingUnderAnotherPickerEndsTheSpinner(t *testing.T) {
	m, c := rootModel(t)

	updated, _ := m.Update(keyCtrlT)
	m = updated.(Model)
	if !c.IsLoading() {
		t.Fatal("the forum picker should start the spinner")
	}

	updated, cmd := m.Update(tea.KeyPressMsg{Code: tea.KeyEscape})
	m, _ = settleModel(t, updated.(Model), cmd)
	updated, cmd = m.Update(keyCtrlO)
	m, _ = settleModel(t, updated.(Model), cmd)
	if !m.wm.IsOpen(popuplist.WIN_ID) {
		t.Fatal("the order picker should be open")
	}

	updated, cmd = m.Update(msgs.PickerItems{Kind: msgs.PickForum})
	m, _ = settleModel(t, updated.(Model), cmd)
	if c.IsLoading() {
		t.Error("the forum list arrived, but the spinner keeps running")
	}
	if !m.wm.IsOpen(popuplist.WIN_ID) {
		t.Error("the order picker should still be open")
	}
}

func TestAFinishedPostLoadKeepsAPendingFeedSpinner(t *testing.T) {
	m, c := rootModel(t)
	c.StartLoading(ctx.LoadFeed)

	updated, cmd := m.Update(msgs.OpenPost{Post: post.Post{ID: "p", Subject: "s", SysIDX: 0}})
	settleModel(t, updated.(Model), cmd)

	if !c.IsLoading() {
		t.Error("the feed is still pending, the spinner must keep running")
	}
}
