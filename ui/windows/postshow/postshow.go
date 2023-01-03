package postshow

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/glamour"
	"github.com/charmbracelet/lipgloss"
	"github.com/mrusme/gobbs/aggregator"
	"github.com/mrusme/gobbs/models/post"
	"github.com/mrusme/gobbs/models/reply"
	"github.com/mrusme/gobbs/ui/cmd"
	"github.com/mrusme/gobbs/ui/ctx"
	"github.com/mrusme/gobbs/ui/toolkit"
	"github.com/mrusme/gobbs/ui/windows/postcreate"
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

func handleReply(mi interface{}) (bool, []tea.Cmd) {
	var m *Model = mi.(*Model)
	var cmds []tea.Cmd
	var replyToIdx int = 0
	var err error

	// m.viewcache = m.buildView(false)
	m.tk.CacheView(m)

	if m.buffer != "" {
		replyToIdx, err = strconv.Atoi(m.buffer)

		if err != nil {
			// TODO: Handle error
		}

		if replyToIdx >= len(m.replyIDs) {
			// TODO: Handle error
		}
	}

	m.ctx.Logger.Debugf("replyToIdx: %d", replyToIdx)
	var rtype cmd.Arg = cmd.Arg{Name: "replyTo"}
	var rarg cmd.Arg
	var ridx cmd.Arg = cmd.Arg{Name: "replyToIdx", Value: replyToIdx}

	if replyToIdx == 0 {
		rtype.Value = "post"
		rarg.Name = "post"
		rarg.Value = *m.activePost
	} else {
		rtype.Value = "reply"
		rarg.Name = "reply"
		rarg.Value = *m.allReplies[(replyToIdx - 1)]
	}

	cmd := cmd.New(cmd.WinOpen, postcreate.WIN_ID, rtype, rarg, ridx)
	cmds = append(cmds, cmd.Tea())

	m.ctx.Logger.Debugln("caching view")
	m.ctx.Logger.Debugf("buffer: %s", m.buffer)
	// m.viewcache = m.buildView(false)

	return true, cmds
}

func handleNumberKeys(mi interface{}, n int8) (bool, []tea.Cmd) {
	var m *Model = mi.(*Model)
	var cmds []tea.Cmd

	m.buffer += strconv.Itoa(int(n))

	return false, cmds
}

func handleUncaughtKeys(mi interface{}, k tea.KeyMsg) (bool, []tea.Cmd) {
	var m *Model = mi.(*Model)
	var cmds []tea.Cmd

	m.buffer = ""

	return false, cmds
}

func handleViewResize(mi interface{}) (bool, []tea.Cmd) {
	var m *Model = mi.(*Model)
	var cmds []tea.Cmd

	m.ctx.Logger.Debugf("received WindowSizeMsg: %vx%v\n", m.tk.ViewWidth(), m.tk.ViewHeight())
	viewportWidth := m.tk.ViewWidth() - 2
	viewportHeight := m.tk.ViewHeight() - 5

	viewportStyle.Width(viewportWidth)
	viewportStyle.Height(viewportHeight)
	m.viewport = viewport.New(viewportWidth-4, viewportHeight-4)
	m.viewport.Width = viewportWidth - 4
	m.viewport.Height = viewportHeight + 1
	// cmds = append(cmds, viewport.Sync(m.viewport))

	return false, cmds
}

func handleWinOpenCmd(mi interface{}, c cmd.Command) (bool, []tea.Cmd) {
	var m *Model = mi.(*Model)
	var cmds []tea.Cmd

	if c.Target == WIN_ID {
		m.ctx.Logger.Debug("got own WinOpen command")
		m.activePost = c.GetArg("post").(*post.Post)
		m.viewport.SetContent(m.renderViewport(m.activePost))
		m.ctx.Logger.Debugf("loading post: %v", m.activePost.ID)
		m.ctx.Loading = true
		cmds = append(cmds, m.loadPost(m.activePost))
		return true, cmds
	}

	return false, cmds
}

