package ui

import (
	// "fmt"

	"strings"

	"github.com/mrusme/gobbs/ui/cmd"
	"github.com/mrusme/gobbs/ui/ctx"
	"github.com/mrusme/gobbs/ui/header"
	"github.com/mrusme/gobbs/ui/views/posts"
	"github.com/mrusme/gobbs/ui/windowmanager"
	"github.com/mrusme/gobbs/ui/windows/postcreate"
	"github.com/mrusme/gobbs/ui/windows/postshow"

	"github.com/mrusme/gobbs/ui/views"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
)

type KeyMap struct {
	Up    key.Binding
	Down  key.Binding
	Close key.Binding
}

var DefaultKeyMap = KeyMap{
	// Up: key.NewBinding(
	// 	key.WithKeys("k", "up"),
	// 	key.WithHelp("↑/k", "move up"),
	// ),
	// Down: key.NewBinding(
	// 	key.WithKeys("j", "down"),
	// 	key.WithHelp("↓/j", "move down"),
	// ),
	Close: key.NewBinding(
		key.WithKeys("q", "esc"),
		key.WithHelp("q/esc", "close"),
	),
}

type Model struct {
	keymap      KeyMap
	header      header.Model
	views       []views.View
	currentView int
	wm          *windowmanager.WM
	ctx         *ctx.Ctx

	viewcache         string
	renderOnlyFocused bool
}

func NewModel(c *ctx.Ctx) Model {
	m := Model{
		keymap:      DefaultKeyMap,
		currentView: 0,
		wm:          windowmanager.New(c),
		ctx:         c,

		viewcache:         "",
		renderOnlyFocused: false,
	}

	m.header = header.NewModel(m.ctx)
	m.views = append(m.views, posts.NewModel(m.ctx))

	return m
}

func (m Model) Init() tea.Cmd {
	return tea.Batch(
		tea.EnterAltScreen,
		cmd.New(cmd.ViewFocus, "*").Tea(),
		cmd.New(cmd.ViewRefreshData, "*").Tea(),
	)
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	cmds := make([]tea.Cmd, 0)

	switch msg := msg.(type) {

	case tea.KeyMsg:
		switch {
		case key.Matches(msg, m.keymap.Close):
			m.ctx.Logger.Debug("close received")
			closed, ccmds := m.wm.CloseFocused()
			if !closed {
				m.ctx.Logger.Debug("CloseFocused() was false, quitting")
				return m, tea.Quit
			}
			return m, tea.Batch(ccmds...)
		default:
			if m.wm.GetNumberOpen() > 0 {
				cmd := m.wm.Update(m.wm.Focused(), msg)
				return m, cmd
			}
		}

	case tea.WindowSizeMsg:
		m.setSizes(msg.Width, msg.Height)
		for i := range m.views {
			v, cmd := m.views[i].Update(msg)
			m.views[i] = v
			cmds = append(cmds, cmd)
		}
		m.ctx.Logger.Debugf("resizing all: %v\n", m.ctx.Content)
		ccmds := m.wm.ResizeAll(m.ctx.Content[0], m.ctx.Content[1])
		cmds = append(cmds, ccmds...)

	case cmd.Command:
		var ccmds []tea.Cmd

		switch msg.Call {

		case cmd.WinOpen:
			switch msg.Target {
			case postshow.WIN_ID:
				m.ctx.Logger.Debugln("received WinOpen")
				ccmds = m.wm.Open(
					msg.Target,
					postshow.NewModel(m.ctx),
					[4]int{3, 1, 4, 4},
					&msg,
				)
			case postcreate.WIN_ID:
				m.ctx.Logger.Debugln("received WinOpen")
				m.viewcache = m.buildView(false)
				m.renderOnlyFocused = true
				ccmds = m.wm.Open(
					msg.Target,
					postcreate.NewModel(m.ctx),
					[4]int{6, int(m.ctx.Content[1] / 3), 8, 4},
					&msg,
				)
			}
			m.ctx.Logger.Debugf("got back ccmds: %v\n", ccmds)

		case cmd.WinClose:
			switch msg.Target {
			case postcreate.WIN_ID:
				m.ctx.Logger.Debugln("received WinClose")
				m.renderOnlyFocused = false
			}

		case cmd.WMCloseWin:
			if ok, clcmds := m.wm.Close(msg.Target); ok {
				cmds = append(cmds, clcmds...)
			}

		default:
			if msg.Call < cmd.ViewFocus {
				m.ctx.Logger.Debugf("updating all with cmd: %v\n", msg)
				ccmds = m.wm.UpdateAll(msg)
			}
		}

		cmds = append(cmds, ccmds...)

	case spinner.TickMsg:
		// Do nothing

	default:
		m.ctx.Logger.Debugf("updating focused with default: %v\n", msg)
		cmds = append(cmds, m.wm.UpdateFocused(msg)...)
	}

	v, vcmd := m.views[m.currentView].Update(msg)
	m.views[m.currentView] = v
	cmds = append(cmds, vcmd)

	header, hcmd := m.header.Update(msg)
	m.header = header
	cmds = append(cmds, hcmd)

	return m, tea.Batch(cmds...)
}

func (m Model) View() string {
	return m.buildView(true)
}

func (m Model) buildView(cached bool) string {
	s := strings.Builder{}
	var tmp string = ""

	if m.viewcache != "" && m.renderOnlyFocused {
		tmp = m.viewcache
	} else {
		s.WriteString(m.header.View() + "\n")
		s.WriteString(m.views[m.currentView].View())
		tmp = s.String()
	}

	return m.wm.View(tmp, m.renderOnlyFocused)
}

func (m Model) setSizes(winWidth int, winHeight int) {
	(*m.ctx).Screen[0] = winWidth
	(*m.ctx).Screen[1] = winHeight
	m.ctx.Content[0] = m.ctx.Screen[0]
	m.ctx.Content[1] = m.ctx.Screen[1] - 5
}
