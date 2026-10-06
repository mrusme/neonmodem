package posts

import (
	"context"
	"errors"
	"fmt"
	"sort"

	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/list"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/mrusme/neonmodem/internal/feed"
	"github.com/mrusme/neonmodem/internal/models/post"
	"github.com/mrusme/neonmodem/internal/system"
	"github.com/mrusme/neonmodem/internal/ui/ctx"
	"github.com/mrusme/neonmodem/internal/ui/msgs"
	"github.com/mrusme/neonmodem/internal/ui/views"
)

type KeyMap struct {
	Refresh key.Binding
	NewPost key.Binding
	Select  key.Binding
	Quit    key.Binding
}

var DefaultKeyMap = KeyMap{
	Refresh: key.NewBinding(
		key.WithKeys("ctrl+r"),
		key.WithHelp("ctrl+r", "refresh"),
	),
	NewPost: key.NewBinding(
		key.WithKeys("n"),
		key.WithHelp("n", "new post"),
	),
	Select: key.NewBinding(
		key.WithKeys("r", "enter"),
		key.WithHelp("r/enter", "read"),
	),
	Quit: key.NewBinding(
		key.WithKeys("esc"),
		key.WithHelp("esc", "quit"),
	),
}

type Item struct {
	post.Post
	SystemTitle string
}

func (i Item) Description() string {
	desc := i.Post.Description()
	if score := i.Score.String(); score != "" {
		desc += " · " + score
	}
	switch i.ReplyCount {
	case 0:
	case 1:
		desc += " · 1 reply"
	default:
		desc += fmt.Sprintf(" · %d replies", i.ReplyCount)
	}
	if i.SystemTitle != "" {
		desc += " · " + i.SystemTitle
	}
	return desc
}

type systemFeed struct {
	order system.Order
	posts []post.Post
	err   error
}

type Model struct {
	ctx     *ctx.Ctx
	keymap  KeyMap
	focused bool

	list list.Model

	gen     int64
	pending int
	total   int
	feeds   map[int]systemFeed
	order   system.Order
	status  string

	width  int
	height int
}

func NewModel(c *ctx.Ctx) Model {
	m := Model{
		ctx:    c,
		keymap: DefaultKeyMap,
		feeds:  map[int]systemFeed{},
		order:  c.GetOrder(),
	}

	m.list = list.New(nil, m.delegate(), 0, 0)
	m.list.SetShowTitle(false)
	m.list.SetShowStatusBar(false)
	m.list.DisableQuitKeybindings()

	return m
}

func (m Model) delegate() list.DefaultDelegate {
	t := m.ctx.Theme
	d := list.NewDefaultDelegate()
	d.Styles.NormalTitle = t.PostsList.Item.Focused
	d.Styles.DimmedTitle = t.PostsList.Item.Blurred
	d.Styles.SelectedTitle = t.PostsList.Item.Selected
	d.Styles.NormalDesc = t.PostsList.ItemDetail.Focused
	d.Styles.DimmedDesc = t.PostsList.ItemDetail.Blurred
	d.Styles.SelectedDesc = t.PostsList.ItemDetail.Selected
	return d
}

func (m Model) Update(msg tea.Msg) (views.View, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		switch {
		case key.Matches(msg, m.keymap.Quit):
			if m.list.FilterState() == list.Filtering {
				break
			}
			return m, tea.Quit

		case key.Matches(msg, m.keymap.Refresh):
			if m.list.FilterState() == list.Filtering {
				break
			}
			return m, m.refresh()

		case key.Matches(msg, m.keymap.Select):
			if m.list.FilterState() == list.Filtering {
				break
			}
			if item, ok := m.list.SelectedItem().(Item); ok {
				return m, msgs.Send(msgs.OpenPost{Post: item.Post})
			}

		case key.Matches(msg, m.keymap.NewPost):
			if m.list.FilterState() == list.Filtering {
				break
			}
			return m, m.compose()
		}

	case tea.WindowSizeMsg:
		m.width = m.ctx.Content[0] - 2
		m.height = m.ctx.Content[1] - 1
		m.list.SetSize(m.width-2, m.height-2)

	case msgs.FocusView:
		m.focused = true
		return m, nil

	case msgs.BlurView:
		m.focused = false
		return m, nil

	case msgs.ThemeChanged:
		m.list.SetDelegate(m.delegate())
		return m, nil

	case msgs.RefreshFeed:
		return m, m.refresh()

	case msgs.OrderChanged:
		return m, m.reorder()

	case msgs.FeedResult:
		return m, m.absorb(msg)
	}

	var lcmd tea.Cmd
	m.list, lcmd = m.list.Update(msg)
	cmds = append(cmds, lcmd)

	return m, tea.Batch(cmds...)
}

