package posts

import (
	"encoding/json"
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
	ViewBorderColor = lipgloss.AdaptiveColor{
		Light: "#b0c4de",
		Dark:  "#b0c4de",
	}

	DialogBorderColor = lipgloss.AdaptiveColor{
		Light: "#b0c4de",
		Dark:  "#b0c4de",
	}
)

var (
	listStyle = lipgloss.NewStyle().
			Margin(0, 0, 0, 0).
			Padding(1, 1).
			Border(lipgloss.DoubleBorder()).
			BorderForeground(ViewBorderColor).
			BorderTop(true).
			BorderLeft(true).
			BorderRight(true).
			BorderBottom(true)

	viewportStyle = lipgloss.NewStyle().
			Margin(0, 0, 0, 0).
			Padding(0, 0).
			BorderTop(false).
			BorderLeft(false).
			BorderRight(false).
			BorderBottom(false)

	dialogBoxStyle = lipgloss.NewStyle().
			Border(lipgloss.ThickBorder()).
			BorderForeground(DialogBorderColor).
			Padding(0, 0).
			Margin(0, 0, 0, 0).
			BorderTop(false).
			BorderLeft(true).
			BorderRight(true).
			BorderBottom(true)

	dialogBoxTitlebarStyle = lipgloss.NewStyle().
				Align(lipgloss.Center).
				Background(lipgloss.Color("#87cefa")).
				Foreground(lipgloss.Color("#000000")).
				Padding(0, 1).
				Margin(0, 0, 1, 0)

	dialogBoxBottombarStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#999999")).
				Padding(0, 1).
				Margin(1, 0, 0, 0)

	postAuthorStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#F25D94")).
			Padding(0, 1)

	postSubjectStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#FFFFFF")).
				Background(lipgloss.Color("#F25D94")).
				Padding(0, 1)

	replyAuthorStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#000000")).
				Background(lipgloss.Color("#874BFD")).
				Padding(0, 1)
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

	viewportOpen bool
}

func (m Model) Init() tea.Cmd {
	return nil
}

func NewModel(c *ctx.Ctx) Model {
	m := Model{
		keymap:       DefaultKeyMap,
		viewportOpen: false,
	}

	m.list = list.New(m.items, list.NewDefaultDelegate(), 0, 0)
	m.list.Title = "Posts"
	m.ctx = c
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
			if m.viewportOpen {
				m.viewportOpen = false
				return m, nil
			}
		}

	case tea.WindowSizeMsg:
		listWidth := m.ctx.Content[0] - 2
		listHeight := m.ctx.Content[1] - 1
		viewportWidth := m.ctx.Content[0] - 9
		viewportHeight := m.ctx.Content[1] - 10

		listStyle.Width(listWidth)
		listStyle.Height(listHeight)
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

	if m.viewportOpen == false {
		// listStyle.BorderForeground(lipgloss.Color("#FFFFFF"))
		// viewportStyle.BorderForeground(lipgloss.Color("#874BFD"))
		m.list, cmd = m.list.Update(msg)
		cmds = append(cmds, cmd)
	} else if m.viewportOpen == true {
		// listStyle.BorderForeground(lipgloss.Color("#874BFD"))
		// viewportStyle.BorderForeground(lipgloss.Color("#FFFFFF"))
		m.viewport, cmd = m.viewport.Update(msg)
		cmds = append(cmds, cmd)
	}

	return m, tea.Batch(cmds...)
}

func (m Model) View() string {
	var view strings.Builder = strings.Builder{}

	view.WriteString(lipgloss.JoinHorizontal(
		lipgloss.Top,
		listStyle.Render(m.list.View()),
	))

	if m.viewportOpen {
		titlebar := dialogBoxTitlebarStyle.
			Width(m.viewport.Width + 4).
			Render("Post")

		bottombar := dialogBoxBottombarStyle.
			Width(m.viewport.Width + 4).
			Render("r reply · esc close")

		ui := lipgloss.JoinVertical(
			lipgloss.Center,
			titlebar,
			viewportStyle.Render(m.viewport.View()),
			bottombar,
		)

		return helpers.PlaceOverlay(3, 2, dialogBoxStyle.Render(ui), view.String())
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
		postAuthorStyle.Render(
			fmt.Sprintf("%s %s:", p.Author.Name, adj),
		),
		postSubjectStyle.Render(p.Subject),
		body,
	)

	bla, _ := json.Marshal(p.Replies)
	m.ctx.Logger.Debugf("%s", bla)
	out += m.renderReplies(0, p.Author.Name, &p.Replies)

	m.viewportOpen = true
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
			replyAuthorStyle.Render(
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
