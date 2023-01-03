package msgerror

import (
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/mrusme/gobbs/aggregator"
	"github.com/mrusme/gobbs/ui/ctx"
	"github.com/mrusme/gobbs/ui/toolkit"
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

type Model struct {
	ctx  *ctx.Ctx
	tk   *toolkit.ToolKit
	xywh [4]int

	viewport viewport.Model

	a *aggregator.Aggregator

	err error

	viewcache           string
	viewcacheTextareaXY []int
}

func (m Model) Init() tea.Cmd {
	return nil
}

func NewModel(c *ctx.Ctx) Model {
	m := Model{
		ctx: c,
		tk: toolkit.New(
			WIN_ID,
			c.Theme,
			c.Logger,
		),
		xywh: [4]int{0, 0, 0, 0},

		err: nil,

		viewcache:           "",
		viewcacheTextareaXY: []int{0, 0, 0, 0},
	}

	m.a, _ = aggregator.New(m.ctx)
	m.tk.SetViewFunc(buildView)
	m.tk.SetMsgHandling(toolkit.MsgHandling{
		OnViewResize:  handleViewResize,
		OnMsgErrorCmd: handleMsgErrorCmd,
		OnWinCloseCmd: handleWinCloseCmd,
	})

	return m
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	ret, cmds := m.tk.HandleMsg(&m, msg)
	if ret {
		return m, tea.Batch(cmds...)
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
