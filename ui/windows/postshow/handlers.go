package postshow

import (
	"strconv"

	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/mrusme/gobbs/models/post"
	"github.com/mrusme/gobbs/ui/cmd"
	"github.com/mrusme/gobbs/ui/windows/postcreate"
)

func handleReply(mi interface{}) (bool, []tea.Cmd) {
	var m *Model = mi.(*Model)
	var cmds []tea.Cmd
	var replyToIdx int = 0
	var err error

	m.tk.CacheView(m)

	if m.buffer != "" {
		replyToIdx, err = strconv.Atoi(m.buffer)

		if err != nil {
			// TODO: Handle error
		}

		if replyToIdx >= len(m.replyIDs) {
			// TODO: Handle error
		}
	}

	m.ctx.Logger.Debugf("replyToIdx: %d", replyToIdx)
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

	cmd := cmd.New(cmd.WinOpen, postcreate.WIN_ID, rtype, rarg, ridx)
	cmds = append(cmds, cmd.Tea())

	m.ctx.Logger.Debugln("caching view")
	m.ctx.Logger.Debugf("buffer: %s", m.buffer)
	// m.viewcache = m.buildView(false)

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

	m.ctx.Logger.Debugf("received WindowSizeMsg: %vx%v\n", m.tk.ViewWidth(), m.tk.ViewHeight())
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
		m.ctx.Logger.Debug("got own WinOpen command")
		m.activePost = c.GetArg("post").(*post.Post)
		m.viewport.SetContent(m.renderViewport(m.activePost))
		m.ctx.Logger.Debugf("loading post: %v", m.activePost.ID)
		m.ctx.Loading = true
		cmds = append(cmds, m.loadPost(m.activePost))
		return true, cmds
	}

	return false, cmds
}

func handleWinFreshDataCmd(mi interface{}, c cmd.Command) (bool, []tea.Cmd) {
	var m *Model = mi.(*Model)
	var cmds []tea.Cmd

	if c.Target == WIN_ID ||
		c.Target == "*" {
		m.ctx.Logger.Debug("got *post.Post")
		m.activePost = c.GetArg("post").(*post.Post)
		m.viewport.SetContent(m.renderViewport(m.activePost))
		m.ctx.Loading = false
		return true, cmds
	}

	return false, cmds
}