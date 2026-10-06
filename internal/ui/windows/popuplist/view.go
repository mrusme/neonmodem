package popuplist

func (m *Model) View() string {
	return m.tk.View(m, true)
}

func buildView(mi interface{}, cached bool) string {
	m := mi.(*Model)

	if vcache := m.tk.DefaultCaching(cached); vcache != "" {
		return vcache
	}

	t := m.ctx.Theme
	frame := t.PopupList.List.Blurred
	if m.tk.IsFocused() {
		frame = t.PopupList.List.Focused
	}

	width := m.tk.ViewWidth() - 2
	height := m.tk.ViewHeight() - 4
	body := frame.Width(width).Height(height).Render(m.list.View())

	if m.loading {
		return m.tk.DialogWithStatus(m.title, body,
			t.Muted.Render("Loading forums, the list fills in as systems answer"))
	}

	return m.tk.Dialog(m.title, body, true)
}
