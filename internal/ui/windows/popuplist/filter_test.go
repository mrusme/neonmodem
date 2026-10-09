package popuplist

import (
	"strings"
	"testing"
	"time"

	"charm.land/bubbles/v2/list"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/mrusme/neonmodem/internal/models/forum"
	"github.com/mrusme/neonmodem/internal/ui/msgs"
)

func filterMatches(cmd tea.Cmd) []tea.Msg {
	if cmd == nil {
		return nil
	}
	done := make(chan tea.Msg, 1)
	go func() { done <- cmd() }()

	var msg tea.Msg
	select {
	case msg = <-done:
	case <-time.After(100 * time.Millisecond):
		return nil
	}

	switch msg := msg.(type) {
	case tea.BatchMsg:
		var out []tea.Msg
		for _, c := range msg {
			out = append(out, filterMatches(c)...)
		}
		return out
	case list.FilterMatchesMsg:
		return []tea.Msg{msg}
	}
	return nil
}

func typeInto(m *Model, keys ...tea.KeyPressMsg) {
	for _, k := range keys {
		_, cmd := m.Update(k)
		for _, msg := range filterMatches(cmd) {
			m.Update(msg)
		}
	}
}

func TestTheBarFollowsTheFilterAndTheSizeHolds(t *testing.T) {
	const width, height = 44, 14

	m := testModel(t)
	m.Update(msgs.OpenPicker{Kind: msgs.PickSystem, Items: []list.Item{
		forum.Forum{Name: "alpha", Info: "first"},
		forum.Forum{Name: "beta", Info: "second"},
	}})
	m.Update(tea.WindowSizeMsg{Width: width, Height: height})
	m.Update(msgs.FocusWindow{ID: WIN_ID})

	check := func(state string, want ...string) {
		t.Helper()
		view := m.View()
		if w, h := lipgloss.Width(view), lipgloss.Height(view); w != width || h != height {
			t.Errorf("%s: the picker renders %dx%d", state, w, h)
		}
		bar := strings.Join(strings.Fields(ansi.Strip(view)), " ")
		for _, text := range want {
			if !strings.Contains(bar, text) {
				t.Errorf("%s: the picker doesn't show %q:\n%s", state, text, ansi.Strip(view))
			}
		}
	}

	check("no filter", "enter choose", "esc close")

	typeInto(m, tea.KeyPressMsg{Code: '/', Text: "/"}, tea.KeyPressMsg{Code: 'a', Text: "a"})
	if m.FilterState() != list.Filtering {
		t.Fatalf("typing gives the state %v", m.FilterState())
	}
	check("typing", "enter apply filter", "esc cancel filter")

	typeInto(m, tea.KeyPressMsg{Code: tea.KeyEnter})
	if m.FilterState() != list.FilterApplied {
		t.Fatalf("enter gives the state %v", m.FilterState())
	}
	check("applied", "enter choose", "esc clear filter")

	typeInto(m, tea.KeyPressMsg{Code: tea.KeyEscape})
	if m.FilterState() != list.Unfiltered {
		t.Fatalf("esc gives the state %v", m.FilterState())
	}
	check("cleared", "enter choose", "esc close")
}
