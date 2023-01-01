package posts

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/glamour"
	"github.com/charmbracelet/lipgloss"
	"github.com/mrusme/gobbs/aggregator"
	"github.com/mrusme/gobbs/models/post"
	"github.com/mrusme/gobbs/models/reply"
	"github.com/mrusme/gobbs/ui/ctx"
	"github.com/mrusme/gobbs/ui/helpers"
)

var (
	viewportStyle = lipgloss.NewStyle().
		Margin(0, 0, 0, 0).
		Padding(0, 0).
		BorderTop(false).
		BorderLeft(false).
		BorderRight(false).
		BorderBottom(false)
)

type KeyMap struct {
	Refresh key.Binding
	Select  key.Binding
	Close   key.Binding
}

var DefaultKeyMap = KeyMap{
	Refresh: key.NewBinding(
		key.WithKeys("r", "R"),
		key.WithHelp("r/R", "refresh"),
	),
	Select: key.NewBinding(
		key.WithKeys("enter"),
		key.WithHelp("enter", "select"),
	),
	Close: key.NewBinding(
		key.WithKeys("esc"),
		key.WithHelp("esc", "close"),
	),
}

type Model struct {
	keymap   KeyMap
	list     list.Model
	items    []list.Item
	viewport viewport.Model
	textarea textarea.Model
	ctx      *ctx.Ctx
	a        *aggregator.Aggregator

	glam *glamour.TermRenderer

	focused  string
	buffer   string
	replyIDs []string
}

func (m Model) Init() tea.Cmd {
	// TODO: Doesn't seem to be working
	// return m.refresh()
	return nil
}

func NewModel(c *ctx.Ctx) Model {
	m := Model{
		ctx:     c,
		keymap:  DefaultKeyMap,
		focused: "list",
		buffer:  "",
	}

	listDelegate := list.NewDefaultDelegate()
	listDelegate.Styles.NormalTitle = m.ctx.Theme.PostsList.Item.Focused
	listDelegate.Styles.DimmedTitle = m.ctx.Theme.PostsList.Item.Blurred
	listDelegate.Styles.SelectedTitle = m.ctx.Theme.PostsList.Item.Selected
	listDelegate.Styles.NormalDesc = m.ctx.Theme.PostsList.ItemDetail.Focused
	listDelegate.Styles.DimmedDesc = m.ctx.Theme.PostsList.ItemDetail.Blurred
	listDelegate.Styles.SelectedDesc = m.ctx.Theme.PostsList.ItemDetail.Selected

	m.list = list.New(m.items, listDelegate, 0, 0)
	m.list.SetShowTitle(false)
	m.list.SetShowStatusBar(false)

	m.textarea = textarea.New()
	m.textarea.Placeholder = "Type in your reply ..."
	m.textarea.Prompt = ""

	m.a, _ = aggregator.New(m.ctx)

	return m
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch {
		case key.Matches(msg, m.keymap.Refresh):
			if m.focused == "list" {
				m.ctx.Loading = true
				cmds = append(cmds, m.refresh())
			} else if m.focused == "post" {
				m.focused = "reply"
				return m, m.textarea.Focus()
			}

		case key.Matches(msg, m.keymap.Select):
			if m.focused == "list" {
				m.ctx.Loading = true
				i, ok := m.list.SelectedItem().(post.Post)
				if ok {
					cmds = append(cmds, m.loadItem(&i))
				}
			}

		case key.Matches(msg, m.keymap.Close):
			if m.focused == "post" {
				// Let's make sure we reset the texarea
				m.textarea.Reset()
				m.focused = "list"
				return m, nil
			} else if m.focused == "reply" {
				m.focused = "post"
				m.buffer = ""
				return m, nil
			}

		default:
			switch msg.String() {
			case "1", "2", "3", "4", "5", "6", "7", "8", "9", "0":
				if m.focused == "post" {
					m.buffer += msg.String()
					return m, nil
				}
			default:
				if m.focused != "reply" {
					m.buffer = ""
				}
			}
		}

	case tea.WindowSizeMsg:
		listWidth := m.ctx.Content[0] - 2
		listHeight := m.ctx.Content[1] - 1
		viewportWidth := m.ctx.Content[0] - 9
		viewportHeight := m.ctx.Content[1] - 10

		m.ctx.Theme.PostsList.List.Focused.Width(listWidth)
		m.ctx.Theme.PostsList.List.Blurred.Width(listWidth)
		m.ctx.Theme.PostsList.List.Focused.Height(listHeight)
		m.ctx.Theme.PostsList.List.Blurred.Height(listHeight)
		m.list.SetSize(
			listWidth-2,
			listHeight-2,
		)

		viewportStyle.Width(viewportWidth)
		viewportStyle.Height(viewportHeight)
		m.viewport = viewport.New(viewportWidth-4, viewportHeight-4)
		m.viewport.Width = viewportWidth - 4
		m.viewport.Height = viewportHeight + 1
		// cmds = append(cmds, viewport.Sync(m.viewport))

	case []list.Item:
		m.items = msg
		m.list.SetItems(m.items)
		m.ctx.Loading = false

	case *post.Post:
		m.viewport.SetContent(m.renderViewport(msg))
		m.ctx.Loading = false
		return m, nil

	}

	var cmd tea.Cmd

	if m.focused == "list" {
		m.list, cmd = m.list.Update(msg)
	} else if m.focused == "post" {
		m.viewport, cmd = m.viewport.Update(msg)
	} else if m.focused == "reply" {
		if !m.textarea.Focused() {
			cmds = append(cmds, m.textarea.Focus())
		}
		m.textarea, cmd = m.textarea.Update(msg)
	}
	cmds = append(cmds, cmd)

	return m, tea.Batch(cmds...)
}

