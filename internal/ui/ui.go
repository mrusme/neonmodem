package ui

import (
	"context"
	"fmt"
	"slices"
	"strings"
	"time"

	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/list"
	"charm.land/bubbles/v2/spinner"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/mrusme/neonmodem/internal/feed"
	"github.com/mrusme/neonmodem/internal/models/forum"
	"github.com/mrusme/neonmodem/internal/models/post"
	"github.com/mrusme/neonmodem/internal/openwith"
	"github.com/mrusme/neonmodem/internal/system"
	"github.com/mrusme/neonmodem/internal/ui/ctx"
	"github.com/mrusme/neonmodem/internal/ui/header"
	"github.com/mrusme/neonmodem/internal/ui/msgs"
	"github.com/mrusme/neonmodem/internal/ui/views"
	"github.com/mrusme/neonmodem/internal/ui/views/posts"
	"github.com/mrusme/neonmodem/internal/ui/views/splash"
	"github.com/mrusme/neonmodem/internal/ui/windowmanager"
	"github.com/mrusme/neonmodem/internal/ui/windows"
	"github.com/mrusme/neonmodem/internal/ui/windows/msgerror"
	"github.com/mrusme/neonmodem/internal/ui/windows/popuplist"
	"github.com/mrusme/neonmodem/internal/ui/windows/postcreate"
	"github.com/mrusme/neonmodem/internal/ui/windows/postshow"
)

const (
	minWidth  = 60
	minHeight = 20

	noticeDuration  = 6 * time.Second
	maxNotices      = 50
	headerHeight    = header.Height
	pickerTextInset = 8
	noticeHeight    = 1
)

type KeyMap struct {
	SystemSelect key.Binding
	ForumSelect  key.Binding
	OrderSelect  key.Binding
	Close        key.Binding
	Quit         key.Binding
	Interrupt    key.Binding
}

var DefaultKeyMap = KeyMap{
	Quit: key.NewBinding(
		key.WithKeys("ctrl+q"),
		key.WithHelp("C-q", "quit"),
	),
	Interrupt: key.NewBinding(
		key.WithKeys("ctrl+c"),
	),
	SystemSelect: key.NewBinding(
		key.WithKeys("ctrl+e"),
		key.WithHelp("C-e", "System selector"),
	),
	ForumSelect: key.NewBinding(
		key.WithKeys("ctrl+t"),
		key.WithHelp("C-t", "Forum selector"),
	),
	OrderSelect: key.NewBinding(
		key.WithKeys("ctrl+o"),
		key.WithHelp("C-o", "Sort order selector"),
	),
	Close: key.NewBinding(
		key.WithKeys("esc"),
		key.WithHelp("esc", "close"),
	),
}

type SystemItem struct {
	Index int
	Name  string
	Info  string
}

func (s SystemItem) FilterValue() string { return s.Name + " " + s.Info }
func (s SystemItem) Title() string       { return s.Name }
func (s SystemItem) Description() string { return s.Info }

type OpenWithItem struct {
	Name string
	Line string
	Env  []string
}

func (o OpenWithItem) FilterValue() string { return o.Name }
func (o OpenWithItem) Title() string       { return o.Name }
func (o OpenWithItem) Description() string { return o.Line }

type OrderItem struct {
	Order system.Order
	Note  string
}

func (o OrderItem) FilterValue() string { return o.Order.Label() }
func (o OrderItem) Title() string       { return o.Order.Label() }

func (o OrderItem) Description() string {
	if o.Note == "" {
		return o.Order.Info()
	}
	return o.Order.Info() + "\n" + o.Note
}

type noticeExpiredMsg struct {
	id int
}

type Model struct {
	keymap      KeyMap
	header      header.Model
	views       []views.View
	currentView int
	wm          *windowmanager.WM
	ctx         *ctx.Ctx

	notice   string
	noticeID int
	alert    bool
	status   string

	notices []msgs.NoticeEntry
	unseen  int
}

func NewModel(c *ctx.Ctx) Model {
	m := Model{
		keymap: DefaultKeyMap,
		wm:     windowmanager.New(c),
		ctx:    c,
	}

	m.header = header.NewModel(c)
	m.views = append(m.views, splash.NewModel(c))
	m.views = append(m.views, posts.NewModel(c))

	return m
}

func (m Model) Init() tea.Cmd {
	return tea.RequestBackgroundColor
}

