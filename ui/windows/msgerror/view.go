package msgerror

func (m Model) View() string {
	return m.tk.View(&m, true)
}

func buildView(mi interface{}, cached bool) string {
	var m *Model = mi.(*Model)
	if cached && !m.tk.IsFocused() && m.tk.IsCached() {
		m.ctx.Logger.Debugln("Cached View()")

		return m.tk.GetCachedView()
	}
	m.ctx.Logger.Debugln("View()")
	m.ctx.Logger.Debugf("IsFocused: %v\n", m.tk.IsFocused())

	return m.tk.Dialog(
		"Post",
		viewportStyle.Render(m.viewport.View()),
	)
}
