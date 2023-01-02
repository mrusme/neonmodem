package posts

var WM_ROOT_ID = "list"

func (m *Model) WMOpen(id string) bool {
	if m.WMisOpen(id) {
		if m.WMisFocused(id) {
			return true
		}
		return false
	}

	m.wm = append(m.wm, id)
	return true
}

func (m *Model) WMCloseFocused() bool {
	return m.WMClose(m.WMFocused())
}

func (m *Model) WMClose(id string) bool {
	for i := len(m.wm) - 1; i > 0; i-- {
		if m.wm[i] == id {
			m.wm = append(m.wm[:i], m.wm[i+1:]...)
			return true
		}
	}

	return false
}

func (m *Model) WMFocused() string {
	return m.wm[len(m.wm)-1]
}

func (m *Model) WMisOpen(id string) bool {
	for _, openID := range m.wm {
		if openID == id {
			return true
		}
	}
	return false
}

func (m *Model) WMisFocused(id string) bool {
	return id == m.WMFocused()
}
