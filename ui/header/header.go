package header

import (
	"github.com/mrusme/gobbs/ui/ctx"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

var (
	highlight = lipgloss.AdaptiveColor{Light: "#874BFD", Dark: "#7D56F4"}
)

type Model struct {
	ctx     *ctx.Ctx
	spinner spinner.Model
}

func NewModel(c *ctx.Ctx) Model {
	m := Model{
		ctx: c,
	}

	m.spinner = spinner.New()
	m.spinner.Spinner = spinner.Dot
	m.spinner.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("205"))

	return m
}

func (m Model) Init() tea.Cmd {
	return m.spinner.Tick
}

func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	var cmds []tea.Cmd

	if m.ctx.Loading == true {
		cmds = append(cmds, m.spinner.Tick)
	}

	switch msg := msg.(type) {
	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		cmds = append(cmds, cmd)
	}

	return m, tea.Batch(cmds...)
}

func (m Model) View() string {
	var items []string

	row := lipgloss.JoinHorizontal(
		lipgloss.Top,
		items...,
	)

	if m.ctx.Loading == false {
		row = lipgloss.JoinHorizontal(lipgloss.Bottom, row, " THING HERE ")
	} else {
		row = lipgloss.JoinHorizontal(lipgloss.Bottom, row, " THING HERE ", " ", m.spinner.View())
	}

	return lipgloss.JoinHorizontal(lipgloss.Top, row, "\n\n")
}
