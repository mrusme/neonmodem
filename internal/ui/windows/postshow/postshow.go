package postshow

import (
	"context"
	"time"

	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"
	"github.com/mrusme/neonmodem/internal/feed"
	"github.com/mrusme/neonmodem/internal/models/post"
	"github.com/mrusme/neonmodem/internal/models/reply"
	"github.com/mrusme/neonmodem/internal/ui/ctx"
	"github.com/mrusme/neonmodem/internal/ui/msgs"
	"github.com/mrusme/neonmodem/internal/ui/toolkit"
	"github.com/mrusme/neonmodem/internal/ui/windows"
)

const WIN_ID = "postshow"

type loadedMsg struct {
	gen      int64
	post     *post.Post
	rendered renderedPost
}

type loadFailedMsg struct {
	gen int64
	err error
}

type Model struct {
	ctx *ctx.Ctx
	tk  *toolkit.ToolKit

	viewport viewport.Model

	buffer   string
	replyIDs []string

	activePost *post.Post
	allReplies []*reply.Reply
	loading    bool
}

func NewModel(c *ctx.Ctx) *Model {
	m := &Model{
		ctx:      c,
		tk:       toolkit.New(WIN_ID, c),
		viewport: viewport.New(),
		replyIDs: []string{},
	}

	m.tk.KeymapAdd("reply", "reply (prefix with #, e.g. '2r')", "r")
	m.tk.KeymapAdd("open", "open in browser", "o")
	m.tk.KeymapAdd("older", "older replies", "z")
	keys := []toolkit.MsgHandlingKeymapKey{
		{ID: "reply", Handler: handleReply},
		{ID: "open", Handler: handleOpen},
		{ID: "older", Handler: handleOlder},
	}
	if len(c.OpenWith) > 0 {
		m.tk.KeymapAdd("openwith", "open with", "O")
		keys = append(keys, toolkit.MsgHandlingKeymapKey{ID: "openwith", Handler: handleOpenWith})
	}

	m.tk.SetViewFunc(buildView)
	m.tk.SetMsgHandling(toolkit.MsgHandling{
		OnKeymapKey:      keys,
		OnAnyNumberKey:   handleNumberKeys,
		OnAnyUncaughtKey: handleUncaughtKeys,
		OnViewResize:     handleViewResize,
	})

	return m
}

func (m *Model) Update(msg tea.Msg) (windows.Window, tea.Cmd) {
	switch msg := msg.(type) {
	case msgs.OpenPost:
		return m, m.open(msg.Post)

	case msgs.ReloadPost:
		if m.activePost == nil {
			return m, nil
		}
		m.loading = true
		m.ctx.Loading = true
		return m, m.loadPost(m.activePost, m.ctx.NextLoadGen(), msg.Delay)

	case loadedMsg:
		if !m.ctx.IsCurrentLoadGen(msg.gen) {
			return m, nil
		}
		m.activePost = msg.post
		m.viewport.SetContent(msg.rendered.content)
		m.replyIDs = msg.rendered.replyIDs
		m.allReplies = msg.rendered.allReplies
		m.loading = false
		m.ctx.Loading = false
		m.tk.InvalidateCache()
		return m, nil

	case loadFailedMsg:
		if !m.ctx.IsCurrentLoadGen(msg.gen) {
			return m, nil
		}
		m.loading = false
		m.ctx.Loading = false
		return m, msgs.Send(msgs.Error(msg.err))
	}

	if handled, cmds := m.tk.HandleMsg(m, msg); handled {
		return m, tea.Batch(cmds...)
	}

	var cmd tea.Cmd
	m.viewport, cmd = m.viewport.Update(msg)
	return m, cmd
}

func (m *Model) open(p post.Post) tea.Cmd {
	m.activePost = &p
	m.viewport.SetContent(renderLoadingPlaceholder(m.ctx, m.activePost, m.viewport.Width()))
	m.viewport.GotoTop()
	m.replyIDs = []string{m.activePost.ID}
	m.allReplies = nil
	m.loading = true
	m.ctx.Loading = true
	m.tk.InvalidateCache()
	return m.loadPost(m.activePost, m.ctx.NextLoadGen(), 0)
}

func (m *Model) loadPost(p *post.Post, gen int64, delay time.Duration) tea.Cmd {
	c := m.ctx
	viewportWidth := m.viewport.Width()
	imageWidth := viewportWidth - 2
	systems := c.Systems

	return func() tea.Msg {
		if delay > 0 {
			time.Sleep(delay)
		}

		isCurrent := func() bool { return c.IsCurrentLoadGen(gen) }
		if !isCurrent() {
			return nil
		}

		if err := feed.LoadPost(context.Background(), systems, p); err != nil {
			c.Logger.Error("loading post failed", "id", p.ID, "error", err)
			if !isCurrent() {
				return nil
			}
			return loadFailedMsg{gen: gen, err: err}
		}

		rendered, ok := renderPost(c, p, viewportWidth, imageWidth, isCurrent)
		if !ok {
			return nil
		}

		return loadedMsg{gen: gen, post: p, rendered: rendered}
	}
}
