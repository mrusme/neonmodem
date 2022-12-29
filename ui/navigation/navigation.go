package navigation

import (
	"strings"

	"github.com/mrusme/gobbs/ui/ctx"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

var (
	highlight = lipgloss.AdaptiveColor{Light: "#874BFD", Dark: "#7D56F4"}

	activeTabBorder = lipgloss.Border{
		Top:         "─",
		Bottom:      " ",
		Left:        "│",
		Right:       "│",
		TopLeft:     "╭",
		TopRight:    "╮",
		BottomLeft:  "┘",
		BottomRight: "└",
	}

	tabBorder = lipgloss.Border{
		Top:         "─",
		Bottom:      "─",
		Left:        "│",
		Right:       "│",
		TopLeft:     "╭",
		TopRight:    "╮",
		BottomLeft:  "┴",
		BottomRight: "┴",
	}

	tab = lipgloss.NewStyle().
		Border(tabBorder, true).
		BorderForeground(highlight).
		Padding(0, 1)

	activeTab = tab.Copy().Border(activeTabBorder, true)

	tabGap = tab.Copy().
		BorderTop(false).
		BorderLeft(false).
		BorderRight(false)
)

var Navigation = []string{}

type Model struct {
	CurrentId int
	ctx       *ctx.Ctx
	spinner   spinner.Model
}

func NewModel(c *ctx.Ctx) Model {
	m := Model{
		CurrentId: 0,
		ctx:       c,
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

	for i, nav := range Navigation {
		if m.CurrentId == i {
			items = append(items, activeTab.Render(nav))
		} else {
			items = append(items, tab.Render(nav))
		}
	}

	row := lipgloss.JoinHorizontal(
		lipgloss.Top,
		items...,
	)

	if m.ctx.Loading == false {
		gap := tabGap.Render(strings.Repeat(" ", max(0, m.ctx.Screen[0]-lipgloss.Width(row)-2)))
		row = lipgloss.JoinHorizontal(lipgloss.Bottom, row, gap)
	} else {
		gap := tabGap.Render(strings.Repeat(" ", max(0, m.ctx.Screen[0]-lipgloss.Width(row)-4)))
		row = lipgloss.JoinHorizontal(lipgloss.Bottom, row, gap, " ", m.spinner.View())
	}

	return lipgloss.JoinHorizontal(lipgloss.Top, row, "\n\n")
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func (m *Model) NthTab(nth int) {
	if nth > len(Navigation) {
		nth = len(Navigation)
	} else if nth < 1 {
		nth = 1
	}

	m.CurrentId = nth - 1
}

func (m *Model) PrevTab() {
	m.CurrentId--

	if m.CurrentId < 0 {
		m.CurrentId = len(Navigation) - 1
	}
}

func (m *Model) NextTab() {
	m.CurrentId++

	if m.CurrentId >= len(Navigation) {
		m.CurrentId = 0
	}
}