func (m Model) systemItems() []list.Item {
	items := []list.Item{
		SystemItem{Index: feed.All, Name: "All", Info: "Aggregate all systems"},
	}
	for idx, sys := range m.ctx.Systems {
		items = append(items, SystemItem{
			Index: idx,
			Name:  sys.Title(),
			Info:  sys.Description(),
		})
	}
	return items
}

func (m Model) pickerGeometry() windowmanager.Geometry {
	width, height := m.pickerSize()
	return windowmanager.Centered(m.ctx.Content[0], m.ctx.Content[1], width, height)
}

func (m Model) pickerSize() (int, int) {
	w, h := m.ctx.Content[0], m.ctx.Content[1]
	width := min(max(w*2/3, 44), w-4)
	height := min(max(h*3/4, 12), h-2)
	return width, height
}

func (m Model) composeGeometry() windowmanager.Geometry {
	w, h := m.ctx.Content[0], m.ctx.Content[1]
	width := w - 16
	if width < 40 {
		width = w - 2
	}
	return windowmanager.Centered(w, h, width, 14)
}

func (m Model) errorGeometry() windowmanager.Geometry {
	w, h := m.ctx.Content[0], m.ctx.Content[1]
	return windowmanager.Centered(w, h, w/2+8, h/2)
}

func (m Model) openSystemPicker() tea.Cmd {
	cmds := m.wm.Open(
		popuplist.WIN_ID,
		popuplist.NewModel(m.ctx),
		m.pickerGeometry(),
		msgs.OpenPicker{Kind: msgs.PickSystem, Title: "Select a system", Items: m.systemItems()},
	)
	return tea.Batch(cmds...)
}

func (m Model) openForumPicker() tea.Cmd {
	all := forum.Forum{ID: "", Name: "All", Info: "Every forum of the selected system", SysIDX: m.ctx.GetCurrentSystem()}

	m.ctx.StartLoading(ctx.LoadForums)
	cmds := []tea.Cmd{m.listForums(all)}
	cmds = append(cmds, m.wm.Open(
		popuplist.WIN_ID,
		popuplist.NewModel(m.ctx),
		m.pickerGeometry(),
		msgs.OpenPicker{Kind: msgs.PickForum, Title: "Select a forum", Items: []list.Item{all}},
	)...)

	return tea.Batch(cmds...)
}

func (m Model) orderItems(noteWidth int) ([]list.Item, int) {
	forumID := m.ctx.GetCurrentForum().ID
	indexes := feed.Selected(m.ctx.Systems, m.ctx.GetCurrentSystem())
	current := m.ctx.GetOrder()

	var items []list.Item
	selected := 0
	for _, order := range system.AllOrders() {
		supported := false
		uses := make([]feed.Use, 0, len(indexes))
		for _, idx := range indexes {
			sys := m.ctx.Systems[idx]
			ordering := sys.Orders(forumID)
			if ordering.Supports(order) {
				supported = true
			}
			uses = append(uses, feed.Use{Name: sys.Title(), Order: feed.Choose(ordering, order)})
		}
		if !supported {
			continue
		}

		note := ""
		if len(indexes) > 1 {
			note = ansi.Wordwrap(feed.Status(order, uses), noteWidth, "")
		}
		if order == current {
			selected = len(items)
		}
		items = append(items, OrderItem{Order: order, Note: note})
	}

	return items, selected
}

func (m Model) openOrderPicker() tea.Cmd {
	width, _ := m.pickerSize()
	items, selected := m.orderItems(width - pickerTextInset)
	if len(items) == 0 {
		return nil
	}

	cmds := m.wm.Open(
		popuplist.WIN_ID,
		popuplist.NewModel(m.ctx),
		m.pickerGeometry(),
		msgs.OpenPicker{Kind: msgs.PickOrder, Title: "Select a sort order", Items: items, Selected: selected},
	)
	return tea.Batch(cmds...)
}

func (m Model) openOpenWithPicker(p post.Post) tea.Cmd {
	if len(m.ctx.OpenWith) == 0 || p.SysIDX < 0 || p.SysIDX >= len(m.ctx.Systems) {
		return nil
	}

	env := openwith.Env(m.ctx.Systems[p.SysIDX], p)
	items := make([]list.Item, 0, len(m.ctx.OpenWith))
	for _, command := range m.ctx.OpenWith {
		items = append(items, OpenWithItem{Name: command.Name, Line: command.Cmd, Env: env})
	}

	cmds := m.wm.Open(
		popuplist.WIN_ID,
		popuplist.NewModel(m.ctx),
		m.pickerGeometry(),
		msgs.OpenPicker{Kind: msgs.PickOpenWith, Title: "Open with", Items: items},
	)
	return tea.Batch(cmds...)
}

