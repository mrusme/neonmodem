package popuplist

import (
	"charm.land/bubbles/v2/list"
	tea "charm.land/bubbletea/v2"
	"github.com/mrusme/neonmodem/internal/ui/msgs"
)

func (m *Model) handleSelect() (bool, []tea.Cmd) {

	if m.list.FilterState() == list.Filtering {
		return false, nil
	}

	item := m.list.SelectedItem()
	if item == nil {
		return true, nil
	}

	return true, []tea.Cmd{msgs.Send(msgs.Picked{Kind: m.kind, Item: item})}
}

func (m *Model) handleViewResize() (bool, []tea.Cmd) {

	frame := m.ctx.Theme.PopupList.List.Focused
	m.list.SetSize(
		max(m.tk.InnerWidth()-frame.GetHorizontalFrameSize(), 1),
		max(m.tk.InnerHeight()-frame.GetVerticalFrameSize(), 1),
	)

	return false, nil
}
