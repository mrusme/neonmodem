package ui

import (
	"context"
	"fmt"
	"strings"
	"time"

	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/list"
	"charm.land/bubbles/v2/spinner"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/mrusme/neonmodem/internal/feed"
	"github.com/mrusme/neonmodem/internal/models/forum"
	"github.com/mrusme/neonmodem/internal/ui/ctx"
	"github.com/mrusme/neonmodem/internal/ui/header"
	"github.com/mrusme/neonmodem/internal/ui/msgs"
	"github.com/mrusme/neonmodem/internal/ui/views"
	"github.com/mrusme/neonmodem/internal/ui/views/posts"
	"github.com/mrusme/neonmodem/internal/ui/views/splash"
	"github.com/mrusme/neonmodem/internal/ui/windowmanager"
	"github.com/mrusme/neonmodem/internal/ui/windows/msgerror"
	"github.com/mrusme/neonmodem/internal/ui/windows/popuplist"
	"github.com/mrusme/neonmodem/internal/ui/windows/postcreate"
	"github.com/mrusme/neonmodem/internal/ui/windows/postshow"
)

const (
	minWidth  = 60
	minHeight = 20

	noticeDuration = 6 * time.Second
	headerHeight   = header.Height
	noticeHeight   = 1
)

type KeyMap struct {
	SystemSelect key.Binding
	ForumSelect  key.Binding
	Close        key.Binding
}

