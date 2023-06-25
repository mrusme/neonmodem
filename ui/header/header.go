package header

import (
	"fmt"

	"github.com/mrusme/neonmodem/ui/ctx"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

var (
	highlight = lipgloss.AdaptiveColor{Light: "#874BFD", Dark: "#7D56F4"}

	overdrive = lipgloss.NewStyle().Foreground(lipgloss.Color("#f119a0"))

	banner = lipgloss.NewStyle().Foreground(lipgloss.Color("#3c4f92")).Render(" ________ _____ _____ ________") + "\n" +
		lipgloss.NewStyle().Foreground(lipgloss.Color("#bff1fe")).Render("|     |  |   __|     |     |  | ") + overdrive.Render("O") + "\n" +
		lipgloss.NewStyle().Foreground(lipgloss.Color("#1b0d35")).Render("|   | |  |   __|  |  |   | |  | ") + overdrive.Render("V") + "\n" +
		lipgloss.NewStyle().Foreground(lipgloss.Color("#7c8eb5")).Render("|___|____|_____|_____|___|____| ") + overdrive.Render("R") + "\n" +
		lipgloss.NewStyle().Foreground(lipgloss.Color("#2d3588")).Render("|     |     |    \\|   __|     | ") + overdrive.Render("D") + "\n" +
		lipgloss.NewStyle().Foreground(lipgloss.Color("#b4effe")).Render("| | | |  |  |  |  |   __| | | | ") + overdrive.Render("R") + "\n" +
		lipgloss.NewStyle().Foreground(lipgloss.Color("#28254c")).Render("|_|_|_|_____|____/|_____|_|_|_| ") + overdrive.Render("V")
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
	m.spinner.Style = lipgloss.NewStyle().Foreground(
		m.ctx.Theme.Header.Spinner.GetForeground())

	return m
}

func (m Model) Init() tea.Cmd {
	return m.spinner.Tick
}

func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	var cmds []tea.Cmd

	if m.ctx.Loading == true {
		cmds = append(cmds, m.spinner.Tick)
	} else {
		return m, nil
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
	var row string
	var spinner string = ""

	selectorWidth := 40
	selectorTextLen := selectorWidth - 7

	curSysIdx := m.ctx.GetCurrentSystem()
	var currentSystem string = "All"
	if curSysIdx >= 0 {
		currentSystem = (*m.ctx.Systems[curSysIdx]).Title()
		if len(currentSystem) > selectorTextLen {
			currentSystem = currentSystem[0:selectorTextLen]
		}
	}

	curForum := m.ctx.GetCurrentForum()
	var currentForum string = "All"
	if curForum.ID != "" {
		currentForum = curForum.Title()
		if len(currentForum) > selectorTextLen {
			currentForum = currentForum[0:selectorTextLen]
		}
	}

	systemSelector := m.ctx.Theme.Header.Selector.
		Width(selectorWidth).Render(fmt.Sprintf("⏷  %s", currentSystem))
	forumSelector := m.ctx.Theme.Header.Selector.
		Width(selectorWidth).Render(fmt.Sprintf("⏷  %s", currentForum))

	selectorColumn := lipgloss.JoinVertical(lipgloss.Center,
		lipgloss.JoinHorizontal(lipgloss.Bottom, "System: \n   "+
			lipgloss.NewStyle().Foreground(
				m.ctx.Theme.DialogBox.Bottombar.GetForeground(),
			).Render("C-e"),
			systemSelector),
		lipgloss.JoinHorizontal(lipgloss.Bottom, "Forum: \n  "+
			lipgloss.NewStyle().Foreground(
				m.ctx.Theme.DialogBox.Bottombar.GetForeground(),
			).Render("C-t"),
			forumSelector),
	)

	if m.ctx.Loading == true {
		spinner = m.spinner.View()
	}

	if !m.ctx.Config.RenderBanner{
		banner = ""
	}

	row = lipgloss.JoinHorizontal(lipgloss.Bottom,
		banner,
		"   ",
		selectorColumn,
		" ",
		spinner,
	)

	return row
}
