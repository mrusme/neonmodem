package ui

import (
	// "fmt"

	"strings"

	"github.com/mrusme/gobbs/ui/ctx"
	"github.com/mrusme/gobbs/ui/navigation"
	"github.com/mrusme/gobbs/ui/views/posts"

	"github.com/mrusme/gobbs/ui/views"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
)

type KeyMap struct {
	FirstTab      key.Binding
	SecondTab     key.Binding
	ThirdTab      key.Binding
	FourthTab     key.Binding
	FifthTab      key.Binding
	SixthTab      key.Binding
	SeventhTab    key.Binding
	EightTab      key.Binding
	NinthTab      key.Binding
	TenthTab      key.Binding
	EleventhTab   key.Binding
	TwelfthTab    key.Binding
	ThirteenthTab key.Binding
	PrevTab       key.Binding
	NextTab       key.Binding
	Up            key.Binding
	Down          key.Binding
	Quit          key.Binding
}

var DefaultKeyMap = KeyMap{
	FirstTab: key.NewBinding(
		key.WithKeys("f1"),
		key.WithHelp("f1", "first tab"),
	),
	SecondTab: key.NewBinding(
		key.WithKeys("f2"),
		key.WithHelp("f2", "second tab"),
	),
	ThirdTab: key.NewBinding(
		key.WithKeys("f3"),
		key.WithHelp("f3", "third tab"),
	),
	FourthTab: key.NewBinding(
		key.WithKeys("f4"),
		key.WithHelp("f4", "fourth tab"),
	),
	FifthTab: key.NewBinding(
		key.WithKeys("f5"),
		key.WithHelp("f5", "fifth tab"),
	),
	SixthTab: key.NewBinding(
		key.WithKeys("f6"),
		key.WithHelp("f6", "sixth tab"),
	),
	SeventhTab: key.NewBinding(
		key.WithKeys("f7"),
		key.WithHelp("f7", "seventh tab"),
	),
	EightTab: key.NewBinding(
		key.WithKeys("f8"),
		key.WithHelp("f8", "eight tab"),
	),
	NinthTab: key.NewBinding(
		key.WithKeys("f9"),
		key.WithHelp("f9", "ninth tab"),
	),
	TenthTab: key.NewBinding(
		key.WithKeys("f10"),
		key.WithHelp("f10", "tenth tab"),
	),
	EleventhTab: key.NewBinding(
		key.WithKeys("f11"),
		key.WithHelp("f11", "eleventh tab"),
	),
	TwelfthTab: key.NewBinding(
		key.WithKeys("f12"),
		key.WithHelp("f12", "twelfth tab"),
	),
	ThirteenthTab: key.NewBinding(
		key.WithKeys("f13"),
		key.WithHelp("f13", "thirteenth tab"),
	),
	PrevTab: key.NewBinding(
		key.WithKeys("ctrl+p"),
		key.WithHelp("ctrl+p", "previous tab"),
	),
	NextTab: key.NewBinding(
		key.WithKeys("ctrl+n"),
		key.WithHelp("ctrl+n", "next tab"),
	),
	Up: key.NewBinding(
		key.WithKeys("k", "up"),
		key.WithHelp("↑/k", "move up"),
	),
	Down: key.NewBinding(
		key.WithKeys("j", "down"),
		key.WithHelp("↓/j", "move down"),
	),
	Quit: key.NewBinding(
		key.WithKeys("q", "ctrl+q"),
		key.WithHelp("q/Q", "quit"),
	),
}

type Model struct {
	keymap KeyMap
	nav    navigation.Model
	views  []views.View
	ctx    *ctx.Ctx
}

func NewModel(c *ctx.Ctx) Model {
	m := Model{
		keymap: DefaultKeyMap,
		ctx:    c,
	}

	m.nav = navigation.NewModel(m.ctx)
	for _, capability := range (*m.ctx.Systems[0]).GetCapabilities() { // TODO
		switch capability.ID {
		case "posts":
			m.views = append(m.views, posts.NewModel(m.ctx))
			// case "groups":
			// 	m.views = append(m.views, groups.NewModel(m.ctx))
			// case "search":
			// 	m.views = append(m.views, search.NewModel(m.ctx))
		}
	}

	return m
}

func (m Model) Init() tea.Cmd {
	return tea.Batch(tea.EnterAltScreen)
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	cmds := make([]tea.Cmd, 0)

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch {
		case key.Matches(msg, m.keymap.Quit):
			return m, tea.Quit

		case key.Matches(msg, m.keymap.FirstTab):
			m.nav.NthTab(1)
			return m, nil

		case key.Matches(msg, m.keymap.SecondTab):
			m.nav.NthTab(2)
			return m, nil

		case key.Matches(msg, m.keymap.ThirdTab):
			m.nav.NthTab(3)
			return m, nil

		case key.Matches(msg, m.keymap.FourthTab):
			m.nav.NthTab(4)
			return m, nil

		case key.Matches(msg, m.keymap.FifthTab):
			m.nav.NthTab(5)
			return m, nil

		case key.Matches(msg, m.keymap.SixthTab):
			m.nav.NthTab(6)
			return m, nil

		case key.Matches(msg, m.keymap.SeventhTab):
			m.nav.NthTab(7)
			return m, nil

		case key.Matches(msg, m.keymap.EightTab):
			m.nav.NthTab(8)
			return m, nil

		case key.Matches(msg, m.keymap.NinthTab):
			m.nav.NthTab(9)
			return m, nil

		case key.Matches(msg, m.keymap.TenthTab):
			m.nav.NthTab(10)
			return m, nil

		case key.Matches(msg, m.keymap.EleventhTab):
			m.nav.NthTab(11)
			return m, nil

		case key.Matches(msg, m.keymap.TwelfthTab):
			m.nav.NthTab(12)
			return m, nil

		case key.Matches(msg, m.keymap.ThirteenthTab):
			m.nav.NthTab(13)
			return m, nil

		case key.Matches(msg, m.keymap.PrevTab):
			m.nav.PrevTab()
			return m, nil

		case key.Matches(msg, m.keymap.NextTab):
			m.nav.NextTab()
			return m, nil
		}

	case tea.WindowSizeMsg:
		m.setSizes(msg.Width, msg.Height)
		for i := range m.views {
			v, cmd := m.views[i].Update(msg)
			m.views[i] = v
			cmds = append(cmds, cmd)
		}
	}

	v, cmd := m.views[m.nav.CurrentId].Update(msg)
	m.views[m.nav.CurrentId] = v
	cmds = append(cmds, cmd)

	nav, cmd := m.nav.Update(msg)
	m.nav = nav
	cmds = append(cmds, cmd)

	return m, tea.Batch(cmds...)
}

func (m Model) View() string {
	s := strings.Builder{}
	s.WriteString(m.nav.View() + "\n\n")
	s.WriteString(m.views[m.nav.CurrentId].View())
	return s.String()
}

func (m Model) setSizes(winWidth int, winHeight int) {
	(*m.ctx).Screen[0] = winWidth
	(*m.ctx).Screen[1] = winHeight
	m.ctx.Content[0] = m.ctx.Screen[0]
	m.ctx.Content[1] = m.ctx.Screen[1] - 5
}