func (m *Model) compose() tea.Cmd {
	item, ok := m.list.SelectedItem().(Item)
	if !ok {
		if len(m.ctx.Systems) == 0 {
			return msgs.Send(msgs.Error(errors.New(
				"No systems are connected. Run `neonmodem connect` first.")))
		}
		return msgs.Send(msgs.Error(errors.New(
			"Select a post first; a new post goes to the forum of the selected post.")))
	}

	sys := m.ctx.Systems[item.SysIDX]
	if !sys.Capabilities().Has(system.CapCreatePost) {
		return msgs.Send(msgs.Error(fmt.Errorf(
			"%s is connected without an account, so posting isn't available. "+
				"Run `neonmodem connect --type %s` again with credentials to post.",
			sys.Title(), sys.Kind())))
	}

	return msgs.Send(msgs.Compose{Action: msgs.ComposePost, Post: item.Post})
}

func (m *Model) selected() []int {
	return feed.Selected(m.ctx.Systems, m.ctx.GetCurrentSystem())
}

func (m *Model) refresh() tea.Cmd {
	m.feeds = map[int]systemFeed{}
	m.list.SetItems(nil)
	m.list.ResetSelected()
	return tea.Batch(m.load(m.selected()), m.sendStatus())
}

func (m *Model) reorder() tea.Cmd {
	want := m.ctx.GetOrder()
	forumID := m.ctx.GetCurrentForum().ID

	var reload []int
	for _, idx := range m.selected() {
		state, ok := m.feeds[idx]
		listed := feed.Choose(m.ctx.Systems[idx].Orders(forumID), want)
		if ok && state.err == nil && state.order == listed {
			continue
		}
		delete(m.feeds, idx)
		reload = append(reload, idx)
	}

	m.list.ResetSelected()
	cmd := m.load(reload)
	m.rebuild()
	return tea.Batch(cmd, m.sendStatus())
}

func (m *Model) load(indexes []int) tea.Cmd {
	m.gen = m.ctx.NextLoadGen()
	m.order = m.ctx.GetOrder()
	m.pending = len(indexes)
	m.total = len(m.selected())
	m.updateProgress()

	if len(indexes) == 0 {
		return nil
	}

	systems := m.ctx.Systems
	forumID := m.ctx.GetCurrentForum().ID
	want := m.order
	gen := m.gen

	cmds := make([]tea.Cmd, 0, len(indexes))
	for _, idx := range indexes {
		sys := systems[idx]
		cmds = append(cmds, func() tea.Msg {
			res, err := feed.List(context.Background(), idx, sys, forumID, want)
			return msgs.FeedResult{Gen: gen, System: idx, Order: res.Order, Posts: res.Posts, Err: err}
		})
	}

	return tea.Batch(cmds...)
}

