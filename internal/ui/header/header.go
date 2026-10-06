package header

import (
	"fmt"

	"charm.land/bubbles/v2/spinner"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/mrusme/neonmodem/internal/system/text"
	"github.com/mrusme/neonmodem/internal/ui/ctx"
)

const (
	Height        = 8
	selectorWidth = 40
)

var (
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
	loading bool
}

func NewModel(c *ctx.Ctx) Model {
	m := Model{ctx: c}
	m.spinner = spinner.New()
	m.spinner.Spinner = spinner.Dot
	m.spinner.Style = lipgloss.NewStyle().Foreground(c.Theme.Header.Spinner.GetForeground())
	return m
}

func (m Model) Init() tea.Cmd {
	return nil
}

func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	var cmds []tea.Cmd

	if m.ctx.Loading && !m.loading {
		m.loading = true
		cmds = append(cmds, m.spinner.Tick)
	} else if !m.ctx.Loading && m.loading {
		m.loading = false
	}

	if tick, ok := msg.(spinner.TickMsg); ok && m.loading {
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(tick)
		cmds = append(cmds, cmd)
	}

	return m, tea.Batch(cmds...)
}

func (m Model) View() string {
	t := m.ctx.Theme
	selectorTextLen := selectorWidth - 7

	currentSystem := "All"
	if idx := m.ctx.GetCurrentSystem(); idx >= 0 && idx < len(m.ctx.Systems) {
		currentSystem = text.Truncate(m.ctx.Systems[idx].Title(), selectorTextLen)
	}

	currentForum := "All"
	if f := m.ctx.GetCurrentForum(); f.ID != "" {
		currentForum = text.Truncate(f.Title(), selectorTextLen)
	}

	systemSelector := t.Header.Selector.Width(selectorWidth).Render(fmt.Sprintf("⏷  %s", currentSystem))
	forumSelector := t.Header.Selector.Width(selectorWidth).Render(fmt.Sprintf("⏷  %s", currentForum))

	keyStyle := lipgloss.NewStyle().Foreground(t.DialogBox.Bottombar.GetForeground())

	selectorColumn := lipgloss.JoinVertical(lipgloss.Center,
		lipgloss.JoinHorizontal(lipgloss.Bottom,
			"System: \n   "+keyStyle.Render("C-e"), systemSelector),
		lipgloss.JoinHorizontal(lipgloss.Bottom,
			"Forum: \n  "+keyStyle.Render("C-t"), forumSelector),
	)

	status := ""
	if m.ctx.Loading {
		status = m.spinner.View()
		if m.ctx.Progress != "" {
			status += " " + t.Muted.Render(m.ctx.Progress)
		}
	}

	logo := banner
	if !m.ctx.Config.RenderBanner {
		logo = ""
	}

	return lipgloss.JoinHorizontal(lipgloss.Bottom,
		logo,
		"   ",
		selectorColumn,
		" ",
		status,
	)
}