func (m Model) runOpenWith(item OpenWithItem) tea.Cmd {
	launcher := m.ctx.Launcher
	logger := m.ctx.Logger

	return func() tea.Msg {
		if launcher == nil {
			return msgs.Notice{Text: item.Name + ": open with commands aren't available", IsError: true}
		}
		if err := launcher.Start(item.Name, item.Line, item.Env); err != nil {
			logger.Error("open with failed", "name", item.Name, "error", err)
			return msgs.Notice{Text: fmt.Sprintf("%s: %v", item.Name, err), IsError: true}
		}
		return msgs.Notice{Text: "Running " + item.Name}
	}
}

func (m Model) listForums(all forum.Forum) tea.Cmd {
	f := m.ctx.Feed
	only := m.ctx.GetCurrentSystem()

	return func() tea.Msg {
		forums, errs := f.ListForums(context.Background(), only)

		items := []list.Item{all}
		for _, f := range forums {
			items = append(items, f)
		}

		var failures []error
		for idx, err := range errs {
			if err != nil && idx < len(f.Systems) {
				failures = append(failures, fmt.Errorf("%s: %w", f.Systems[idx].Title(), err))
			}
		}

		return msgs.PickerItems{Kind: msgs.PickForum, Items: items, Errors: failures}
	}
}

var postGeometry = windowmanager.Geometry{Left: 4, Top: 1, Right: 6, Bottom: 3}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.BackgroundColorMsg:
		return m.changeBackground(msg)

	case tea.KeyPressMsg:
		if model, cmd, handled := m.handleKey(msg); handled {
			return model, cmd
		}
		return m.updateViewAndHeader(msg, nil)

	case tea.WindowSizeMsg:
		return m.resize(msg)

	case msgs.ShowPosts:
		return m.showPosts()

	case msgs.OpenWithMenu:
		return m, m.openOpenWithPicker(msg.Post)

	case msgs.OpenPost:
		return m.openWindow(postshow.WIN_ID, postshow.NewModel(m.ctx), postGeometry, msg)

	case msgs.Compose:
		return m.openWindow(postcreate.WIN_ID, postcreate.NewModel(m.ctx), m.composeGeometry(), msg)

	case msgs.ShowError:
		return m.openWindow(msgerror.WIN_ID, msgerror.NewModel(m.ctx), m.errorGeometry(), msg)

	case msgs.Picked:
		return m.handlePicked(msg)

	case msgs.PickerItems:
		return m.handlePickerItems(msg)

	case msgs.CloseWindow:
		_, cmds := m.wm.Close(msg.ID)
		return m, tea.Batch(cmds...)

	case msgs.WindowClosed:
		return m, tea.Batch(m.wm.UpdateAll(msg)...)

	case msgs.ReloadPost:
		if m.wm.IsOpen(postshow.WIN_ID) {
			return m, m.wm.Update(postshow.WIN_ID, msg)
		}
		return m, nil

	case msgs.Notice:
		return m.showNotice(msg)

	case msgs.OpenNotices:
		return m.openNotices()

	case noticeExpiredMsg:
		if msg.id == m.noticeID {
			m.notice = ""
		}
		return m, nil

	case msgs.FeedStatus:
		m.status = msg.Text
		return m, nil

	case msgs.FocusView, msgs.BlurView, msgs.RefreshFeed, msgs.OrderChanged, msgs.FeedResult, msgs.NoticesChanged:
		return m.updateViewAndHeader(msg, nil)

	case spinner.TickMsg:
		hdr, cmd := m.header.Update(msg)
		m.header = hdr
		return m, cmd

	default:
		return m.updateViewAndHeader(msg, m.wm.UpdateFocused(msg))
	}
}

func (m Model) changeBackground(msg tea.BackgroundColorMsg) (tea.Model, tea.Cmd) {
	if !m.ctx.SetDarkBackground(msg.IsDark()) {
		return m, nil
	}
	m.header = header.NewModel(m.ctx)
	return m, tea.Batch(m.broadcast(msgs.ThemeChanged{})...)
}

type filterer interface {
	FilterState() list.FilterState
}

func (m Model) filterState() list.FilterState {
	var target any = m.views[m.currentView]
	if m.wm.GetNumberOpen() > 0 {
		target = m.wm.FocusedWindow()
	}
	if f, ok := target.(filterer); ok {
		return f.FilterState()
	}
	return list.Unfiltered
}

