package posts

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/list"
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
	ctx      *ctx.Ctx
	a        *aggregator.Aggregator

	glam *glamour.TermRenderer

	focused string
}

func (m Model) Init() tea.Cmd {
	return nil
}

func NewModel(c *ctx.Ctx) Model {
	m := Model{
		ctx:     c,
		keymap:  DefaultKeyMap,
		focused: "list",
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
	m.a, _ = aggregator.New(m.ctx)

	return m
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch {
		case key.Matches(msg, m.keymap.Refresh):
			m.ctx.Loading = true
			cmds = append(cmds, m.refresh())

		case key.Matches(msg, m.keymap.Select):
			m.ctx.Loading = true
			i, ok := m.list.SelectedItem().(post.Post)
			if ok {
				cmds = append(cmds, m.loadItem(&i))
			}

		case key.Matches(msg, m.keymap.Close):
			if m.focused == "post" {
				m.focused = "list"
				return m, nil
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
		m.viewport.Height = viewportHeight - 4
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

	if m.focused == "post" {
		titlebar := m.ctx.Theme.DialogBox.Titlebar.
			Align(lipgloss.Center).
			Width(m.viewport.Width + 4).
			Render("Post")

		bottombar := m.ctx.Theme.DialogBox.Bottombar.
			Width(m.viewport.Width + 4).
			Render("r reply · esc close")

		ui := lipgloss.JoinVertical(
			lipgloss.Center,
			titlebar,
			viewportStyle.Render(m.viewport.View()),
			bottombar,
		)

		return helpers.PlaceOverlay(3, 2,
			m.ctx.Theme.DialogBox.Window.Render(ui),
			view.String(), true)
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
		" %s\n %s\n%s",
		m.ctx.Theme.Post.Author.Render(
			fmt.Sprintf("%s %s:", p.Author.Name, adj),
		),
		m.ctx.Theme.Post.Subject.Render(p.Subject),
		body,
	)

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
			body = "\n  DELETED"
			author = "DELETED"
		} else {
			body, err = m.glam.Render(re.Body)
			if err != nil {
				m.ctx.Logger.Error(err)
				body = re.Body
			}

			author = re.Author.Name
		}
		out += fmt.Sprintf(
			"\n\n %s %s\n%s",
			m.ctx.Theme.Reply.Author.Render(
				author,
			),
			lipgloss.NewStyle().Foreground(lipgloss.Color("#874BFD")).Render(
				fmt.Sprintf("writes in reply to %s:", inReplyTo),
			),
			body,
		)

		out += m.renderReplies(level+1, re.Author.Name, &re.Replies)
	}

	return out
}
