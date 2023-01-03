package postcreate

import (
	"github.com/charmbracelet/bubbles/textarea"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/mrusme/gobbs/aggregator"
	"github.com/mrusme/gobbs/ui/ctx"
	"github.com/mrusme/gobbs/ui/toolkit"
)

var (
	WIN_ID = "postcreate"
)

type Model struct {
	ctx *ctx.Ctx
	tk  *toolkit.ToolKit

	xywh [4]int

	textarea textarea.Model

	a *aggregator.Aggregator

	replyToIdx   int
	replyTo      string
	replyToIface interface{}

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

		replyToIdx:   0,
		replyTo:      "",
		replyToIface: nil,

		viewcache:           "",
		viewcacheTextareaXY: []int{0, 0, 0, 0},
	}

	m.textarea = textarea.New()
	m.textarea.Placeholder = "Type in your reply ..."
	m.textarea.Prompt = ""

	m.tk.KeymapAdd("submit", "submit", "ctrl+s")

	m.a, _ = aggregator.New(m.ctx)

	m.tk.SetViewFunc(buildView)
	m.tk.SetMsgHandling(toolkit.MsgHandling{
		OnKeymapKey: []toolkit.MsgHandlingKeymapKey{
			{
				ID:      "submit",
				Handler: handleSubmit,
			},
		},
		OnWinOpenCmd:  handleWinOpenCmd,
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

	var tcmd tea.Cmd

	if !m.textarea.Focused() {
		cmds = append(cmds, m.textarea.Focus())
	}
	m.textarea, tcmd = m.textarea.Update(msg)
	cmds = append(cmds, tcmd)

	return m, tea.Batch(cmds...)
}