func handleWinFreshDataCmd(mi interface{}, c cmd.Command) (bool, []tea.Cmd) {
	var m *Model = mi.(*Model)
	var cmds []tea.Cmd

	if c.Target == WIN_ID ||
		c.Target == "*" {
		m.ctx.Logger.Debug("got *post.Post")
		m.activePost = c.GetArg("post").(*post.Post)
		m.viewport.SetContent(m.renderViewport(m.activePost))
		m.ctx.Loading = false
		return true, cmds
	}

	return false, cmds
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

func (m *Model) loadPost(p *post.Post) tea.Cmd {
	return func() tea.Msg {
		m.ctx.Logger.Debug("------ EXECUTED -----")
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

func (m Model) View() string {
	return m.tk.View(&m, true)
}

func buildView(mi interface{}, cached bool) string {
	var m *Model = mi.(*Model)

	if cached && !m.tk.IsFocused() && m.tk.IsCached() {
		m.ctx.Logger.Debugln("Cached View()")

		return m.tk.GetCachedView()
	}
	m.ctx.Logger.Debugln("View()")
	m.ctx.Logger.Debugf("IsFocused: %v\n", m.tk.IsFocused())

	return m.tk.Dialog(
		"Post",
		viewportStyle.Render(m.viewport.View()),
	)
}

func (m *Model) renderViewport(p *post.Post) string {
	var out string = ""

	var err error
	m.glam, err = glamour.NewTermRenderer(
		glamour.WithAutoStyle(),
		glamour.WithWordWrap(m.viewport.Width),
	)
	if err != nil {
		m.ctx.Logger.Error(err)
		m.glam = nil
	}

	adj := "writes"
	if p.Subject[len(p.Subject)-1:] == "?" {
		adj = "asks"
	}

	body, err := m.glam.Render(p.Body)
	if err != nil {
		m.ctx.Logger.Error(err)
		body = p.Body
	}
	out += fmt.Sprintf(
		" %s\n\n %s\n%s",
		m.ctx.Theme.Post.Author.Render(
			fmt.Sprintf("%s %s:", p.Author.Name, adj),
		),
		m.ctx.Theme.Post.Subject.Render(p.Subject),
		body,
	)

	m.replyIDs = []string{p.ID}
	m.activePost = p
	out += m.renderReplies(0, p.Author.Name, &p.Replies)

	return out
}

func (m *Model) renderReplies(
	level int,
	inReplyTo string,
	replies *[]reply.Reply,
) string {
	var out string = ""

	if replies == nil {
		return ""
	}

	for ri, re := range *replies {
		var err error = nil
		var body string = ""
		var author string = ""

		if re.Deleted {
			body = "\n  DELETED\n\n"
			author = "DELETED"
		} else {
			body, err = m.glam.Render(re.Body)
			if err != nil {
				m.ctx.Logger.Error(err)
				body = re.Body
			}

			author = re.Author.Name
		}

		m.replyIDs = append(m.replyIDs, re.ID)
		m.allReplies = append(m.allReplies, &(*replies)[ri])
		idx := len(m.replyIDs) - 1

		replyIdPadding := (m.viewport.Width - len(author) - len(inReplyTo) - 28)
		if replyIdPadding < 0 {
			replyIdPadding = 0
		}

		out += fmt.Sprintf(
			"\n\n %s %s%s%s\n%s",
			m.ctx.Theme.Reply.Author.Render(
				author,
			),
			lipgloss.NewStyle().
				Foreground(m.ctx.Theme.Reply.Author.GetBackground()).
				Render(fmt.Sprintf("writes in reply to %s:", inReplyTo)),
			strings.Repeat(" ", replyIdPadding),
			lipgloss.NewStyle().
				Foreground(lipgloss.Color("#777777")).
				Render(fmt.Sprintf("#%d", idx)),
			body,
		)

		idx++
		out += m.renderReplies(level+1, re.Author.Name, &re.Replies)
	}

	return out
}
