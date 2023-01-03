package postshow

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/glamour"
	"github.com/charmbracelet/lipgloss"
	"github.com/mrusme/gobbs/aggregator"
	"github.com/mrusme/gobbs/models/post"
	"github.com/mrusme/gobbs/models/reply"
	"github.com/mrusme/gobbs/ui/cmd"
	"github.com/mrusme/gobbs/ui/ctx"
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

type KeyMap struct {
	Reply key.Binding
}

var DefaultKeyMap = KeyMap{
	Reply: key.NewBinding(
		key.WithKeys("r"),
		key.WithHelp("r", "reply"),
	),
}

type Model struct {
	ctx      *ctx.Ctx
	keymap   KeyMap
	wh       [2]int
	focused  bool
	viewport viewport.Model

	a    *aggregator.Aggregator
	glam *glamour.TermRenderer

	buffer   string
	replyIDs []string

	activePost  *post.Post
	allReplies  []*reply.Reply
	activeReply *reply.Reply

	viewcache string
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
		ctx:    c,
		keymap: DefaultKeyMap,
		wh:     [2]int{0, 0},

		buffer:   "",
		replyIDs: []string{},

		viewcache: "",
	}

	m.a, _ = aggregator.New(m.ctx)

	return m
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch {

		case key.Matches(msg, m.keymap.Reply):
			var replyToIdx int = 0
			var err error

			m.viewcache = m.buildView(false)

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

			return m, tea.Batch(cmds...)

		default:
			switch msg.String() {
			case "1", "2", "3", "4", "5", "6", "7", "8", "9", "0":
				m.buffer += msg.String()
				return m, nil
			default:
				m.buffer = ""
			}
		}

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
		// cmds = append(cmds, viewport.Sync(m.viewport))

	case cmd.Command:
		m.ctx.Logger.Debugf("got command: %v\n", msg)
		switch msg.Call {
		case cmd.WinOpen, cmd.WinRefreshData:
			if msg.Target == WIN_ID {
				m.ctx.Logger.Debug("got own WinOpen command")
				m.activePost = msg.GetArg("post").(*post.Post)
				m.viewport.SetContent(m.renderViewport(m.activePost))
				m.ctx.Logger.Debugf("loading post: %v", m.activePost.ID)
				m.ctx.Loading = true
				return m, m.loadPost(m.activePost)
			}
			return m, nil
		case cmd.WinFocus:
			if msg.Target == WIN_ID ||
				msg.Target == "*" {
				m.focused = true
			}
			return m, nil
		case cmd.WinBlur:
			if msg.Target == WIN_ID ||
				msg.Target == "*" {
				m.focused = false
			}
			return m, nil
		case cmd.WinFreshData:
			if msg.Target == WIN_ID ||
				msg.Target == "*" {
				m.ctx.Logger.Debug("got *post.Post")
				m.activePost = msg.GetArg("post").(*post.Post)
				m.viewport.SetContent(m.renderViewport(m.activePost))
				m.ctx.Loading = false
				return m, nil
			}
		default:
			m.ctx.Logger.Debugf("received unhandled command: %v\n", msg)
		}

	default:
		m.ctx.Logger.Debugf("received unhandled msg: %v\n", msg)
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
	return m.buildView(true)
}

func (m Model) buildView(cached bool) string {
	var view strings.Builder = strings.Builder{}

	if cached && m.focused == false && m.viewcache != "" {
		m.ctx.Logger.Debugln("Cached View()")

		return m.viewcache
	}

	var style lipgloss.Style
	if m.focused {
		style = m.ctx.Theme.DialogBox.Titlebar.Focused
	} else {
		style = m.ctx.Theme.DialogBox.Titlebar.Blurred
	}
	titlebar := style.Align(lipgloss.Center).
		Width(m.wh[0]).
		Render("Post")

	bottombar := m.ctx.Theme.DialogBox.Bottombar.
		Width(m.wh[0]).
		Render("[#]r reply · esc close")

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
