package windows

import (
	tea "charm.land/bubbletea/v2"
)

type Window interface {
	Update(msg tea.Msg) (Window, tea.Cmd)
	View() string
}
