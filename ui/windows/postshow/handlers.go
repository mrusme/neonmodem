package postshow

import (
	"errors"
	"os"
	"os/exec"
	"strconv"
	"time"

	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/mrusme/neonmodem/models/post"
	"github.com/mrusme/neonmodem/ui/cmd"
	"github.com/mrusme/neonmodem/ui/windows/postcreate"
	"github.com/pkg/browser"
)

func handleReply(mi interface{}) (bool, []tea.Cmd) {
	var m *Model = mi.(*Model)
	var cmds []tea.Cmd
	var replyToIdx int = 0
	var err error

	m.tk.CacheView(m)

	caps := (*m.ctx.Systems[m.activePost.SysIDX]).GetCapabilities()
	if !caps.IsCapableOf("create:reply") {
		cmds = append(cmds, cmd.New(
			cmd.MsgError,
			WIN_ID,
			cmd.Arg{
				Name: "error",
				Value: errors.New(
					"This system doesn't support replies yet!\n" +
						"However, you can use `o` to open this post in your browser and " +
						"reply there!",
				),
			},
		).Tea())
		return true, cmds
	}

	if m.buffer != "" {
		replyToIdx, err = strconv.Atoi(m.buffer)
		if err != nil {
			cmds = append(cmds, cmd.New(
				cmd.MsgError,
				WIN_ID,
				cmd.Arg{
					Name:  "error",
					Value: err,
				},
			).Tea())
			return true, cmds
		}

		if replyToIdx >= len(m.replyIDs) {
			cmds = append(cmds, cmd.New(
				cmd.MsgError,
				WIN_ID,
				cmd.Arg{
					Name:  "error",
					Value: errors.New("Reply # does not exist!"),
				},
			).Tea())
			return true, cmds
		}
	}

	var ract cmd.Arg = cmd.Arg{Name: "action", Value: "reply"}
	var rtype cmd.Arg = cmd.Arg{Name: "replyTo"}
	var rarg cmd.Arg
	var ridx cmd.Arg = cmd.Arg{Name: "replyToIdx", Value: replyToIdx}

	if replyToIdx == 0 {
		rtype.Value = "post"
		rarg.Name = "post"
		rarg.Value = *m.activePost
	} else {
		rtype.Value = "reply"
		rarg.Name = "reply"
		rarg.Value = *m.allReplies[(replyToIdx - 1)]
	}

	cmd := cmd.New(cmd.WinOpen, postcreate.WIN_ID, ract, rtype, rarg, ridx)
	cmds = append(cmds, cmd.Tea())

	return true, cmds
}

func handleOpen(mi interface{}) (bool, []tea.Cmd) {
	var m *Model = mi.(*Model)
	var cmds []tea.Cmd

	openURL := m.activePost.URL

	browserPath := m.ctx.Config.Browser
	if browserPath != "" {
		if _, err := os.Stat(browserPath); err != nil {
			m.ctx.Logger.Error(err)
			cmds = append(cmds, cmd.New(
				cmd.MsgError,
				WIN_ID,
				cmd.Arg{
					Name:  "error",
					Value: err,
				},
			).Tea())
			return true, cmds
		}
		cmd := exec.Command(browserPath, openURL)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		cmd.Run()
		return true, cmds
	}

	browser.Stderr = nil
	browser.Stdout = nil
	if err := browser.OpenURL(openURL); err != nil {
		m.ctx.Logger.Error(err)
		cmds = append(cmds, cmd.New(
			cmd.MsgError,
			WIN_ID,
			cmd.Arg{
				Name:  "error",
				Value: err,
			},
		).Tea())
		return true, cmds
	}

	return true, cmds
}

func handleOlder(mi interface{}) (bool, []tea.Cmd) {
	var m *Model = mi.(*Model)
	var cmds []tea.Cmd

	m.activePost.CurrentRepliesStartIDX -= 20
	m.ctx.Loading = true
	cmds = append(cmds, m.loadPost(m.activePost))
	return true, cmds
}

func handleNumberKeys(mi interface{}, n int8) (bool, []tea.Cmd) {
	var m *Model = mi.(*Model)
	var cmds []tea.Cmd

	m.buffer += strconv.Itoa(int(n))

	return false, cmds
}

func handleUncaughtKeys(mi interface{}, k tea.KeyMsg) (bool, []tea.Cmd) {
	var m *Model = mi.(*Model)
	var cmds []tea.Cmd

	m.buffer = ""

	return false, cmds
}

func handleViewResize(mi interface{}) (bool, []tea.Cmd) {
	var m *Model = mi.(*Model)
	var cmds []tea.Cmd

	viewportWidth := m.tk.ViewWidth() - 2
	viewportHeight := m.tk.ViewHeight() - 5

	viewportStyle.Width(viewportWidth)
	viewportStyle.Height(viewportHeight)
	m.viewport = viewport.New(viewportWidth-4, viewportHeight-4)
	m.viewport.Width = viewportWidth - 4
	m.viewport.Height = viewportHeight + 1
	// cmds = append(cmds, viewport.Sync(m.viewport))

	return false, cmds
}

func handleWinOpenCmd(mi interface{}, c cmd.Command) (bool, []tea.Cmd) {
	var m *Model = mi.(*Model)
	var cmds []tea.Cmd

	if c.Target == WIN_ID {
		m.activePost = c.GetArg("post").(*post.Post)
		m.viewport.SetContent(m.renderViewport(m.activePost))
		m.ctx.Logger.Debugf("loading post: %s\n", m.activePost.ID)
		m.ctx.Loading = true
		cmds = append(cmds, m.loadPost(m.activePost))
		return true, cmds
	}

	return false, cmds
}

func handleWinRefreshDataCmd(mi interface{}, c cmd.Command) (bool, []tea.Cmd) {
	var m *Model = mi.(*Model)
	var cmds []tea.Cmd

	if c.Target == WIN_ID ||
		c.Target == "*" {
		m.ctx.Loading = true
		if delay := c.GetArg("delay"); delay != nil {
			cmds = append(cmds, m.loadPost(m.activePost, delay.(time.Duration)))
		} else {
			cmds = append(cmds, m.loadPost(m.activePost))
		}
		return true, cmds
	}
	return false, cmds
}

func handleWinFreshDataCmd(mi interface{}, c cmd.Command) (bool, []tea.Cmd) {
	var m *Model = mi.(*Model)
	var cmds []tea.Cmd

	if c.Target == WIN_ID ||
		c.Target == "*" {
		m.activePost = c.GetArg("post").(*post.Post)
		m.viewport.SetContent(m.renderViewport(m.activePost))
		m.ctx.Loading = false
		return true, cmds
	}

	return false, cmds
}