var DefaultKeyMap = KeyMap{
	SystemSelect: key.NewBinding(
		key.WithKeys("ctrl+e"),
		key.WithHelp("C-e", "System selector"),
	),
	ForumSelect: key.NewBinding(
		key.WithKeys("ctrl+t"),
		key.WithHelp("C-t", "Forum selector"),
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
	w, h := m.ctx.Content[0], m.ctx.Content[1]
	width := w * 2 / 3
	if width < 44 {
		width = 44
	}
	if width > w-4 {
		width = w - 4
	}
	height := h * 3 / 4
	if height < 12 {
		height = 12
	}
	if height > h-2 {
		height = h - 2
	}
	return windowmanager.Centered(w, h, width, height)
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

	m.ctx.Loading = true
	cmds := []tea.Cmd{m.listForums(all)}
	cmds = append(cmds, m.wm.Open(
		popuplist.WIN_ID,
		popuplist.NewModel(m.ctx),
		m.pickerGeometry(),
		msgs.OpenPicker{Kind: msgs.PickForum, Title: "Select a forum", Items: []list.Item{all}},
	)...)

	return tea.Batch(cmds...)
}

func (m Model) listForums(all forum.Forum) tea.Cmd {
	systems := m.ctx.Systems
	only := m.ctx.GetCurrentSystem()

	return func() tea.Msg {
		forums, errs := feed.ListForums(context.Background(), systems, only)

		items := []list.Item{all}
		for _, f := range forums {
			items = append(items, f)
		}

		var failures []error
		for idx, err := range errs {
			if err != nil && idx < len(systems) {
				failures = append(failures, fmt.Errorf("%s: %w", systems[idx].Title(), err))
			}
		}

		return msgs.PickerItems{Kind: msgs.PickForum, Items: items, Errors: failures}
	}
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {

	case tea.BackgroundColorMsg:
		if m.ctx.SetDarkBackground(msg.IsDark()) {
			m.header = header.NewModel(m.ctx)
			cmds = append(cmds, m.broadcast(msgs.ThemeChanged{})...)
		}
		return m, tea.Batch(cmds...)

	case tea.KeyPressMsg:
		switch {
		case key.Matches(msg, m.keymap.Close):
			if m.wm.GetNumberOpen() == 0 {
				break
			}
			focused := m.wm.Focused()
			closed, ccmds := m.wm.CloseFocused()
			if closed && focused == postshow.WIN_ID {
				m.ctx.CancelLoad()
				m.ctx.Loading = false
			}
			return m, tea.Batch(ccmds...)

		case key.Matches(msg, m.keymap.SystemSelect):
			if m.currentView == 0 {
				return m, nil
			}
			return m, m.openSystemPicker()

		case key.Matches(msg, m.keymap.ForumSelect):
			if m.currentView == 0 {
				return m, nil
			}
			return m, m.openForumPicker()

		default:
			if m.wm.GetNumberOpen() > 0 {
				return m, tea.Batch(m.wm.UpdateFocused(msg)...)
			}
		}

	case tea.WindowSizeMsg:
		m.setSizes(msg.Width, msg.Height)
		for i := range m.views {
			v, cmd := m.views[i].Update(tea.WindowSizeMsg{Width: m.ctx.Content[0], Height: m.ctx.Content[1]})
			m.views[i] = v
			cmds = append(cmds, cmd)
		}
		cmds = append(cmds, m.wm.ResizeAll(m.ctx.Content[0], m.ctx.Content[1])...)
		return m, tea.Batch(cmds...)

	case msgs.ShowPosts:
		if m.currentView == 1 {
			return m, nil
		}
		m.currentView = 1
		cmds = append(cmds, msgs.Send(msgs.FocusView{}), msgs.Send(msgs.RefreshFeed{}))
		for _, err := range m.ctx.StartupErrors {
			cmds = append(cmds, msgs.Send(msgs.Notice{Text: "System unavailable: " + err.Error(), IsError: true}))
		}
		return m, tea.Batch(cmds...)

	case msgs.OpenPost:
		cmds = m.wm.Open(
			postshow.WIN_ID,
			postshow.NewModel(m.ctx),
			windowmanager.Geometry{Left: 4, Top: 1, Right: 6, Bottom: 3},
			msg,
		)
		return m, tea.Batch(cmds...)

	case msgs.Compose:
		cmds = m.wm.Open(
			postcreate.WIN_ID,
			postcreate.NewModel(m.ctx),
			m.composeGeometry(),
			msg,
		)
		return m, tea.Batch(cmds...)

	case msgs.ShowError:
		cmds = m.wm.Open(
			msgerror.WIN_ID,
			msgerror.NewModel(m.ctx),
			m.errorGeometry(),
			msg,
		)
		return m, tea.Batch(cmds...)

	case msgs.Picked:
		if _, ccmds := m.wm.Close(popuplist.WIN_ID); ccmds != nil {
			cmds = append(cmds, ccmds...)
		}
		switch item := msg.Item.(type) {
		case SystemItem:
			m.ctx.SetCurrentSystem(item.Index)
			m.ctx.SetCurrentForum(forum.Forum{})
		case forum.Forum:
			m.ctx.SetCurrentSystem(item.SysIDX)
			m.ctx.SetCurrentForum(item)
		}
		m.ctx.Loading = false
		cmds = append(cmds, msgs.Send(msgs.RefreshFeed{}))
		return m, tea.Batch(cmds...)

	case msgs.PickerItems:
		if !m.wm.IsOpen(popuplist.WIN_ID) {
			m.ctx.Loading = false
			return m, nil
		}
		return m, m.wm.Update(popuplist.WIN_ID, msg)

	case msgs.CloseWindow:
		_, ccmds := m.wm.Close(msg.ID)
		return m, tea.Batch(ccmds...)

	case msgs.WindowClosed:
		return m, tea.Batch(m.wm.UpdateAll(msg)...)

	case msgs.ReloadPost:
		if m.wm.IsOpen(postshow.WIN_ID) {
			return m, m.wm.Update(postshow.WIN_ID, msg)
		}
		return m, nil

	case msgs.Notice:
		m.noticeID++
		m.notice = msg.Text
		m.alert = msg.IsError
		id := m.noticeID
		return m, tea.Tick(noticeDuration, func(time.Time) tea.Msg {
			return noticeExpiredMsg{id: id}
		})

	case noticeExpiredMsg:
		if msg.id == m.noticeID {
			m.notice = ""
		}
		return m, nil

	case msgs.FocusView, msgs.BlurView, msgs.RefreshFeed, msgs.FeedResult:
		v, cmd := m.views[m.currentView].Update(msg)
		m.views[m.currentView] = v
		hdr, hcmd := m.header.Update(msg)
		m.header = hdr
		return m, tea.Batch(cmd, hcmd)

	case spinner.TickMsg:
		hdr, hcmd := m.header.Update(msg)
		m.header = hdr
		return m, hcmd

	default:
		cmds = append(cmds, m.wm.UpdateFocused(msg)...)
	}

	v, vcmd := m.views[m.currentView].Update(msg)
	m.views[m.currentView] = v
	cmds = append(cmds, vcmd)

	hdr, hcmd := m.header.Update(msg)
	m.header = hdr
	cmds = append(cmds, hcmd)

	return m, tea.Batch(cmds...)
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
		return strings.Repeat(" ", width)
	}

	style := m.ctx.Theme.Notice
	if m.alert {
		style = m.ctx.Theme.Alert
	}
	return style.Width(width).MaxHeight(1).Render(" " + m.notice)
}

func (m Model) setSizes(winWidth int, winHeight int) {
	m.ctx.Screen[0] = winWidth
	m.ctx.Screen[1] = winHeight
	m.ctx.Content[0] = winWidth
	m.ctx.Content[1] = winHeight - headerHeight - noticeHeight
	if m.ctx.Content[1] < 1 {
		m.ctx.Content[1] = 1
	}
}
