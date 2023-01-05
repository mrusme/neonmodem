package ui

import (
	// "fmt"

	"strings"

	"github.com/mrusme/gobbs/aggregator"
	"github.com/mrusme/gobbs/models/forum"
	"github.com/mrusme/gobbs/system"
	"github.com/mrusme/gobbs/ui/cmd"
	"github.com/mrusme/gobbs/ui/ctx"
	"github.com/mrusme/gobbs/ui/header"
	"github.com/mrusme/gobbs/ui/views/posts"
	"github.com/mrusme/gobbs/ui/windowmanager"
	"github.com/mrusme/gobbs/ui/windows/msgerror"
	"github.com/mrusme/gobbs/ui/windows/popuplist"
	"github.com/mrusme/gobbs/ui/windows/postcreate"
	"github.com/mrusme/gobbs/ui/windows/postshow"

	"github.com/mrusme/gobbs/ui/views"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
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
		key.WithKeys("q", "esc"),
		key.WithHelp("q/esc", "close"),
	),
}

type Model struct {
	keymap      KeyMap
	header      header.Model
	views       []views.View
	currentView int
	wm          *windowmanager.WM
	ctx         *ctx.Ctx

	a *aggregator.Aggregator

	viewcache         string
	viewcacheID       string
	renderOnlyFocused bool
}

func NewModel(c *ctx.Ctx) Model {
	m := Model{
		keymap:      DefaultKeyMap,
		currentView: 0,
		wm:          windowmanager.New(c),
		ctx:         c,

		viewcache:         "",
		renderOnlyFocused: false,
	}

	m.header = header.NewModel(m.ctx)
	m.views = append(m.views, posts.NewModel(m.ctx))

	m.a, _ = aggregator.New(m.ctx)

	return m
}

