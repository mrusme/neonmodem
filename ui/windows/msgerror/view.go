package msgerror

func (m Model) View() string {
	return m.tk.View(&m, true)
}

func buildView(mi interface{}, cached bool) string {
	var m *Model = mi.(*Model)

	if vcache := m.tk.DefaultCaching(cached); vcache != "" {
		return vcache
	}

	return m.tk.ErrorDialog(
		"Error",
		viewportStyle.Render(m.viewport.View()),
	)
}
