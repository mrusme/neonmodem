package msgerror

import (
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/mrusme/gobbs/ui/cmd"
)

func handleViewResize(mi interface{}) (bool, []tea.Cmd) {
	var m *Model = mi.(*Model)
	var cmds []tea.Cmd

	m.ctx.Logger.Debugf("received WindowSizeMsg: %vx%v\n", m.tk.ViewWidth(), m.tk.ViewHeight())
	viewportWidth := m.tk.ViewWidth() - 2
	viewportHeight := m.tk.ViewHeight() - 5

	viewportStyle.Width(viewportWidth)
	viewportStyle.Height(viewportHeight)
	m.viewport = viewport.New(viewportWidth-4, viewportHeight-4)
	m.viewport.Width = viewportWidth - 4
	m.viewport.Height = viewportHeight + 1

	return false, cmds
}

func handleMsgErrorCmd(mi interface{}, c cmd.Command) (bool, []tea.Cmd) {
	var m *Model = mi.(*Model)
	var cmds []tea.Cmd

	if err := c.GetArg("error"); err != nil {
		m.errs = append(m.errs, err.(error))
	}
	if errs := c.GetArg("errors"); errs != nil {
		m.errs = append(m.errs, errs.([]error)...)
	}

	m.setErrorContent()
	m.tk.CacheView(m)
	return true, cmds
}

func handleWinCloseCmd(mi interface{}, c cmd.Command) (bool, []tea.Cmd) {
	var m *Model = mi.(*Model)
	var cmds []tea.Cmd

	if c.Target == WIN_ID {
		m.errs = []error{}
		return true, cmds
	}

	return false, cmds
}
