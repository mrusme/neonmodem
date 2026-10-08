package postcreate

import (
	"context"
	"time"

	"charm.land/bubbles/v2/textarea"
	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"github.com/mrusme/neonmodem/internal/models/post"
	"github.com/mrusme/neonmodem/internal/models/reply"
	"github.com/mrusme/neonmodem/internal/ui/ctx"
	"github.com/mrusme/neonmodem/internal/ui/msgs"
	"github.com/mrusme/neonmodem/internal/ui/toolkit"
	"github.com/mrusme/neonmodem/internal/ui/windows"
)

const (
	WIN_ID = "postcreate"

	refreshDelay = 3 * time.Second

	composePadding = 1
	subjectRows    = 2
)

type submittedMsg struct {
	err error
}

type Model struct {
	ctx *ctx.Ctx
	tk  *toolkit.ToolKit

	textinput    textinput.Model
	textarea     textarea.Model
	inputFocused int

	action msgs.ComposeAction
	post   post.Post
	parent *reply.Reply
	index  int

	submitting bool
}

func NewModel(c *ctx.Ctx) *Model {
	m := &Model{
		ctx: c,
		tk:  toolkit.New(WIN_ID, c),
	}

	m.textinput = textinput.New()
	m.textinput.Placeholder = "Subject goes here"
	m.textinput.Prompt = ""
	m.textinput.SetStyles(textinput.DefaultStyles(c.DarkBackground))

	m.textarea = textarea.New()
	m.textarea.Placeholder = "Type in your post ..."
	m.textarea.Prompt = ""
	m.textarea.ShowLineNumbers = false
	m.textarea.SetStyles(textarea.DefaultStyles(c.DarkBackground))

	m.tk.KeymapAdd("tab", "switch field", "tab")
	m.tk.KeymapAdd("submit", "submit", "ctrl+s")

	m.tk.SetViewFunc(m.buildView)
	m.tk.SetMsgHandling(toolkit.MsgHandling{
		OnKeymapKey: []toolkit.MsgHandlingKeymapKey{
			{ID: "tab", Handler: m.handleTab},
			{ID: "submit", Handler: m.handleSubmit},
		},
		OnViewResize: m.handleViewResize,
	})

	return m
}

func (m *Model) Update(msg tea.Msg) (windows.Window, tea.Cmd) {
	switch msg := msg.(type) {
	case msgs.Compose:
		return m, m.open(msg)

	case submittedMsg:
		return m, m.submitted(msg.err)

	case msgs.ThemeChanged:
		m.textinput.SetStyles(textinput.DefaultStyles(m.ctx.DarkBackground))
		m.textarea.SetStyles(textarea.DefaultStyles(m.ctx.DarkBackground))
	}

	if handled, cmds := m.tk.HandleMsg(msg); handled {
		return m, tea.Batch(cmds...)
	}

	if m.submitting {
		if _, isKey := msg.(tea.KeyPressMsg); isKey {
			return m, nil
		}
	}

	var cmd tea.Cmd
	switch m.inputFocused {
	case 0:
		m.textinput, cmd = m.textinput.Update(msg)
	default:
		m.textarea, cmd = m.textarea.Update(msg)
	}

	return m, cmd
}

func (m *Model) resize() {
	width := max(m.tk.InnerWidth()-composePadding*2, 1)
	height := m.tk.InnerHeight()
	if m.action == msgs.ComposePost {
		height -= subjectRows
	}
	m.textinput.SetWidth(width)
	m.textarea.SetWidth(width)
	m.textarea.SetHeight(max(height, 1))
}

func (m *Model) open(c msgs.Compose) tea.Cmd {
	m.action = c.Action
	m.post = c.Post
	m.parent = c.Parent
	m.index = c.Index
	m.submitting = false
	m.textinput.Reset()
	m.textarea.Reset()
	m.resize()

	if m.action == msgs.ComposePost {
		m.inputFocused = 0
		m.textarea.Blur()
		return m.textinput.Focus()
	}

	m.inputFocused = 1
	m.textinput.Blur()
	return m.textarea.Focus()
}

func (m *Model) submitted(err error) tea.Cmd {
	m.submitting = false
	m.ctx.StopLoading(ctx.LoadSubmit)

	if err != nil {
		m.ctx.Logger.Error("submitting failed", "action", m.action, "error", err)
		return msgs.Send(msgs.Error(err))
	}

	notice := "Post created"
	if m.action == msgs.ComposeReply {
		notice = "Reply posted"
	}

	m.textinput.Reset()
	m.textarea.Reset()

	return tea.Batch(
		msgs.Send(msgs.CloseWindow{ID: WIN_ID}),
		msgs.Send(msgs.Notice{Text: notice}),
		msgs.Send(msgs.ReloadPost{Delay: refreshDelay}),
	)
}

func (m *Model) createPost(p post.Post) tea.Cmd {
	f := m.ctx.Feed
	return func() tea.Msg {
		return submittedMsg{err: f.CreatePost(context.Background(), &p)}
	}
}

func (m *Model) createReply(r reply.Reply) tea.Cmd {
	f := m.ctx.Feed
	return func() tea.Msg {
		return submittedMsg{err: f.CreateReply(context.Background(), &r)}
	}
}
