package views

import (
	tea "charm.land/bubbletea/v2"
)

type View interface {
	Update(msg tea.Msg) (View, tea.Cmd)
	View() string
}
