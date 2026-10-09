package posts

import (
	"slices"
	"testing"

	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/list"
	tea "charm.land/bubbletea/v2"
	"github.com/mrusme/neonmodem/internal/models/post"
	"github.com/mrusme/neonmodem/internal/system"
	"github.com/mrusme/neonmodem/internal/ui/msgs"
)

func helpKeys(bindings []key.Binding) []string {
	var out []string
	for _, b := range bindings {
		if b.Enabled() {
			out = append(out, b.Help().Key+" "+b.Help().Desc)
		}
	}
	return out
}

func fullHelpKeys(m Model) []string {
	var out []string
	for _, group := range m.list.FullHelp() {
		out = append(out, helpKeys(group)...)
	}
	return out
}

func TestTheHelpLineNamesTheQuitKey(t *testing.T) {
	lemmy, tags := newSystems()
	m, _ := testModel(t, lemmy, tags)
	m, _ = run(t, m, m.refresh())

	short := helpKeys(m.list.ShortHelp())
	if !slices.Contains(short, "ctrl+q quit") || slices.Contains(short, "! notices") {
		t.Errorf("without notices the help line is %q", short)
	}

	v, _ := m.Update(msgs.NoticesChanged{Count: 2})
	m = v.(Model)
	short = helpKeys(m.list.ShortHelp())
	notices, quit := slices.Index(short, "! notices"), slices.Index(short, "ctrl+q quit")
	if notices < 0 || quit < 0 || notices > quit {
		t.Errorf("with notices the help line is %q", short)
	}

	full := fullHelpKeys(m)
	if !slices.Contains(full, "ctrl+q quit") || !slices.Contains(full, "! notices") {
		t.Errorf("the full help is %q", full)
	}
}

func typeKeys(m Model, keys ...tea.KeyPressMsg) (Model, []tea.Cmd) {
	var cmds []tea.Cmd
	for _, k := range keys {
		v, cmd := m.Update(k)
		m = v.(Model)
		cmds = append(cmds, cmd)
	}
	return m, cmds
}

func quitsAny(cmds []tea.Cmd) bool {
	for _, cmd := range cmds {
		if cmd == nil {
			continue
		}
		if _, ok := cmd().(tea.QuitMsg); ok {
			return true
		}
	}
	return false
}

func letters(s string) []tea.KeyPressMsg {
	var keys []tea.KeyPressMsg
	for _, r := range s {
		keys = append(keys, tea.KeyPressMsg{Code: r, Text: string(r)})
	}
	return keys
}

var keyEsc = tea.KeyPressMsg{Code: tea.KeyEscape}

func TestEscClearsTheFilterAndNeverQuits(t *testing.T) {
	sys := &fakeSystem{
		title:    "lemmy.example",
		ordering: system.Only(system.OrderNew),
		lists: map[system.Order][]post.Post{
			system.OrderNew: {
				{ID: "a", Subject: "alpha", CreatedAt: at(3)},
				{ID: "b", Subject: "beta", CreatedAt: at(2)},
			},
		},
	}
	m, _ := testModel(t, sys)
	m, _ = run(t, m, m.refresh())

	m.list.SetFilterText("alp")
	if m.list.FilterState() != list.FilterApplied || len(m.list.VisibleItems()) != 1 {
		t.Fatalf("filter state %v with %d visible posts", m.list.FilterState(), len(m.list.VisibleItems()))
	}

	m, cmds := typeKeys(m, keyEsc)
	if m.list.FilterState() != list.Unfiltered || len(m.list.VisibleItems()) != 2 {
		t.Errorf("esc left the filter state %v with %d visible posts", m.list.FilterState(), len(m.list.VisibleItems()))
	}

	m, _ = typeKeys(m, letters("/be")...)
	if m.list.FilterState() != list.Filtering {
		t.Fatalf("typing a filter gives the state %v", m.list.FilterState())
	}
	m, more := typeKeys(m, keyEsc)
	cmds = append(cmds, more...)
	if m.list.FilterState() != list.Unfiltered {
		t.Errorf("esc while typing left the filter state %v", m.list.FilterState())
	}

	_, more = typeKeys(m, keyEsc)
	cmds = append(cmds, more...)
	if quitsAny(cmds) {
		t.Error("esc quit the program")
	}
}
