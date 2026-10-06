package popuplist

import (
	"charm.land/bubbles/v2/list"
	tea "charm.land/bubbletea/v2"
	"github.com/mrusme/neonmodem/internal/ui/msgs"
)

func handleSelect(mi interface{}) (bool, []tea.Cmd) {
	m := mi.(*Model)

	if m.list.FilterState() == list.Filtering {
		return false, nil
	}

	item := m.list.SelectedItem()
	if item == nil {
		return true, nil
	}

	return true, []tea.Cmd{msgs.Send(msgs.Picked{Kind: m.kind, Item: item})}
}

func handleViewResize(mi interface{}) (bool, []tea.Cmd) {
	m := mi.(*Model)

	width := m.tk.ViewWidth() - 4
	height := m.tk.ViewHeight() - 4
	if width < 10 {
		width = 10
	}
	if height < 3 {
		height = 3
	}
	m.list.SetSize(width, height)

	return false, nil
}
