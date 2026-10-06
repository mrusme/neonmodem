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
	if i.SystemTitle != "" {
		desc += " · " + i.SystemTitle
	}
	switch i.ReplyCount {
	case 0:
	case 1:
		desc += " · 1 reply"
	default:
		desc += fmt.Sprintf(" · %d replies", i.ReplyCount)
	}
	return desc
}

type Model struct {
	ctx     *ctx.Ctx
	keymap  KeyMap
	focused bool

	list list.Model

	gen      int64
	pending  int
	total    int
	results  map[int][]post.Post
	failures map[int]error
	loaded   bool

	width  int
	height int
}

func NewModel(c *ctx.Ctx) Model {
	m := Model{
		ctx:      c,
		keymap:   DefaultKeyMap,
		results:  map[int][]post.Post{},
		failures: map[int]error{},
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

func (m *Model) refresh() tea.Cmd {
	systems := m.ctx.Systems
	only := m.ctx.GetCurrentSystem()
	forumID := m.ctx.GetCurrentForum().ID

	indexes := feed.Selected(systems, only)
	m.gen = m.ctx.NextLoadGen()
	m.pending = len(indexes)
	m.total = len(indexes)
	m.results = map[int][]post.Post{}
	m.failures = map[int]error{}
	m.loaded = false
	m.list.SetItems(nil)
	m.list.ResetSelected()
	m.updateProgress()

	if len(indexes) == 0 {
		m.loaded = true
		m.ctx.Loading = false
		return nil
	}

	gen := m.gen
	cmds := make([]tea.Cmd, 0, len(indexes))
	for _, idx := range indexes {
		idx := idx
		sys := systems[idx]
		cmds = append(cmds, func() tea.Msg {
			posts, err := sys.ListPosts(context.Background(), forumID)
			return msgs.FeedResult{Gen: gen, System: idx, Posts: posts, Err: err}
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
		m.failures[res.System] = res.Err
		m.ctx.Logger.Error("listing posts failed", "system", res.System, "error", res.Err)
	} else {
		m.results[res.System] = res.Posts
	}

	var merged []post.Post
	for _, posts := range m.results {
		merged = append(merged, posts...)
	}
	feed.SortPosts(merged)

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

	var cmds []tea.Cmd
	if res.Err != nil && res.System >= 0 && res.System < len(m.ctx.Systems) {
		cmds = append(cmds, msgs.Send(msgs.Notice{
			Text:    fmt.Sprintf("%s: %v", m.ctx.Systems[res.System].Title(), res.Err),
			IsError: true,
		}))
	}
	if m.pending <= 0 {
		m.loaded = true
	}

	return tea.Batch(cmds...)
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
	case len(m.failures) > 0:
		keys := make([]int, 0, len(m.failures))
		for idx := range m.failures {
			keys = append(keys, idx)
		}
		sort.Ints(keys)
		out := t.Alert.Render("No posts could be loaded.") + "\n"
		for _, idx := range keys {
			name := fmt.Sprintf("system %d", idx)
			if idx >= 0 && idx < len(m.ctx.Systems) {
				name = m.ctx.Systems[idx].Title()
			}
			out += "\n" + t.Muted.Render(name+": "+m.failures[idx].Error())
		}
		return out
	default:
		return t.Muted.Render("No posts here yet. Press ctrl+r to refresh.")
	}
}
