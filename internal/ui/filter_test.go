package ui

import (
	"context"
	"log/slog"
	"testing"

	"charm.land/bubbles/v2/list"
	tea "charm.land/bubbletea/v2"
	"github.com/mrusme/neonmodem/internal/config"
	"github.com/mrusme/neonmodem/internal/models/post"
	"github.com/mrusme/neonmodem/internal/system"
	"github.com/mrusme/neonmodem/internal/ui/ctx"
	"github.com/mrusme/neonmodem/internal/ui/msgs"
	"github.com/mrusme/neonmodem/internal/ui/windows/popuplist"
)

type listingSystem struct {
	countingSystem
}

func (s *listingSystem) ListPosts(context.Context, string, system.Order) ([]post.Post, error) {
	return []post.Post{
		{ID: "a", Subject: "alpha", SysIDX: 0},
		{ID: "b", Subject: "beta", SysIDX: 0},
	}, nil
}

func listingModel(t *testing.T) Model {
	t.Helper()

	cfg := config.Defaults("/cache")
	cfg.RenderImages = false
	cfg.RenderShadows = false
	c := ctx.New(nil, &cfg, slog.New(slog.DiscardHandler), []system.System{&listingSystem{}})

	m := NewModel(&c)
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	m = updated.(Model)
	m.currentView = 1
	updated, cmd := m.Update(msgs.RefreshFeed{})
	m, _ = settleModel(t, updated.(Model), cmd)
	return m
}

var (
	keyCtrlE = tea.KeyPressMsg{Code: 'e', Mod: tea.ModCtrl}
	keySlash = tea.KeyPressMsg{Code: '/', Text: "/"}
	keyA     = tea.KeyPressMsg{Code: 'a', Text: "a"}
)

func pressAll(t *testing.T, m Model, keys ...tea.KeyPressMsg) Model {
	t.Helper()
	for _, k := range keys {
		updated, cmd := m.Update(k)
		if quits(cmd) {
			t.Fatalf("%s quit the program", k)
		}
		m, _ = settleModel(t, updated.(Model), cmd)
	}
	return m
}

func TestEscClearsTheFilterOfThePostsList(t *testing.T) {
	m := listingModel(t)

	m = pressAll(t, m, keySlash, keyA, keyEnter)
	if s := m.filterState(); s != list.FilterApplied {
		t.Fatalf("the filter state is %v after typing and enter", s)
	}

	m = pressAll(t, m, keyEsc)
	if s := m.filterState(); s != list.Unfiltered {
		t.Errorf("esc left the filter state %v", s)
	}

	pressAll(t, m, keyEsc)
}

func TestEscInAPickerClearsItsFilterBeforeClosing(t *testing.T) {
	m := listingModel(t)
	m = pressAll(t, m, keyCtrlE)
	if !m.wm.IsOpen(popuplist.WIN_ID) {
		t.Fatal("ctrl+e should open the system picker")
	}

	m = pressAll(t, m, keySlash, keyA)
	if s := m.filterState(); s != list.Filtering {
		t.Fatalf("the picker's filter state is %v while typing", s)
	}
	m = pressAll(t, m, keyEsc)
	if !m.wm.IsOpen(popuplist.WIN_ID) || m.filterState() != list.Unfiltered {
		t.Errorf("esc while typing should cancel the filter and keep the picker open, state %v", m.filterState())
	}

	m = pressAll(t, m, keySlash, keyA, keyEnter)
	if s := m.filterState(); s != list.FilterApplied {
		t.Fatalf("the picker's filter state is %v after enter", s)
	}
	m = pressAll(t, m, keyEsc)
	if !m.wm.IsOpen(popuplist.WIN_ID) || m.filterState() != list.Unfiltered {
		t.Errorf("esc with a filter applied should clear it and keep the picker open, state %v", m.filterState())
	}

	m = pressAll(t, m, keyEsc)
	if m.wm.IsOpen(popuplist.WIN_ID) {
		t.Error("esc without a filter should close the picker")
	}
}

func TestPickerKeysGoToAFilterBeingTyped(t *testing.T) {
	m := listingModel(t)

	m = pressAll(t, m, keySlash, keyA, keyCtrlE)
	if m.wm.GetNumberOpen() != 0 {
		t.Fatal("ctrl+e while typing a filter opened a window")
	}
	if s := m.filterState(); s != list.Filtering {
		t.Errorf("ctrl+e while typing changed the filter state to %v", s)
	}

	m = pressAll(t, m, keyEnter, keyCtrlE)
	if !m.wm.IsOpen(popuplist.WIN_ID) {
		t.Error("ctrl+e with a filter applied should open the system picker")
	}
}
