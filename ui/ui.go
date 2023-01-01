package ui

import (
	// "fmt"

	"strings"

	"github.com/mrusme/gobbs/ui/ctx"
	"github.com/mrusme/gobbs/ui/header"
	"github.com/mrusme/gobbs/ui/views/posts"

	"github.com/mrusme/gobbs/ui/views"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
)

type KeyMap struct {
	Up   key.Binding
	Down key.Binding
	Quit key.Binding
}

var DefaultKeyMap = KeyMap{
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
	keymap      KeyMap
	header      header.Model
	views       []views.View
	currentView int
	ctx         *ctx.Ctx
}

func NewModel(c *ctx.Ctx) Model {
	m := Model{
		keymap:      DefaultKeyMap,
		currentView: 0,
		ctx:         c,
	}

	m.header = header.NewModel(m.ctx)
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
		}

	case tea.WindowSizeMsg:
		m.setSizes(msg.Width, msg.Height)
		for i := range m.views {
			v, cmd := m.views[i].Update(msg)
			m.views[i] = v
			cmds = append(cmds, cmd)
		}
	}

	v, cmd := m.views[m.currentView].Update(msg)
	m.views[m.currentView] = v
	cmds = append(cmds, cmd)

	header, cmd := m.header.Update(msg)
	m.header = header
	cmds = append(cmds, cmd)

	return m, tea.Batch(cmds...)
}

func (m Model) View() string {
	s := strings.Builder{}
	s.WriteString(m.header.View() + "\n\n")
	s.WriteString(m.views[m.currentView].View())
	return s.String()
}

func (m Model) setSizes(winWidth int, winHeight int) {
	(*m.ctx).Screen[0] = winWidth
	(*m.ctx).Screen[1] = winHeight
	m.ctx.Content[0] = m.ctx.Screen[0]
	m.ctx.Content[1] = m.ctx.Screen[1] - 5
}
