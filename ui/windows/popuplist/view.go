package popuplist

import "github.com/charmbracelet/lipgloss"

func (m Model) View() string {
	return m.tk.View(&m, true)
}

func buildView(mi interface{}, cached bool) string {
	var m *Model = mi.(*Model)

	if vcache := m.tk.DefaultCaching(cached); vcache != "" {
		return vcache
	}

	var style lipgloss.Style
	if m.tk.IsFocused() {
		style = m.ctx.Theme.PopupList.List.Focused
	} else {
		style = m.ctx.Theme.PopupList.List.Blurred
	}
	l := style.Render(m.list.View())

	return m.tk.Dialog(
		"Select",
		l,
		false,
	)
}
