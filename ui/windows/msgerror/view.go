package msgerror

func (m Model) View() string {
	return m.tk.View(&m, true)
}

func buildView(mi interface{}, cached bool) string {
	var m *Model = mi.(*Model)

	if vcache := m.tk.DefaultCaching(cached); vcache != "" {
		m.ctx.Logger.Debugln("Cached View()")
		return vcache
	}

	m.ctx.Logger.Debugln("View()")
	m.ctx.Logger.Debugf("IsFocused: %v\n", m.tk.IsFocused())

	return m.tk.Dialog(
		"Post",
		viewportStyle.Render(m.viewport.View()),
	)
}