func (m Model) View() string {
	var view strings.Builder = strings.Builder{}

	var l string = ""
	if m.focused == "list" {
		l = m.ctx.Theme.PostsList.List.Focused.Render(m.list.View())
	} else {
		l = m.ctx.Theme.PostsList.List.Blurred.Render(m.list.View())
	}
	view.WriteString(lipgloss.JoinHorizontal(
		lipgloss.Top,
		l,
	))

	if m.focused == "post" || m.focused == "reply" {
		var style lipgloss.Style
		if m.focused == "post" {
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
		if m.focused == "post" {
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
	}

	if m.focused == "reply" {
		title := "Reply"
		if m.buffer != "" && m.buffer != "0" {
			title += " to reply #" + m.buffer
		}
		titlebar := m.ctx.Theme.DialogBox.Titlebar.Focused.
			Align(lipgloss.Center).
			Width(m.viewport.Width - 2).
			Render(title)

		m.textarea.SetWidth(m.viewport.Width - 2)
		m.textarea.SetHeight(6)

		bottombar := m.ctx.Theme.DialogBox.Bottombar.
			Width(m.viewport.Width - 2).
			Render("ctrl+enter reply · esc close")

		replyWindow := lipgloss.JoinVertical(
			lipgloss.Center,
			titlebar,
			m.textarea.View(),
			bottombar,
		)

		tmp := helpers.PlaceOverlay(5, m.ctx.Screen[1]-21,
			m.ctx.Theme.DialogBox.Window.Focused.Render(replyWindow),
			view.String(), true)

		view = strings.Builder{}
		view.WriteString(tmp)
	}

	return view.String()
}

func (m *Model) refresh() tea.Cmd {
	return func() tea.Msg {
		var items []list.Item

		posts, errs := m.a.ListPosts()
		if len(errs) > 0 {
			fmt.Printf("%s", errs) // TODO: Implement error message
		}
		for _, post := range posts {
			items = append(items, post)
		}

		return items
	}
}

func (m *Model) loadItem(p *post.Post) tea.Cmd {
	return func() tea.Msg {
		m.a.LoadPost(p)
		return p
	}
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
	out += m.renderReplies(0, p.Author.Name, &p.Replies)

	m.focused = "post"
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

	for _, re := range *replies {
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
