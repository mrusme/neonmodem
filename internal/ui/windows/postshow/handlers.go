package postshow

import (
	"strconv"

	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"
	"github.com/mrusme/neonmodem/internal/browser"
	"github.com/mrusme/neonmodem/internal/system"
	"github.com/mrusme/neonmodem/internal/ui/ctx"
	"github.com/mrusme/neonmodem/internal/ui/msgs"
)

func (m *Model) handleReply() (bool, []tea.Cmd) {
	buffer := m.buffer
	m.buffer = ""

	if m.activePost == nil || m.loading {
		return true, nil
	}

	sys := m.ctx.Systems[m.activePost.SysIDX]
	if !sys.Capabilities().Has(system.CapCreateReply) {
		return true, []tea.Cmd{msgs.Send(msgs.Message(
			sys.Title() + " is connected without an account, so replying isn't " +
				"available here. Press `o` to open the post in your browser, or run `" +
				system.ConnectCommand(sys.Kind(), sys.URL()) + "` again with credentials."))}
	}

	if m.activePost.Closed {
		return true, []tea.Cmd{msgs.Send(msgs.Message(
			"This post is closed, replies aren't accepted anymore."))}
	}

	replyToIdx := 0
	if buffer != "" {
		n, err := strconv.Atoi(buffer)
		if err != nil {
			return true, []tea.Cmd{msgs.Send(msgs.Error(err))}
		}
		if n >= len(m.replyIDs) {
			return true, []tea.Cmd{msgs.Send(msgs.Message(
				"Reply #" + buffer + " does not exist."))}
		}
		replyToIdx = n
	}

	compose := msgs.Compose{
		Action: msgs.ComposeReply,
		Post:   *m.activePost,
		Index:  replyToIdx,
	}
	if replyToIdx > 0 {
		parent := m.allReplies[replyToIdx-1]
		if parent.Deleted {
			return true, []tea.Cmd{msgs.Send(msgs.Message(
				"That reply was deleted and can't be replied to."))}
		}
		copied := *parent
		copied.Replies = nil
		compose.Parent = &copied
	}

	return true, []tea.Cmd{msgs.Send(compose)}
}

func (m *Model) handleOpen() (bool, []tea.Cmd) {
	m.buffer = ""

	if m.activePost == nil || m.activePost.URL == "" {
		return true, nil
	}
	openURL := m.activePost.URL
	program := m.ctx.Config.Browser
	logger := m.ctx.Logger

	return true, []tea.Cmd{func() tea.Msg {
		if err := browser.Open(openURL, program, logger); err != nil {
			logger.Error("opening the browser failed", "url", openURL, "error", err)
			return msgs.Error(err)
		}
		return msgs.Notice{Text: "Opened in the browser"}
	}}
}

func (m *Model) handleOlder() (bool, []tea.Cmd) {
	m.buffer = ""

	if m.activePost == nil || m.loading {
		return true, nil
	}

	page := &m.activePost.ReplyPage
	if !page.HasOlder() {
		return true, []tea.Cmd{msgs.Send(msgs.Notice{Text: "No older replies"})}
	}
	page.Offset -= page.Size
	if page.Offset < 0 {
		page.Offset = 0
	}

	m.loading = true
	m.ctx.StartLoading(ctx.LoadPost)
	return true, []tea.Cmd{m.loadPost(m.activePost, 0)}
}

func (m *Model) handleOpenWith() (bool, []tea.Cmd) {
	m.buffer = ""

	if m.activePost == nil {
		return true, nil
	}
	return true, []tea.Cmd{msgs.Send(msgs.OpenWithMenu{Post: *m.activePost})}
}

func (m *Model) handleNumberKeys(n int8) (bool, []tea.Cmd) {
	m.buffer += strconv.Itoa(int(n))
	return false, nil
}

func (m *Model) handleUncaughtKeys(tea.KeyPressMsg) (bool, []tea.Cmd) {
	m.buffer = ""
	return false, nil
}

func (m *Model) handleViewResize() (bool, []tea.Cmd) {

	offset := m.viewport.YOffset()
	content := m.viewport.GetContent()
	m.viewport = viewport.New(
		viewport.WithWidth(m.tk.InnerWidth()),
		viewport.WithHeight(m.tk.InnerHeight()),
	)
	m.viewport.SetContent(content)
	m.viewport.SetYOffset(offset)

	return false, nil
}