func (m Model) handleKey(msg tea.KeyPressMsg) (Model, tea.Cmd, bool) {
	switch {
	case key.Matches(msg, m.keymap.Quit):
		return m, tea.Quit, true

	case key.Matches(msg, m.keymap.Interrupt):
		return m, msgs.Send(msgs.Notice{Text: "Press ctrl+q to quit"}), true

	case key.Matches(msg, m.keymap.Close):
		if m.wm.GetNumberOpen() == 0 || m.filterState() != list.Unfiltered {
			break
		}
		return m.closeFocused()

	case key.Matches(msg, m.keymap.SystemSelect, m.keymap.ForumSelect, m.keymap.OrderSelect):
		if m.filterState() == list.Filtering {
			break
		}
		switch {
		case key.Matches(msg, m.keymap.SystemSelect):
			return m.onPostsView(m.openSystemPicker)
		case key.Matches(msg, m.keymap.ForumSelect):
			return m.onPostsView(m.openForumPicker)
		default:
			return m.onPostsView(m.openOrderPicker)
		}
	}

	if m.wm.GetNumberOpen() > 0 {
		return m, tea.Batch(m.wm.UpdateFocused(msg)...), true
	}
	return m, nil, false
}

func (m Model) onPostsView(open func() tea.Cmd) (Model, tea.Cmd, bool) {
	if m.currentView == 0 {
		return m, nil, true
	}
	return m, open(), true
}

func (m Model) closeFocused() (Model, tea.Cmd, bool) {
	focused := m.wm.Focused()
	closed, cmds := m.wm.CloseFocused()
	if closed && focused == postshow.WIN_ID {
		m.ctx.CancelLoad()
		m.ctx.StopLoading(ctx.LoadPost)
	}
	return m, tea.Batch(cmds...), true
}

func (m Model) resize(msg tea.WindowSizeMsg) (tea.Model, tea.Cmd) {
	m.setSizes(msg.Width, msg.Height)

	var cmds []tea.Cmd
	for i := range m.views {
		v, cmd := m.views[i].Update(tea.WindowSizeMsg{Width: m.ctx.Content[0], Height: m.ctx.Content[1]})
		m.views[i] = v
		cmds = append(cmds, cmd)
	}
	cmds = append(cmds, m.wm.ResizeAll(m.ctx.Content[0], m.ctx.Content[1])...)
	return m, tea.Batch(cmds...)
}

func (m Model) showPosts() (tea.Model, tea.Cmd) {
	if m.currentView == 1 {
		return m, nil
	}
	m.currentView = 1

	cmds := []tea.Cmd{msgs.Send(msgs.FocusView{}), msgs.Send(msgs.RefreshFeed{})}
	for _, err := range m.ctx.StartupErrors {
		cmds = append(cmds, msgs.Send(msgs.Notice{Text: "System unavailable: " + err.Error(), IsError: true}))
	}
	for _, notice := range m.ctx.StartupNotices {
		cmds = append(cmds, msgs.Send(msgs.Notice{Text: notice, IsError: true}))
	}
	return m, tea.Batch(cmds...)
}

func (m Model) openWindow(id string, win windows.Window, geom windowmanager.Geometry, init tea.Msg) (tea.Model, tea.Cmd) {
	return m, tea.Batch(m.wm.Open(id, win, geom, init)...)
}

func (m Model) handlePicked(msg msgs.Picked) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd
	if _, closing := m.wm.Close(popuplist.WIN_ID); closing != nil {
		cmds = append(cmds, closing...)
	}
	if msg.Kind == msgs.PickForum {
		m.ctx.StopLoading(ctx.LoadForums)
	}

	switch item := msg.Item.(type) {
	case SystemItem:
		m.ctx.SetCurrentSystem(item.Index)
		m.ctx.SetCurrentForum(forum.Forum{})
		cmds = append(cmds, msgs.Send(msgs.RefreshFeed{}))
	case forum.Forum:
		m.ctx.SetCurrentSystem(item.SysIDX)
		m.ctx.SetCurrentForum(item)
		cmds = append(cmds, msgs.Send(msgs.RefreshFeed{}))
	case OrderItem:
		m.ctx.SetOrder(item.Order)
		cmds = append(cmds, msgs.Send(msgs.OrderChanged{}))
	case OpenWithItem:
		cmds = append(cmds, m.runOpenWith(item))
	}
	return m, tea.Batch(cmds...)
}

