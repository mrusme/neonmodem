package postcreate

import (
	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/mrusme/neonmodem/aggregator"
	"github.com/mrusme/neonmodem/ui/ctx"
	"github.com/mrusme/neonmodem/ui/toolkit"
)

var (
	WIN_ID = "postcreate"
)

type Model struct {
	ctx *ctx.Ctx
	tk  *toolkit.ToolKit

	xywh [4]int

	textinput    textinput.Model
	textarea     textarea.Model
	inputFocused int

	a *aggregator.Aggregator

	action     string
	iface      interface{}
	replyToIdx int
	replyTo    string

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

		inputFocused: 0,

		action:     "",
		iface:      nil,
		replyToIdx: 0,
		replyTo:    "",

		viewcache:           "",
		viewcacheTextareaXY: []int{0, 0, 0, 0},
	}

	m.textinput = textinput.New()
	m.textinput.Placeholder = "Subject goes here"
	m.textinput.Prompt = ""

	m.textarea = textarea.New()
	m.textarea.Placeholder = "Type in your post ..."
	m.textarea.Prompt = ""

	m.tk.KeymapAdd("tab", "tab", "tab") // TODO CONTINUE HERE
	m.tk.KeymapAdd("submit", "submit", "ctrl+s")

	m.a, _ = aggregator.New(m.ctx)

	m.tk.SetViewFunc(buildView)
	m.tk.SetMsgHandling(toolkit.MsgHandling{
		OnKeymapKey: []toolkit.MsgHandlingKeymapKey{
			{
				ID:      "tab",
				Handler: handleTab,
			},
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

	switch m.inputFocused {

	case 0:
		if !m.textinput.Focused() {
			cmds = append(cmds, m.textinput.Focus())
		}
		m.textinput, tcmd = m.textinput.Update(msg)

	case 1:
		if !m.textarea.Focused() {
			cmds = append(cmds, m.textarea.Focus())
		}
		m.textarea, tcmd = m.textarea.Update(msg)

	}
	cmds = append(cmds, tcmd)

	return m, tea.Batch(cmds...)
}
