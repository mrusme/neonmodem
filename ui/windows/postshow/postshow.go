package postshow

import (
	"time"

	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/glamour"
	"github.com/charmbracelet/lipgloss"
	"github.com/mrusme/neonmodem/aggregator"
	"github.com/mrusme/neonmodem/models/post"
	"github.com/mrusme/neonmodem/models/reply"
	"github.com/mrusme/neonmodem/ui/cmd"
	"github.com/mrusme/neonmodem/ui/ctx"
	"github.com/mrusme/neonmodem/ui/toolkit"
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
	ctx *ctx.Ctx
	tk  *toolkit.ToolKit

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

	m.tk.KeymapAdd("reply", "reply (prefix with #, e.g. '2r')", "r")
	m.tk.KeymapAdd("open", "open", "o")

	m.a, _ = aggregator.New(m.ctx)

	m.tk.SetViewFunc(buildView)
	m.tk.SetMsgHandling(toolkit.MsgHandling{
		OnKeymapKey: []toolkit.MsgHandlingKeymapKey{
			{
				ID:      "reply",
				Handler: handleReply,
			},
			{
				ID:      "open",
				Handler: handleOpen,
			},
		},
		OnAnyNumberKey:      handleNumberKeys,
		OnAnyUncaughtKey:    handleUncaughtKeys,
		OnViewResize:        handleViewResize,
		OnWinOpenCmd:        handleWinOpenCmd,
		OnWinRefreshDataCmd: handleWinRefreshDataCmd,
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

func (m *Model) loadPost(p *post.Post, delay ...time.Duration) tea.Cmd {
	return func() tea.Msg {
		if len(delay) == 1 {
			time.Sleep(delay[0])
		}

		if err := m.a.LoadPost(p); err != nil {
			m.ctx.Logger.Error(err)
			c := cmd.New(
				cmd.MsgError,
				WIN_ID,
				cmd.Arg{Name: "error", Value: err},
			)
			return *c
		}

		c := cmd.New(
			cmd.WinFreshData,
			WIN_ID,
			cmd.Arg{Name: "post", Value: p},
		)
		return *c
	}
}
