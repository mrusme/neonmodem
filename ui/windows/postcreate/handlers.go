package postcreate

import (
	"net/url"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/mrusme/gobbs/models/post"
	"github.com/mrusme/gobbs/models/reply"
	"github.com/mrusme/gobbs/ui/cmd"
)

func handleTab(mi interface{}) (bool, []tea.Cmd) {
	var m *Model = mi.(*Model)
	var cmds []tea.Cmd

	if m.action == "reply" {
		return false, cmds
	}

	if m.inputFocused == 0 {
		m.inputFocused = 1
		m.textinput.Blur()
		cmds = append(cmds, m.textarea.Focus())
	} else {
		m.inputFocused = 0
		m.textarea.Blur()
		cmds = append(cmds, m.textinput.Focus())
	}

	return true, cmds
}

func handleSubmit(mi interface{}) (bool, []tea.Cmd) {
	var m *Model = mi.(*Model)
	var cmds []tea.Cmd

	if m.action == "post" {
		// --- NEW POST ---
		subject := m.textinput.Value()
		body := m.textarea.Value()
		typ := "post"

		if _, err := url.Parse(body); err == nil {
			typ = "url"
		}

		x := m.iface.(*post.Post)
		p := post.Post{
			Subject: subject,
			Body:    body,
			Type:    typ,

			Forum: x.Forum,

			SysIDX: x.SysIDX,
		}

		err := m.a.CreatePost(&p)
		if err != nil {
			m.ctx.Logger.Error(err)
			cmds = append(cmds, cmd.New(
				cmd.MsgError,
				WIN_ID,
				cmd.Arg{Name: "error", Value: err},
			).Tea())
			return true, cmds
		}
	} else if m.action == "reply" {
		// --- REPLY TO EXISTING POST ---
		var r reply.Reply
		if m.replyToIdx == 0 {
			// No numbers were typed before hitting `r` so we're replying to the actual
			// Post
			x := m.iface.(post.Post)
			r = reply.Reply{
				ID:        x.ID,
				InReplyTo: "",
				Index:     -1,
				SysIDX:    x.SysIDX,
			}
		} else {
			// Numbers were typed before hitting `r`, so we're taking the actual reply
			// here
			r = m.iface.(reply.Reply)
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
	} // </IF POST || REPLY>

	m.inputFocused = 0
	m.textinput.Reset()
	m.textarea.Reset()
	m.replyToIdx = 0
	cmds = append(cmds, cmd.New(cmd.WMCloseWin, WIN_ID).Tea())
	cmds = append(cmds, cmd.New(cmd.WinRefreshData, "*", cmd.Arg{
		Name: "delay", Value: (3 * time.Second),
	}).Tea())
	return true, cmds
}

func handleWinOpenCmd(mi interface{}, c cmd.Command) (bool, []tea.Cmd) {
	var m *Model = mi.(*Model)
	var cmds []tea.Cmd

	if c.Target == WIN_ID {
		m.xywh = c.GetArg("xywh").([4]int)

		m.action = c.GetArg("action").(string)

		if m.action == "post" {
			m.iface = c.GetArg("post").(*post.Post)
			m.inputFocused = 0
			cmds = append(cmds, m.textinput.Focus())
		} else if m.action == "reply" {
			m.replyToIdx = c.GetArg("replyToIdx").(int)
			m.replyTo = c.GetArg("replyTo").(string)
			m.iface = c.GetArg(m.replyTo)
			m.inputFocused = 1
			cmds = append(cmds, m.textarea.Focus())
		}

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
