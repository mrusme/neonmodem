package views

import (
	tea "github.com/charmbracelet/bubbletea"
)

type View interface {
	View() string
	Update(msg tea.Msg) (tea.Model, tea.Cmd)
}
