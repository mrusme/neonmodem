package posts

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/mrusme/gobbs/aggregator"
	"github.com/mrusme/gobbs/models/post"
	"github.com/mrusme/gobbs/ui/cmd"
	"github.com/mrusme/gobbs/ui/ctx"
	"github.com/mrusme/gobbs/ui/windows/postshow"
)

var (
	VIEW_ID = "posts"
)

type KeyMap struct {
	Refresh key.Binding
	Select  key.Binding
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
}

type Model struct {
	ctx     *ctx.Ctx
	keymap  KeyMap
	focused bool

	list  list.Model
	items []list.Item

	a *aggregator.Aggregator

	viewcache           string
	viewcacheTextareaXY []int
}

func (m Model) Init() tea.Cmd {
	return nil
}

func NewModel(c *ctx.Ctx) Model {
	m := Model{
		ctx:     c,
		keymap:  DefaultKeyMap,
		focused: false,

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
			i, ok := m.list.SelectedItem().(post.Post)
			if ok {
				m.viewcache = m.buildView(false)
				cmd := cmd.New(cmd.WinOpen, postshow.WIN_ID, cmd.Arg{
					Name:  "post",
					Value: &i,
				})
				cmds = append(cmds, cmd.Tea())
			}
		}

	case tea.WindowSizeMsg:
		listWidth := m.ctx.Content[0] - 2
		listHeight := m.ctx.Content[1] - 1

		m.ctx.Theme.PostsList.List.Focused.Width(listWidth)
		m.ctx.Theme.PostsList.List.Blurred.Width(listWidth)
		m.ctx.Theme.PostsList.List.Focused.Height(listHeight)
		m.ctx.Theme.PostsList.List.Blurred.Height(listHeight)
		m.list.SetSize(
			listWidth-2+60,
			listHeight-2,
		)
		msg.Width = listWidth + 60
		msg.Height = listHeight

	case cmd.Command:
		switch msg.Call {
		case cmd.ViewFocus:
			if msg.Target == VIEW_ID ||
				msg.Target == "*" {
				m.Focus()
			}
			return m, nil
		case cmd.ViewBlur:
			if msg.Target == VIEW_ID ||
				msg.Target == "*" {
				m.Blur()
			}
			return m, nil
		case cmd.ViewRefreshData:
			if msg.Target == VIEW_ID ||
				msg.Target == "*" {
				m.ctx.Loading = true
				cmds = append(cmds, m.refresh())
			}
		case cmd.ViewFreshData:
			if msg.Target == VIEW_ID ||
				msg.Target == "*" {
				m.items = msg.GetArg("items").([]list.Item)
				m.list.SetItems(m.items)
				m.ctx.Loading = false
				return m, nil
			}
		}

	}

	var lcmd tea.Cmd
	m.list, lcmd = m.list.Update(msg)
	cmds = append(cmds, lcmd)

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

		c := cmd.New(
			cmd.ViewFreshData,
			VIEW_ID,
			cmd.Arg{Name: "items", Value: items},
		)

		return *c
	}
}

func (m *Model) Focus() {
	m.focused = true
	m.viewcache = m.buildView(false)
}

func (m *Model) Blur() {
	m.focused = false
	m.viewcache = m.buildView(false)
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

	m.ctx.Logger.Debugln("Posts.View()")
	var l string = ""
	if m.focused {
		l = m.ctx.Theme.PostsList.List.Focused.Render(m.list.View())
	} else {
		l = m.ctx.Theme.PostsList.List.Blurred.Render(m.list.View())
	}
	view.WriteString(lipgloss.JoinHorizontal(
		lipgloss.Top,
		l,
	))

	m.viewcache = view.String()
	return m.viewcache
}
