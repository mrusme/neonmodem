package postshow

import (
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/glamour"
	"github.com/charmbracelet/lipgloss"
	"github.com/mrusme/gobbs/aggregator"
	"github.com/mrusme/gobbs/models/post"
	"github.com/mrusme/gobbs/models/reply"
	"github.com/mrusme/gobbs/ui/ctx"
	"github.com/mrusme/gobbs/ui/toolkit"
)

var (
	WIN_ID = "postshow"

	viewportStyle = lipgloss.NewStyle().
			Margin(0, 0, 0, 0).
			Padding(0, 0).
			BorderTop(false).
			BorderLeft(false).
			BorderRight(false).
			BorderBottom(false)
)

type Model struct {
	ctx      *ctx.Ctx
	tk       *toolkit.ToolKit
	viewport viewport.Model

	a    *aggregator.Aggregator
	glam *glamour.TermRenderer

	buffer   string
	replyIDs []string

	activePost  *post.Post
	allReplies  []*reply.Reply
	activeReply *reply.Reply
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

		buffer:   "",
		replyIDs: []string{},
	}

	m.tk.SetViewFunc(buildView)
	m.a, _ = aggregator.New(m.ctx)

	m.tk.KeymapAdd("reply", "reply", "r")

	m.tk.SetMsgHandling(toolkit.MsgHandling{
		OnKeymapKey: []toolkit.MsgHandlingKeymapKey{
			{
				ID:      "reply",
				Handler: handleReply,
			},
		},
		OnAnyNumberKey:      handleNumberKeys,
		OnAnyUncaughtKey:    handleUncaughtKeys,
		OnViewResize:        handleViewResize,
		OnWinOpenCmd:        handleWinOpenCmd,
		OnWinRefreshDataCmd: handleWinOpenCmd,
		OnWinFreshDataCmd:   handleWinFreshDataCmd,
	})

	return m
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	ret, cmds := m.tk.HandleMsg(&m, msg)
	if ret {
		return m, tea.Batch(cmds...)
	}

	var cmd tea.Cmd

	m.viewport, cmd = m.viewport.Update(msg)
	cmds = append(cmds, cmd)

	return m, tea.Batch(cmds...)
}
