package msgerror

import (
	"strings"

	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/mrusme/gobbs/aggregator"
	"github.com/mrusme/gobbs/ui/cmd"
	"github.com/mrusme/gobbs/ui/ctx"
)

var (
	WIN_ID = "msgerror"

	viewportStyle = lipgloss.NewStyle().
			Margin(0, 0, 0, 0).
			Padding(0, 0).
			BorderTop(false).
			BorderLeft(false).
			BorderRight(false).
			BorderBottom(false)
)

type KeyMap struct {
}

var DefaultKeyMap = KeyMap{}

type Model struct {
	ctx      *ctx.Ctx
	keymap   KeyMap
	wh       [2]int
	focused  bool
	xywh     [4]int
	viewport viewport.Model

	a *aggregator.Aggregator

	err error

	viewcache           string
	viewcacheTextareaXY []int
}

func (m Model) Init() tea.Cmd {
	return nil
}

func (m Model) Focus() {
	m.focused = true
}

func (m Model) Blur() {
	m.focused = false
}

func NewModel(c *ctx.Ctx) Model {
	m := Model{
		ctx:     c,
		keymap:  DefaultKeyMap,
		focused: false,
		xywh:    [4]int{0, 0, 0, 0},

		err: nil,

		viewcache:           "",
		viewcacheTextareaXY: []int{0, 0, 0, 0},
	}

	m.a, _ = aggregator.New(m.ctx)

	return m
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {

	case tea.WindowSizeMsg:
		m.wh[0] = msg.Width
		m.wh[1] = msg.Height
		m.ctx.Logger.Debugf("received WindowSizeMsg: %v\n", m.wh)
		viewportWidth := m.wh[0] - 2
		viewportHeight := m.wh[1] - 5

		viewportStyle.Width(viewportWidth)
		viewportStyle.Height(viewportHeight)
		m.viewport = viewport.New(viewportWidth-4, viewportHeight-4)
		m.viewport.Width = viewportWidth - 4
		m.viewport.Height = viewportHeight + 1

	case cmd.Command:
		m.ctx.Logger.Debugf("got command: %v\n", msg)
		switch msg.Call {
		case cmd.MsgError:
			m.err = msg.GetArg("error").(error)
			if m.err != nil {
				m.viewport.SetContent(m.err.Error())
				m.viewcache = m.buildView(false)
			}
			return m, nil
		case cmd.WinClose:
			if msg.Target == WIN_ID {
				m.err = nil
				return m, nil
			}
		case cmd.WinFocus:
			if msg.Target == WIN_ID ||
				msg.Target == "*" {
				m.focused = true
				m.viewcache = m.buildView(false)
			}
			return m, nil
		case cmd.WinBlur:
			if msg.Target == WIN_ID ||
				msg.Target == "*" {
				m.focused = false
			}
			return m, nil
		default:
			m.ctx.Logger.Debugf("received unhandled command: %v\n", msg)
		}

		// default:
		// 	m.ctx.Logger.Debugf("received unhandled msg: %v\n", msg)
	}

	var vcmd tea.Cmd

	m.ctx.Logger.Debugf("ERROR IS: %v\n", m.err)
	if m.err != nil {
		m.viewport.SetContent(m.err.Error())
	}
	m.viewport, vcmd = m.viewport.Update(msg)
	cmds = append(cmds, vcmd)

	return m, tea.Batch(cmds...)
}

func (m Model) View() string {
	return m.buildView(true)
}

func (m Model) buildView(cached bool) string {
	var view strings.Builder = strings.Builder{}

	if cached && m.focused == false && m.viewcache != "" {
		m.ctx.Logger.Debugln("Cached View()")

		return m.viewcache
	}

	title := "Error"
	titlebar := m.ctx.Theme.DialogBox.Titlebar.Focused.
		Align(lipgloss.Center).
		Width(m.wh[0]).
		Render(title)

	bottombar := m.ctx.Theme.DialogBox.Bottombar.
		Width(m.wh[0]).
		Render("esc close")

	ui := lipgloss.JoinVertical(
		lipgloss.Center,
		titlebar,
		viewportStyle.Render(m.viewport.View()),
		bottombar,
	)

	var tmp string
	if m.focused {
		tmp = m.ctx.Theme.DialogBox.Window.Focused.Render(ui)
	} else {
		tmp = m.ctx.Theme.DialogBox.Window.Blurred.Render(ui)
	}

	view.WriteString(tmp)

	return view.String()
}
