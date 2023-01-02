package windows

import (
	tea "github.com/charmbracelet/bubbletea"
)

type Window interface {
	View() string
	Update(msg tea.Msg) (tea.Model, tea.Cmd)
}
