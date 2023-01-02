package postcreate

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/glamour"
	"github.com/charmbracelet/lipgloss"
	"github.com/mrusme/gobbs/aggregator"
	"github.com/mrusme/gobbs/models/post"
	"github.com/mrusme/gobbs/models/reply"
	"github.com/mrusme/gobbs/ui/cmd"
	"github.com/mrusme/gobbs/ui/ctx"
	"github.com/mrusme/gobbs/ui/helpers"
)

var (
	WIN_ID = "postcreate"
)

type KeyMap struct {
	Refresh key.Binding
	Select  key.Binding
	Esc     key.Binding
	Quit    key.Binding
	Reply   key.Binding
}

var DefaultKeyMap = KeyMap{
	Refresh: key.NewBinding(
		key.WithKeys("ctrl+r"),
		key.WithHelp("ctrl+r", "refresh"),
	),
	Select: key.NewBinding(
		key.WithKeys("r", "enter"),
		key.WithHelp("r/enter", "read"),
	),
	Esc: key.NewBinding(
		key.WithKeys("esc"),
		key.WithHelp("esc", "close"),
	),
	Quit: key.NewBinding(
		key.WithKeys("q"),
	),
	Reply: key.NewBinding(
		key.WithKeys("ctrl+s"),
		key.WithHelp("ctrl+s", "reply"),
	),
}

type Model struct {
	ctx      *ctx.Ctx
	keymap   KeyMap
	textarea textarea.Model
	focused  bool

	a    *aggregator.Aggregator
	glam *glamour.TermRenderer

	activeReply *reply.Reply
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
			replyToIdx, _ := strconv.Atoi(m.buffer)

			m.ctx.Logger.Debugf("replyToIdx: %d", replyToIdx)

			var irtID string = ""
			var irtIRT string = ""
			var irtSysIDX int = 0

			if replyToIdx == 0 {
				irtID = m.activePost.ID
				irtSysIDX = m.activePost.SysIDX
			} else {
				irt := m.allReplies[(replyToIdx - 1)]
				irtID = strconv.Itoa(replyToIdx + 1)
				irtIRT = irt.InReplyTo
				irtSysIDX = irt.SysIDX
			}

			r := reply.Reply{
				ID:        irtID,
				InReplyTo: irtIRT,
				Body:      m.textarea.Value(),
				SysIDX:    irtSysIDX,
			}
			err := m.a.CreateReply(&r)
			if err != nil {
				m.ctx.Logger.Error(err)
			}

			m.textarea.Reset()
			m.buffer = ""
			m.WMClose("reply")
			return m, nil

		}

	case tea.WindowSizeMsg:
		m.ctx.Logger.Debug("received WindowSizeMsg")
		viewportWidth := m.ctx.Content[0] - 9
		viewportHeight := m.ctx.Content[1] - 10

		viewportStyle.Width(viewportWidth)
		viewportStyle.Height(viewportHeight)
		m.viewport = viewport.New(viewportWidth-4, viewportHeight-4)
		m.viewport.Width = viewportWidth - 4
		m.viewport.Height = viewportHeight + 1
		// cmds = append(cmds, viewport.Sync(m.viewport))

	case *post.Post:
		m.ctx.Logger.Debug("got *post.Post")
		m.activePost = msg
		m.viewport.SetContent(m.renderViewport(m.activePost))
		m.ctx.Loading = false
		return m, nil

	case cmd.Command:
		m.ctx.Logger.Debugf("got command: %v\n", msg)
		switch msg.Call {
		case cmd.WinRefreshData:
			if msg.Target == "post" {
				m.activePost = msg.GetArg("post").(*post.Post)
				m.ctx.Logger.Debugf("loading post: %v", m.activePost.ID)
				m.ctx.Loading = true
				return m, m.loadPost(m.activePost)
			}
			return m, nil
		case cmd.WinFocus:
			if msg.Target == "post" {
				m.focused = true
			}
			return m, nil
		case cmd.WinBlur:
			if msg.Target == "post" {
				m.focused = false
			}
			return m, nil
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
		}
		return p
	}
}

func (m Model) View() string {
	return m.buildView(true)
}

func (m Model) buildView(cached bool) string {
	var view strings.Builder = strings.Builder{}

	var l string = ""
	view.WriteString(lipgloss.JoinHorizontal(
		lipgloss.Top,
		l,
	))

	var style lipgloss.Style
	if m.focused {
		style = m.ctx.Theme.DialogBox.Titlebar.Focused
	} else {
		style = m.ctx.Theme.DialogBox.Titlebar.Blurred
	}
	titlebar := style.Align(lipgloss.Center).
		Width(m.viewport.Width + 4).
		Render("Post")

	bottombar := m.ctx.Theme.DialogBox.Bottombar.
		Width(m.viewport.Width + 4).
		Render("[#]r reply · esc close")

	ui := lipgloss.JoinVertical(
		lipgloss.Center,
		titlebar,
		viewportStyle.Render(m.viewport.View()),
		bottombar,
	)

	var tmp string
	if m.focused {
		tmp = helpers.PlaceOverlay(3, 2,
			m.ctx.Theme.DialogBox.Window.Focused.Render(ui),
			view.String(), true)
	} else {
		tmp = helpers.PlaceOverlay(3, 2,
			m.ctx.Theme.DialogBox.Window.Blurred.Render(ui),
			view.String(), true)
	}

	view = strings.Builder{}
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

		out += fmt.Sprintf(
			"\n\n %s %s%s%s\n%s",
			m.ctx.Theme.Reply.Author.Render(
				author,
			),
			lipgloss.NewStyle().
				Foreground(m.ctx.Theme.Reply.Author.GetBackground()).
				Render(fmt.Sprintf("writes in reply to %s:", inReplyTo)),
			strings.Repeat(" ", (m.viewport.Width-len(author)-len(inReplyTo)-28)),
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
