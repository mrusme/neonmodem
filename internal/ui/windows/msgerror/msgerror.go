package msgerror

import (
	"strings"

	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/mrusme/neonmodem/internal/ui/ctx"
	"github.com/mrusme/neonmodem/internal/ui/msgs"
	"github.com/mrusme/neonmodem/internal/ui/toolkit"
	"github.com/mrusme/neonmodem/internal/ui/windows"
)

const (
	WIN_ID = "msgerror"

	errorPadding = 1
)

type Model struct {
	ctx *ctx.Ctx
	tk  *toolkit.ToolKit

	viewport viewport.Model
	errs     []error
}

func NewModel(c *ctx.Ctx) *Model {
	m := &Model{
		ctx:      c,
		tk:       toolkit.New(WIN_ID, c),
		viewport: viewport.New(),
	}

	m.tk.SetErrorDialog(true)
	m.tk.SetViewFunc(buildView)
	m.tk.SetMsgHandling(toolkit.MsgHandling{
		OnViewResize: handleViewResize,
	})

	return m
}

func (m *Model) Update(msg tea.Msg) (windows.Window, tea.Cmd) {
	switch msg := msg.(type) {
	case msgs.ShowError:
		m.errs = append(m.errs, msg.Errors...)
		m.setContent()
		m.tk.InvalidateCache()
		return m, nil

	case msgs.WindowClosed:
		if msg.ID == WIN_ID {
			m.errs = nil
		}
		return m, nil
	}

	if handled, cmds := m.tk.HandleMsg(m, msg); handled {
		return m, tea.Batch(cmds...)
	}

	var cmd tea.Cmd
	m.viewport, cmd = m.viewport.Update(msg)
	return m, cmd
}

func (m *Model) setContent() {
	var lines []string
	for _, err := range m.errs {
		lines = append(lines, err.Error())
	}
	m.viewport.SetContent(lipgloss.NewStyle().Width(m.viewport.Width()).Render(strings.Join(lines, "\n\n")))
}

func handleViewResize(mi interface{}) (bool, []tea.Cmd) {
	m := mi.(*Model)

	m.viewport = viewport.New(
		viewport.WithWidth(max(m.tk.InnerWidth()-errorPadding*2, 1)),
		viewport.WithHeight(m.tk.InnerHeight()),
	)
	m.setContent()

	return false, nil
}

func (m *Model) View() string {
	return m.tk.View(m, true)
}

func buildView(mi interface{}, cached bool) string {
	m := mi.(*Model)

	if vcache := m.tk.DefaultCaching(cached); vcache != "" {
		return vcache
	}

	content := lipgloss.NewStyle().Padding(0, errorPadding).Render(m.viewport.View())
	return m.tk.Dialog("Error", content)
}
