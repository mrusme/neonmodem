package postcreate

import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/mrusme/gobbs/models/post"
	"github.com/mrusme/gobbs/models/reply"
	"github.com/mrusme/gobbs/ui/cmd"
)

func handleSubmit(mi interface{}) (bool, []tea.Cmd) {
	var m *Model = mi.(*Model)
	var cmds []tea.Cmd

	var r reply.Reply
	if m.replyToIdx == 0 {
		// No numbers were typed before hitting `r` so we're replying to the actual
		// Post
		x := m.replyToIface.(post.Post)
		r = reply.Reply{
			ID:        x.ID,
			InReplyTo: "",
			Index:     -1,
			SysIDX:    x.SysIDX,
		}
	} else {
		// Numbers were typed before hitting `r`, so we're taking the actual reply
		// here
		r = m.replyToIface.(reply.Reply)
	}

	r.Body = m.textarea.Value()

	err := m.a.CreateReply(&r)
	if err != nil {
		m.ctx.Logger.Error(err)
		cmds = append(cmds, cmd.New(
			cmd.MsgError,
			WIN_ID,
			cmd.Arg{Name: "error", Value: err},
		).Tea())
		return true, cmds
	}

	m.textarea.Reset()
	m.replyToIdx = 0
	cmds = append(cmds, cmd.New(cmd.WMCloseWin, WIN_ID).Tea())
	return true, cmds
}

func handleWinOpenCmd(mi interface{}, c cmd.Command) (bool, []tea.Cmd) {
	var m *Model = mi.(*Model)
	var cmds []tea.Cmd

	if c.Target == WIN_ID {
		m.xywh = c.GetArg("xywh").([4]int)
		m.replyToIdx = c.GetArg("replyToIdx").(int)
		m.replyTo = c.GetArg("replyTo").(string)
		m.replyToIface = c.GetArg(m.replyTo)
		cmds = append(cmds, m.textarea.Focus())
		return true, cmds
	}

	return false, cmds
}

func handleWinCloseCmd(mi interface{}, c cmd.Command) (bool, []tea.Cmd) {
	var m *Model = mi.(*Model)
	var cmds []tea.Cmd

	if c.Target == WIN_ID {
		m.textarea.Reset()
		return true, cmds
	}

	return false, cmds
}
