package posts

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"

	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/list"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/mrusme/neonmodem/internal/feed"
	"github.com/mrusme/neonmodem/internal/models/post"
	"github.com/mrusme/neonmodem/internal/system"
	"github.com/mrusme/neonmodem/internal/system/text"
	"github.com/mrusme/neonmodem/internal/ui/ctx"
	"github.com/mrusme/neonmodem/internal/ui/msgs"
	"github.com/mrusme/neonmodem/internal/ui/views"
)

type KeyMap struct {
	Refresh key.Binding
	NewPost key.Binding
	Select  key.Binding
	Notices key.Binding
	Quit    key.Binding
}

var DefaultKeyMap = KeyMap{
	Refresh: key.NewBinding(
		key.WithKeys("ctrl+r"),
		key.WithHelp("ctrl+r", "refresh"),
	),
	Notices: key.NewBinding(
		key.WithKeys("!"),
		key.WithHelp("!", "notices"),
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
		key.WithKeys("ctrl+q"),
		key.WithHelp("ctrl+q", "quit"),
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
	m.setHelpKeys(0)

	return m
}

func (m *Model) setHelpKeys(notices int) {
	bindings := []key.Binding{m.keymap.Quit}
	if notices > 0 {
		bindings = []key.Binding{m.keymap.Notices, m.keymap.Quit}
	}
	help := func() []key.Binding { return bindings }
	m.list.AdditionalShortHelpKeys = help
	m.list.AdditionalFullHelpKeys = help
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

		case key.Matches(msg, m.keymap.Notices):
			if m.list.FilterState() == list.Filtering {
				break
			}
			return m, msgs.Send(msgs.OpenNotices{})
		}

	case msgs.NoticesChanged:
		m.setHelpKeys(msg.Count)
		return m, nil

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.resize()

	case msgs.FocusView:
		m.focused = true
		return m, nil

	case msgs.BlurView:
		m.focused = false
		return m, nil

	case msgs.ThemeChanged:
		m.list.SetDelegate(m.delegate())
		m.resize()
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

func (m Model) FilterState() list.FilterState {
	return m.list.FilterState()
}

func (m *Model) frameSize() (int, int) {
	t := m.ctx.Theme.PostsList.List
	return max(t.Focused.GetHorizontalFrameSize(), t.Blurred.GetHorizontalFrameSize()),
		max(t.Focused.GetVerticalFrameSize(), t.Blurred.GetVerticalFrameSize())
}

func (m *Model) innerSize() (int, int) {
	h, v := m.frameSize()
	return max(m.width-h, 1), max(m.height-v, 1)
}

func (m *Model) resize() {
	m.list.SetSize(m.innerSize())
}

func (m *Model) compose() tea.Cmd {
	item, ok := m.list.SelectedItem().(Item)
	if !ok {
		if len(m.ctx.Systems) == 0 {
			return msgs.Send(msgs.Message(
				"No systems are connected. Run `neonmodem connect` first."))
		}
		return msgs.Send(msgs.Message(
			"Select a post first; a new post goes to the forum of the selected post."))
	}

	sys := m.ctx.Systems[item.SysIDX]
	if !sys.Capabilities().Has(system.CapCreatePost) {
		return msgs.Send(msgs.Message(fmt.Sprintf(
			"%s is connected without an account, so posting isn't available. "+
				"Run `%s` again with credentials to post.",
			sys.Title(), system.ConnectCommand(sys.Kind(), sys.URL()))))
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
	m.gen++
	m.order = m.ctx.GetOrder()
	m.pending = len(indexes)
	m.total = len(m.selected())
	m.updateProgress()

	if len(indexes) == 0 {
		return nil
	}

	f := m.ctx.Feed
	forumID := m.ctx.GetCurrentForum().ID
	want := m.order
	gen := m.gen

	cmds := make([]tea.Cmd, 0, len(indexes))
	for _, idx := range indexes {
		cmds = append(cmds, func() tea.Msg {
			res, err := f.List(context.Background(), idx, forumID, want)
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
	var reconnect []string
	for _, idx := range m.selected() {
		f, ok := m.feeds[idx]
		if !ok {
			continue
		}
		if f.err != nil {
			if errors.Is(f.err, system.ErrNeedsConnect) {
				reconnect = append(reconnect, m.ctx.Systems[idx].Title())
			}
			continue
		}
		uses = append(uses, feed.Use{Name: m.ctx.Systems[idx].Title(), Order: f.order})
	}

	var parts []string
	for _, part := range []string{needsConnect(reconnect), feed.Status(m.order, uses)} {
		if part != "" {
			parts = append(parts, part)
		}
	}
	return strings.Join(parts, "; ")
}

func needsConnect(names []string) string {
	switch len(names) {
	case 0:
		return ""
	case 1:
		return names[0] + " needs `neonmodem connect`"
	}
	return feed.JoinNames(names) + " need `neonmodem connect`"
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
		m.ctx.StartLoading(ctx.LoadFeed)
		if m.total > 1 {
			m.ctx.Progress = fmt.Sprintf("%d/%d", m.total-m.pending, m.total)
		} else {
			m.ctx.Progress = ""
		}
		return
	}
	m.ctx.StopLoading(ctx.LoadFeed)
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
		width, height := m.innerSize()
		return frame.Render(lipgloss.Place(
			width, height, lipgloss.Center, lipgloss.Center, m.placeholder()))
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
		slices.Sort(keys)
		var out strings.Builder
		out.WriteString(t.Alert.Render("No posts could be loaded.") + "\n")
		for _, idx := range keys {
			name := fmt.Sprintf("system %d", idx)
			if idx >= 0 && idx < len(m.ctx.Systems) {
				name = m.ctx.Systems[idx].Title()
			}
			out.WriteString("\n" + t.Muted.Render(name+": "+text.Printable(m.feeds[idx].err.Error())))
		}
		return out.String()
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