func (m *Model) absorb(res msgs.FeedResult) tea.Cmd {
	if res.Gen != m.gen {
		return nil
	}

	m.pending--
	if res.Err != nil {
		m.feeds[res.System] = systemFeed{err: res.Err}
		m.ctx.Logger.Error("listing posts failed", "system", res.System, "error", res.Err)
	} else {
		m.feeds[res.System] = systemFeed{order: res.Order, posts: res.Posts}
	}
	m.rebuild()

	var cmds []tea.Cmd
	if res.Err != nil && res.System >= 0 && res.System < len(m.ctx.Systems) {
		cmds = append(cmds, msgs.Send(msgs.Notice{
			Text:    fmt.Sprintf("%s: %v", m.ctx.Systems[res.System].Title(), res.Err),
			IsError: true,
		}))
	}
	cmds = append(cmds, m.sendStatus())

	return tea.Batch(cmds...)
}

func (m *Model) rebuild() {
	results := make([]feed.Result, 0, len(m.feeds))
	for idx, f := range m.feeds {
		if f.err == nil {
			results = append(results, feed.Result{System: idx, Order: f.order, Posts: f.posts})
		}
	}

	merged := feed.Merge(results, m.order)
	items := make([]list.Item, 0, len(merged))
	for _, p := range merged {
		title := ""
		if len(m.ctx.Systems) > 1 && p.SysIDX >= 0 && p.SysIDX < len(m.ctx.Systems) {
			title = m.ctx.Systems[p.SysIDX].Title()
		}
		items = append(items, Item{Post: p, SystemTitle: title})
	}
	m.list.SetItems(items)
	m.updateProgress()
}

func (m *Model) statusText() string {
	var uses []feed.Use
	for _, idx := range m.selected() {
		f, ok := m.feeds[idx]
		if !ok || f.err != nil {
			continue
		}
		uses = append(uses, feed.Use{Name: m.ctx.Systems[idx].Title(), Order: f.order})
	}
	return feed.Status(m.order, uses)
}

func (m *Model) sendStatus() tea.Cmd {
	text := m.statusText()
	if text == m.status {
		return nil
	}
	m.status = text
	return msgs.Send(msgs.FeedStatus{Text: text})
}

func (m *Model) updateProgress() {
	if m.pending > 0 {
		m.ctx.Loading = true
		if m.total > 1 {
			m.ctx.Progress = fmt.Sprintf("%d/%d", m.total-m.pending, m.total)
		} else {
			m.ctx.Progress = ""
		}
		return
	}
	m.ctx.Loading = false
	m.ctx.Progress = ""
}

func (m Model) View() string {
	t := m.ctx.Theme

	frame := t.PostsList.List.Blurred
	if m.focused {
		frame = t.PostsList.List.Focused
	}
	frame = frame.Width(m.width).Height(m.height)

	if len(m.list.Items()) == 0 {
		return frame.Render(lipgloss.Place(
			m.width-2, m.height-2, lipgloss.Center, lipgloss.Center, m.placeholder()))
	}

	return frame.Render(m.list.View())
}

func (m Model) placeholder() string {
	t := m.ctx.Theme

	switch {
	case len(m.ctx.Systems) == 0:
		return t.Muted.Render("No systems connected yet.\n\nRun `neonmodem connect --type <system>` and start again.")
	case m.pending > 0:
		if m.total > 1 {
			return t.Muted.Render(fmt.Sprintf(
				"Loading posts from %d systems (%d done)", m.total, m.total-m.pending))
		}
		return t.Muted.Render("Loading posts")
	case m.failed() > 0:
		keys := make([]int, 0, len(m.feeds))
		for idx, f := range m.feeds {
			if f.err != nil {
				keys = append(keys, idx)
			}
		}
		sort.Ints(keys)
		out := t.Alert.Render("No posts could be loaded.") + "\n"
		for _, idx := range keys {
			name := fmt.Sprintf("system %d", idx)
			if idx >= 0 && idx < len(m.ctx.Systems) {
				name = m.ctx.Systems[idx].Title()
			}
			out += "\n" + t.Muted.Render(name+": "+m.feeds[idx].err.Error())
		}
		return out
	default:
		return t.Muted.Render("No posts here yet. Press ctrl+r to refresh.")
	}
}

func (m Model) failed() int {
	n := 0
	for _, f := range m.feeds {
		if f.err != nil {
			n++
		}
	}
	return n
}
