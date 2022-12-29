package main

import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/mrusme/gobbs/system"
	"github.com/mrusme/gobbs/ui"
	"github.com/mrusme/gobbs/ui/ctx"
)

func main() {
	c := ctx.New()

	discourse, err := system.New("discourse", nil)
	if err != nil {
		panic(err)
	}

	c.AddSystem(&discourse)

	tui := tea.NewProgram(ui.NewModel(&c), tea.WithAltScreen())
	err = tui.Start()
	if err != nil {
		panic(err)
	}
}