func (m Model) Init() tea.Cmd {
	return tea.Batch(
		tea.EnterAltScreen,
		cmd.New(cmd.ViewFocus, "*").Tea(),
		cmd.New(cmd.ViewRefreshData, "*").Tea(),
	)
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	cmds := make([]tea.Cmd, 0)

	m.viewcacheID = m.wm.Focused()

	switch msg := msg.(type) {

	case tea.KeyMsg:
		switch {
		case key.Matches(msg, m.keymap.Close):
			m.ctx.Logger.Debug("close received")
			closed, ccmds := m.wm.CloseFocused()
			if !closed {
				m.ctx.Logger.Debug("CloseFocused() was false, quitting")
				return m, tea.Quit
			}
			return m, tea.Batch(ccmds...)
		case key.Matches(msg, m.keymap.SystemSelect):
			var listItems []list.Item

			all, _ := system.New("all", nil, m.ctx.Logger)
			all.SetID(-1)
			listItems = append(listItems, all)

			for _, sys := range m.ctx.Systems {
				listItems = append(listItems, *sys)
			}

			ccmds := m.wm.Open(
				popuplist.WIN_ID,
				popuplist.NewModel(m.ctx),
				[4]int{
					int(m.ctx.Content[1] / 2),
					int(m.ctx.Content[1] / 4),
					int(m.ctx.Content[1] / 2),
					int(m.ctx.Content[1] / 4),
				},
				cmd.New(
					cmd.WinOpen,
					popuplist.WIN_ID,
					cmd.Arg{Name: "selectionID", Value: "system"},
					cmd.Arg{Name: "items", Value: listItems},
				),
			)

			return m, tea.Batch(ccmds...)

		case key.Matches(msg, m.keymap.ForumSelect):
			var listItems []list.Item
			ccmds := make([]tea.Cmd, 0)

			all := forum.Forum{ID: "", Name: "All", SysIDX: m.ctx.GetCurrentSystem()}
			listItems = append(listItems, all)

			forums, errs := m.a.ListForums()
			for _, err := range errs {
				if err != nil {
					m.ctx.Logger.Error(errs)
					ccmds = append(ccmds, cmd.New(
						cmd.MsgError,
						"*",
						cmd.Arg{Name: "errors", Value: errs},
					).Tea())
				}
			}

			for _, f := range forums {
				m.ctx.Logger.Debugf("Adding forum to list: %s ID %d\n", f.Title(), f.SysIDX)
				listItems = append(listItems, f)
			}

			ccmds = m.wm.Open(
				popuplist.WIN_ID,
				popuplist.NewModel(m.ctx),
				[4]int{
					int(m.ctx.Content[1] / 2),
					int(m.ctx.Content[1] / 4),
					int(m.ctx.Content[1] / 2),
					int(m.ctx.Content[1] / 4),
				},
				cmd.New(
					cmd.WinOpen,
					popuplist.WIN_ID,
					cmd.Arg{Name: "selectionID", Value: "forum"},
					cmd.Arg{Name: "items", Value: listItems},
				),
			)

			return m, tea.Batch(ccmds...)

		default:
			if m.wm.GetNumberOpen() > 0 {
				cmd := m.wm.Update(m.wm.Focused(), msg)
				return m, cmd
			}
		}

	case tea.WindowSizeMsg:
		m.setSizes(msg.Width, msg.Height)
		for i := range m.views {
			v, cmd := m.views[i].Update(msg)
			m.views[i] = v
			cmds = append(cmds, cmd)
		}
		m.ctx.Logger.Debugf("resizing all: %v\n", m.ctx.Content)
		ccmds := m.wm.ResizeAll(m.ctx.Content[0], m.ctx.Content[1])
		cmds = append(cmds, ccmds...)

	case cmd.Command:
		var ccmds []tea.Cmd

		switch msg.Call {

		case cmd.WinOpen:
			switch msg.Target {
			case postshow.WIN_ID:
				m.ctx.Logger.Debugln("received WinOpen")
				ccmds = m.wm.Open(
					msg.Target,
					postshow.NewModel(m.ctx),
					[4]int{
						3,
						1,
						6,
						4,
					},
					&msg,
				)
			case postcreate.WIN_ID:
				m.ctx.Logger.Debugln("received WinOpen")
				ccmds = m.wm.Open(
					msg.Target,
					postcreate.NewModel(m.ctx),
					[4]int{
						6,
						m.ctx.Content[1] - 16,
						10,
						4,
					},
					&msg,
				)
				m.viewcache = m.buildView(false)
			}
			m.ctx.Logger.Debugf("got back ccmds: %v\n", ccmds)

		case cmd.WinClose:
			switch msg.Target {

			case postcreate.WIN_ID:
				m.ctx.Logger.Debugln("received WinClose")

			case popuplist.WIN_ID:
				selectionIDIf := msg.GetArg("selectionID")
				if selectionIDIf == nil {
					return m, nil
				}
				switch selectionIDIf.(string) {
				case "system":
					selected := msg.GetArg("selected").(system.System)
					m.ctx.SetCurrentSystem(selected.GetID())
					m.ctx.SetCurrentForum(forum.Forum{})
				case "forum":
					selected := msg.GetArg("selected").(forum.Forum)
					m.ctx.Logger.Debugf("selecting system ID %d\n", selected.SysIDX)
					m.ctx.SetCurrentSystem(selected.SysIDX)
					m.ctx.SetCurrentForum(selected)
				}
				return m, cmd.New(cmd.ViewRefreshData, "*").Tea()

			}

		case cmd.WMCloseWin:
			if ok, clcmds := m.wm.Close(msg.Target, msg.GetArgs()...); ok {
				cmds = append(cmds, clcmds...)
			}

		case cmd.MsgError:
			m.ctx.Logger.Debugln("received MsgError")
			ccmds = m.wm.Open(
				msgerror.WIN_ID,
				msgerror.NewModel(m.ctx),
				[4]int{
					int(m.ctx.Content[1] / 2),
					int(m.ctx.Content[1] / 4),
					int(m.ctx.Content[1] / 2),
					int(m.ctx.Content[1] / 4),
				},
				&msg,
			)

		default:
			// if msg.Call < cmd.ViewFocus {
			m.ctx.Logger.Debugf("updating all with cmd: %v\n", msg)
			ccmds = m.wm.UpdateAll(msg)
			// }
		}

		cmds = append(cmds, ccmds...)

	case spinner.TickMsg:
		// Do nothing

	default:
		m.ctx.Logger.Debugf("updating focused with default: %v\n", msg)
		cmds = append(cmds, m.wm.UpdateFocused(msg)...)
	}

	v, vcmd := m.views[m.currentView].Update(msg)
	m.views[m.currentView] = v
	cmds = append(cmds, vcmd)

	header, hcmd := m.header.Update(msg)
	m.header = header
	cmds = append(cmds, hcmd)

	return m, tea.Batch(cmds...)
}

func (m Model) View() string {
	return m.buildView(true)
}

func (m Model) buildView(cached bool) string {
	s := strings.Builder{}
	var tmp string = ""

	m.ctx.Logger.Debugf("viewcacheID: %s\n", m.viewcacheID)
	if cached && m.viewcache != "" && m.viewcacheID == m.wm.Focused() &&
		m.viewcacheID == postcreate.WIN_ID {
		m.ctx.Logger.Debug("hitting UI viewcache")
		tmp = m.viewcache
		m.renderOnlyFocused = true
	} else {
		m.ctx.Logger.Debug("generating UI viewcache")
		m.renderOnlyFocused = false
		s.WriteString(m.header.View() + "\n")
		s.WriteString(m.views[m.currentView].View())
		tmp = s.String()
	}

	return m.wm.View(tmp, m.renderOnlyFocused)
}

func (m Model) setSizes(winWidth int, winHeight int) {
	(*m.ctx).Screen[0] = winWidth
	(*m.ctx).Screen[1] = winHeight
	m.ctx.Content[0] = m.ctx.Screen[0]
	m.ctx.Content[1] = m.ctx.Screen[1] - 8
}
