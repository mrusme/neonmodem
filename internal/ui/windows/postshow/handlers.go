package postshow

import (
	"errors"
	"os"
	"os/exec"
	"strconv"

	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"
	"github.com/mrusme/neonmodem/internal/system"
	"github.com/mrusme/neonmodem/internal/ui/msgs"
	"github.com/pkg/browser"
)

func handleReply(mi interface{}) (bool, []tea.Cmd) {
	m := mi.(*Model)
	buffer := m.buffer
	m.buffer = ""

	if m.activePost == nil || m.loading {
		return true, nil
	}

	sys := m.ctx.Systems[m.activePost.SysIDX]
	if !sys.Capabilities().Has(system.CapCreateReply) {
		return true, []tea.Cmd{msgs.Send(msgs.Error(errors.New(
			sys.Title() + " is connected without an account, so replying isn't " +
				"available here. Press `o` to open the post in your browser, or run " +
				"`neonmodem connect --type " + sys.Kind() + "` again with credentials.")))}
	}

	if m.activePost.Closed {
		return true, []tea.Cmd{msgs.Send(msgs.Error(errors.New(
			"This post is closed, replies aren't accepted anymore.")))}
	}

	replyToIdx := 0
	if buffer != "" {
		n, err := strconv.Atoi(buffer)
		if err != nil {
			return true, []tea.Cmd{msgs.Send(msgs.Error(err))}
		}
		if n >= len(m.replyIDs) {
			return true, []tea.Cmd{msgs.Send(msgs.Error(errors.New(
				"Reply #" + buffer + " does not exist.")))}
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
			return true, []tea.Cmd{msgs.Send(msgs.Error(errors.New(
				"That reply was deleted and can't be replied to.")))}
		}
		copied := *parent
		copied.Replies = nil
		compose.Parent = &copied
	}

	return true, []tea.Cmd{msgs.Send(compose)}
}

func handleOpen(mi interface{}) (bool, []tea.Cmd) {
	m := mi.(*Model)
	m.buffer = ""

	if m.activePost == nil || m.activePost.URL == "" {
		return true, nil
	}
	openURL := m.activePost.URL

	if browserPath := m.ctx.Config.Browser; browserPath != "" {
		if _, err := os.Stat(browserPath); err != nil {
			m.ctx.Logger.Error("configured browser not found", "path", browserPath, "error", err)
			return true, []tea.Cmd{msgs.Send(msgs.Error(err))}
		}
		return true, []tea.Cmd{func() tea.Msg {
			cmd := exec.Command(browserPath, openURL)
			if err := cmd.Run(); err != nil {
				return msgs.Error(err)
			}
			return msgs.Notice{Text: "Opened in the browser"}
		}}
	}

	browser.Stderr = nil
	browser.Stdout = nil
	return true, []tea.Cmd{func() tea.Msg {
		if err := browser.OpenURL(openURL); err != nil {
			return msgs.Error(err)
		}
		return msgs.Notice{Text: "Opened in the browser"}
	}}
}

func handleOlder(mi interface{}) (bool, []tea.Cmd) {
	m := mi.(*Model)
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
	m.ctx.Loading = true
	return true, []tea.Cmd{m.loadPost(m.activePost, m.ctx.NextLoadGen(), 0)}
}

func handleOpenWith(mi interface{}) (bool, []tea.Cmd) {
	m := mi.(*Model)
	m.buffer = ""

	if m.activePost == nil {
		return true, nil
	}
	return true, []tea.Cmd{msgs.Send(msgs.OpenWithMenu{Post: *m.activePost})}
}

func handleNumberKeys(mi interface{}, n int8) (bool, []tea.Cmd) {
	m := mi.(*Model)
	m.buffer += strconv.Itoa(int(n))
	return false, nil
}

func handleUncaughtKeys(mi interface{}, k tea.KeyPressMsg) (bool, []tea.Cmd) {
	m := mi.(*Model)
	m.buffer = ""
	return false, nil
}

func handleViewResize(mi interface{}) (bool, []tea.Cmd) {
	m := mi.(*Model)

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
