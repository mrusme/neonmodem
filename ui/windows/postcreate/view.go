package postcreate

import (
	"fmt"

	"github.com/charmbracelet/lipgloss"
	"github.com/mrusme/gobbs/models/post"
	"github.com/mrusme/gobbs/ui/helpers"
)

func (m Model) View() string {
	return m.tk.View(&m, true)
}

func buildView(mi interface{}, cached bool) string {
	var m *Model = mi.(*Model)

	if cached && m.viewcache != "" {
		m.ctx.Logger.Debugln("Cached View()")

		m.textarea.SetWidth(m.viewcacheTextareaXY[2])
		m.textarea.SetHeight(m.viewcacheTextareaXY[3])

		return helpers.PlaceOverlay(
			m.viewcacheTextareaXY[0], m.viewcacheTextareaXY[1],
			m.textarea.View(), m.viewcache,
			false)
	}

	title := ""

	if m.action == "reply" {
		title = "Reply"
		if m.replyToIdx != 0 {
			title += fmt.Sprintf(" to reply #%d", m.replyToIdx)
		}
	} else if m.action == "post" {
		p := m.iface.(*post.Post)
		sysTitle := (*m.ctx.Systems[p.SysIDX]).Title()
		title = fmt.Sprintf("New Post in %s on %s", p.Forum.Name, sysTitle)
	}

	// textinputWidth := m.tk.ViewWidth() - 2
	// m.textinput.SetWidth(textinputWidth)

	textareaWidth := m.tk.ViewWidth() - 2
	textareaHeight := 6
	m.textarea.SetWidth(textareaWidth)
	m.textarea.SetHeight(textareaHeight)

	m.viewcacheTextareaXY[0] = 1
	m.viewcacheTextareaXY[1] = 2
	m.viewcacheTextareaXY[2] = textareaWidth
	m.viewcacheTextareaXY[3] = textareaHeight

	m.ctx.Logger.Debugln("View()")
	m.ctx.Logger.Debugf("IsFocused: %v\n", m.tk.IsFocused())

	var tmp string = ""
	if m.action == "post" {
		tmp = lipgloss.JoinVertical(
			lipgloss.Left,
			m.textinput.View(),
			"",
			m.textarea.View(),
		)
	} else if m.action == "reply" {
		tmp = m.textarea.View()
	}

	return m.tk.Dialog(
		title,
		tmp,
		true,
	)

}
