package popuplist

func (m *Model) View() string {
	return m.tk.View(true)
}

func (m *Model) buildView(cached bool) string {

	if vcache := m.tk.DefaultCaching(cached); vcache != "" {
		return vcache
	}

	t := m.ctx.Theme
	frame := t.PopupList.List.Blurred
	if m.tk.IsFocused() {
		frame = t.PopupList.List.Focused
	}

	body := frame.Width(m.tk.InnerWidth()).Height(m.tk.InnerHeight()).Render(m.list.View())

	if m.loading {
		return m.tk.DialogWithStatus(m.title, body,
			t.Muted.Render("Loading forums, the list fills in as systems answer"))
	}

	return m.tk.Dialog(m.title, body)
}
