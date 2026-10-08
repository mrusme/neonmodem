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

func TestCtrlCQuitsFromEveryState(t *testing.T) {
	m, _ := rootModel(t)
	if _, cmd := m.Update(keyCtrlC); !quits(cmd) {
		t.Error("ctrl+c in the posts list doesn't quit")
	}

	updated, cmd := m.Update(msgs.OpenPost{Post: post.Post{ID: "p", Subject: "s", SysIDX: 0}})
	m, _ = settleModel(t, updated.(Model), cmd)
	if _, cmd := m.Update(keyCtrlC); !quits(cmd) {
		t.Error("ctrl+c over the post window doesn't quit")
	}

	updated, cmd = m.Update(msgs.Compose{Action: msgs.ComposeReply, Post: post.Post{ID: "p", SysIDX: 0}})
	m, _ = settleModel(t, updated.(Model), cmd)
	if _, cmd := m.Update(keyCtrlC); !quits(cmd) {
		t.Error("ctrl+c over the compose window doesn't quit")
	}

	updated, cmd = m.Update(keyCtrlO)
	m, _ = settleModel(t, updated.(Model), cmd)
	if _, cmd := m.Update(keyCtrlC); !quits(cmd) {
		t.Error("ctrl+c over a picker doesn't quit")
	}
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
