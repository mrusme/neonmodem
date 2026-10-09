package msgerror

import (
	"strings"

	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/mrusme/neonmodem/internal/system/text"
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
	messages []string
	title    string
}

func NewModel(c *ctx.Ctx) *Model {
	m := &Model{
		ctx:      c,
		tk:       toolkit.New(WIN_ID, c),
		viewport: viewport.New(),
		title:    "Error",
	}

	m.tk.SetErrorDialog(true)
	m.tk.SetViewFunc(m.buildView)
	m.tk.SetMsgHandling(toolkit.MsgHandling{
		OnViewResize: m.handleViewResize,
	})

	return m
}

func (m *Model) Update(msg tea.Msg) (windows.Window, tea.Cmd) {
	switch msg := msg.(type) {
	case msgs.ShowError:
		m.title = "Error"
		m.tk.SetErrorDialog(true)
		m.messages = append(m.messages, msg.Messages...)
		m.setContent()
		m.tk.InvalidateCache()
		return m, nil

	case msgs.ShowNotices:
		m.title = "Notices"
		m.tk.SetErrorDialog(false)
		m.messages = noticeLines(msg.Entries)
		m.setContent()
		m.tk.InvalidateCache()
		return m, nil

	case msgs.WindowClosed:
		if msg.ID == WIN_ID {
			m.messages = nil
		}
		return m, nil
	}

	if handled, cmds := m.tk.HandleMsg(msg); handled {
		return m, tea.Batch(cmds...)
	}

	var cmd tea.Cmd
	m.viewport, cmd = m.viewport.Update(msg)
	return m, cmd
}

func (m *Model) setContent() {
	shown := make([]string, len(m.messages))
	for i, message := range m.messages {
		shown[i] = text.PrintableLines(message)
	}
	m.viewport.SetContent(lipgloss.NewStyle().Width(m.viewport.Width()).Render(strings.Join(shown, "\n\n")))
}

func (m *Model) handleViewResize() (bool, []tea.Cmd) {

	m.viewport = viewport.New(
		viewport.WithWidth(max(m.tk.InnerWidth()-errorPadding*2, 1)),
		viewport.WithHeight(m.tk.InnerHeight()),
	)
	m.setContent()

	return false, nil
}

func (m *Model) View() string {
	return m.tk.View(true)
}

func (m *Model) buildView(cached bool) string {

	if vcache := m.tk.DefaultCaching(cached); vcache != "" {
		return vcache
	}

	content := lipgloss.NewStyle().Padding(0, errorPadding).Render(m.viewport.View())
	return m.tk.Dialog(m.title, content)
}

func noticeLines(entries []msgs.NoticeEntry) []string {
	if len(entries) == 0 {
		return []string{"No notices so far."}
	}

	lines := make([]string, 0, len(entries))
	for _, e := range entries {
		lines = append(lines, e.At.Format("15:04:05")+"  "+e.Text)
	}
	return lines
}
