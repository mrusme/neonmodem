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
	"github.com/mrusme/gobbs/models/reply"
	"github.com/mrusme/gobbs/ui/cmd"
	"github.com/mrusme/gobbs/ui/ctx"
	"github.com/mrusme/gobbs/ui/windows/postshow"
)

var (
	VIEW_ID = "posts"

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
	// Esc     key.Binding
	// Quit    key.Binding
	Reply key.Binding
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
	// Esc: key.NewBinding(
	// 	key.WithKeys("esc"),
	// 	key.WithHelp("esc", "close"),
	// ),
	// Quit: key.NewBinding(
	// 	key.WithKeys("q"),
	// ),
	Reply: key.NewBinding(
		key.WithKeys("ctrl+s"),
		key.WithHelp("ctrl+s", "reply"),
	),
}

type Model struct {
	ctx      *ctx.Ctx
	keymap   KeyMap
	focused  bool
	list     list.Model
	items    []list.Item
	viewport viewport.Model
	textarea textarea.Model

	a    *aggregator.Aggregator
	glam *glamour.TermRenderer

	// wm []string

	buffer   string
	replyIDs []string

	activePost  *post.Post
	allReplies  []*reply.Reply
	activeReply *reply.Reply

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
		ctx:     c,
		keymap:  DefaultKeyMap,
		focused: false,

		// wm: []string{WM_ROOT_ID},

		buffer:   "",
		replyIDs: []string{},

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

	// m.textarea = textarea.New()
	// m.textarea.Placeholder = "Type in your reply ..."
	// m.textarea.Prompt = ""

	m.a, _ = aggregator.New(m.ctx)

	return m
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch {

		case key.Matches(msg, m.keymap.Refresh):
			// if m.WMisFocused("list") {
			m.ctx.Loading = true
			cmds = append(cmds, m.refresh())
			// }

		case key.Matches(msg, m.keymap.Select):
			// switch m.WMFocused() {
			//
			// case "list":
			i, ok := m.list.SelectedItem().(post.Post)
			if ok {
				// m.ctx.Loading = true
				// cmds = append(cmds, m.loadItem(&i))
				m.viewcache = m.buildView(false)
				cmd := cmd.New(cmd.WinOpen, postshow.WIN_ID, cmd.Arg{
					Name:  "post",
					Value: &i,
				})
				cmds = append(cmds, cmd.Tea())
			}
			//
			// case "post":
			// 	if m.buffer != "" {
			// 		replyToID, err := strconv.Atoi(m.buffer)
			// 		if err != nil {
			// 			// TODO: Handle error
			// 		}
			//
			// 		if replyToID >= len(m.replyIDs) {
			// 			// TODO: Handle error
			// 		}
			// 	}
			// 	m.WMOpen("reply")
			//
			// 	m.ctx.Logger.Debugln("caching view")
			// 	m.ctx.Logger.Debugf("buffer: %s", m.buffer)
			// 	m.viewcache = m.buildView(false)
			//
			// 	return m, m.textarea.Focus()
			// }

		// case key.Matches(msg, m.keymap.Esc), key.Matches(msg, m.keymap.Quit):
		// switch m.WMFocused() {
		//
		// case "list":
		// return m, tea.Quit
		//
		// case "post":
		// 	// Let's make sure we reset the texarea
		// 	m.textarea.Reset()
		// 	m.WMClose("post")
		// 	return m, nil
		//
		// case "reply":
		// 	if key.Matches(msg, m.keymap.Esc) {
		// 		m.buffer = ""
		// 		m.WMClose("reply")
		// 		return m, nil
		// 	}
		// }

		case key.Matches(msg, m.keymap.Reply):
			// if m.WMisFocused("reply") {
			// 	replyToIdx, _ := strconv.Atoi(m.buffer)
			//
			// 	m.ctx.Logger.Debugf("replyToIdx: %d", replyToIdx)
			//
			// 	var irtID string = ""
			// 	var irtIRT string = ""
			// 	var irtSysIDX int = 0
			//
			// 	if replyToIdx == 0 {
			// 		irtID = m.activePost.ID
			// 		irtSysIDX = m.activePost.SysIDX
			// 	} else {
			// 		irt := m.allReplies[(replyToIdx - 1)]
			// 		irtID = strconv.Itoa(replyToIdx + 1)
			// 		irtIRT = irt.InReplyTo
			// 		irtSysIDX = irt.SysIDX
			// 	}
			//
			// 	r := reply.Reply{
			// 		ID:        irtID,
			// 		InReplyTo: irtIRT,
			// 		Body:      m.textarea.Value(),
			// 		SysIDX:    irtSysIDX,
			// 	}
			// 	err := m.a.CreateReply(&r)
			// 	if err != nil {
			// 		m.ctx.Logger.Error(err)
			// 	}
			//
			// 	m.textarea.Reset()
			// 	m.buffer = ""
			// 	m.WMClose("reply")
			// 	return m, nil
			// }

			// default:
			// 	switch msg.String() {
			// 	case "1", "2", "3", "4", "5", "6", "7", "8", "9", "0":
			// 		if m.WMisFocused("post") {
			// 			m.buffer += msg.String()
			// 			return m, nil
			// 		}
			// 	default:
			// 		if m.WMFocused() != "reply" {
			// 			m.buffer = ""
			// 		}
			// 	}
		}

	case tea.WindowSizeMsg:
		listWidth := m.ctx.Content[0] - 2
		listHeight := m.ctx.Content[1] - 1
		// viewportWidth := m.ctx.Content[0] - 9
		// viewportHeight := m.ctx.Content[1] - 10

		m.ctx.Theme.PostsList.List.Focused.Width(listWidth)
		m.ctx.Theme.PostsList.List.Blurred.Width(listWidth)
		m.ctx.Theme.PostsList.List.Focused.Height(listHeight)
		m.ctx.Theme.PostsList.List.Blurred.Height(listHeight)
		m.list.SetSize(
			listWidth-2,
			listHeight-2,
		)

		// viewportStyle.Width(viewportWidth)
		// viewportStyle.Height(viewportHeight)
		// m.viewport = viewport.New(viewportWidth-4, viewportHeight-4)
		// m.viewport.Width = viewportWidth - 4
		// m.viewport.Height = viewportHeight + 1
		// // cmds = append(cmds, viewport.Sync(m.viewport))

		// case *post.Post:
		// 	m.viewport.SetContent(m.renderViewport(msg))
		// 	m.WMOpen("post")
		// 	m.ctx.Loading = false
		// 	return m, nil

	case cmd.Command:
		switch msg.Call {
		case cmd.ViewFocus:
			if msg.Target == VIEW_ID ||
				msg.Target == "*" {
				m.focused = true
			}
			return m, nil
		case cmd.ViewBlur:
			if msg.Target == VIEW_ID ||
				msg.Target == "*" {
				m.focused = false
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

	// switch m.WMFocused() {
	// case "list":
	m.list, lcmd = m.list.Update(msg)
	// case "post":
	// 	m.viewport, lcmd = m.viewport.Update(msg)
	// case "reply":
	// 	if !m.textarea.Focused() {
	// 		cmds = append(cmds, m.textarea.Focus())
	// 	}
	// 	m.textarea, lcmd = m.textarea.Update(msg)
	// }
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

func (m *Model) loadItem(p *post.Post) tea.Cmd {
	return func() tea.Msg {
		m.a.LoadPost(p)
		return p
	}
}
