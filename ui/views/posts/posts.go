package posts

import (
	"fmt"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/glamour"
	"github.com/charmbracelet/lipgloss"
	"github.com/mrusme/gobbs/aggregator"
	"github.com/mrusme/gobbs/models/post"
	"github.com/mrusme/gobbs/ui/ctx"
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
	Esc     key.Binding
	Quit    key.Binding
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

	focused             string
	buffer              string
	replyIDs            []string
	viewcache           string
	viewcacheTextareaXY []int
}

func (m Model) Init() tea.Cmd {
	// TODO: Doesn't seem to be working
	// return m.refresh()
	return nil
}

func NewModel(c *ctx.Ctx) Model {
	m := Model{
		ctx:                 c,
		keymap:              DefaultKeyMap,
		focused:             "list",
		buffer:              "",
		viewcache:           "",
		viewcacheTextareaXY: []int{0, 0, 0, 0},
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
			}

		case key.Matches(msg, m.keymap.Select):
			if m.focused == "list" {
				i, ok := m.list.SelectedItem().(post.Post)
				if ok {
					m.ctx.Loading = true
					cmds = append(cmds, m.loadItem(&i))
				}
			} else if m.focused == "post" {
				m.focused = "reply"

				m.ctx.Logger.Debugln("caching view")
				m.ctx.Logger.Debugf("buffer: %s", m.buffer)
				m.viewcache = m.buildView(false)

				return m, m.textarea.Focus()
			}

		case key.Matches(msg, m.keymap.Esc), key.Matches(msg, m.keymap.Quit):
			if m.focused == "list" {
				return m, tea.Quit
			} else if m.focused == "post" {
				// Let's make sure we reset the texarea
				m.textarea.Reset()
				m.focused = "list"
				return m, nil
			} else if m.focused == "reply" && key.Matches(msg, m.keymap.Esc) {
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
		return m, nil

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