func (m Model) handlePickerItems(msg msgs.PickerItems) (tea.Model, tea.Cmd) {
	if msg.Kind == msgs.PickForum {
		m.ctx.StopLoading(ctx.LoadForums)
	}
	if !m.wm.IsOpen(popuplist.WIN_ID) {
		return m, nil
	}
	return m, m.wm.Update(popuplist.WIN_ID, msg)
}

func (m Model) showNotice(msg msgs.Notice) (tea.Model, tea.Cmd) {
	m.noticeID++
	m.notice = msg.Text
	m.alert = msg.IsError
	m.notices = append(m.notices, msgs.NoticeEntry{At: time.Now(), Text: msg.Text, IsError: msg.IsError})
	if len(m.notices) > maxNotices {
		m.notices = slices.Clone(m.notices[len(m.notices)-maxNotices:])
	}
	m.unseen++

	id := m.noticeID
	expire := tea.Tick(noticeDuration, func(time.Time) tea.Msg {
		return noticeExpiredMsg{id: id}
	})
	return m, tea.Batch(expire, m.noticesChanged())
}

func (m Model) noticesChanged() tea.Cmd {
	return msgs.Send(msgs.NoticesChanged{Count: len(m.notices)})
}

func (m Model) openNotices() (tea.Model, tea.Cmd) {
	entries := make([]msgs.NoticeEntry, 0, len(m.notices))
	for _, e := range slices.Backward(m.notices) {
		entries = append(entries, e)
	}
	m.unseen = 0

	model, cmd := m.openWindow(msgerror.WIN_ID, msgerror.NewModel(m.ctx), m.errorGeometry(),
		msgs.ShowNotices{Entries: entries})
	return model, tea.Batch(cmd, m.noticesChanged())
}

func (m Model) updateViewAndHeader(msg tea.Msg, cmds []tea.Cmd) (tea.Model, tea.Cmd) {
	v, vcmd := m.views[m.currentView].Update(msg)
	m.views[m.currentView] = v

	hdr, hcmd := m.header.Update(msg)
	m.header = hdr

	return m, tea.Batch(append(cmds, vcmd, hcmd)...)
}

func (m Model) broadcast(msg tea.Msg) []tea.Cmd {
	var cmds []tea.Cmd
	for i := range m.views {
		v, cmd := m.views[i].Update(msg)
		m.views[i] = v
		cmds = append(cmds, cmd)
	}
	cmds = append(cmds, m.wm.UpdateAll(msg)...)
	return cmds
}

func (m Model) View() tea.View {
	view := tea.NewView(m.render())
	view.AltScreen = true
	return view
}

func (m Model) render() string {
	if m.ctx.Screen[0] == 0 || m.ctx.Screen[1] == 0 {
		return ""
	}

	if m.ctx.Screen[0] < minWidth || m.ctx.Screen[1] < minHeight {
		return lipgloss.Place(m.ctx.Screen[0], m.ctx.Screen[1], lipgloss.Center, lipgloss.Center,
			m.ctx.Theme.Muted.Render(fmt.Sprintf(
				"The terminal is too small.\nNeon Modem needs at least %dx%d, this one is %dx%d.",
				minWidth, minHeight, m.ctx.Screen[0], m.ctx.Screen[1])))
	}

	if m.currentView == 0 {
		return m.views[0].View()
	}

	var s strings.Builder
	s.WriteString(m.header.View())
	s.WriteString("\n")
	s.WriteString(m.noticeLine())
	s.WriteString("\n")
	s.WriteString(m.views[m.currentView].View())

	return m.wm.View(s.String())
}

func (m Model) noticeLine() string {
	width := m.ctx.Screen[0]
	if m.notice == "" {
		if m.status != "" {
			return m.ctx.Theme.Muted.Width(width).MaxHeight(1).Render(" " + m.status)
		}
		return strings.Repeat(" ", width)
	}

	style := m.ctx.Theme.Notice
	if m.alert {
		style = m.ctx.Theme.Alert
	}
	text := m.notice
	if m.unseen > 1 {
		text += fmt.Sprintf(" (+%d)", m.unseen-1)
	}
	return style.Width(width).MaxHeight(1).Render(" " + text)
}

func (m Model) setSizes(winWidth int, winHeight int) {
	m.ctx.Screen[0] = winWidth
	m.ctx.Screen[1] = winHeight
	m.ctx.Content[0] = winWidth
	m.ctx.Content[1] = winHeight - headerHeight - noticeHeight
	m.ctx.Content[1] = max(m.ctx.Content[1], 1)
}
