package postcreate

import (
	"fmt"

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

	title := "Reply"
	if m.replyToIdx != 0 {
		title += fmt.Sprintf(" to reply #%d", m.replyToIdx)
	}

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

	return m.tk.Dialog(
		title,
		m.textarea.View(),
	)

}
