package postcreate

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/charmbracelet/bubbles/cursor"
	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/textarea"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/glamour"
	"github.com/charmbracelet/lipgloss"
	"github.com/mrusme/gobbs/aggregator"
	"github.com/mrusme/gobbs/models/post"
	"github.com/mrusme/gobbs/models/reply"
	"github.com/mrusme/gobbs/ui/cmd"
	"github.com/mrusme/gobbs/ui/ctx"
	"github.com/mrusme/gobbs/ui/helpers"
)

var (
	WIN_ID = "postcreate"
)

type KeyMap struct {
	Refresh key.Binding
	Select  key.Binding
	Esc     key.Binding
	Quit    key.Binding
	Reply   key.Binding
}

var DefaultKeyMap = KeyMap{
	Reply: key.NewBinding(
		key.WithKeys("ctrl+s"),
		key.WithHelp("ctrl+s", "reply"),
	),
}

type Model struct {
	ctx      *ctx.Ctx
	keymap   KeyMap
	wh       [2]int
	focused  bool
	xywh     [4]int
	textarea textarea.Model

	a    *aggregator.Aggregator
	glam *glamour.TermRenderer

	replyToIdx   int
	replyTo      string
	replyToIface interface{}

	viewcache           string
	viewcacheTextareaXY []int
}

func (m Model) Init() tea.Cmd {
	return nil
}

func (m Model) Focus() {
	m.focused = true
}

func (m Model) Blur() {
	m.focused = false
}

func NewModel(c *ctx.Ctx) Model {
	m := Model{
		ctx:     c,
		keymap:  DefaultKeyMap,
		focused: false,
		xywh:    [4]int{0, 0, 0, 0},

		replyToIdx:   0,
		replyTo:      "",
		replyToIface: nil,

		viewcache:           "",
		viewcacheTextareaXY: []int{0, 0, 0, 0},
	}

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

		case key.Matches(msg, m.keymap.Reply):

			var irtID string = ""
			var irtIRT string = ""
			var irtSysIDX int = 0

			if m.replyToIdx == 0 {
				pst := m.replyToIface.(post.Post)
				irtID = pst.ID
				irtSysIDX = pst.SysIDX
			} else {
				rply := m.replyToIface.(reply.Reply)
				irtID = strconv.Itoa(m.replyToIdx + 1)
				irtIRT = rply.InReplyTo
				irtSysIDX = rply.SysIDX
			}

			r := reply.Reply{
				ID:        irtID,
				InReplyTo: irtIRT,
				Body:      m.textarea.Value(),
				SysIDX:    irtSysIDX,
			}

			err := m.a.CreateReply(&r)
			if err != nil {
				m.ctx.Logger.Error(err)
				// TODO
			}

			m.textarea.Reset()
			m.replyToIdx = 0
			return m, cmd.New(cmd.WMCloseWin, WIN_ID).Tea()

		}

	case tea.WindowSizeMsg:
		m.wh[0] = msg.Width
		m.wh[1] = msg.Height
		m.ctx.Logger.Debugf("received WindowSizeMsg: %v\n", m.wh)

	case cmd.Command:
		m.ctx.Logger.Debugf("got command: %v\n", msg)
		switch msg.Call {
		case cmd.WinOpen:
			if msg.Target == WIN_ID {
				m.xywh = msg.GetArg("xywh").([4]int)
				m.replyToIdx = msg.GetArg("replyToIdx").(int)
				m.replyTo = msg.GetArg("replyTo").(string)
				m.replyToIface = msg.GetArg(m.replyTo)
				return m, m.textarea.Focus()
			}
		case cmd.WinClose:
			if msg.Target == WIN_ID {
				m.textarea.Reset()
				return m, nil
			}
		case cmd.WinFocus:
			if msg.Target == WIN_ID ||
				msg.Target == "*" {
				m.focused = true
				m.viewcache = m.buildView(false)
			}
			return m, nil
		case cmd.WinBlur:
			if msg.Target == WIN_ID ||
				msg.Target == "*" {
				m.focused = false
			}
			return m, nil
		default:
			m.ctx.Logger.Debugf("received unhandled command: %v\n", msg)
		}

	case cursor.BlinkMsg:
		m.ctx.Logger.Debugf("textarea is focused: %v\n", m.textarea.Focused())

		// default:
		// 	m.ctx.Logger.Debugf("received unhandled msg: %v\n", msg)
	}

	var tcmd tea.Cmd

	if !m.textarea.Focused() {
		cmds = append(cmds, m.textarea.Focus())
	}
	m.textarea, tcmd = m.textarea.Update(msg)
	cmds = append(cmds, tcmd)

	return m, tea.Batch(cmds...)
}

func (m *Model) loadPost(p *post.Post) tea.Cmd {
	return func() tea.Msg {
		m.ctx.Logger.Debug("------ EXECUTED -----")
		if err := m.a.LoadPost(p); err != nil {
			m.ctx.Logger.Error(err)
		}
		return p
	}
}

func (m Model) View() string {
	return m.buildView(true)
}

func (m Model) buildView(cached bool) string {
	var view strings.Builder = strings.Builder{}

	if cached && m.viewcache != "" {
		m.ctx.Logger.Debugln("Cached View()")

		m.textarea.SetWidth(m.viewcacheTextareaXY[2])
		m.textarea.SetHeight(m.viewcacheTextareaXY[3])

		return helpers.PlaceOverlay(
			m.viewcacheTextareaXY[0], m.viewcacheTextareaXY[1],
			m.textarea.View(), m.viewcache,
			false)
	}

	title := "Reply"
	if m.replyToIdx != 0 {
		title += fmt.Sprintf(" to reply #%d", m.replyToIdx)
	}
	titlebar := m.ctx.Theme.DialogBox.Titlebar.Focused.
		Align(lipgloss.Center).
		Width(m.wh[0]).
		Render(title)

	textareaWidth := m.wh[0] - 2
	textareaHeight := 6
	m.textarea.SetWidth(textareaWidth)
	m.textarea.SetHeight(textareaHeight)

	bottombar := m.ctx.Theme.DialogBox.Bottombar.
		Width(m.wh[0]).
		Render("ctrl+enter reply · esc close")

	replyWindow := lipgloss.JoinVertical(
		lipgloss.Center,
		titlebar,
		m.textarea.View(),
		bottombar,
	)

	tmp := m.ctx.Theme.DialogBox.Window.Focused.Render(replyWindow)

	m.viewcacheTextareaXY[0] = 1
	m.viewcacheTextareaXY[1] = 2
	m.viewcacheTextareaXY[2] = textareaWidth
	m.viewcacheTextareaXY[3] = textareaHeight

	view = strings.Builder{}
	view.WriteString(tmp)

	return view.String()
}
